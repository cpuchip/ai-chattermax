package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
	files, err := filepath.Glob("../../migrations/*.sql")
	if err != nil || len(files) == 0 {
		t.Fatalf("glob migrations: %v (found %d)", err, len(files))
	}
	sort.Strings(files) // lexical order, same as the boot-time runner
	for _, f := range files {
		sql, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read migration %s: %v", f, err)
		}
		if _, err := pool.Exec(ctx, string(sql)); err != nil {
			t.Fatalf("apply %s: %v", f, err)
		}
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

	// reactions: human + persona on the persona's message, idempotent add,
	// channel guard, backfill on history, remove
	if ok, _ := st.MessageInChannel(ctx, pm.ID, room.ID); !ok {
		t.Fatal("message should be in its own room")
	}
	if ok, _ := st.MessageInChannel(ctx, pm.ID, uniq("other-channel")); ok {
		t.Fatal("message must not match a foreign channel")
	}
	if err := st.AddReaction(ctx, pm.ID, &u.ID, nil, "👍"); err != nil {
		t.Fatalf("add human reaction: %v", err)
	}
	if err := st.AddReaction(ctx, pm.ID, &u.ID, nil, "👍"); err != nil {
		t.Fatalf("re-add must be a no-op, got: %v", err)
	}
	if err := st.AddReaction(ctx, pm.ID, nil, &p.ID, "👀"); err != nil {
		t.Fatalf("add persona reaction: %v", err)
	}
	msgs, err = st.ListRoomMessages(ctx, room.ID, 10)
	if err != nil {
		t.Fatalf("history with reactions: %v", err)
	}
	last := msgs[len(msgs)-1]
	if len(last.Reactions) != 2 {
		t.Fatalf("backfill = %d reactions, want 2: %+v", len(last.Reactions), last.Reactions)
	}
	if last.Reactions[0].Emoji != "👍" || last.Reactions[0].ReactorKind != "human" || last.Reactions[0].ReactorName != "Tester" {
		t.Fatalf("human reaction resolved wrong: %+v", last.Reactions[0])
	}
	if last.Reactions[1].Emoji != "👀" || last.Reactions[1].ReactorKind != "persona" || last.Reactions[1].ReactorName != "Gandalf" {
		t.Fatalf("persona reaction resolved wrong: %+v", last.Reactions[1])
	}
	if err := st.RemoveReaction(ctx, pm.ID, nil, &p.ID, "👀"); err != nil {
		t.Fatalf("remove persona reaction: %v", err)
	}
	msgs, _ = st.ListRoomMessages(ctx, room.ID, 10)
	last = msgs[len(msgs)-1]
	if len(last.Reactions) != 1 || last.Reactions[0].Emoji != "👍" {
		t.Fatalf("after remove = %+v", last.Reactions)
	}

	// notifications (REM-3): members lookup, create, list resolved, mark read
	members, err := st.MembersForRoom(ctx, room.ID)
	if err != nil || len(members) != 1 || members[0].UserID != u.ID {
		t.Fatalf("members for room = %+v, err=%v", members, err)
	}
	nid, _, err := st.CreateMentionNotification(ctx, u.ID, pm.ID, room.ID)
	if err != nil || nid == "" {
		t.Fatalf("create notification: %v", err)
	}
	ns, err := st.ListNotifications(ctx, u.ID, 10)
	if err != nil || len(ns) == 0 {
		t.Fatalf("list notifications = %d, err=%v", len(ns), err)
	}
	if ns[0].From != "Gandalf" || ns[0].Snippet == "" || ns[0].ReadAt != nil {
		t.Fatalf("notification resolved wrong: %+v", ns[0])
	}
	if err := st.MarkNotificationsRead(ctx, u.ID, []string{nid}); err != nil {
		t.Fatalf("mark read: %v", err)
	}
	ns, _ = st.ListNotifications(ctx, u.ID, 10)
	if ns[0].ReadAt == nil {
		t.Fatal("notification should be read")
	}

	// respond_policy + mood round trips
	if p.RespondPolicy != "all" {
		t.Fatalf("default respond_policy = %q, want all", p.RespondPolicy)
	}
	if err := st.SetPersonaRespondPolicy(ctx, p.ID, "mentioned"); err != nil {
		t.Fatalf("set respond_policy: %v", err)
	}
	if np, _ := st.GetPersona(ctx, p.ID); np.RespondPolicy != "mentioned" {
		t.Fatalf("respond_policy = %q after set", np.RespondPolicy)
	}
	if err := st.SetPersonaRespondPolicy(ctx, p.ID, "bogus"); err == nil {
		t.Fatal("bogus respond_policy must be rejected by the CHECK")
	}
	if err := st.SetUserMood(ctx, u.ID, "😎"); err != nil {
		t.Fatalf("set mood: %v", err)
	}
	if nu, _ := st.GetUserByID(ctx, u.ID); nu.Mood != "😎" {
		t.Fatalf("mood = %q after set", nu.Mood)
	}

	// initiative (DH-1/D8): start, one-active guard, entries sort, advance
	// wraps + bumps round, remove, end
	ir, err := st.StartInitiative(ctx, room.ID, &u.ID, nil)
	if err != nil {
		t.Fatalf("start initiative: %v", err)
	}
	if _, err := st.StartInitiative(ctx, room.ID, &u.ID, nil); err == nil {
		t.Fatal("second active round must be rejected")
	}
	_ = st.UpsertInitiativeEntry(ctx, ir.ID, "Grimble", 2, 16, 18)
	_ = st.UpsertInitiativeEntry(ctx, ir.ID, "Vex", 3, 11, 14)
	_ = st.UpsertInitiativeEntry(ctx, ir.ID, "Goblin", 0, 9, 9)
	ar, ok, _ := st.ActiveInitiative(ctx, room.ID)
	if !ok || len(ar.Entries) != 3 || ar.Entries[0].Name != "Grimble" || ar.Entries[2].Name != "Goblin" {
		t.Fatalf("entries sort wrong: %+v", ar.Entries)
	}
	ar, err = st.AdvanceInitiative(ctx, room.ID)
	if err != nil || ar.CurrentEntryID != ar.Entries[0].ID || ar.Round != 1 {
		t.Fatalf("first advance: %+v err=%v", ar, err)
	}
	st.AdvanceInitiative(ctx, room.ID)
	st.AdvanceInitiative(ctx, room.ID)
	ar, _ = st.AdvanceInitiative(ctx, room.ID) // wraps to Grimble
	if ar.Round != 2 || ar.CurrentEntryID != ar.Entries[0].ID {
		t.Fatalf("wrap should bump round: round=%d", ar.Round)
	}
	if gone, _ := st.RemoveInitiativeEntry(ctx, ar.ID, "goblin"); !gone {
		t.Fatal("case-insensitive remove failed")
	}
	er, err := st.EndInitiative(ctx, room.ID)
	if err != nil || er.Active {
		t.Fatalf("end: %+v err=%v", er, err)
	}
	if _, ok, _ := st.ActiveInitiative(ctx, room.ID); ok {
		t.Fatal("round should be inactive after end")
	}
}

func TestMentionedUserIDs(t *testing.T) {
	members := []Member{
		{UserID: "u1", DisplayName: "Michael Stufflebeam"},
		{UserID: "u2", DisplayName: "Claude Codetest"},
		{UserID: "u3", DisplayName: "Claude Opus"}, // first-word collision with u2
	}
	got := MentionedUserIDs("hey @michael and @ClaudeCodetest, look at this", "u9", members)
	if len(got) != 2 || got[0] != "u1" || got[1] != "u2" {
		t.Fatalf("got %v, want [u1 u2]", got)
	}
	// Ambiguous first word matches nobody.
	if got := MentionedUserIDs("@claude what do you think?", "u9", members); len(got) != 0 {
		t.Fatalf("ambiguous first word must not match, got %v", got)
	}
	// The sender never mentions themselves.
	if got := MentionedUserIDs("@michael note to self", "u1", members); len(got) != 0 {
		t.Fatalf("self-mention must not notify, got %v", got)
	}
	// No @ tokens → nil fast path.
	if got := MentionedUserIDs("michael stufflebeam is here", "u9", members); got != nil {
		t.Fatalf("plain names without @ must not notify, got %v", got)
	}
}
