package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// CreatePersona registers a member-owned persona in a server.
func (s *Store) CreatePersona(ctx context.Context, serverID, ownerUserID, slug, displayName, avatarURL, hostRef string) (Persona, error) {
	var p Persona
	err := s.pool.QueryRow(ctx, `
		INSERT INTO personas (server_id, owner_user_id, slug, display_name, avatar_url, host_ref)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, server_id, owner_user_id, slug, display_name, COALESCE(avatar_url,''), host_kind, COALESCE(host_ref,''), status, dm_enabled, created_at`,
		serverID, ownerUserID, slug, displayName, nullIfEmpty(avatarURL), nullIfEmpty(hostRef),
	).Scan(&p.ID, &p.ServerID, &p.OwnerUserID, &p.Slug, &p.DisplayName, &p.AvatarURL, &p.HostKind, &p.HostRef, &p.Status, &p.DMEnabled, &p.CreatedAt)
	if err != nil {
		return Persona{}, fmt.Errorf("insert persona: %w", err)
	}
	return p, nil
}

// ListPersonasForServer returns a server's personas.
func (s *Store) ListPersonasForServer(ctx context.Context, serverID string) ([]Persona, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, server_id, owner_user_id, slug, display_name, COALESCE(avatar_url,''), host_kind, COALESCE(host_ref,''), status, dm_enabled, created_at
		FROM personas WHERE server_id = $1 ORDER BY display_name`, serverID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Persona
	for rows.Next() {
		var p Persona
		if err := rows.Scan(&p.ID, &p.ServerID, &p.OwnerUserID, &p.Slug, &p.DisplayName, &p.AvatarURL, &p.HostKind, &p.HostRef, &p.Status, &p.DMEnabled, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// GetPersona loads a persona by id.
func (s *Store) GetPersona(ctx context.Context, id string) (Persona, error) {
	var p Persona
	err := s.pool.QueryRow(ctx, `
		SELECT id, server_id, owner_user_id, slug, display_name, COALESCE(avatar_url,''), host_kind, COALESCE(host_ref,''), status, dm_enabled, created_at
		FROM personas WHERE id = $1`, id,
	).Scan(&p.ID, &p.ServerID, &p.OwnerUserID, &p.Slug, &p.DisplayName, &p.AvatarURL, &p.HostKind, &p.HostRef, &p.Status, &p.DMEnabled, &p.CreatedAt)
	if err != nil {
		return Persona{}, err
	}
	return p, nil
}

// PersonaByServerSlug loads a persona by (server, slug).
func (s *Store) PersonaByServerSlug(ctx context.Context, serverID, slug string) (Persona, error) {
	var p Persona
	err := s.pool.QueryRow(ctx, `
		SELECT id, server_id, owner_user_id, slug, display_name, COALESCE(avatar_url,''), host_kind, COALESCE(host_ref,''), status, dm_enabled, created_at
		FROM personas WHERE server_id = $1 AND slug = $2`, serverID, slug,
	).Scan(&p.ID, &p.ServerID, &p.OwnerUserID, &p.Slug, &p.DisplayName, &p.AvatarURL, &p.HostKind, &p.HostRef, &p.Status, &p.DMEnabled, &p.CreatedAt)
	if err != nil {
		return Persona{}, err
	}
	return p, nil
}

// hashKey returns the hex SHA-256 of a raw persona key (only the hash is stored).
func hashKey(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// MintPersonaKey creates a new key for a persona and returns the RAW key (shown
// once — only its hash is persisted). Raw format: "cmk_<hex>".
func (s *Store) MintPersonaKey(ctx context.Context, personaID, label string) (string, error) {
	tok, err := randomToken(24)
	if err != nil {
		return "", err
	}
	raw := "cmk_" + tok
	if _, err := s.pool.Exec(ctx, `
		INSERT INTO persona_keys (persona_id, key_hash, label) VALUES ($1, $2, $3)`,
		personaID, hashKey(raw), nullIfEmpty(label)); err != nil {
		return "", fmt.Errorf("mint persona key: %w", err)
	}
	return raw, nil
}

// EnsureDevKey idempotently registers a fixed raw key for a persona (seed/dev
// convenience — a stable key the persona-host can be configured with). Real
// keys are random + UI-minted via MintPersonaKey.
func (s *Store) EnsureDevKey(ctx context.Context, personaID, raw string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO persona_keys (persona_id, key_hash, label) VALUES ($1, $2, 'dev-seed')
		ON CONFLICT (key_hash) DO NOTHING`, personaID, hashKey(raw))
	return err
}

// ListPersonaKeys returns a persona's keys (metadata only — never the hash),
// newest first, so an owner can review and revoke them.
func (s *Store) ListPersonaKeys(ctx context.Context, personaID string) ([]PersonaKey, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, COALESCE(label,''), created_at, last_used_at, revoked_at
		FROM persona_keys WHERE persona_id = $1 ORDER BY created_at DESC`, personaID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PersonaKey
	for rows.Next() {
		var k PersonaKey
		if err := rows.Scan(&k.ID, &k.Label, &k.CreatedAt, &k.LastUsedAt, &k.RevokedAt); err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

// RevokePersonaKey marks a key revoked (soft — the row stays for audit, but the
// key stops validating). Scoped to the persona so one persona can't revoke
// another's key. No-op if already revoked or not found.
func (s *Store) RevokePersonaKey(ctx context.Context, personaID, keyID string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE persona_keys SET revoked_at = now()
		WHERE id = $1 AND persona_id = $2 AND revoked_at IS NULL`, keyID, personaID)
	return err
}

// SetPersonaDMEnabled toggles whether a persona accepts direct messages.
func (s *Store) SetPersonaDMEnabled(ctx context.Context, personaID string, enabled bool) error {
	_, err := s.pool.Exec(ctx, `UPDATE personas SET dm_enabled = $2 WHERE id = $1`, personaID, enabled)
	return err
}

// ValidatePersonaKey resolves a raw key to its persona (active, key not revoked),
// touching last_used_at. ok=false when the key is unknown or revoked.
func (s *Store) ValidatePersonaKey(ctx context.Context, raw string) (Persona, bool, error) {
	var p Persona
	err := s.pool.QueryRow(ctx, `
		SELECT p.id, p.server_id, p.owner_user_id, p.slug, p.display_name, COALESCE(p.avatar_url,''), p.host_kind, COALESCE(p.host_ref,''), p.status, p.dm_enabled, p.created_at
		FROM persona_keys k JOIN personas p ON p.id = k.persona_id
		WHERE k.key_hash = $1 AND k.revoked_at IS NULL AND p.status = 'active'`, hashKey(raw),
	).Scan(&p.ID, &p.ServerID, &p.OwnerUserID, &p.Slug, &p.DisplayName, &p.AvatarURL, &p.HostKind, &p.HostRef, &p.Status, &p.DMEnabled, &p.CreatedAt)
	if err != nil {
		return Persona{}, false, nil //nolint:nilerr // unknown key is not an error
	}
	_, _ = s.pool.Exec(ctx, `UPDATE persona_keys SET last_used_at = now() WHERE key_hash = $1`, hashKey(raw))
	return p, true, nil
}

// GrantPersonaRoom grants a persona into a room (idempotent).
func (s *Store) GrantPersonaRoom(ctx context.Context, personaID, roomID, grantedBy string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO persona_room_grants (persona_id, room_id, granted_by) VALUES ($1, $2, $3)
		ON CONFLICT (persona_id, room_id) DO NOTHING`, personaID, roomID, nullIfEmpty(grantedBy))
	return err
}

// PersonaGrantedRooms returns the room ids a persona is granted into.
func (s *Store) PersonaGrantedRooms(ctx context.Context, personaID string) ([]string, error) {
	rows, err := s.pool.Query(ctx, `SELECT room_id FROM persona_room_grants WHERE persona_id = $1`, personaID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// PersonaRooms returns the rooms a persona is granted into, with details — so a
// persona-host can subscribe to all of them and a model can see its access.
func (s *Store) PersonaRooms(ctx context.Context, personaID string) ([]Room, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT r.id, r.server_id, r.slug, r.name, r.visibility, COALESCE(r.topic,''), r.created_at
		FROM persona_room_grants g JOIN rooms r ON r.id = g.room_id
		WHERE g.persona_id = $1
		ORDER BY r.created_at`, personaID)
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

// RevokePersonaRoom removes a persona's grant to a room.
func (s *Store) RevokePersonaRoom(ctx context.Context, personaID, roomID string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM persona_room_grants WHERE persona_id = $1 AND room_id = $2`, personaID, roomID)
	return err
}

// PersonaCanAccessRoom reports whether a persona is granted into a room.
func (s *Store) PersonaCanAccessRoom(ctx context.Context, personaID, roomID string) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM persona_room_grants WHERE persona_id = $1 AND room_id = $2)`,
		personaID, roomID).Scan(&ok)
	return ok, err
}
