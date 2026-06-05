// Package seed idempotently ensures a demo server exists so a fresh deployment
// is immediately usable: the "Tavern Keep" workspace with D&D rooms and the two
// pg-ai-stewards personas, each with a stable dev key the persona-host can use.
package seed

import (
	"context"
	"errors"
	"fmt"

	"github.com/cpuchip/ai-chattermax/internal/db"
	"github.com/cpuchip/ai-chattermax/internal/store"
)

// Demo holds the ids the rest of the app needs after seeding.
type Demo struct {
	ServerID    string
	MainRoomID  string
	PersonaKeys map[string]string // host_ref -> raw dev key (logged in dev only)
}

const (
	adminSubject = "seed:game-master"
	serverSlug   = "tavern-keep"
)

// EnsureDemo find-or-creates the demo server, rooms, personas, grants, and dev
// keys. Idempotent across boots.
func EnsureDemo(ctx context.Context, st *store.Store) (Demo, error) {
	var d Demo
	d.PersonaKeys = map[string]string{}

	admin, err := st.UpsertUserBySubject(ctx, adminSubject, "Game Master", "", "")
	if err != nil {
		return d, fmt.Errorf("seed admin: %w", err)
	}

	server, err := st.ServerBySlug(ctx, serverSlug)
	if errors.Is(err, db.ErrNoRows) {
		server, err = st.CreateServer(ctx, serverSlug, "Tavern Keep", admin.ID)
	}
	if err != nil {
		return d, fmt.Errorf("seed server: %w", err)
	}
	d.ServerID = server.ID

	mainGame, err := ensureRoom(ctx, st, server.ID, "main-game", "main-game", "The table where the adventure happens.", admin.ID)
	if err != nil {
		return d, err
	}
	d.MainRoomID = mainGame.ID
	if _, err := ensureRoom(ctx, st, server.ID, "side-table", "side-table", "Out-of-character chatter.", admin.ID); err != nil {
		return d, err
	}

	personas := []struct{ slug, name, hostRef, devKey string }{
		{"dm-assistant", "DM Assistant", "dm-assistant", "cmk_dev_dm_assistant"},
		{"npc-ally", "NPC Ally", "npc-ally", "cmk_dev_npc_ally"},
	}
	for _, p := range personas {
		persona, err := ensurePersona(ctx, st, server.ID, admin.ID, p.slug, p.name, p.hostRef)
		if err != nil {
			return d, err
		}
		if err := st.GrantPersonaRoom(ctx, persona.ID, mainGame.ID, admin.ID); err != nil {
			return d, fmt.Errorf("seed grant %s: %w", p.slug, err)
		}
		if err := st.EnsureDevKey(ctx, persona.ID, p.devKey); err != nil {
			return d, fmt.Errorf("seed key %s: %w", p.slug, err)
		}
		d.PersonaKeys[p.hostRef] = p.devKey
	}
	return d, nil
}

func ensureRoom(ctx context.Context, st *store.Store, serverID, slug, name, topic, createdBy string) (store.Room, error) {
	rooms, err := st.ListRoomsInServer(ctx, serverID)
	if err == nil {
		for _, r := range rooms {
			if r.Slug == slug {
				return r, nil
			}
		}
	}
	r, err := st.CreateRoom(ctx, serverID, slug, name, "public", topic, createdBy)
	if err != nil {
		return store.Room{}, fmt.Errorf("seed room %s: %w", slug, err)
	}
	return r, nil
}

func ensurePersona(ctx context.Context, st *store.Store, serverID, ownerID, slug, name, hostRef string) (store.Persona, error) {
	p, err := st.PersonaByServerSlug(ctx, serverID, slug)
	if err == nil {
		return p, nil
	}
	if !errors.Is(err, db.ErrNoRows) {
		return store.Persona{}, fmt.Errorf("seed persona lookup %s: %w", slug, err)
	}
	p, err = st.CreatePersona(ctx, serverID, ownerID, slug, name, "", hostRef)
	if err != nil {
		return store.Persona{}, fmt.Errorf("seed persona %s: %w", slug, err)
	}
	return p, nil
}
