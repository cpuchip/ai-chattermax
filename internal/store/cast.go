package store

import (
	"context"
	"fmt"
)

// SubPersona is one cast member — a named character a persona speaks as in a
// room. Display identity only: what thinks behind it is the host's business
// (the DH-2 decoupling principle).
type SubPersona struct {
	ID          string `json:"id"`
	PersonaID   string `json:"personaId"`
	PersonaName string `json:"personaName,omitempty"`
	RoomID      string `json:"roomId"`
	DisplayName string `json:"displayName"`
}

// maxCastPerRoom bounds runaway cast creation by a chatty model.
const maxCastPerRoom = 50

// ResolveSubPersona finds or creates a cast member by name for (persona, room).
// Auto-create on first use is the cast UX: the DM just speaks as "Grimble" and
// Grimble exists. Case-insensitive match keeps "grimble" and "Grimble" one
// character. Cast names are ROOM-unique across personas — at a table, one name
// is one character: if another persona already voices this name here, resolve
// fails and the caller falls back to its own voice (two Grimbles confused the
// 2026-06-10 Holodeck-3 table). Returns created=true when this use minted the
// character.
func (s *Store) ResolveSubPersona(ctx context.Context, personaID, roomID, name string) (SubPersona, bool, error) {
	var sp SubPersona
	err := s.pool.QueryRow(ctx, `
		SELECT id, persona_id, room_id, display_name FROM sub_personas
		WHERE room_id = $1 AND lower(display_name) = lower($2)`,
		roomID, name,
	).Scan(&sp.ID, &sp.PersonaID, &sp.RoomID, &sp.DisplayName)
	if err == nil {
		if sp.PersonaID != personaID {
			return SubPersona{}, false, fmt.Errorf("%q is already voiced by another persona in this room", sp.DisplayName)
		}
		return sp, false, nil
	}
	var n int
	if err := s.pool.QueryRow(ctx, `
		SELECT count(*) FROM sub_personas WHERE persona_id = $1 AND room_id = $2`,
		personaID, roomID).Scan(&n); err != nil {
		return SubPersona{}, false, err
	}
	if n >= maxCastPerRoom {
		return SubPersona{}, false, fmt.Errorf("cast limit reached (%d) in this room", maxCastPerRoom)
	}
	err = s.pool.QueryRow(ctx, `
		INSERT INTO sub_personas (persona_id, room_id, display_name)
		VALUES ($1, $2, $3)
		ON CONFLICT (room_id, lower(display_name)) DO UPDATE SET display_name = sub_personas.display_name
		RETURNING id, persona_id, room_id, display_name`,
		personaID, roomID, name,
	).Scan(&sp.ID, &sp.PersonaID, &sp.RoomID, &sp.DisplayName)
	if err != nil {
		return SubPersona{}, false, fmt.Errorf("create sub-persona: %w", err)
	}
	if sp.PersonaID != personaID { // lost a creation race to another persona
		return SubPersona{}, false, fmt.Errorf("%q is already voiced by another persona in this room", sp.DisplayName)
	}
	return sp, true, nil
}

// RoomCast lists every cast member in a room across its granted personas,
// with the owning persona's name (for roster nesting).
func (s *Store) RoomCast(ctx context.Context, roomID string) ([]SubPersona, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT sp.id, sp.persona_id, p.display_name, sp.room_id, sp.display_name
		FROM sub_personas sp
		JOIN personas p ON p.id = sp.persona_id
		WHERE sp.room_id = $1 AND p.status = 'active'
		ORDER BY p.display_name, lower(sp.display_name)`, roomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SubPersona
	for rows.Next() {
		var sp SubPersona
		if err := rows.Scan(&sp.ID, &sp.PersonaID, &sp.PersonaName, &sp.RoomID, &sp.DisplayName); err != nil {
			return nil, err
		}
		out = append(out, sp)
	}
	return out, rows.Err()
}

// RetireSubPersona removes a cast member (its past messages keep the name via
// historical attribution… actually messages reference the row; ON DELETE SET
// NULL on messages.sub_persona_id means old lines fall back to the persona's
// own name — acceptable: retire sparingly, usually at campaign end).
func (s *Store) RetireSubPersona(ctx context.Context, personaID, subPersonaID string) (bool, error) {
	tag, err := s.pool.Exec(ctx, `
		DELETE FROM sub_personas WHERE id = $1 AND persona_id = $2`, subPersonaID, personaID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}
