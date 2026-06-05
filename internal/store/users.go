package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

const sessionDuration = 30 * 24 * time.Hour

// UpsertUserBySubject finds-or-creates a user keyed by its external auth subject
// (e.g. "becoming:<id>" or "dev:<name>"), updating display/email/avatar on
// re-login. Returns the resolved user.
func (s *Store) UpsertUserBySubject(ctx context.Context, subject, displayName, email, avatar string) (User, error) {
	var u User
	err := s.pool.QueryRow(ctx, `
		INSERT INTO users (external_subject, display_name, email, avatar_url, last_seen_at)
		VALUES ($1, $2, $3, $4, now())
		ON CONFLICT (external_subject) DO UPDATE SET
			display_name = EXCLUDED.display_name,
			email        = COALESCE(EXCLUDED.email, users.email),
			avatar_url   = COALESCE(EXCLUDED.avatar_url, users.avatar_url),
			last_seen_at = now()
		RETURNING id, external_subject, display_name, COALESCE(email,''), COALESCE(avatar_url,''), created_at, last_seen_at`,
		subject, displayName, nullIfEmpty(email), nullIfEmpty(avatar),
	).Scan(&u.ID, &u.ExternalSubject, &u.DisplayName, &u.Email, &u.AvatarURL, &u.CreatedAt, &u.LastSeenAt)
	if err != nil {
		return User{}, fmt.Errorf("upsert user: %w", err)
	}
	return u, nil
}

// GetUserByID loads a user by id.
func (s *Store) GetUserByID(ctx context.Context, id string) (User, error) {
	var u User
	err := s.pool.QueryRow(ctx, `
		SELECT id, external_subject, display_name, COALESCE(email,''), COALESCE(avatar_url,''), created_at, last_seen_at
		FROM users WHERE id = $1`, id,
	).Scan(&u.ID, &u.ExternalSubject, &u.DisplayName, &u.Email, &u.AvatarURL, &u.CreatedAt, &u.LastSeenAt)
	if err != nil {
		return User{}, err
	}
	return u, nil
}

// CreateSession mints a session token for a user.
func (s *Store) CreateSession(ctx context.Context, userID string) (string, error) {
	token, err := randomToken(32)
	if err != nil {
		return "", err
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO sessions (token, user_id, expires_at)
		VALUES ($1, $2, now() + $3::interval)`,
		token, userID, fmt.Sprintf("%d seconds", int(sessionDuration.Seconds())))
	if err != nil {
		return "", fmt.Errorf("create session: %w", err)
	}
	return token, nil
}

// SessionUser resolves a session token to its (non-expired) user, sliding the
// expiry forward. Returns ErrNoRows-equivalent (nil error, ok=false) when absent
// or expired.
func (s *Store) SessionUser(ctx context.Context, token string) (User, bool, error) {
	var u User
	err := s.pool.QueryRow(ctx, `
		SELECT u.id, u.external_subject, u.display_name, COALESCE(u.email,''), COALESCE(u.avatar_url,''), u.created_at, u.last_seen_at
		FROM sessions se JOIN users u ON u.id = se.user_id
		WHERE se.token = $1 AND se.expires_at > now()`, token,
	).Scan(&u.ID, &u.ExternalSubject, &u.DisplayName, &u.Email, &u.AvatarURL, &u.CreatedAt, &u.LastSeenAt)
	if err != nil {
		return User{}, false, nil //nolint:nilerr // absent/expired session is not an error
	}
	// Slide expiry + last seen (best-effort).
	_, _ = s.pool.Exec(ctx, `
		UPDATE sessions SET last_active = now(), expires_at = now() + $2::interval WHERE token = $1`,
		token, fmt.Sprintf("%d seconds", int(sessionDuration.Seconds())))
	_, _ = s.pool.Exec(ctx, `UPDATE users SET last_seen_at = now() WHERE id = $1`, u.ID)
	return u, true, nil
}

// DeleteSession removes a session (logout).
func (s *Store) DeleteSession(ctx context.Context, token string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE token = $1`, token)
	return err
}

func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("random token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
