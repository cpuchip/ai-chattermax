package presence

import (
	"fmt"
	"sync"
	"testing"
)

func TestTrackerJoinLeave(t *testing.T) {
	tr := New()
	tr.Join("room-1", "alice", KindHuman)
	tr.Join("room-1", "bot-1", KindPersona)

	roster := tr.Roster("room-1")
	if len(roster) != 2 {
		t.Fatalf("expected 2 participants, got %d", len(roster))
	}

	ids := []string{roster[0].ID, roster[1].ID}
	if ids[0] != "alice" || ids[1] != "bot-1" {
		t.Fatalf("unexpected roster ordering: %v", ids)
	}

	tr.Leave("room-1", "alice")
	roster = tr.Roster("room-1")
	if len(roster) != 1 {
		t.Fatalf("expected 1 participant after leave, got %d", len(roster))
	}
	if roster[0].ID != "bot-1" {
		t.Fatalf("expected bot-1 remaining, got %s", roster[0].ID)
	}
}

func TestTrackerRoomIsolation(t *testing.T) {
	tr := New()
	tr.Join("room-a", "alice", KindHuman)
	tr.Join("room-b", "bob", KindHuman)

	if len(tr.Roster("room-a")) != 1 {
		t.Fatal("room-a should have 1 participant")
	}
	if len(tr.Roster("room-b")) != 1 {
		t.Fatal("room-b should have 1 participant")
	}
	if len(tr.Roster("room-c")) != 0 {
		t.Fatal("room-c should be empty")
	}
}

func TestTrackerSetThinking(t *testing.T) {
	tr := New()
	tr.Join("room-1", "bot-1", KindPersona)
	tr.SetThinking("room-1", "bot-1", true)

	roster := tr.Roster("room-1")
	if len(roster) != 1 {
		t.Fatal("expected 1 participant")
	}
	if !roster[0].Thinking {
		t.Fatal("expected Thinking=true")
	}

	tr.SetThinking("room-1", "bot-1", false)
	roster = tr.Roster("room-1")
	if roster[0].Thinking {
		t.Fatal("expected Thinking=false")
	}
}

func TestTrackerTouch(t *testing.T) {
	tr := New()
	tr.Join("room-1", "alice", KindHuman)
	before := tr.Roster("room-1")[0].LastSeen

	tr.Touch("room-1", "alice")
	after := tr.Roster("room-1")[0].LastSeen
	if !after.After(before) {
		t.Fatal("expected LastSeen to advance after Touch")
	}
}

func TestTrackerRosterImmutability(t *testing.T) {
	tr := New()
	tr.Join("room-1", "alice", KindHuman)

	roster := tr.Roster("room-1")
	roster[0].ID = "tampered"

	roster2 := tr.Roster("room-1")
	if roster2[0].ID != "alice" {
		t.Fatal("mutating returned roster should not affect tracker state")
	}
}

func TestTrackerDoubleJoin(t *testing.T) {
	tr := New()
	tr.Join("room-1", "alice", KindHuman)
	before := tr.Roster("room-1")[0].JoinedAt

	tr.Join("room-1", "alice", KindHuman) // duplicate
	roster := tr.Roster("room-1")
	if len(roster) != 1 {
		t.Fatalf("double-join should not create duplicate, got %d", len(roster))
	}
	if !roster[0].JoinedAt.Equal(before) {
		t.Fatal("double-join should not reset JoinedAt")
	}
}

func TestTrackerLeaveUnknown(t *testing.T) {
	tr := New()
	// Should not panic.
	tr.Leave("no-room", "nobody")
	tr.Leave("room-1", "nobody")
	tr.SetThinking("room-1", "nobody", true)
	tr.Touch("room-1", "nobody")
}

func TestTrackerKindString(t *testing.T) {
	if KindHuman.String() != "human" {
		t.Fatalf("unexpected human string: %s", KindHuman.String())
	}
	if KindPersona.String() != "persona" {
		t.Fatalf("unexpected persona string: %s", KindPersona.String())
	}
	if Kind(99).String() != "unknown" {
		t.Fatalf("unexpected unknown string: %s", Kind(99).String())
	}
}

func TestTrackerConcurrent(t *testing.T) {
	tr := New()
	const goroutines = 200

	var wg sync.WaitGroup

	// Each goroutine joins a unique participant, mutates state,
	// and leaves it.  The room should be empty at the end.
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			pid := fmt.Sprintf("p-%d", id)
			tr.Join("room", pid, KindHuman)
			tr.Touch("room", pid)
			tr.SetThinking("room", pid, true)
			tr.SetThinking("room", pid, false)
			tr.Leave("room", pid)
		}(i)
	}

	// Concurrent roster reads while the churn happens.
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				_ = tr.Roster("room")
			}
		}()
	}

	wg.Wait()

	roster := tr.Roster("room")
	if len(roster) != 0 {
		t.Fatalf("expected empty room after all leaves, got %d", len(roster))
	}
}
