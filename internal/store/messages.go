package store

import (
	"context"
	"fmt"
)

// resolvedCols is the SELECT list that resolves a message's sender (human or
// persona, with an optional sub-persona display override) for rendering.
const resolvedCols = `
	m.id,
	COALESCE(m.room_id::text, ''),
	COALESCE(m.dm_id::text, ''),
	COALESCE(m.sender_persona_id::text, m.sender_user_id::text) AS sender_id,
	COALESCE(sp.display_name, p.display_name, u.display_name) AS sender_name,
	CASE WHEN m.sender_persona_id IS NOT NULL THEN 'persona' ELSE 'human' END AS sender_kind,
	COALESCE(p.avatar_url, u.avatar_url, '') AS sender_avatar,
	m.body, m.created_at`

const resolvedJoins = `
	LEFT JOIN users u         ON u.id  = m.sender_user_id
	LEFT JOIN personas p      ON p.id  = m.sender_persona_id
	LEFT JOIN sub_personas sp ON sp.id = m.sub_persona_id`

func scanMessage(row interface {
	Scan(...any) error
}) (Message, error) {
	var m Message
	err := row.Scan(&m.ID, &m.RoomID, &m.DMID, &m.SenderID, &m.SenderName, &m.SenderKind, &m.SenderAvatar, &m.Body, &m.CreatedAt)
	return m, err
}

// InsertRoomUserMessage stores a human's room message and returns it resolved.
func (s *Store) InsertRoomUserMessage(ctx context.Context, roomID, userID, body string) (Message, error) {
	return s.insertRoomMessage(ctx, roomID, &userID, nil, nil, body)
}

// InsertRoomPersonaMessage stores a persona's room message (optionally under a
// sub-persona) and returns it resolved.
func (s *Store) InsertRoomPersonaMessage(ctx context.Context, roomID, personaID string, subPersonaID *string, body string) (Message, error) {
	return s.insertRoomMessage(ctx, roomID, nil, &personaID, subPersonaID, body)
}

func (s *Store) insertRoomMessage(ctx context.Context, roomID string, userID, personaID, subPersonaID *string, body string) (Message, error) {
	q := `
		WITH ins AS (
			INSERT INTO messages (room_id, sender_user_id, sender_persona_id, sub_persona_id, body)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING id, room_id, dm_id, sender_user_id, sender_persona_id, sub_persona_id, body, created_at
		)
		SELECT ` + resolvedCols + `
		FROM ins m ` + resolvedJoins
	m, err := scanMessage(s.pool.QueryRow(ctx, q, roomID, userID, personaID, subPersonaID, body))
	if err != nil {
		return Message{}, fmt.Errorf("insert room message: %w", err)
	}
	return m, nil
}

// ListRoomMessages returns the most recent messages in a room, oldest-first.
func (s *Store) ListRoomMessages(ctx context.Context, roomID string, limit int) ([]Message, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	q := `
		SELECT * FROM (
			SELECT ` + resolvedCols + `
			FROM messages m ` + resolvedJoins + `
			WHERE m.room_id = $1
			ORDER BY m.created_at DESC
			LIMIT $2
		) t ORDER BY t.created_at ASC`
	rows, err := s.pool.Query(ctx, q, roomID, limit)
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

// SearchRoomMessages runs a full-text search over a room's messages.
func (s *Store) SearchRoomMessages(ctx context.Context, roomID, query string, limit int) ([]Message, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	q := `
		SELECT ` + resolvedCols + `
		FROM messages m ` + resolvedJoins + `
		WHERE m.room_id = $1 AND m.tsv @@ websearch_to_tsquery('english', $2)
		ORDER BY m.created_at DESC
		LIMIT $3`
	rows, err := s.pool.Query(ctx, q, roomID, query, limit)
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
	return out, rows.Err()
}
