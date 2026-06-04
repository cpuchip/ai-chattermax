package presence

import (
	"sort"
	"sync"
	"testing"
)

func TestTracker_TableDriven(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		run      func(tr *Tracker)
		roomID   string
		wantIDs  []string
		wantKind map[string]Kind
	}{
		{
			name: "join adds participant to room",
			run: func(tr *Tracker) {
				tr.Join("r1", "alice", KindHuman)
			},
			roomID:   "r1",
			wantIDs:  []string{"alice"},
			wantKind: map[string]Kind{"alice": KindHuman},
		},
		{
			name: "multiple participants in one room",
			run: func(tr *Tracker) {
				tr.Join("lobby", "alice", KindHuman)
				tr.Join("lobby", "bot1", KindPersona)
				tr.Join("lobby", "bob", KindHuman)
			},
			roomID:  "lobby",
			wantIDs: []string{"alice", "bob", "bot1"},
			wantKind: map[string]Kind{
				"alice": KindHuman,
				"bob":   KindHuman,
				"bot1":  KindPersona,
			},
		},
		{
			name: "roster isolation between rooms",
			run: func(tr *Tracker) {
				tr.Join("a", "alice", KindHuman)
				tr.Join("b", "bob", KindHuman)
			},
			roomID:   "a",
			wantIDs:  []string{"alice"},
			wantKind: map[string]Kind{"alice": KindHuman},
		},
		{
			name: "leave removes participant",
			run: func(tr *Tracker) {
				tr.Join("r", "alice", KindHuman)
				tr.Join("r", "bob", KindHuman)
				tr.Leave("r", "alice")
			},
			roomID:   "r",
			wantIDs:  []string{"bob"},
			wantKind: map[string]Kind{"bob": KindHuman},
		},
		{
			name: "duplicate join is idempotent",
			run: func(tr *Tracker) {
				tr.Join("r", "alice", KindHuman)
				tr.Join("r", "alice", KindHuman)
			},
			roomID:   "r",
			wantIDs:  []string{"alice"},
			wantKind: map[string]Kind{"alice": KindHuman},
		},
		{
			name: "leave non-member is no-op",
			run: func(tr *Tracker) {
				tr.Join("r", "alice", KindHuman)
				tr.Leave("r", "stranger")
			},
			roomID:   "r",
			wantIDs:  []string{"alice"},
			wantKind: map[string]Kind{"alice": KindHuman},
		},
		{
			name: "roster of empty room returns nil",
			run: func(tr *Tracker) {
				// do nothing
			},
			roomID:  "empty",
			wantIDs: nil,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tr := NewTracker()
			tc.run(tr)

			roster := tr.Roster(tc.roomID)
			if tc.wantIDs == nil {
				if roster != nil {
					t.Errorf("expected nil roster, got %v", roster)
				}
				return
			}

			if len(roster) != len(tc.wantIDs) {
				t.Errorf("roster length: got %d, want %d", len(roster), len(tc.wantIDs))
			}

			ids := make([]string, len(roster))
			for i, p := range roster {
				ids[i] = p.ID
			}
			sort.Strings(ids)
			sort.Strings(tc.wantIDs)

			for i, id := range ids {
				if i >= len(tc.wantIDs) || id != tc.wantIDs[i] {
					t.Errorf("roster IDs mismatch at %d: got %s, want %s", i, id, tc.wantIDs[i])
				}
			}

			for _, p := range roster {
				if want, ok := tc.wantKind[p.ID]; ok && p.Kind != want {
					t.Errorf("participant %s: got kind %s, want %s", p.ID, p.Kind, want)
				}
			}
		})
	}
}

func TestTracker_ThinkingFlag(t *testing.T) {
	t.Parallel()
	tr := NewTracker()

	tr.Join("r", "bot1", KindPersona)
	tr.SetThinking("r", "bot1", true)

	roster := tr.Roster("r")
	if len(roster) != 1 {
		t.Fatalf("expected 1 participant, got %d", len(roster))
	}
	if !roster[0].Thinking {
		t.Error("expected Thinking=true after SetThinking(true)")
	}

	tr.SetThinking("r", "bot1", false)
	roster = tr.Roster("r")
	if roster[0].Thinking {
		t.Error("expected Thinking=false after SetThinking(false)")
	}
}

func TestTracker_ThinkingNonMember(t *testing.T) {
	t.Parallel()
	tr := NewTracker()
	tr.Join("r", "alice", KindHuman)
	// Set thinking on a non-member should be a no-op.
	tr.SetThinking("r", "stranger", true)

	roster := tr.Roster("r")
	if len(roster) != 1 {
		t.Fatalf("expected 1 participant, got %d", len(roster))
	}
	if roster[0].ID != "alice" {
		t.Errorf("expected alice, got %s", roster[0].ID)
	}
}

func TestTracker_Status(t *testing.T) {
	t.Parallel()
	tr := NewTracker()

	tr.Join("r", "alice", KindHuman)
	tr.SetStatus("r", "alice", "away")

	roster := tr.Roster("r")
	if len(roster) != 1 {
		t.Fatalf("expected 1 participant, got %d", len(roster))
	}
	if roster[0].Status != "away" {
		t.Errorf("expected status 'away', got %q", roster[0].Status)
	}
}

func TestTracker_StatusNonMember(t *testing.T) {
	t.Parallel()
	tr := NewTracker()
	tr.SetStatus("r", "stranger", "online")
	// Should not crash or create a phantom participant.
	roster := tr.Roster("r")
	if roster != nil {
		t.Errorf("expected nil roster, got %v", roster)
	}
}

func TestTracker_LeaveEmptiesRoom(t *testing.T) {
	t.Parallel()
	tr := NewTracker()
	tr.Join("r", "solo", KindHuman)
	tr.Leave("r", "solo")

	tr.mu.RLock()
	_, exists := tr.rooms["r"]
	tr.mu.RUnlock()

	if exists {
		t.Error("expected empty room to be cleaned up")
	}
}

func TestTracker_RosterReturnsCopies(t *testing.T) {
	t.Parallel()
	tr := NewTracker()
	tr.Join("r", "alice", KindHuman)

	roster1 := tr.Roster("r")
	if len(roster1) != 1 {
		t.Fatalf("expected 1 participant, got %d", len(roster1))
	}

	// Mutate the returned participant.
	roster1[0].Status = "mutated"

	// Fetch again and verify the original is unchanged.
	roster2 := tr.Roster("r")
	if roster2[0].Status == "mutated" {
		t.Error("Roster should return copies, not references")
	}
}

func TestTracker_Concurrent(t *testing.T) {
	t.Parallel()
	tr := NewTracker()

	const goroutines = 32
	const ops = 200

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		g := g
		go func() {
			defer wg.Done()
			roomID := "room"
			for i := 0; i < ops; i++ {
				id := "user"
				tr.Join(roomID, id, KindHuman)
				tr.SetThinking(roomID, id, true)
				tr.SetStatus(roomID, id, "busy")
				tr.Roster(roomID)
				tr.Leave(roomID, id)
			}
			_ = g
		}()
	}
	wg.Wait()
}

func TestTracker_ConcurrentMultiRoom(t *testing.T) {
	t.Parallel()
	tr := NewTracker()

	rooms := []string{"alpha", "beta", "gamma"}
	const perRoom = 16
	const ops = 50

	// Seed each room with participants.
	for _, r := range rooms {
		for i := 0; i < perRoom; i++ {
			tr.Join(r, "user", KindHuman)
		}
	}

	var wg sync.WaitGroup
	for _, r := range rooms {
		r := r
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < ops; i++ {
				tr.Roster(r)
				tr.SetThinking(r, "user", true)
				tr.SetStatus(r, "user", "active")
			}
		}()
	}
	wg.Wait()

	// Verify each room still has the expected number of participants.
	for _, r := range rooms {
		roster := tr.Roster(r)
		if len(roster) != 1 {
			t.Errorf("room %s: expected 1 participant, got %d", r, len(roster))
		}
	}
}
