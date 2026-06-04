package presence

import (
	"fmt"
	"sync"
	"testing"
)

func TestTracker_JoinAndRoster(t *testing.T) {
	tr := NewTracker()
	tr.Join("room-a", "u1", Human)
	tr.Join("room-a", "u2", Persona)

	roster := tr.Roster("room-a")
	if len(roster) != 2 {
		t.Fatalf("expected 2 participants, got %d", len(roster))
	}
}

func TestTracker_CrossRoomIsolation(t *testing.T) {
	tr := NewTracker()
	tr.Join("room-a", "u1", Human)

	roster := tr.Roster("room-b")
	if len(roster) != 0 {
		t.Fatalf("expected 0 participants in room-b, got %d", len(roster))
	}
}

func TestTracker_Leave(t *testing.T) {
	tr := NewTracker()
	tr.Join("room-a", "u1", Human)
	tr.Leave("room-a", "u1")

	roster := tr.Roster("room-a")
	if len(roster) != 0 {
		t.Fatalf("expected 0 participants after leave, got %d", len(roster))
	}
}

func TestTracker_SetThinking(t *testing.T) {
	tr := NewTracker()
	tr.Join("room-a", "bot1", Persona)
	tr.SetThinking("room-a", "bot1", true)

	roster := tr.Roster("room-a")
	if len(roster) != 1 {
		t.Fatalf("expected 1 participant, got %d", len(roster))
	}
	if !roster[0].Thinking {
		t.Fatalf("expected Thinking=true, got false")
	}
}

func TestTracker_RosterDefensiveCopy(t *testing.T) {
	tr := NewTracker()
	tr.Join("room-a", "u1", Human)

	roster := tr.Roster("room-a")
	roster[0].ID = "tampered"

	roster2 := tr.Roster("room-a")
	if roster2[0].ID != "u1" {
		t.Fatalf("expected ID to remain 'u1', got %q", roster2[0].ID)
	}
}

func TestTracker_ConcurrentStress(t *testing.T) {
	tr := NewTracker()
	const (
		goroutines = 50
		rooms      = 5
		opsPerG    = 100
	)

	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for j := 0; j < opsPerG; j++ {
				roomID := fmt.Sprintf("room-%d", j%rooms)
				id := fmt.Sprintf("user-%d-%d", g, j)
				tr.Join(roomID, id, Human)
				_ = tr.Roster(roomID)
				tr.SetThinking(roomID, id, j%2 == 0)
				tr.Leave(roomID, id)
			}
		}(i)
	}
	wg.Wait()
}
