package store

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Notification is a mention alert for a human, resolved for rendering.
type Notification struct {
	ID        string     `json:"id"`
	Kind      string     `json:"kind"`
	RoomID    string     `json:"roomId"`
	RoomName  string     `json:"roomName"`
	MessageID string     `json:"messageId"`
	From      string     `json:"from"`
	Snippet   string     `json:"snippet"`
	CreatedAt time.Time  `json:"createdAt"`
	ReadAt    *time.Time `json:"readAt,omitempty"`
}

// MembersForRoom returns the server members for a room's server (the candidate
// set for @mention matching).
func (s *Store) MembersForRoom(ctx context.Context, roomID string) ([]Member, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT u.id, u.display_name, COALESCE(u.avatar_url,''), m.role
		FROM rooms r
		JOIN server_members m ON m.server_id = r.server_id
		JOIN users u          ON u.id = m.user_id
		WHERE r.id = $1`, roomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Member
	for rows.Next() {
		var m Member
		if err := rows.Scan(&m.UserID, &m.DisplayName, &m.AvatarURL, &m.Role); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// UserIsRoomAdmin reports whether the user is owner/admin of the room's server.
func (s *Store) UserIsRoomAdmin(ctx context.Context, roomID, userID string) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM rooms r
			JOIN server_members m ON m.server_id = r.server_id
			WHERE r.id = $1 AND m.user_id = $2 AND m.role IN ('owner','admin')
		)`, roomID, userID).Scan(&ok)
	return ok, err
}

var mentionToken = regexp.MustCompile(`@([\p{L}\p{N}_.-]+)`)

// MentionedUserIDs resolves the @tokens in a body against a member list. A
// token matches a member when it equals (case-insensitively) the display name,
// the display name with spaces stripped, or — when unique among members — the
// display name's first word. The sender never mentions themselves.
func MentionedUserIDs(body, senderID string, members []Member) []string {
	tokens := map[string]bool{}
	for _, m := range mentionToken.FindAllStringSubmatch(body, -1) {
		tokens[strings.ToLower(m[1])] = true
	}
	if len(tokens) == 0 {
		return nil
	}
	// First-word collision counting so "@michael" stays unambiguous.
	firstCount := map[string]int{}
	for _, m := range members {
		fw := strings.ToLower(strings.Fields(m.DisplayName + " ")[0])
		if fw != "" {
			firstCount[fw]++
		}
	}
	var out []string
	for _, m := range members {
		if m.UserID == senderID {
			continue
		}
		dn := strings.ToLower(m.DisplayName)
		nospace := strings.ReplaceAll(dn, " ", "")
		first := strings.ToLower(strings.Fields(m.DisplayName + " ")[0])
		if tokens[dn] || tokens[nospace] || (first != "" && firstCount[first] == 1 && tokens[first]) {
			out = append(out, m.UserID)
		}
	}
	return out
}

// CreateMentionNotification writes one mention row, returning its id and
// timestamp so the gateway can push a live notification frame.
func (s *Store) CreateMentionNotification(ctx context.Context, userID, messageID, roomID string) (id string, createdAt time.Time, err error) {
	err = s.pool.QueryRow(ctx, `
		INSERT INTO notifications (user_id, kind, message_id, room_id)
		VALUES ($1, 'mention', $2, $3)
		RETURNING id, created_at`, userID, messageID, roomID).Scan(&id, &createdAt)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("create notification: %w", err)
	}
	return id, createdAt, nil
}

// ListNotifications returns a user's latest notifications, newest first, with
// the sender and message snippet resolved.
func (s *Store) ListNotifications(ctx context.Context, userID string, limit int) ([]Notification, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		SELECT n.id, n.kind,
		       COALESCE(n.room_id::text,''), COALESCE(r.name,''),
		       COALESCE(n.message_id::text,''),
		       COALESCE(sp.display_name, p.display_name, u.display_name, ''),
		       COALESCE(left(m.body, 120),''),
		       n.created_at, n.read_at
		FROM notifications n
		LEFT JOIN rooms r     ON r.id = n.room_id
		LEFT JOIN messages m  ON m.id = n.message_id
		LEFT JOIN users u     ON u.id = m.sender_user_id
		LEFT JOIN personas p  ON p.id = m.sender_persona_id
		LEFT JOIN sub_personas sp ON sp.id = m.sub_persona_id
		WHERE n.user_id = $1
		ORDER BY n.created_at DESC
		LIMIT $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Notification
	for rows.Next() {
		var n Notification
		if err := rows.Scan(&n.ID, &n.Kind, &n.RoomID, &n.RoomName, &n.MessageID, &n.From, &n.Snippet, &n.CreatedAt, &n.ReadAt); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// MarkNotificationsRead marks the given ids read for the user; with no ids it
// marks ALL the user's unread notifications read.
func (s *Store) MarkNotificationsRead(ctx context.Context, userID string, ids []string) error {
	var err error
	if len(ids) == 0 {
		_, err = s.pool.Exec(ctx, `
			UPDATE notifications SET read_at = now()
			WHERE user_id = $1 AND read_at IS NULL`, userID)
	} else {
		_, err = s.pool.Exec(ctx, `
			UPDATE notifications SET read_at = now()
			WHERE user_id = $1 AND read_at IS NULL AND id = ANY($2::uuid[])`, userID, ids)
	}
	return err
}
