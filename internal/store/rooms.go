package store

import (
	"context"
	"fmt"
)

// CreateRoom creates a channel in a server.
func (s *Store) CreateRoom(ctx context.Context, serverID, slug, name, visibility, topic, createdBy string) (Room, error) {
	var r Room
	err := s.pool.QueryRow(ctx, `
		INSERT INTO rooms (server_id, slug, name, visibility, topic, created_by)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, server_id, slug, name, visibility, COALESCE(topic,''), created_at`,
		serverID, slug, name, visibility, nullIfEmpty(topic), nullIfEmpty(createdBy),
	).Scan(&r.ID, &r.ServerID, &r.Slug, &r.Name, &r.Visibility, &r.Topic, &r.CreatedAt)
	if err != nil {
		return Room{}, fmt.Errorf("insert room: %w", err)
	}
	return r, nil
}

// ListRoomsForUser returns the rooms in a server the user may see: all public
// rooms plus the private rooms they belong to.
func (s *Store) ListRoomsForUser(ctx context.Context, serverID, userID string) ([]Room, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT r.id, r.server_id, r.slug, r.name, r.visibility, COALESCE(r.topic,''), r.created_at
		FROM rooms r
		WHERE r.server_id = $1
		  AND (r.visibility = 'public'
		       OR EXISTS (SELECT 1 FROM room_members rm WHERE rm.room_id = r.id AND rm.user_id = $2))
		ORDER BY r.created_at`, serverID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Room
	for rows.Next() {
		var r Room
		if err := rows.Scan(&r.ID, &r.ServerID, &r.Slug, &r.Name, &r.Visibility, &r.Topic, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ListRoomsInServer returns every room in a server (no per-user filter) — used
// by the seed and admin views.
func (s *Store) ListRoomsInServer(ctx context.Context, serverID string) ([]Room, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, server_id, slug, name, visibility, COALESCE(topic,''), created_at
		FROM rooms WHERE server_id = $1 ORDER BY created_at`, serverID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Room
	for rows.Next() {
		var r Room
		if err := rows.Scan(&r.ID, &r.ServerID, &r.Slug, &r.Name, &r.Visibility, &r.Topic, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// GetRoom loads a room by id.
func (s *Store) GetRoom(ctx context.Context, id string) (Room, error) {
	var r Room
	err := s.pool.QueryRow(ctx, `
		SELECT id, server_id, slug, name, visibility, COALESCE(topic,''), created_at FROM rooms WHERE id = $1`, id,
	).Scan(&r.ID, &r.ServerID, &r.Slug, &r.Name, &r.Visibility, &r.Topic, &r.CreatedAt)
	if err != nil {
		return Room{}, err
	}
	return r, nil
}

// AddRoomMember adds a user to a (private) room; idempotent.
func (s *Store) AddRoomMember(ctx context.Context, roomID, userID, role string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO room_members (room_id, user_id, role) VALUES ($1, $2, $3)
		ON CONFLICT (room_id, user_id) DO NOTHING`, roomID, userID, role)
	return err
}

// UserCanAccessRoom reports whether a user may read/post in a room: a server
// member for a public room, or a room member for a private room.
func (s *Store) UserCanAccessRoom(ctx context.Context, roomID, userID string) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM rooms r
			JOIN server_members sm ON sm.server_id = r.server_id AND sm.user_id = $2
			WHERE r.id = $1
			  AND (r.visibility = 'public'
			       OR EXISTS (SELECT 1 FROM room_members rm WHERE rm.room_id = r.id AND rm.user_id = $2)))`,
		roomID, userID).Scan(&ok)
	return ok, err
}
