package presence

import (
	"sort"
	"sync"
	"testing"
)

func TestTrackerJoinAndRoster(t *testing.T) {
	tr := NewTracker()
	tr.Join("r1", "alice", Human)
	tr.Join("r1", "bob", Human)
	tr.Join("r2", "eve", Persona)

	r1 := tr.Roster("r1")
	if len(r1) != 2 {
		t.Fatalf("expected 2 participants in r1, got %d", len(r1))
	}

	r2 := tr.Roster("r2")
	if len(r2) != 1 {
		t.Fatalf("expected 1 participant in r2, got %d", len(r2))
	}
	if r2[0].ID != "eve" || r2[0].Kind != Persona {
		t.Fatalf("unexpected participant in r2: %+v", r2[0])
	}
}

func TestTrackerRosterReturnsCopy(t *testing.T) {
	tr := NewTracker()
	tr.Join("r1", "alice", Human)

	r1a := tr.Roster("r1")
	if len(r1a) != 1 {
		t.Fatalf("expected 1 participant, got %d", len(r1a))
	}

	tr.Join("r1", "bob", Human)
	r1b := tr.Roster("r1")
	if len(r1a) != 1 {
		t.Fatalf("original slice should not have grown, got %d", len(r1a))
	}
	if len(r1b) != 2 {
		t.Fatalf("new slice should have 2 participants, got %d", len(r1b))
	}
}

func TestTrackerLeave(t *testing.T) {
	tr := NewTracker()
	tr.Join("r1", "alice", Human)
	tr.Leave("r1", "alice")

	r1 := tr.Roster("r1")
	if len(r1) != 0 {
		t.Fatalf("expected empty roster after leave, got %d", len(r1))
	}
}

func TestTrackerLeaveUnknownRoomIsSafe(t *testing.T) {
	tr := NewTracker()
	// Should not panic.
	tr.Leave("nonexistent", "nobody")
}

func TestTrackerSetThinking(t *testing.T) {
	tr := NewTracker()
	tr.Join("r1", "eve", Persona)
	tr.SetThinking("r1", "eve", true)

	r1 := tr.Roster("r1")
	if len(r1) != 1 {
		t.Fatalf("expected 1 participant, got %d", len(r1))
	}
	if !r1[0].Thinking {
		t.Fatalf("expected Thinking=true")
	}

	tr.SetThinking("r1", "eve", false)
	r1 = tr.Roster("r1")
	if r1[0].Thinking {
		t.Fatalf("expected Thinking=false")
	}
}

func TestTrackerSetKind(t *testing.T) {
	tr := NewTracker()
	tr.Join("r1", "alice", Human)
	tr.SetKind("r1", "alice", Persona)

	r1 := tr.Roster("r1")
	if r1[0].Kind != Persona {
		t.Fatalf("expected Kind=Persona, got %s", r1[0].Kind)
	}
}

func TestTrackerRosterEmptyRoom(t *testing.T) {
	tr := NewTracker()
	roster := tr.Roster("nonexistent")
	if roster != nil {
		t.Fatalf("expected nil roster for empty room, got %v", roster)
	}
}

func TestTrackerConcurrentStress(t *testing.T) {
	tr := NewTracker()
	const rooms = 10
	const participants = 50
	const iterations = 100

	var wg sync.WaitGroup

	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func(iter int) {
			defer wg.Done()
			for r := 0; r < rooms; r++ {
				roomID := string(rune('a' + r))
				for p := 0; p < participants; p++ {
					id := string(rune('A'+p%26)) + string(rune('0'+iter%10))
					tr.Join(roomID, id, Human)
					if p%2 == 0 {
						tr.SetThinking(roomID, id, true)
					}
				}
				for p := 0; p < participants; p++ {
					id := string(rune('A'+p%26)) + string(rune('0'+iter%10))
					tr.Leave(roomID, id)
				}
			}
		}(i)
	}

	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func(iter int) {
			defer wg.Done()
			for r := 0; r < rooms; r++ {
				roomID := string(rune('a' + r))
				roster := tr.Roster(roomID)
				// Just verify we can read without races.
				_ = len(roster)
			}
		}(i)
	}

	wg.Wait()

	// Verify all rooms are empty.
	for r := 0; r < rooms; r++ {
		roomID := string(rune('a' + r))
		roster := tr.Roster(roomID)
		if len(roster) != 0 {
			t.Fatalf("expected empty roster for room %s, got %d", roomID, len(roster))
		}
	}
}

func TestTrackerKindsAreDistinct(t *testing.T) {
	tr := NewTracker()
	tr.Join("r1", "alice", Human)
	tr.Join("r1", "bot", Persona)

	roster := tr.Roster("r1")
	sort.Slice(roster, func(i, j int) bool { return roster[i].ID < roster[j].ID })

	if len(roster) != 2 {
		t.Fatalf("expected 2 participants, got %d", len(roster))
	}
	if roster[0].Kind != Human {
		t.Fatalf("expected alice to be Human")
	}
	if roster[1].Kind != Persona {
		t.Fatalf("expected bot to be Persona")
	}
}
