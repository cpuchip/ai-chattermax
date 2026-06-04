package presence

import (
	"fmt"
	"reflect"
	"sort"
	"sync"
	"testing"
)

func sortParticipants(pp []Participant) {
	sort.Slice(pp, func(i, j int) bool {
		return pp[i].ID < pp[j].ID
	})
}

func TestTracker_Join(t *testing.T) {
	var tr Tracker
	tr.Join("r1", "alice", KindHuman)

	got := tr.Roster("r1")
	want := []Participant{{ID: "alice", Kind: KindHuman}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Roster after Join = %v, want %v", got, want)
	}
}

func TestTracker_JoinReplace(t *testing.T) {
	var tr Tracker
	tr.Join("r1", "alice", KindHuman)
	tr.SetThinking("r1", "alice", true)
	tr.Join("r1", "alice", KindPersona)

	got := tr.Roster("r1")
	want := []Participant{{ID: "alice", Kind: KindPersona}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Roster after re-Join = %v, want %v", got, want)
	}
}

func TestTracker_Leave(t *testing.T) {
	var tr Tracker
	tr.Join("r1", "alice", KindHuman)
	tr.Join("r1", "bob", KindPersona)
	tr.Leave("r1", "alice")

	got := tr.Roster("r1")
	want := []Participant{{ID: "bob", Kind: KindPersona}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Roster after Leave = %v, want %v", got, want)
	}
}

func TestTracker_LeaveLastCleansRoom(t *testing.T) {
	var tr Tracker
	tr.Join("r1", "alice", KindHuman)
	tr.Leave("r1", "alice")

	if got := tr.Roster("r1"); got != nil {
		t.Fatalf("Roster for empty room = %v, want nil", got)
	}
}

func TestTracker_SetThinking(t *testing.T) {
	var tr Tracker
	tr.Join("r1", "alice", KindHuman)
	tr.SetThinking("r1", "alice", true)

	got := tr.Roster("r1")
	want := []Participant{{ID: "alice", Kind: KindHuman, Thinking: true}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Roster after SetThinking = %v, want %v", got, want)
	}
}

func TestTracker_SetThinkingNoRoom(t *testing.T) {
	var tr Tracker
	// Should not panic.
	tr.SetThinking("nosuch", "alice", true)
}

func TestTracker_SetThinkingNoParticipant(t *testing.T) {
	var tr Tracker
	tr.Join("r1", "alice", KindHuman)
	// Should not panic.
	tr.SetThinking("r1", "bob", true)
}

func TestTracker_RosterEmptyRoom(t *testing.T) {
	var tr Tracker
	if got := tr.Roster("nosuch"); got != nil {
		t.Fatalf("Roster for missing room = %v, want nil", got)
	}
}

func TestTracker_RoomIsolation(t *testing.T) {
	var tr Tracker
	tr.Join("r1", "alice", KindHuman)
	tr.Join("r2", "bob", KindPersona)

	got1 := tr.Roster("r1")
	want1 := []Participant{{ID: "alice", Kind: KindHuman}}
	if !reflect.DeepEqual(got1, want1) {
		t.Fatalf("Roster(r1) = %v, want %v", got1, want1)
	}

	got2 := tr.Roster("r2")
	want2 := []Participant{{ID: "bob", Kind: KindPersona}}
	if !reflect.DeepEqual(got2, want2) {
		t.Fatalf("Roster(r2) = %v, want %v", got2, want2)
	}
}

func TestTracker_RosterCopy(t *testing.T) {
	var tr Tracker
	tr.Join("r1", "alice", KindHuman)

	got := tr.Roster("r1")
	// Mutate the returned slice; internals should be unaffected.
	got[0].Thinking = true

	got2 := tr.Roster("r1")
	if got2[0].Thinking {
		t.Fatal("mutating returned roster affected internal state")
	}
}

func TestTracker_TableDriven(t *testing.T) {
	tests := []struct {
		name   string
		ops    func(*Tracker)
		roomID string
		want   []Participant
	}{
		{
			name:   "empty tracker",
			ops:    func(_ *Tracker) {},
			roomID: "r1",
			want:   nil,
		},
		{
			name: "single human",
			ops: func(tr *Tracker) {
				tr.Join("r1", "alice", KindHuman)
			},
			roomID: "r1",
			want:   []Participant{{ID: "alice", Kind: KindHuman}},
		},
		{
			name: "human and persona",
			ops: func(tr *Tracker) {
				tr.Join("r1", "alice", KindHuman)
				tr.Join("r1", "p1", KindPersona)
			},
			roomID: "r1",
			want: []Participant{
				{ID: "alice", Kind: KindHuman},
				{ID: "p1", Kind: KindPersona},
			},
		},
		{
			name: "leave removes participant",
			ops: func(tr *Tracker) {
				tr.Join("r1", "alice", KindHuman)
				tr.Join("r1", "bob", KindHuman)
				tr.Leave("r1", "alice")
			},
			roomID: "r1",
			want:   []Participant{{ID: "bob", Kind: KindHuman}},
		},
		{
			name: "thinking flag",
			ops: func(tr *Tracker) {
				tr.Join("r1", "alice", KindHuman)
				tr.SetThinking("r1", "alice", true)
			},
			roomID: "r1",
			want:   []Participant{{ID: "alice", Kind: KindHuman, Thinking: true}},
		},
		{
			name: "cross-room isolation",
			ops: func(tr *Tracker) {
				tr.Join("r1", "alice", KindHuman)
				tr.Join("r2", "bob", KindPersona)
			},
			roomID: "r2",
			want:   []Participant{{ID: "bob", Kind: KindPersona}},
		},
		{
			name: "re-join resets kind and thinking",
			ops: func(tr *Tracker) {
				tr.Join("r1", "alice", KindHuman)
				tr.SetThinking("r1", "alice", true)
				tr.Join("r1", "alice", KindPersona)
			},
			roomID: "r1",
			want:   []Participant{{ID: "alice", Kind: KindPersona}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var tr Tracker
			tt.ops(&tr)
			got := tr.Roster(tt.roomID)
			sortParticipants(got)
			sortParticipants(tt.want)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("Roster = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTracker_ConcurrentJoinLeave(t *testing.T) {
	const (
		rooms    = 4
		perRoom  = 50
		workers  = 10
	)

	var tr Tracker
	var wg sync.WaitGroup

	// Concurrently add participants.
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func(wid int) {
			defer wg.Done()
			for r := 0; r < rooms; r++ {
				for i := 0; i < perRoom; i++ {
					id := fmt.Sprintf("w%d-r%d-p%d", wid, r, i)
					tr.Join(fmt.Sprintf("room%d", r), id, KindHuman)
				}
			}
		}(w)
	}
	wg.Wait()

	// Verify counts.
	for r := 0; r < rooms; r++ {
		roomID := fmt.Sprintf("room%d", r)
		roster := tr.Roster(roomID)
		if len(roster) != workers*perRoom {
			t.Fatalf("room %s has %d participants, want %d", roomID, len(roster), workers*perRoom)
		}
	}

	// Concurrently remove half the participants.
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func(wid int) {
			defer wg.Done()
			for r := 0; r < rooms; r++ {
				for i := 0; i < perRoom/2; i++ {
					id := fmt.Sprintf("w%d-r%d-p%d", wid, r, i)
					tr.Leave(fmt.Sprintf("room%d", r), id)
				}
			}
		}(w)
	}
	wg.Wait()

	// Verify counts after removal.
	for r := 0; r < rooms; r++ {
		roomID := fmt.Sprintf("room%d", r)
		roster := tr.Roster(roomID)
		want := workers * perRoom / 2
		if len(roster) != want {
			t.Fatalf("room %s has %d participants after Leave, want %d", roomID, len(roster), want)
		}
	}
}

func TestTracker_ConcurrentMixedOps(t *testing.T) {
	const (
		rooms   = 3
		ids     = 30
		workers = 20
		ops     = 200
	)

	var tr Tracker
	var wg sync.WaitGroup

	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func(wid int) {
			defer wg.Done()
			for i := 0; i < ops; i++ {
				roomID := fmt.Sprintf("room%d", i%rooms)
				id := fmt.Sprintf("user%d", i%ids)
				switch i % 4 {
				case 0:
					tr.Join(roomID, id, KindHuman)
				case 1:
					tr.Leave(roomID, id)
				case 2:
					tr.SetThinking(roomID, id, true)
				case 3:
					_ = tr.Roster(roomID)
				}
			}
		}(w)
	}
	wg.Wait()

	// No assertion on exact final state — the point is that the race
	// detector passes and we don't panic or deadlock.
	for r := 0; r < rooms; r++ {
		_ = tr.Roster(fmt.Sprintf("room%d", r))
	}
}
