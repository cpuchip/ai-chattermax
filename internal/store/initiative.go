package store

import (
	"context"
	"errors"
	"fmt"
	"sort"
)

// InitiativeEntry is one combatant in a round.
type InitiativeEntry struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Modifier int    `json:"modifier"`
	Roll     int    `json:"roll"`
	Total    int    `json:"total"`
}

// InitiativeRound is a room's turn order. Entries come back sorted: total
// DESC, then roll DESC, then name — the order the panel displays.
type InitiativeRound struct {
	ID             string            `json:"id"`
	RoomID         string            `json:"roomId"`
	Round          int               `json:"round"`
	CurrentEntryID string            `json:"currentEntryId,omitempty"`
	StarterID      string            `json:"starterId,omitempty"`
	Active         bool              `json:"active"`
	Entries        []InitiativeEntry `json:"entries"`
}

// ErrInitiativeActive signals a /initiative start while a round is running.
var ErrInitiativeActive = errors.New("an initiative round is already running")

// StartInitiative opens a round for a room (one active per room).
func (s *Store) StartInitiative(ctx context.Context, roomID string, userID, personaID *string) (InitiativeRound, error) {
	var r InitiativeRound
	err := s.pool.QueryRow(ctx, `
		INSERT INTO initiative_rounds (room_id, started_by_user_id, started_by_persona_id)
		VALUES ($1, $2, $3)
		RETURNING id, room_id, round, COALESCE(current_entry_id::text,''),
		          COALESCE(started_by_user_id::text, started_by_persona_id::text, ''), active`,
		roomID, userID, personaID,
	).Scan(&r.ID, &r.RoomID, &r.Round, &r.CurrentEntryID, &r.StarterID, &r.Active)
	if err != nil {
		// the partial unique index trips here when a round is already active
		return InitiativeRound{}, ErrInitiativeActive
	}
	return r, nil
}

// ActiveInitiative loads a room's active round with sorted entries.
func (s *Store) ActiveInitiative(ctx context.Context, roomID string) (InitiativeRound, bool, error) {
	var r InitiativeRound
	err := s.pool.QueryRow(ctx, `
		SELECT id, room_id, round, COALESCE(current_entry_id::text,''),
		       COALESCE(started_by_user_id::text, started_by_persona_id::text, ''), active
		FROM initiative_rounds WHERE room_id = $1 AND active`, roomID,
	).Scan(&r.ID, &r.RoomID, &r.Round, &r.CurrentEntryID, &r.StarterID, &r.Active)
	if err != nil {
		return InitiativeRound{}, false, nil //nolint:nilerr // no active round
	}
	if err := s.loadEntries(ctx, &r); err != nil {
		return InitiativeRound{}, false, err
	}
	return r, true, nil
}

func (s *Store) loadEntries(ctx context.Context, r *InitiativeRound) error {
	rows, err := s.pool.Query(ctx, `
		SELECT id, name, modifier, roll, total
		FROM initiative_entries WHERE round_id = $1`, r.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	r.Entries = nil
	for rows.Next() {
		var e InitiativeEntry
		if err := rows.Scan(&e.ID, &e.Name, &e.Modifier, &e.Roll, &e.Total); err != nil {
			return err
		}
		r.Entries = append(r.Entries, e)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	sort.Slice(r.Entries, func(i, j int) bool {
		a, b := r.Entries[i], r.Entries[j]
		if a.Total != b.Total {
			return a.Total > b.Total
		}
		if a.Roll != b.Roll {
			return a.Roll > b.Roll
		}
		return a.Name < b.Name
	})
	return nil
}

// UpsertInitiativeEntry adds a combatant (or re-rolls them — same name
// replaces). The caller rolled; we store roll + total.
func (s *Store) UpsertInitiativeEntry(ctx context.Context, roundID, name string, modifier, roll, total int) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO initiative_entries (round_id, name, modifier, roll, total)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (round_id, name) DO UPDATE SET
			modifier = EXCLUDED.modifier, roll = EXCLUDED.roll, total = EXCLUDED.total`,
		roundID, name, modifier, roll, total)
	if err != nil {
		return fmt.Errorf("initiative entry: %w", err)
	}
	return nil
}

// RemoveInitiativeEntry drops a combatant; clears current if it was them.
func (s *Store) RemoveInitiativeEntry(ctx context.Context, roundID, name string) (bool, error) {
	tag, err := s.pool.Exec(ctx, `
		DELETE FROM initiative_entries WHERE round_id = $1 AND lower(name) = lower($2)`, roundID, name)
	if err != nil {
		return false, err
	}
	_, _ = s.pool.Exec(ctx, `
		UPDATE initiative_rounds r SET current_entry_id = NULL
		WHERE r.id = $1 AND current_entry_id IS NOT NULL
		  AND NOT EXISTS (SELECT 1 FROM initiative_entries e WHERE e.id = r.current_entry_id)`, roundID)
	return tag.RowsAffected() > 0, nil
}

// AdvanceInitiative moves the turn marker to the next combatant in sorted
// order (wrapping bumps the round counter). Returns the updated round.
func (s *Store) AdvanceInitiative(ctx context.Context, roomID string) (InitiativeRound, error) {
	r, ok, err := s.ActiveInitiative(ctx, roomID)
	if err != nil || !ok {
		return InitiativeRound{}, fmt.Errorf("no active initiative round")
	}
	if len(r.Entries) == 0 {
		return InitiativeRound{}, fmt.Errorf("nobody has rolled initiative yet")
	}
	next, wrapped := 0, false
	if r.CurrentEntryID != "" {
		for i, e := range r.Entries {
			if e.ID == r.CurrentEntryID {
				next = i + 1
				break
			}
		}
		if next >= len(r.Entries) {
			next, wrapped = 0, true
		}
	}
	if wrapped {
		r.Round++
	}
	r.CurrentEntryID = r.Entries[next].ID
	_, err = s.pool.Exec(ctx, `
		UPDATE initiative_rounds SET current_entry_id = $2, round = $3 WHERE id = $1`,
		r.ID, r.CurrentEntryID, r.Round)
	if err != nil {
		return InitiativeRound{}, err
	}
	return r, nil
}

// EndInitiative closes a room's active round.
func (s *Store) EndInitiative(ctx context.Context, roomID string) (InitiativeRound, error) {
	var r InitiativeRound
	err := s.pool.QueryRow(ctx, `
		UPDATE initiative_rounds SET active = false, ended_at = now()
		WHERE room_id = $1 AND active
		RETURNING id, room_id, round, COALESCE(current_entry_id::text,''),
		          COALESCE(started_by_user_id::text, started_by_persona_id::text, ''), active`,
		roomID,
	).Scan(&r.ID, &r.RoomID, &r.Round, &r.CurrentEntryID, &r.StarterID, &r.Active)
	if err != nil {
		return InitiativeRound{}, fmt.Errorf("no active initiative round")
	}
	return r, nil
}
