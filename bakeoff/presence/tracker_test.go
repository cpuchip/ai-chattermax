package presence

import (
	"sync"
	"testing"
	"time"
)

func TestJoinAndRoster(t *testing.T) {
	tr := NewTracker()
	tr.Join("room1", "alice", Human)
	tr.Join("room1", "bob", Human)
	tr.Join("room1", "c3po", Persona)

	roster := tr.Roster("room1")
	if len(roster) != 3 {
		t.Fatalf("expected 3 participants, got %d", len(roster))
	}

	kinds := map[string]Kind{}
	for _, p := range roster {
		kinds[p.ID] = p.Kind
	}
	if kinds["alice"] != Human {
		t.Errorf("alice: expected Human, got %s", kinds["alice"])
	}
	if kinds["c3po"] != Persona {
		t.Errorf("c3po: expected Persona, got %s", kinds["c3po"])
	}
}

func TestRoomIsolation(t *testing.T) {
	tr := NewTracker()
	tr.Join("room1", "alice", Human)
	tr.Join("room2", "bob", Human)

	r1 := tr.Roster("room1")
	r2 := tr.Roster("room2")

	if len(r1) != 1 || r1[0].ID != "alice" {
		t.Errorf("room1 roster: expected [alice], got %v", r1)
	}
	if len(r2) != 1 || r2[0].ID != "bob" {
		t.Errorf("room2 roster: expected [bob], got %v", r2)
	}

	// Empty room
	r3 := tr.Roster("nonexistent")
	if len(r3) != 0 {
		t.Errorf("nonexistent room: expected empty, got %d", len(r3))
	}
}

func TestLeave(t *testing.T) {
	tr := NewTracker()
	tr.Join("room1", "alice", Human)
	tr.Join("room1", "bob", Human)

	tr.Leave("room1", "alice")

	roster := tr.Roster("room1")
	if len(roster) != 1 || roster[0].ID != "bob" {
		t.Errorf("after leave: expected [bob], got %v", roster)
	}
}

func TestLeaveNonexistent(t *testing.T) {
	tr := NewTracker()
	// Leave from nonexistent room — no panic
	tr.Leave("room1", "nobody")

	tr.Join("room1", "alice", Human)
	// Leave someone who isn't in the room
	tr.Leave("room1", "nobody")

	if len(tr.Roster("room1")) != 1 {
		t.Fatal("leave of nonexistent participant should not affect others")
	}
}

func TestLeaveEmptiesRoom(t *testing.T) {
	tr := NewTracker()
	tr.Join("room1", "alice", Human)
	tr.Leave("room1", "alice")

	rooms := tr.RoomIDs()
	if len(rooms) != 0 {
		t.Errorf("expected no rooms after last participant leaves, got %v", rooms)
	}
}

func TestSetThinking(t *testing.T) {
	tr := NewTracker()
	tr.Join("room1", "c3po", Persona)

	// Initial: Thinking is false
	roster := tr.Roster("room1")
	if roster[0].Thinking {
		t.Error("expected Thinking to be false initially")
	}

	// Set thinking true
	tr.SetThinking("room1", "c3po", true)
	roster = tr.Roster("room1")
	if !roster[0].Thinking {
		t.Error("expected Thinking to be true after SetThinking(true)")
	}

	// Set thinking false
	tr.SetThinking("room1", "c3po", false)
	roster = tr.Roster("room1")
	if roster[0].Thinking {
		t.Error("expected Thinking to be false after SetThinking(false)")
	}
}

func TestSetThinkingHumanNoOp(t *testing.T) {
	tr := NewTracker()
	tr.Join("room1", "alice", Human)

	// Thinking on a human should be no-op
	tr.SetThinking("room1", "alice", true)
	roster := tr.Roster("room1")
	if roster[0].Thinking {
		t.Error("SetThinking on Human should be no-op, but Thinking is true")
	}
}

func TestSetThinkingNonexistent(t *testing.T) {
	tr := NewTracker()
	// No panic
	tr.SetThinking("room1", "nobody", true)
}

func TestSetStatus(t *testing.T) {
	tr := NewTracker()
	tr.Join("room1", "alice", Human)

	// Default status is online
	roster := tr.Roster("room1")
	if roster[0].Status != StatusOnline {
		t.Errorf("expected online, got %s", roster[0].Status)
	}

	tr.SetStatus("room1", "alice", StatusAway)
	roster = tr.Roster("room1")
	if roster[0].Status != StatusAway {
		t.Errorf("expected away, got %s", roster[0].Status)
	}

	tr.SetStatus("room1", "alice", StatusBusy)
	roster = tr.Roster("room1")
	if roster[0].Status != StatusBusy {
		t.Errorf("expected busy, got %s", roster[0].Status)
	}
}

func TestSetStatusNonexistent(t *testing.T) {
	tr := NewTracker()
	// No panic
	tr.SetStatus("room1", "nobody", StatusAway)
}

func TestKindHelpers(t *testing.T) {
	tests := []struct {
		k       Kind
		isHuman bool
		isPers  bool
	}{
		{Human, true, false},
		{Persona, false, true},
		{Kind("unknown"), false, false},
	}
	for _, tt := range tests {
		if tt.k.IsHuman() != tt.isHuman {
			t.Errorf("Kind(%q).IsHuman() = %v, want %v", tt.k, tt.k.IsHuman(), tt.isHuman)
		}
		if tt.k.IsPersona() != tt.isPers {
			t.Errorf("Kind(%q).IsPersona() = %v, want %v", tt.k, tt.k.IsPersona(), tt.isPers)
		}
	}
}

func TestRejoinUpdatesKindAndJoinedAt(t *testing.T) {
	tr := NewTracker()
	tr.Join("room1", "c3po", Human)

	roster := tr.Roster("room1")
	firstJoined := roster[0].JoinedAt

	// Small sleep to ensure JoinedAt differs
	time.Sleep(2 * time.Millisecond)

	tr.Join("room1", "c3po", Persona)
	roster = tr.Roster("room1")
	if roster[0].Kind != Persona {
		t.Errorf("expected Kind=Persona after rejoin, got %s", roster[0].Kind)
	}
	if !roster[0].JoinedAt.After(firstJoined) {
		t.Error("expected JoinedAt to be updated on rejoin")
	}
	// Rejoin resets Thinking and Status
	if roster[0].Thinking {
		t.Error("expected Thinking=false after rejoin")
	}
	if roster[0].Status != StatusOnline {
		t.Errorf("expected Status=online after rejoin, got %s", roster[0].Status)
	}
}

func TestRoomIDs(t *testing.T) {
	tr := NewTracker()
	tr.Join("a", "1", Human)
	tr.Join("b", "2", Human)
	tr.Join("c", "3", Human)

	rooms := tr.RoomIDs()
	if len(rooms) != 3 {
		t.Fatalf("expected 3 rooms, got %d", len(rooms))
	}

	seen := map[string]bool{}
	for _, r := range rooms {
		seen[r] = true
	}
	if !seen["a"] || !seen["b"] || !seen["c"] {
		t.Errorf("expected rooms a,b,c, got %v", rooms)
	}
}

// ---- Concurrent tests ----

func TestConcurrentJoinLeave(t *testing.T) {
	tr := NewTracker()
	const numGoroutines = 100

	var wg sync.WaitGroup
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := string(rune('A' + i%26))
			kind := Human
			if i%2 == 0 {
				kind = Persona
			}
			tr.Join("room1", id, kind)
		}(i)
	}
	wg.Wait()

	roster := tr.Roster("room1")
	if len(roster) > 26 {
		t.Errorf("expected at most 26 unique participants (A-Z), got %d", len(roster))
	}

	// Now leave concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := string(rune('A' + i%26))
			tr.Leave("room1", id)
		}(i)
	}
	wg.Wait()

	roster = tr.Roster("room1")
	if len(roster) != 0 {
		t.Errorf("expected 0 participants after all leaves, got %d", len(roster))
	}
}

func TestConcurrentMultiRoomOperations(t *testing.T) {
	tr := NewTracker()
	const numRooms = 20
	const clientsPerRoom = 50

	var wg sync.WaitGroup
	for r := 0; r < numRooms; r++ {
		roomID := string(rune('A' + r))
		for c := 0; c < clientsPerRoom; c++ {
			id := roomID + "-" + string(rune('0'+c%10)) + string(rune('a'+c/10))
			kind := Human
			if c%3 == 0 {
				kind = Persona
			}
			wg.Add(1)
			go func(rid, cid string, k Kind) {
				defer wg.Done()
				tr.Join(rid, cid, k)
			}(roomID, id, kind)
		}
	}
	wg.Wait()

	// Verify all rooms have participants
	rooms := tr.RoomIDs()
	if len(rooms) != numRooms {
		t.Errorf("expected %d rooms, got %d", numRooms, len(rooms))
	}

	// Concurrent Roster + SetThinking + SetStatus
	for r := 0; r < numRooms; r++ {
		roomID := string(rune('A' + r))
		for c := 0; c < clientsPerRoom; c++ {
			id := roomID + "-" + string(rune('0'+c%10)) + string(rune('a'+c/10))
			wg.Add(1)
			go func(rid, cid string) {
				defer wg.Done()
				tr.SetThinking(rid, cid, true)
				tr.SetStatus(rid, cid, StatusAway)
			}(roomID, id)
			// Also read concurrently
			if c%10 == 0 {
				wg.Add(1)
				go func(rid string) {
					defer wg.Done()
					_ = tr.Roster(rid)
				}(roomID)
			}
		}
	}
	wg.Wait()
}

func TestConcurrentRosterReads(t *testing.T) {
	tr := NewTracker()
	for i := 0; i < 100; i++ {
		tr.Join("room1", "client-"+string(rune('0'+i%10))+string(rune('A'+i/10)), Human)
	}

	const numReaders = 50
	var wg sync.WaitGroup
	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			roster := tr.Roster("room1")
			if len(roster) == 0 {
				t.Error("roster should not be empty")
			}
		}()
	}
	wg.Wait()
}