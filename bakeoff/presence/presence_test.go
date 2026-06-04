package presence

import (
	"slices"
	"sync"
	"testing"
)

func TestJoinAndRoster(t *testing.T) {
	tr := &Tracker{}

	tr.Join("room-a", "alice", KindHuman)
	tr.Join("room-a", "bob", KindHuman)
	tr.Join("room-a", "droid", KindPersona)

	r := tr.Roster("room-a")
	if len(r) != 3 {
		t.Fatalf("roster len = %d, want 3", len(r))
	}

	// Roster order is non-deterministic; check by ID.
	byID := make(map[string]RosterEntry, len(r))
	for _, e := range r {
		byID[e.ID] = e
	}

	if e, ok := byID["alice"]; !ok || e.Kind != KindHuman {
		t.Errorf("alice: kind=%v, want KindHuman", e.Kind)
	}
	if e, ok := byID["bob"]; !ok || e.Kind != KindHuman {
		t.Errorf("bob: kind=%v, want KindHuman", e.Kind)
	}
	if e, ok := byID["droid"]; !ok || e.Kind != KindPersona {
		t.Errorf("droid: kind=%v, want KindPersona", e.Kind)
	}
}

func TestRosterEmptyRoom(t *testing.T) {
	tr := &Tracker{}
	r := tr.Roster("nonexistent")
	if r != nil {
		t.Fatalf("expected nil roster for empty room, got %v", r)
	}
}

func TestJoinIdempotent(t *testing.T) {
	tr := &Tracker{}

	tr.Join("room-a", "alice", KindHuman)
	tr.SetStatus("room-a", "alice", "away")
	tr.SetThinking("room-a", "alice", true)

	// Re-join: should update kind but preserve other fields.
	tr.Join("room-a", "alice", KindPersona)

	r := tr.Roster("room-a")
	if len(r) != 1 {
		t.Fatalf("roster len = %d, want 1", len(r))
	}
	e := r[0]
	if e.Kind != KindPersona {
		t.Errorf("kind = %v, want KindPersona", e.Kind)
	}
	if e.Status != "away" {
		t.Errorf("status = %q, want \"away\" (preserved)", e.Status)
	}
	if !e.Thinking {
		t.Error("thinking should remain true")
	}
}

func TestLeave(t *testing.T) {
	t.Run("leave present participant", func(t *testing.T) {
		tr := &Tracker{}
		tr.Join("room-a", "alice", KindHuman)
		tr.Join("room-a", "bob", KindHuman)
		tr.Leave("room-a", "alice")

		r := tr.Roster("room-a")
		if len(r) != 1 {
			t.Fatalf("roster len = %d, want 1", len(r))
		}
		if r[0].ID != "bob" {
			t.Errorf("remaining participant = %s, want bob", r[0].ID)
		}
	})

	t.Run("leave non-present is no-op", func(t *testing.T) {
		tr := &Tracker{}
		tr.Join("room-a", "alice", KindHuman)
		tr.Leave("room-a", "nobody") // no-op
		r := tr.Roster("room-a")
		if len(r) != 1 {
			t.Errorf("roster len = %d, want 1 (no-op leave)", len(r))
		}
	})

	t.Run("leave from non-existent room is no-op", func(t *testing.T) {
		tr := &Tracker{}
		tr.Leave("void", "nobody") // must not panic
	})

	t.Run("leave last participant cleans up room", func(t *testing.T) {
		tr := &Tracker{}
		tr.Join("room-a", "alice", KindHuman)
		tr.Leave("room-a", "alice")
		r := tr.Roster("room-a")
		if r != nil {
			t.Errorf("roster after last leave = %v, want nil", r)
		}
	})
}

func TestSetThinkingAndStatus(t *testing.T) {
	t.Run("set thinking", func(t *testing.T) {
		tr := &Tracker{}
		tr.Join("room-a", "droid", KindPersona)
		tr.SetThinking("room-a", "droid", true)
		r := tr.Roster("room-a")
		if !r[0].Thinking {
			t.Error("thinking should be true")
		}
		tr.SetThinking("room-a", "droid", false)
		r = tr.Roster("room-a")
		if r[0].Thinking {
			t.Error("thinking should be false")
		}
	})

	t.Run("set status", func(t *testing.T) {
		tr := &Tracker{}
		tr.Join("room-a", "alice", KindHuman)
		tr.SetStatus("room-a", "alice", "idle")
		r := tr.Roster("room-a")
		if r[0].Status != "idle" {
			t.Errorf("status = %q, want \"idle\"", r[0].Status)
		}
	})

	t.Run("set thinking on non-present is no-op", func(t *testing.T) {
		tr := &Tracker{}
		tr.SetThinking("room-a", "nobody", true) // must not panic
	})

	t.Run("set status on non-present is no-op", func(t *testing.T) {
		tr := &Tracker{}
		tr.SetStatus("room-a", "nobody", "away") // must not panic
	})
}

func TestRosterReturnsCopy(t *testing.T) {
	tr := &Tracker{}
	tr.Join("room-a", "alice", KindHuman)

	r1 := tr.Roster("room-a")
	// Mutate the returned slice
	r1 = append(r1, RosterEntry{ID: "injected"})
	// Clear the slice
	for i := range r1 {
		r1[i] = RosterEntry{}
	}

	// Original roster must be unaffected
	r2 := tr.Roster("room-a")
	if len(r2) != 1 || r2[0].ID != "alice" {
		t.Errorf("roster mutated by caller: %+v", r2)
	}
}

func TestRoomIsolation(t *testing.T) {
	tr := &Tracker{}
	tr.Join("room-a", "alice", KindHuman)
	tr.Join("room-b", "bob", KindHuman)

	if r := tr.Roster("room-a"); len(r) != 1 || r[0].ID != "alice" {
		t.Errorf("room-a roster: %+v", r)
	}
	if r := tr.Roster("room-b"); len(r) != 1 || r[0].ID != "bob" {
		t.Errorf("room-b roster: %+v", r)
	}

	// Leave from room-a shouldn't affect room-b.
	tr.Leave("room-a", "alice")
	if r := tr.Roster("room-b"); len(r) != 1 || r[0].ID != "bob" {
		t.Errorf("room-b after room-a leave: %+v", r)
	}
}

func TestConcurrent(t *testing.T) {
	tr := &Tracker{}

	const numRooms = 4
	const numPerRoom = 25
	const numOps = 200

	var wg sync.WaitGroup

	// Concurrent joins
	for room := range numRooms {
		for i := range numPerRoom {
			wg.Add(1)
			go func(r, n int) {
				defer wg.Done()
				tr.Join(roomName(r), idFor(n), KindHuman)
			}(room, i)
		}
	}

	// Concurrent rosters while joins are in flight
	for range numOps {
		wg.Add(1)
		go func(r int) {
			defer wg.Done()
			_ = tr.Roster(roomName(r))
		}(0)
	}

	// Concurrent thinking toggles
	for room := range numRooms {
		for i := range numPerRoom {
			wg.Add(1)
			go func(r, n int) {
				defer wg.Done()
				tr.SetThinking(roomName(r), idFor(n), n%2 == 0)
			}(room, i)
		}
	}

	wg.Wait()

	// After all joins, each room should have numPerRoom participants.
	for room := range numRooms {
		r := tr.Roster(roomName(room))
		if len(r) != numPerRoom {
			t.Errorf("room %d: roster len = %d, want %d", room, len(r), numPerRoom)
		}
	}

	// Verify snapshot integrity: every entry should have a valid kind.
	all := tr.Roster(roomName(0))
	for _, e := range all {
		if e.Kind != KindHuman && e.Kind != KindPersona {
			t.Errorf("entry %s has invalid kind %v", e.ID, e.Kind)
		}
	}
}

func TestConcurrentLeave(t *testing.T) {
	tr := &Tracker{}

	const numParticipants = 50
	for i := range numParticipants {
		tr.Join("room", idFor(i), KindHuman)
	}

	var wg sync.WaitGroup
	for i := range numParticipants {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			tr.Leave("room", idFor(n))
		}(i)
	}
	wg.Wait()

	r := tr.Roster("room")
	if len(r) != 0 {
		t.Errorf("after concurrent leaves, roster len = %d, want 0", len(r))
	}
}

func TestKindString(t *testing.T) {
	if s := KindHuman.String(); s != "human" {
		t.Errorf("KindHuman.String() = %q, want \"human\"", s)
	}
	if s := KindPersona.String(); s != "persona" {
		t.Errorf("KindPersona.String() = %q, want \"persona\"", s)
	}
	if s := Kind(99).String(); s != "unknown" {
		t.Errorf("Kind(99).String() = %q, want \"unknown\"", s)
	}
}

func TestRosterOrderStable(t *testing.T) {
	// Ensures roster returns consistent snapshots (no accidental mutation
	// of the internal map during iteration).
	tr := &Tracker{}
	tr.Join("room", "alice", KindHuman)
	tr.Join("room", "bob", KindHuman)

	r1 := tr.Roster("room")
	r2 := tr.Roster("room")

	// Extract IDs from both snapshots and sort for comparison.
	ids1 := make([]string, len(r1))
	for i, e := range r1 {
		ids1[i] = e.ID
	}
	ids2 := make([]string, len(r2))
	for i, e := range r2 {
		ids2[i] = e.ID
	}
	slices.Sort(ids1)
	slices.Sort(ids2)

	if !slices.Equal(ids1, ids2) {
		t.Error("roster snapshots differ across calls")
	}
}

// --- helpers ---

func roomName(n int) string {
	return string(rune('A' + n))
}

func idFor(n int) string {
	return string(rune('a' + n))
}
