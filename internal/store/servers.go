package store

import (
	"context"
	"fmt"
)

// CreateServer creates a server and adds the owner as a member with role=owner,
// in one transaction.
func (s *Store) CreateServer(ctx context.Context, slug, name, ownerUserID string) (Server, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Server{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var sv Server
	err = tx.QueryRow(ctx, `
		INSERT INTO servers (slug, name, owner_user_id)
		VALUES ($1, $2, $3)
		RETURNING id, slug, name, owner_user_id, join_token, created_at`,
		slug, name, ownerUserID,
	).Scan(&sv.ID, &sv.Slug, &sv.Name, &sv.OwnerUserID, &sv.JoinToken, &sv.CreatedAt)
	if err != nil {
		return Server{}, fmt.Errorf("insert server: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO server_members (server_id, user_id, role) VALUES ($1, $2, 'owner')`,
		sv.ID, ownerUserID); err != nil {
		return Server{}, fmt.Errorf("add owner member: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Server{}, err
	}
	return sv, nil
}

// ListServersForUser returns the servers a user is a member of.
func (s *Store) ListServersForUser(ctx context.Context, userID string) ([]Server, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT sv.id, sv.slug, sv.name, sv.owner_user_id, sv.created_at
		FROM servers sv JOIN server_members m ON m.server_id = sv.id
		WHERE m.user_id = $1
		ORDER BY sv.created_at`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Server
	for rows.Next() {
		var sv Server
		if err := rows.Scan(&sv.ID, &sv.Slug, &sv.Name, &sv.OwnerUserID, &sv.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, sv)
	}
	return out, rows.Err()
}

// GetServer loads a server by id.
func (s *Store) GetServer(ctx context.Context, id string) (Server, error) {
	var sv Server
	err := s.pool.QueryRow(ctx, `
		SELECT id, slug, name, owner_user_id, join_token, created_at FROM servers WHERE id = $1`, id,
	).Scan(&sv.ID, &sv.Slug, &sv.Name, &sv.OwnerUserID, &sv.JoinToken, &sv.CreatedAt)
	if err != nil {
		return Server{}, err
	}
	return sv, nil
}

// ServerBySlug loads a server by slug.
func (s *Store) ServerBySlug(ctx context.Context, slug string) (Server, error) {
	var sv Server
	err := s.pool.QueryRow(ctx, `
		SELECT id, slug, name, owner_user_id, join_token, created_at FROM servers WHERE slug = $1`, slug,
	).Scan(&sv.ID, &sv.Slug, &sv.Name, &sv.OwnerUserID, &sv.JoinToken, &sv.CreatedAt)
	if err != nil {
		return Server{}, err
	}
	return sv, nil
}

// ServerByJoinToken resolves a join link token to its server (for invites).
func (s *Store) ServerByJoinToken(ctx context.Context, token string) (Server, error) {
	var sv Server
	err := s.pool.QueryRow(ctx, `
		SELECT id, slug, name, owner_user_id, join_token, created_at FROM servers WHERE join_token = $1`, token,
	).Scan(&sv.ID, &sv.Slug, &sv.Name, &sv.OwnerUserID, &sv.JoinToken, &sv.CreatedAt)
	if err != nil {
		return Server{}, err
	}
	return sv, nil
}

// AddServerMember adds (or keeps) a membership; idempotent.
func (s *Store) AddServerMember(ctx context.Context, serverID, userID, role string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO server_members (server_id, user_id, role) VALUES ($1, $2, $3)
		ON CONFLICT (server_id, user_id) DO NOTHING`, serverID, userID, role)
	return err
}

// ServerRole returns a user's role in a server and whether they are a member.
func (s *Store) ServerRole(ctx context.Context, serverID, userID string) (string, bool, error) {
	var role string
	err := s.pool.QueryRow(ctx, `
		SELECT role FROM server_members WHERE server_id = $1 AND user_id = $2`, serverID, userID).Scan(&role)
	if err != nil {
		return "", false, nil //nolint:nilerr // non-member is not an error
	}
	return role, true, nil
}

// ListServerMembers returns a server's members (the registry's human rows).
func (s *Store) ListServerMembers(ctx context.Context, serverID string) ([]Member, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT u.id, u.display_name, COALESCE(u.avatar_url,''), m.role
		FROM server_members m JOIN users u ON u.id = m.user_id
		WHERE m.server_id = $1
		ORDER BY m.role, u.display_name`, serverID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Member
	for rows.Next() {
		var mb Member
		if err := rows.Scan(&mb.UserID, &mb.DisplayName, &mb.AvatarURL, &mb.Role); err != nil {
			return nil, err
		}
		out = append(out, mb)
	}
	return out, rows.Err()
}
