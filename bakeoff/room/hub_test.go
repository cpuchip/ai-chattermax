package room

import (
	"errors"
	"math/rand/v2"
	"sort"
	"strconv"
	"sync"
	"testing"
)

// fakeClient is a transport-agnostic Client whose Send is non-blocking:
// messages are appended to an internal mailbox guarded by a mutex. The
// mailbox pattern keeps Hub tests fully deterministic — no goroutines
// drain a channel, no time.Sleep, no flake.
type fakeClient struct {
	id      string
	mu      sync.Mutex
	mail    [][]byte
	sendErr error
	drops   int
}

func newFakeClient(id string) *fakeClient { return &fakeClient{id: id} }

func (f *fakeClient) ID() string { return f.id }

func (f *fakeClient) Send(b []byte) error {
	if f.sendErr != nil {
		f.drops++
		return f.sendErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := make([]byte, len(b))
	copy(cp, b)
	f.mail = append(f.mail, cp)
	return nil
}

func (f *fakeClient) received() [][]byte {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([][]byte, len(f.mail))
	for i, b := range f.mail {
		cp := make([]byte, len(b))
		copy(cp, b)
		out[i] = cp
	}
	return out
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// collectIDs is a test-only helper that returns the (unsorted) client
// IDs currently registered in roomID.
func collectIDs(h *Hub, roomID string) []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	room, ok := h.rooms[roomID]
	if !ok {
		return nil
	}
	out := make([]string, 0, len(room))
	for id := range room {
		out = append(out, id)
	}
	return out
}

func TestHub_RegisterBroadcastUnregister(t *testing.T) {
	tests := []struct {
		name       string
		actions    func(t *testing.T, h *Hub)
		wantRooms  []string
		wantInRoom map[string][]string // roomID -> sorted client IDs
	}{
		{
			name: "register and broadcast reaches single client",
			actions: func(t *testing.T, h *Hub) {
				c := newFakeClient("alice")
				h.Register("r1", c)
				h.Broadcast("r1", []byte("hi"))
				msgs := c.received()
				if len(msgs) != 1 || string(msgs[0]) != "hi" {
					t.Errorf("alice mail = %q, want [hi]", msgs)
				}
			},
			wantRooms:  []string{"r1"},
			wantInRoom: map[string][]string{"r1": {"alice"}},
		},
		{
			name: "broadcast reaches all clients in a room",
			actions: func(t *testing.T, h *Hub) {
				a := newFakeClient("alice")
				b := newFakeClient("bob")
				h.Register("r1", a)
				h.Register("r1", b)
				h.Broadcast("r1", []byte("hello"))
				for _, c := range []*fakeClient{a, b} {
					msgs := c.received()
					if len(msgs) != 1 || string(msgs[0]) != "hello" {
						t.Errorf("%s mail = %q, want [hello]", c.id, msgs)
					}
				}
			},
			wantRooms:  []string{"r1"},
			wantInRoom: map[string][]string{"r1": {"alice", "bob"}},
		},
		{
			name: "unregister removes a client from the room",
			actions: func(t *testing.T, h *Hub) {
				a := newFakeClient("alice")
				b := newFakeClient("bob")
				h.Register("r1", a)
				h.Register("r1", b)
				h.Unregister("r1", b)
				h.Broadcast("r1", []byte("after"))
				if got := a.received(); len(got) != 1 || string(got[0]) != "after" {
					t.Errorf("alice mail = %q, want [after]", got)
				}
				if got := b.received(); len(got) != 0 {
					t.Errorf("bob mail = %q, want none", got)
				}
			},
			wantRooms:  []string{"r1"},
			wantInRoom: map[string][]string{"r1": {"alice"}},
		},
		{
			name: "broadcast to unknown room is a no-op",
			actions: func(t *testing.T, h *Hub) {
				h.Broadcast("ghost", []byte("nobody"))
			},
			wantRooms:  nil,
			wantInRoom: map[string][]string{},
		},
		{
			name: "rooms are isolated",
			actions: func(t *testing.T, h *Hub) {
				a := newFakeClient("alice")
				b := newFakeClient("bob")
				h.Register("A", a)
				h.Register("B", b)
				h.Broadcast("A", []byte("a-msg"))
				if got := a.received(); len(got) != 1 || string(got[0]) != "a-msg" {
					t.Errorf("alice mail = %q, want [a-msg]", got)
				}
				if got := b.received(); len(got) != 0 {
					t.Errorf("bob (room B) should be silent, got %q", got)
				}
			},
			wantRooms:  []string{"A", "B"},
			wantInRoom: map[string][]string{"A": {"alice"}, "B": {"bob"}},
		},
		{
			name: "unregister of unknown client is a no-op",
			actions: func(t *testing.T, h *Hub) {
				a := newFakeClient("alice")
				ghost := newFakeClient("ghost")
				h.Register("r1", a)
				h.Unregister("r1", ghost)
				h.Unregister("ghost-room", a)
			},
			wantRooms:  []string{"r1"},
			wantInRoom: map[string][]string{"r1": {"alice"}},
		},
		{
			name: "unregistering the last client deletes the room",
			actions: func(t *testing.T, h *Hub) {
				a := newFakeClient("alice")
				h.Register("r1", a)
				h.Unregister("r1", a)
			},
			wantRooms:  nil,
			wantInRoom: map[string][]string{},
		},
		{
			name: "send error drops the client from the room",
			actions: func(t *testing.T, h *Hub) {
				good := newFakeClient("good")
				bad := &fakeClient{id: "bad", sendErr: errors.New("broken pipe")}
				h.Register("r1", good)
				h.Register("r1", bad)
				h.Broadcast("r1", []byte("m1"))
				h.Broadcast("r1", []byte("m2"))
				if got := good.received(); len(got) != 2 {
					t.Errorf("good mail len = %d, want 2", len(got))
				}
				if got := bad.drops; got != 1 {
					t.Errorf("bad drops = %d, want 1 (only the first send errors)", got)
				}
			},
			wantRooms:  []string{"r1"},
			wantInRoom: map[string][]string{"r1": {"good"}},
		},
		{
			name: "register replaces a client with the same ID",
			actions: func(t *testing.T, h *Hub) {
				v1 := newFakeClient("alice")
				v2 := newFakeClient("alice") // distinct object, same ID
				h.Register("r1", v1)
				h.Register("r1", v2)
				h.Broadcast("r1", []byte("ping"))
				if got := v1.received(); len(got) != 0 {
					t.Errorf("v1 mail = %q, want none (replaced)", got)
				}
				if got := v2.received(); len(got) != 1 || string(got[0]) != "ping" {
					t.Errorf("v2 mail = %q, want [ping]", got)
				}
			},
			wantRooms:  []string{"r1"},
			wantInRoom: map[string][]string{"r1": {"alice"}},
		},
		{
			name: "caller can reuse message buffer after broadcast returns",
			actions: func(t *testing.T, h *Hub) {
				c := newFakeClient("alice")
				h.Register("r1", c)
				buf := []byte("original")
				h.Broadcast("r1", buf)
				// Mutate the caller's buffer; alice should still see the
				// original bytes because Hub took a defensive copy.
				copy(buf, "MUTATED!")
				msgs := c.received()
				if len(msgs) != 1 || string(msgs[0]) != "original" {
					t.Errorf("alice mail = %q, want [original] (defensive copy)", msgs)
				}
			},
			wantRooms:  []string{"r1"},
			wantInRoom: map[string][]string{"r1": {"alice"}},
		},
		{
			name: "nil client is a no-op for register and unregister",
			actions: func(t *testing.T, h *Hub) {
				h.Register("r1", nil)
				h.Unregister("r1", nil)
			},
			wantRooms:  nil,
			wantInRoom: map[string][]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHub()
			tt.actions(t, h)

			gotRooms := h.Rooms()
			sort.Strings(gotRooms)
			if !equalStringSlices(gotRooms, tt.wantRooms) {
				t.Errorf("Rooms() = %v, want %v", gotRooms, tt.wantRooms)
			}
			for room, wantIDs := range tt.wantInRoom {
				ids := collectIDs(h, room)
				sort.Strings(ids)
				if !equalStringSlices(ids, wantIDs) {
					t.Errorf("room %q ids = %v, want %v", room, ids, wantIDs)
				}
			}
		})
	}
}

// TestHub_Concurrent is a -race-friendly stress test. Many goroutines
// hammer Register/Unregister/Broadcast against the same Hub while a
// separate goroutine drains a separate room. We assert: no panic, no
// race detector output, and the final state is well-formed (every
// remaining client is one we registered).
func TestHub_Concurrent(t *testing.T) {
	const (
		nClients      = 16
		nWriters      = 16
		opsPerWriter  = 500
		roomA         = "A"
		roomB         = "B"
	)
	clients := make([]*fakeClient, nClients)
	for i := 0; i < nClients; i++ {
		clients[i] = newFakeClient(strconv.Itoa(i))
	}
	h := NewHub()
	for i := 0; i < nClients; i++ {
		h.Register(roomA, clients[i])
		if i%2 == 0 {
			h.Register(roomB, clients[i])
		}
	}

	var wg sync.WaitGroup
	for w := 0; w < nWriters; w++ {
		wg.Add(1)
		go func(seed uint64) {
			defer wg.Done()
			r := rand.New(rand.NewPCG(seed, seed+0x9E3779B97F4A7C15))
			for i := 0; i < opsPerWriter; i++ {
				idx := r.IntN(nClients)
				c := clients[idx]
				switch r.IntN(3) {
				case 0:
					h.Register(roomA, c)
				case 1:
					h.Unregister(roomA, c)
				case 2:
					h.Broadcast(roomA, []byte("yell"))
				}
			}
		}(uint64(w + 1))
	}
	// Separate goroutine hammers roomB independently so the two rooms
	// are not serialized on the same writer's path.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < opsPerWriter; i++ {
			h.Broadcast(roomB, []byte("ping"))
		}
	}()

	wg.Wait()

	// Final-state invariant: every room that still exists contains
	// only clients we registered, and (since we use a map keyed by
	// ID) no client appears twice in the same room. Map semantics
	// guarantee the latter; we check the former.
	for _, room := range h.Rooms() {
		for _, id := range collectIDs(h, room) {
			found := false
			for _, k := range clients {
				if k.id == id {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("room %q contains unknown client %q", room, id)
			}
		}
	}
}
