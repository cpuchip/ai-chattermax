package presence

import (
	"math/rand/v2"
	"reflect"
	"strconv"
	"sync"
	"testing"
)

func TestKind_String(t *testing.T) {
	tests := []struct {
		k    Kind
		want string
	}{
		{KindHuman, "human"},
		{KindPersona, "persona"},
		{Kind(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.k.String(); got != tt.want {
			t.Errorf("Kind(%d).String() = %q, want %q", tt.k, got, tt.want)
		}
	}
}

func TestTracker_BasicLifecycle(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T, tr *Tracker)
	}{
		{
			name: "join adds a human",
			run: func(t *testing.T, tr *Tracker) {
				tr.Join("r1", "alice", KindHuman)
				got := tr.Roster("r1")
				want := []Participant{{ID: "alice", Kind: KindHuman}}
				if !reflect.DeepEqual(got, want) {
					t.Errorf("Roster = %+v, want %+v", got, want)
				}
			},
		},
		{
			name: "join adds a persona",
			run: func(t *testing.T, tr *Tracker) {
				tr.Join("r1", "ada", KindPersona)
				got := tr.Roster("r1")
				want := []Participant{{ID: "ada", Kind: KindPersona}}
				if !reflect.DeepEqual(got, want) {
					t.Errorf("Roster = %+v, want %+v", got, want)
				}
			},
		},
		{
			name: "roster of multiple kinds is sorted by ID",
			run: func(t *testing.T, tr *Tracker) {
				tr.Join("r1", "bob", KindHuman)
				tr.Join("r1", "ada", KindPersona)
				tr.Join("r1", "cleo", KindHuman)
				got := tr.Roster("r1")
				want := []Participant{
					{ID: "ada", Kind: KindPersona},
					{ID: "bob", Kind: KindHuman},
					{ID: "cleo", Kind: KindHuman},
				}
				if !reflect.DeepEqual(got, want) {
					t.Errorf("Roster = %+v, want %+v", got, want)
				}
			},
		},
		{
			name: "leave removes a participant and deletes the empty room",
			run: func(t *testing.T, tr *Tracker) {
				tr.Join("r1", "alice", KindHuman)
				tr.Leave("r1", "alice")
				if got := tr.Roster("r1"); len(got) != 0 {
					t.Errorf("Roster = %+v, want empty", got)
				}
				if rooms := tr.Rooms(); len(rooms) != 0 {
					t.Errorf("Rooms = %v, want empty (empty room should be deleted)", rooms)
				}
			},
		},
		{
			name: "leave is a no-op for unknown id and unknown room",
			run: func(t *testing.T, tr *Tracker) {
				tr.Leave("ghost", "alice")
				tr.Leave("r1", "nobody")
				if got := tr.Rooms(); len(got) != 0 {
					t.Errorf("Rooms = %v, want empty", got)
				}
			},
		},
		{
			name: "re-join updates kind and clears status",
			run: func(t *testing.T, tr *Tracker) {
				tr.Join("r1", "ada", KindPersona)
				if err := tr.SetStatus("r1", "ada", "active"); err != nil {
					t.Fatalf("SetStatus: %v", err)
				}
				tr.Join("r1", "ada", KindHuman)
				got := tr.Roster("r1")
				want := []Participant{{ID: "ada", Kind: KindHuman, Status: ""}}
				if !reflect.DeepEqual(got, want) {
					t.Errorf("Roster = %+v, want %+v", got, want)
				}
			},
		},
		{
			name: "rooms are isolated",
			run: func(t *testing.T, tr *Tracker) {
				tr.Join("A", "alice", KindHuman)
				tr.Join("B", "bob", KindHuman)
				a := tr.Roster("A")
				b := tr.Roster("B")
				if len(a) != 1 || a[0].ID != "alice" {
					t.Errorf("Roster A = %+v", a)
				}
				if len(b) != 1 || b[0].ID != "bob" {
					t.Errorf("Roster B = %+v", b)
				}
			},
		},
		{
			name: "roster of unknown room is nil",
			run: func(t *testing.T, tr *Tracker) {
				if got := tr.Roster("ghost"); got != nil {
					t.Errorf("Roster(ghost) = %+v, want nil", got)
				}
			},
		},
		{
			name: "leaving one of many participants keeps the room",
			run: func(t *testing.T, tr *Tracker) {
				tr.Join("r1", "alice", KindHuman)
				tr.Join("r1", "bob", KindHuman)
				tr.Leave("r1", "alice")
				got := tr.Roster("r1")
				want := []Participant{{ID: "bob", Kind: KindHuman}}
				if !reflect.DeepEqual(got, want) {
					t.Errorf("Roster = %+v, want %+v", got, want)
				}
				rooms := tr.Rooms()
				if len(rooms) != 1 || rooms[0] != "r1" {
					t.Errorf("Rooms = %v, want [r1]", rooms)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := New()
			tt.run(t, tr)
		})
	}
}

func TestTracker_StatusSetters(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(t *testing.T, tr *Tracker)
		op         func(tr *Tracker) error
		wantErr    error
		wantStatus string
		wantThink  bool
	}{
		{
			name: "SetStatus on a present participant",
			setup: func(t *testing.T, tr *Tracker) {
				tr.Join("r1", "alice", KindHuman)
			},
			op:         func(tr *Tracker) error { return tr.SetStatus("r1", "alice", "active") },
			wantStatus: "active",
		},
		{
			name: "SetStatus on an unknown room returns ErrNotInRoom",
			setup: func(t *testing.T, tr *Tracker) {
				tr.Join("r1", "alice", KindHuman)
			},
			op:      func(tr *Tracker) error { return tr.SetStatus("ghost", "alice", "active") },
			wantErr: ErrNotInRoom,
		},
		{
			name: "SetStatus on an unknown id returns ErrNotInRoom",
			setup: func(t *testing.T, tr *Tracker) {
				tr.Join("r1", "alice", KindHuman)
			},
			op:      func(tr *Tracker) error { return tr.SetStatus("r1", "bob", "active") },
			wantErr: ErrNotInRoom,
		},
		{
			name: "SetThinking(true) on a persona",
			setup: func(t *testing.T, tr *Tracker) {
				tr.Join("r1", "ada", KindPersona)
			},
			op:        func(tr *Tracker) error { return tr.SetThinking("r1", "ada", true) },
			wantThink: true,
		},
		{
			name: "SetThinking(false) clears a persona's flag",
			setup: func(t *testing.T, tr *Tracker) {
				tr.Join("r1", "ada", KindPersona)
				if err := tr.SetThinking("r1", "ada", true); err != nil {
					t.Fatalf("setup: %v", err)
				}
			},
			op:        func(tr *Tracker) error { return tr.SetThinking("r1", "ada", false) },
			wantThink: false,
		},
		{
			name: "SetThinking on unknown id returns ErrNotInRoom",
			setup: func(t *testing.T, tr *Tracker) {
				tr.Join("r1", "ada", KindPersona)
			},
			op:      func(tr *Tracker) error { return tr.SetThinking("r1", "ghost", true) },
			wantErr: ErrNotInRoom,
		},
		{
			name: "SetStatus empty string clears status",
			setup: func(t *testing.T, tr *Tracker) {
				tr.Join("r1", "alice", KindHuman)
				if err := tr.SetStatus("r1", "alice", "active"); err != nil {
					t.Fatalf("setup: %v", err)
				}
			},
			op:         func(tr *Tracker) error { return tr.SetStatus("r1", "alice", "") },
			wantStatus: "",
		},
		{
			name: "SetStatus and SetThinking compose",
			setup: func(t *testing.T, tr *Tracker) {
				tr.Join("r1", "ada", KindPersona)
			},
			op: func(tr *Tracker) error {
				if err := tr.SetStatus("r1", "ada", "thinking"); err != nil {
					return err
				}
				return tr.SetThinking("r1", "ada", true)
			},
			wantStatus: "thinking",
			wantThink:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := New()
			tt.setup(t, tr)
			err := tt.op(tr)
			if err != tt.wantErr {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr == nil {
				got := tr.Roster("r1")
				if len(got) != 1 {
					t.Fatalf("roster len = %d, want 1", len(got))
				}
				if got[0].Status != tt.wantStatus {
					t.Errorf("status = %q, want %q", got[0].Status, tt.wantStatus)
				}
				if got[0].Thinking != tt.wantThink {
					t.Errorf("thinking = %v, want %v", got[0].Thinking, tt.wantThink)
				}
			}
		})
	}
}

// TestTracker_Concurrent is a -race-friendly stress test: many
// goroutines hammer Join / Leave / SetStatus / SetThinking / Roster on
// overlapping rooms and ids. We assert: no panics, no data races, and
// the final state of the tracker is internally consistent (every room
// that exists has at least one participant, every participant's ID is
// one of ours and is non-empty).
func TestTracker_Concurrent(t *testing.T) {
	tr := New()
	const (
		nGoroutines     = 32
		opsPerGoroutine = 500
		nIDs            = 16
		nRooms          = 4
	)
	ids := make([]string, nIDs)
	for i := 0; i < nIDs; i++ {
		ids[i] = "id-" + strconv.Itoa(i)
	}
	rooms := make([]string, nRooms)
	for i := 0; i < nRooms; i++ {
		rooms[i] = "room-" + strconv.Itoa(i)
	}
	statuses := []string{"active", "idle", "away", "dnd"}

	var wg sync.WaitGroup
	for g := 0; g < nGoroutines; g++ {
		wg.Add(1)
		go func(seed uint64) {
			defer wg.Done()
			r := rand.New(rand.NewPCG(seed, seed+0x9E3779B97F4A7C15))
			for i := 0; i < opsPerGoroutine; i++ {
				id := ids[r.IntN(nIDs)]
				room := rooms[r.IntN(nRooms)]
				switch r.IntN(5) {
				case 0:
					kind := KindHuman
					if r.IntN(2) == 0 {
						kind = KindPersona
					}
					tr.Join(room, id, kind)
				case 1:
					tr.Leave(room, id)
				case 2:
					_ = tr.SetStatus(room, id, statuses[r.IntN(len(statuses))])
				case 3:
					_ = tr.SetThinking(room, id, r.IntN(2) == 0)
				case 4:
					_ = tr.Roster(room)
				}
			}
		}(uint64(g + 1))
	}
	wg.Wait()

	// Final-state invariant.
	for _, room := range tr.Rooms() {
		got := tr.Roster(room)
		if len(got) == 0 {
			t.Errorf("room %q is in Rooms() but Roster is empty (should be deleted)", room)
			continue
		}
		for _, p := range got {
			if p.ID == "" {
				t.Errorf("room %q has a participant with empty ID", room)
				continue
			}
			found := false
			for _, k := range ids {
				if k == p.ID {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("room %q has unknown participant %q", room, p.ID)
			}
		}
	}
}
