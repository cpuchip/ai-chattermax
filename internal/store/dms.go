package store

import (
	"context"
	"fmt"
)

// DMSummary is a direct-message thread as seen by one participant: the thread id
// plus who the OTHER party is (a user or a persona), for the DM list.
type DMSummary struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"` // user_user | user_persona
	OtherID   string `json:"otherId"`
	OtherName string `json:"otherName"`
	OtherKind string `json:"otherKind"` // human | persona
}

// OpenDMWithPersona finds (or creates) the 1:1 DM between a user and a persona on
// the persona's server. The persona must have dm_enabled. Idempotent — repeated
// calls return the same thread.
func (s *Store) OpenDMWithPersona(ctx context.Context, userID, personaID string) (DMSummary, error) {
	var serverID string
	var dmEnabled bool
	var displayName string
	if err := s.pool.QueryRow(ctx,
		`SELECT server_id, dm_enabled, display_name FROM personas WHERE id = $1`, personaID,
	).Scan(&serverID, &dmEnabled, &displayName); err != nil {
		return DMSummary{}, fmt.Errorf("load persona: %w", err)
	}
	if !dmEnabled {
		return DMSummary{}, fmt.Errorf("this persona does not accept direct messages")
	}

	var dmID string
	err := s.pool.QueryRow(ctx, `
		SELECT d.id FROM dms d
		WHERE d.kind = 'user_persona'
		  AND EXISTS (SELECT 1 FROM dm_participants WHERE dm_id = d.id AND user_id = $1)
		  AND EXISTS (SELECT 1 FROM dm_participants WHERE dm_id = d.id AND persona_id = $2)
		LIMIT 1`, userID, personaID).Scan(&dmID)
	if err != nil {
		// Not found — create it.
		tx, terr := s.pool.Begin(ctx)
		if terr != nil {
			return DMSummary{}, terr
		}
		defer tx.Rollback(ctx) //nolint:errcheck
		if err := tx.QueryRow(ctx,
			`INSERT INTO dms (server_id, kind) VALUES ($1, 'user_persona') RETURNING id`, serverID,
		).Scan(&dmID); err != nil {
			return DMSummary{}, fmt.Errorf("create dm: %w", err)
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO dm_participants (dm_id, user_id) VALUES ($1, $2)`, dmID, userID); err != nil {
			return DMSummary{}, fmt.Errorf("add user participant: %w", err)
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO dm_participants (dm_id, persona_id) VALUES ($1, $2)`, dmID, personaID); err != nil {
			return DMSummary{}, fmt.Errorf("add persona participant: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return DMSummary{}, err
		}
	}
	return DMSummary{ID: dmID, Kind: "user_persona", OtherID: personaID, OtherName: displayName, OtherKind: "persona"}, nil
}

// OpenDMWithUser finds (or creates) the 1:1 DM between two users who share a
// server. Idempotent.
func (s *Store) OpenDMWithUser(ctx context.Context, serverID, userID, otherUserID string) (DMSummary, error) {
	if userID == otherUserID {
		return DMSummary{}, fmt.Errorf("cannot DM yourself")
	}
	var otherName string
	if err := s.pool.QueryRow(ctx, `SELECT display_name FROM users WHERE id = $1`, otherUserID).Scan(&otherName); err != nil {
		return DMSummary{}, fmt.Errorf("load other user: %w", err)
	}
	var dmID string
	err := s.pool.QueryRow(ctx, `
		SELECT d.id FROM dms d
		WHERE d.kind = 'user_user'
		  AND EXISTS (SELECT 1 FROM dm_participants WHERE dm_id = d.id AND user_id = $1)
		  AND EXISTS (SELECT 1 FROM dm_participants WHERE dm_id = d.id AND user_id = $2)
		LIMIT 1`, userID, otherUserID).Scan(&dmID)
	if err != nil {
		tx, terr := s.pool.Begin(ctx)
		if terr != nil {
			return DMSummary{}, terr
		}
		defer tx.Rollback(ctx) //nolint:errcheck
		if err := tx.QueryRow(ctx,
			`INSERT INTO dms (server_id, kind) VALUES ($1, 'user_user') RETURNING id`, serverID,
		).Scan(&dmID); err != nil {
			return DMSummary{}, fmt.Errorf("create dm: %w", err)
		}
		for _, uid := range []string{userID, otherUserID} {
			if _, err := tx.Exec(ctx,
				`INSERT INTO dm_participants (dm_id, user_id) VALUES ($1, $2)`, dmID, uid); err != nil {
				return DMSummary{}, fmt.Errorf("add participant: %w", err)
			}
		}
		if err := tx.Commit(ctx); err != nil {
			return DMSummary{}, err
		}
	}
	return DMSummary{ID: dmID, Kind: "user_user", OtherID: otherUserID, OtherName: otherName, OtherKind: "human"}, nil
}

// ListDMsForUser returns the user's DM threads, each summarized by the OTHER
// participant. Newest thread first.
func (s *Store) ListDMsForUser(ctx context.Context, userID string) ([]DMSummary, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT d.id, d.kind,
		       COALESCE(op.id::text, ou.id::text)                        AS other_id,
		       COALESCE(op.display_name, ou.display_name, '(unknown)')   AS other_name,
		       CASE WHEN op.id IS NOT NULL THEN 'persona' ELSE 'human' END AS other_kind
		FROM dms d
		JOIN dm_participants me    ON me.dm_id = d.id AND me.user_id = $1
		JOIN dm_participants other ON other.dm_id = d.id
		                          AND (other.user_id IS DISTINCT FROM $1)
		LEFT JOIN users    ou ON ou.id = other.user_id
		LEFT JOIN personas op ON op.id = other.persona_id
		ORDER BY d.created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DMSummary
	for rows.Next() {
		var d DMSummary
		if err := rows.Scan(&d.ID, &d.Kind, &d.OtherID, &d.OtherName, &d.OtherKind); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// UserCanAccessDM reports whether a user participates in a DM.
func (s *Store) UserCanAccessDM(ctx context.Context, dmID, userID string) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM dm_participants WHERE dm_id = $1 AND user_id = $2)`,
		dmID, userID).Scan(&ok)
	return ok, err
}

// PersonaCanAccessDM reports whether a persona participates in a DM.
func (s *Store) PersonaCanAccessDM(ctx context.Context, dmID, personaID string) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM dm_participants WHERE dm_id = $1 AND persona_id = $2)`,
		dmID, personaID).Scan(&ok)
	return ok, err
}

// PersonaDMs returns the DM thread ids a persona participates in, with the human
// counterpart's name — so a persona-host can subscribe to its DMs.
func (s *Store) PersonaDMs(ctx context.Context, personaID string) ([]DMSummary, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT d.id, d.kind,
		       ou.id::text, COALESCE(ou.display_name, '(unknown)'), 'human'
		FROM dms d
		JOIN dm_participants me    ON me.dm_id = d.id AND me.persona_id = $1
		JOIN dm_participants other ON other.dm_id = d.id AND other.user_id IS NOT NULL
		JOIN users ou ON ou.id = other.user_id
		ORDER BY d.created_at`, personaID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DMSummary
	for rows.Next() {
		var d DMSummary
		if err := rows.Scan(&d.ID, &d.Kind, &d.OtherID, &d.OtherName, &d.OtherKind); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// DeleteDM removes a DM thread and all its messages (ON DELETE CASCADE drops
// participants + messages). Caller must verify the requester is a participant.
func (s *Store) DeleteDM(ctx context.Context, dmID string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM dms WHERE id = $1`, dmID)
	return err
}

// InsertDMUserMessage stores a human's DM message and returns it resolved.
func (s *Store) InsertDMUserMessage(ctx context.Context, dmID, userID, body string) (Message, error) {
	return s.insertDMMessage(ctx, dmID, &userID, nil, body)
}

// InsertDMPersonaMessage stores a persona's DM message and returns it resolved.
func (s *Store) InsertDMPersonaMessage(ctx context.Context, dmID, personaID, body string) (Message, error) {
	return s.insertDMMessage(ctx, dmID, nil, &personaID, body)
}

func (s *Store) insertDMMessage(ctx context.Context, dmID string, userID, personaID *string, body string) (Message, error) {
	q := `
		WITH ins AS (
			INSERT INTO messages (dm_id, sender_user_id, sender_persona_id, body)
			VALUES ($1, $2, $3, $4)
			RETURNING id, room_id, dm_id, sender_user_id, sender_persona_id, sub_persona_id, body, created_at
		)
		SELECT ` + resolvedCols + `
		FROM ins m ` + resolvedJoins
	m, err := scanMessage(s.pool.QueryRow(ctx, q, dmID, userID, personaID, body))
	if err != nil {
		return Message{}, fmt.Errorf("insert dm message: %w", err)
	}
	return m, nil
}

// ListDMMessages returns the most recent messages in a DM, oldest-first.
func (s *Store) ListDMMessages(ctx context.Context, dmID string, limit int) ([]Message, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	q := `
		SELECT * FROM (
			SELECT ` + resolvedCols + `
			FROM messages m ` + resolvedJoins + `
			WHERE m.dm_id = $1
			ORDER BY m.created_at DESC
			LIMIT $2
		) t ORDER BY t.created_at ASC`
	rows, err := s.pool.Query(ctx, q, dmID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Message
	for rows.Next() {
		m, err := scanMessage(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := s.attachReactions(ctx, out); err != nil {
		return nil, err
	}
	return out, nil
}
