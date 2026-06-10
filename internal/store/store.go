// Package store is the data-access layer over the platform's Postgres schema.
// One Store wraps the pool; methods are grouped by entity across sibling files.
package store

import (
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Store is the repository facade over the connection pool.
type Store struct {
	pool *pgxpool.Pool
}

// New wraps a pool.
func New(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// --- Models -----------------------------------------------------------------

// User is an authenticated human.
type User struct {
	ID              string     `json:"id"`
	ExternalSubject string     `json:"-"`
	DisplayName     string     `json:"displayName"`
	Email           string     `json:"-"`
	AvatarURL       string     `json:"avatarUrl,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
	LastSeenAt      time.Time  `json:"lastSeenAt"`
}

// Server is a workspace.
type Server struct {
	ID          string    `json:"id"`
	Slug        string    `json:"slug"`
	Name        string    `json:"name"`
	OwnerUserID string    `json:"ownerUserId"`
	JoinToken   string    `json:"joinToken,omitempty"` // only exposed to admins
	CreatedAt   time.Time `json:"createdAt"`
}

// Room is a channel.
type Room struct {
	ID         string    `json:"id"`
	ServerID   string    `json:"serverId"`
	Slug       string    `json:"slug"`
	Name       string    `json:"name"`
	Visibility string    `json:"visibility"`
	Topic      string    `json:"topic,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
}

// Persona is an AI participant's social identity (the mind lives in the host).
type Persona struct {
	ID          string    `json:"id"`
	ServerID    string    `json:"serverId"`
	OwnerUserID string    `json:"ownerUserId"`
	Slug        string    `json:"slug"`
	DisplayName string    `json:"displayName"`
	AvatarURL   string    `json:"avatarUrl,omitempty"`
	HostKind    string    `json:"hostKind"`
	HostRef     string    `json:"hostRef,omitempty"`
	Status      string    `json:"status"`
	DMEnabled   bool      `json:"dmEnabled"`
	CreatedAt   time.Time `json:"createdAt"`
}

// PersonaKey is a minted key's metadata (never the hash or raw value). Surfaced
// in Settings so an owner can see and revoke a persona's keys.
type PersonaKey struct {
	ID         string     `json:"id"`
	Label      string     `json:"label,omitempty"`
	CreatedAt  time.Time  `json:"createdAt"`
	LastUsedAt *time.Time `json:"lastUsedAt,omitempty"`
	RevokedAt  *time.Time `json:"revokedAt,omitempty"`
}

// Member is a server membership joined with its user (for the registry).
type Member struct {
	UserID      string `json:"userId"`
	DisplayName string `json:"displayName"`
	AvatarURL   string `json:"avatarUrl,omitempty"`
	Role        string `json:"role"`
}

// Message is a stored message with its sender resolved for rendering.
type Message struct {
	ID         string    `json:"id"`
	RoomID     string    `json:"roomId,omitempty"`
	DMID       string    `json:"dmId,omitempty"`
	SenderID   string    `json:"senderId"`
	SenderName string    `json:"sender"`
	SenderKind string    `json:"senderKind"` // human | persona
	SenderAvatar string  `json:"senderAvatar,omitempty"`
	Body       string    `json:"body"`
	CreatedAt  time.Time `json:"ts"`
	Reactions  []Reaction `json:"reactions,omitempty"`
}
