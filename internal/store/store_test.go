package store

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/cpuchip/ai-chattermax/internal/db"
)

// integration test — runs only when CHATTERMAX_TEST_DSN points at a Postgres the
// test may write to. Applies the (idempotent) schema, then exercises the full
// entity flow with unique slugs so it never collides with existing data.
func testStore(t *testing.T) (*Store, context.Context) {
	t.Helper()
	dsn := os.Getenv("CHATTERMAX_TEST_DSN")
	if dsn == "" {
		t.Skip("set CHATTERMAX_TEST_DSN to run store integration tests")
	}
	ctx := context.Background()
	pool, err := db.Open(ctx, dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(pool.Close)
	sql, err := os.ReadFile("../../migrations/0001_init.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	if _, err := pool.Exec(ctx, string(sql)); err != nil {
		t.Fatalf("apply schema: %v", err)
	}
	return New(pool), ctx
}

func uniq(p string) string { return fmt.Sprintf("%s-%d", p, time.Now().UnixNano()) }

func TestStoreFullFlow(t *testing.T) {
	st, ctx := testStore(t)

	// user
	u, err := st.UpsertUserBySubject(ctx, uniq("dev:tester"), "Tester", "", "")
	if err != nil {
		t.Fatalf("upsert user: %v", err)
	}

	// session round-trip
	tok, err := st.CreateSession(ctx, u.ID)
	if err != nil {
		t.Fatalf("session: %v", err)
	}
	if got, ok, _ := st.SessionUser(ctx, tok); !ok || got.ID != u.ID {
		t.Fatalf("session user mismatch")
	}

	// server (owner auto-membered)
	sv, err := st.CreateServer(ctx, uniq("srv"), "Test Server", u.ID)
	if err != nil {
		t.Fatalf("server: %v", err)
	}
	if role, ok, _ := st.ServerRole(ctx, sv.ID, u.ID); !ok || role != "owner" {
		t.Fatalf("owner role = %q ok=%v", role, ok)
	}

	// room
	room, err := st.CreateRoom(ctx, sv.ID, uniq("rm"), "general", "public", "", u.ID)
	if err != nil {
		t.Fatalf("room: %v", err)
	}
	if ok, _ := st.UserCanAccessRoom(ctx, room.ID, u.ID); !ok {
		t.Fatal("owner should access public room")
	}

	// persona + key mint/validate
	p, err := st.CreatePersona(ctx, sv.ID, u.ID, uniq("p"), "Gandalf", "", "dm-assistant")
	if err != nil {
		t.Fatalf("persona: %v", err)
	}
	raw, err := st.MintPersonaKey(ctx, p.ID, "test")
	if err != nil {
		t.Fatalf("mint key: %v", err)
	}
	got, ok, err := st.ValidatePersonaKey(ctx, raw)
	if err != nil || !ok || got.ID != p.ID {
		t.Fatalf("validate key: ok=%v err=%v", ok, err)
	}
	if _, ok, _ := st.ValidatePersonaKey(ctx, "cmk_bogus"); ok {
		t.Fatal("bogus key must not validate")
	}

	// grant + access
	if err := st.GrantPersonaRoom(ctx, p.ID, room.ID, u.ID); err != nil {
		t.Fatalf("grant: %v", err)
	}
	if ok, _ := st.PersonaCanAccessRoom(ctx, p.ID, room.ID); !ok {
		t.Fatal("granted persona should access room")
	}

	// messages: human + persona, resolved sender, history + FTS
	if _, err := st.InsertRoomUserMessage(ctx, room.ID, u.ID, "hello tavern keeper"); err != nil {
		t.Fatalf("user msg: %v", err)
	}
	pm, err := st.InsertRoomPersonaMessage(ctx, room.ID, p.ID, nil, "well met, traveler")
	if err != nil {
		t.Fatalf("persona msg: %v", err)
	}
	if pm.SenderKind != "persona" || pm.SenderName != "Gandalf" {
		t.Fatalf("persona msg resolved = %+v", pm)
	}
	msgs, err := st.ListRoomMessages(ctx, room.ID, 10)
	if err != nil || len(msgs) != 2 {
		t.Fatalf("history = %d msgs, err=%v", len(msgs), err)
	}
	if msgs[0].SenderKind != "human" {
		t.Fatalf("history order wrong: %+v", msgs[0])
	}
	hits, err := st.SearchRoomMessages(ctx, room.ID, "traveler", 10)
	if err != nil || len(hits) != 1 {
		t.Fatalf("FTS = %d hits, err=%v", len(hits), err)
	}
}
