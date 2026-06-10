package store

import (
	"context"
	"fmt"
)

// Reaction is one reactor's emoji on a message, resolved for rendering.
type Reaction struct {
	Emoji       string `json:"emoji"`
	ReactorID   string `json:"reactorId"`
	ReactorName string `json:"reactor"`
	ReactorKind string `json:"reactorKind"` // human | persona
}

// MessageInChannel reports whether a message belongs to the given room or DM —
// the gateway's guard that a reaction targets a message in the channel the
// reactor is acting in (channel access itself is checked separately).
func (s *Store) MessageInChannel(ctx context.Context, messageID, channel string) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM messages
			WHERE id = $1 AND (room_id::text = $2 OR dm_id::text = $2)
		)`, messageID, channel).Scan(&ok)
	return ok, err
}

// AddReaction stores a reaction. Exactly one of userID/personaID is set.
// Re-adding an existing reaction is a no-op (unique index).
func (s *Store) AddReaction(ctx context.Context, messageID string, userID, personaID *string, emoji string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO message_reactions (message_id, reactor_user_id, reactor_persona_id, emoji)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT DO NOTHING`, messageID, userID, personaID, emoji)
	if err != nil {
		return fmt.Errorf("add reaction: %w", err)
	}
	return nil
}

// RemoveReaction deletes the reactor's emoji from a message (no-op if absent).
func (s *Store) RemoveReaction(ctx context.Context, messageID string, userID, personaID *string, emoji string) error {
	_, err := s.pool.Exec(ctx, `
		DELETE FROM message_reactions
		WHERE message_id = $1 AND emoji = $4
		  AND COALESCE(reactor_user_id, reactor_persona_id) = COALESCE($2::uuid, $3::uuid)`,
		messageID, userID, personaID, emoji)
	if err != nil {
		return fmt.Errorf("remove reaction: %w", err)
	}
	return nil
}

// attachReactions decorates messages with their stored reactions in one batched
// query (history and REST backfill). Messages without reactions are untouched.
func (s *Store) attachReactions(ctx context.Context, msgs []Message) error {
	if len(msgs) == 0 {
		return nil
	}
	ids := make([]string, len(msgs))
	for i, m := range msgs {
		ids[i] = m.ID
	}
	rows, err := s.pool.Query(ctx, `
		SELECT r.message_id,
		       r.emoji,
		       COALESCE(r.reactor_user_id::text, r.reactor_persona_id::text),
		       COALESCE(u.display_name, p.display_name, ''),
		       CASE WHEN r.reactor_persona_id IS NOT NULL THEN 'persona' ELSE 'human' END
		FROM message_reactions r
		LEFT JOIN users u    ON u.id = r.reactor_user_id
		LEFT JOIN personas p ON p.id = r.reactor_persona_id
		WHERE r.message_id = ANY($1::uuid[])
		ORDER BY r.created_at`, ids)
	if err != nil {
		return fmt.Errorf("attach reactions: %w", err)
	}
	defer rows.Close()
	byMsg := make(map[string][]Reaction)
	for rows.Next() {
		var msgID string
		var r Reaction
		if err := rows.Scan(&msgID, &r.Emoji, &r.ReactorID, &r.ReactorName, &r.ReactorKind); err != nil {
			return err
		}
		byMsg[msgID] = append(byMsg[msgID], r)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for i := range msgs {
		if rs, ok := byMsg[msgs[i].ID]; ok {
			msgs[i].Reactions = rs
		}
	}
	return nil
}
