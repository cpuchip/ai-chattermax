package presence

import (
	"fmt"
	"sync"
	"testing"
)

func TestTracker_JoinAndRoster(t *testing.T) {
	tr := NewTracker()
	room := RoomID("room-1")

	tr.Join(room, Participant{ID: "p1", Name: "Alice", Kind: HumanKind, Status: Online})
	tr.Join(room, Participant{ID: "p2", Name: "Claude", Kind: PersonaKind, Status: Online})

	r := tr.Roster(room)
	if len(r.Participants) != 2 {
		t.Fatalf("expected 2 participants, got %d", len(r.Participants))
	}

	byID := make(map[ID]Participant)
	for _, p := range r.Participants {
		byID[p.ID] = p
	}

	if p1, ok := byID["p1"]; !ok {
		t.Fatal("missing participant p1")
	} else if p1.Name != "Alice" || p1.Kind != HumanKind || p1.Status != Online || p1.Thinking {
		t.Errorf("p1 mismatch: %+v", p1)
	}

	if p2, ok := byID["p2"]; !ok {
		t.Fatal("missing participant p2")
	} else if p2.Name != "Claude" || p2.Kind != PersonaKind || p2.Status != Online || p2.Thinking {
		t.Errorf("p2 mismatch: %+v", p2)
	}
}

func TestTracker_Leave(t *testing.T) {
	tr := NewTracker()
	room := RoomID("room-1")

	tr.Join(room, Participant{ID: "p1", Name: "Alice", Kind: HumanKind, Status: Online})
	tr.Leave(room, "p1")

	r := tr.Roster(room)
	if len(r.Participants) != 0 {
		t.Fatalf("expected 0 participants after leave, got %d", len(r.Participants))
	}
}

func TestTracker_SetStatus(t *testing.T) {
	tr := NewTracker()
	room := RoomID("room-1")

	tr.Join(room, Participant{ID: "p1", Name: "Alice", Kind: HumanKind, Status: Online})
	tr.SetStatus(room, "p1", Idle)

	r := tr.Roster(room)
	if len(r.Participants) != 1 {
		t.Fatalf("expected 1 participant, got %d", len(r.Participants))
	}
	if r.Participants[0].Status != Idle {
		t.Errorf("expected status idle, got %s", r.Participants[0].Status)
	}
}

func TestTracker_SetThinking_Persona(t *testing.T) {
	tr := NewTracker()
	room := RoomID("room-1")

	tr.Join(room, Participant{ID: "p1", Name: "Claude", Kind: PersonaKind, Status: Online})
	tr.SetThinking(room, "p1", true)

	r := tr.Roster(room)
	if len(r.Participants) != 1 {
		t.Fatalf("expected 1 participant, got %d", len(r.Participants))
	}
	if !r.Participants[0].Thinking {
		t.Error("expected Thinking=true for persona")
	}
}

func TestTracker_SetThinking_HumanNoOp(t *testing.T) {
	tr := NewTracker()
	room := RoomID("room-1")

	tr.Join(room, Participant{ID: "p1", Name: "Alice", Kind: HumanKind, Status: Online, Thinking: false})
	tr.SetThinking(room, "p1", true)

	r := tr.Roster(room)
	if len(r.Participants) != 1 {
		t.Fatalf("expected 1 participant, got %d", len(r.Participants))
	}
	if r.Participants[0].Thinking {
		t.Error("expected Thinking=false for human; SetThinking should be a no-op")
	}
}

func TestTracker_RoomIsolation(t *testing.T) {
	tr := NewTracker()
	roomA := RoomID("room-a")
	roomB := RoomID("room-b")

	tr.Join(roomA, Participant{ID: "p1", Name: "Alice", Kind: HumanKind, Status: Online})
	tr.Join(roomB, Participant{ID: "p2", Name: "Bob", Kind: HumanKind, Status: Online})

	rA := tr.Roster(roomA)
	rB := tr.Roster(roomB)

	if len(rA.Participants) != 1 {
		t.Fatalf("expected room-a to have 1 participant, got %d", len(rA.Participants))
	}
	if rA.Participants[0].ID != "p1" {
		t.Errorf("expected room-a participant p1, got %s", rA.Participants[0].ID)
	}

	if len(rB.Participants) != 1 {
		t.Fatalf("expected room-b to have 1 participant, got %d", len(rB.Participants))
	}
	if rB.Participants[0].ID != "p2" {
		t.Errorf("expected room-b participant p2, got %s", rB.Participants[0].ID)
	}

	// Empty room should return empty roster
	empty := tr.Roster(RoomID("room-c"))
	if len(empty.Participants) != 0 {
		t.Fatalf("expected empty room to have 0 participants, got %d", len(empty.Participants))
	}
}

func TestTracker_ConcurrentOps(t *testing.T) {
	tr := NewTracker()
	room := RoomID("room-1")

	const n = 50
	var wg sync.WaitGroup
	wg.Add(n * 4) // join + status + thinking + roster

	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			id := ID(fmt.Sprintf("p%d", i))
			tr.Join(room, Participant{
				ID:     id,
				Name:   string(id),
				Kind:   PersonaKind,
				Status: Online,
			})
		}(i)

		go func(i int) {
			defer wg.Done()
			id := ID(fmt.Sprintf("p%d", i))
			tr.SetStatus(room, id, Idle)
		}(i)

		go func(i int) {
			defer wg.Done()
			id := ID(fmt.Sprintf("p%d", i))
			tr.SetThinking(room, id, i%2 == 0)
		}(i)

		go func() {
			defer wg.Done()
			_ = tr.Roster(room)
		}()
	}

	wg.Wait()

	r := tr.Roster(room)
	if len(r.Participants) != n {
		t.Fatalf("expected %d participants after concurrent ops, got %d", n, len(r.Participants))
	}

	for _, p := range r.Participants {
		if p.Kind != PersonaKind {
			t.Errorf("expected persona kind, got %s", p.Kind)
		}
		if p.Status != Online && p.Status != Idle {
			t.Errorf("unexpected status for %s: %s", p.ID, p.Status)
		}
	}
}

func TestTracker_ConcurrentJoinLeave(t *testing.T) {
	tr := NewTracker()
	room := RoomID("room-1")

	const n = 50
	var wg sync.WaitGroup
	wg.Add(n * 2)

	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			id := ID(fmt.Sprintf("p%d", i))
			tr.Join(room, Participant{
				ID:     id,
				Name:   string(id),
				Kind:   HumanKind,
				Status: Online,
			})
		}(i)

		go func(i int) {
			defer wg.Done()
			id := ID(fmt.Sprintf("p%d", i))
			tr.Leave(room, id)
		}(i)
	}

	wg.Wait()

	// After concurrent join+leave, the key assertion is that it doesn't panic or race.
	_ = tr.Roster(room)
}
