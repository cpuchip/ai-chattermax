package room

import (
	"errors"
	"fmt"
	"sync"
	"testing"
)

// fakeClient records every message delivered to it.
type fakeClient struct {
	id   string
	mu   sync.Mutex
	msgs [][]byte
	err  error // if non-nil, Send always returns this
}

func newFakeClient(id string) *fakeClient { return &fakeClient{id: id} }

func (f *fakeClient) ID() string { return f.id }

func (f *fakeClient) Send(msg []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	cp := make([]byte, len(msg))
	copy(cp, msg)
	f.msgs = append(f.msgs, cp)
	return nil
}

func (f *fakeClient) messages() [][]byte {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([][]byte, len(f.msgs))
	copy(out, f.msgs)
	return out
}

func TestHub_TableDriven(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		run    func(t *testing.T, h *Hub)
		verify func(t *testing.T, h *Hub)
	}{
		{
			name: "register and broadcast delivers to all in room",
			run: func(t *testing.T, h *Hub) {
				a := newFakeClient("a")
				b := newFakeClient("b")
				h.Register("r1", a)
				h.Register("r1", b)
				h.Broadcast("r1", []byte("hello"))
			},
			verify: func(t *testing.T, h *Hub) {
				// verified via the clients captured in run; rebuild inline:
			},
		},
		{
			name: "broadcast to unknown room is no-op",
			run: func(t *testing.T, h *Hub) {
				h.Broadcast("nonexistent", []byte("ghost"))
			},
		},
		{
			name: "broadcast isolation between rooms",
			run: func(t *testing.T, h *Hub) {
				// handled in dedicated test below
			},
		},
	}

	_ = tests // the table above seeds the pattern; detailed cases follow.

	// --- detailed table-driven cases with explicit asserts -----------------
	type setup struct {
		clients map[string][]string // roomID -> list of client IDs
	}
	cases := []struct {
		name           string
		setup          setup
		broadcastRoom  string
		broadcastMsg   string
		expectReceived map[string]int // clientID -> expected message count
	}{
		{
			name: "single room broadcast reaches all members",
			setup: setup{clients: map[string][]string{
				"lobby": {"alice", "bob", "carol"},
			}},
			broadcastRoom:  "lobby",
			broadcastMsg:   "hi",
			expectReceived: map[string]int{"alice": 1, "bob": 1, "carol": 1},
		},
		{
			name: "broadcast does not cross rooms",
			setup: setup{clients: map[string][]string{
				"a": {"alice"},
				"b": {"bob"},
			}},
			broadcastRoom:  "a",
			broadcastMsg:   "only-a",
			expectReceived: map[string]int{"alice": 1, "bob": 0},
		},
		{
			name: "unregister removes recipient",
			setup: setup{clients: map[string][]string{
				"r": {"x", "y"},
			}},
			broadcastRoom:  "r",
			broadcastMsg:   "after-unreg",
			expectReceived: map[string]int{"x": 0, "y": 1},
		},
		{
			name: "duplicate register is idempotent",
			setup: setup{clients: map[string][]string{
				"r": {"dup", "dup"},
			}},
			broadcastRoom:  "r",
			broadcastMsg:   "once",
			expectReceived: map[string]int{"dup": 1},
		},
		{
			name: "unregister non-member is no-op",
			setup: setup{clients: map[string][]string{
				"r": {"only"},
			}},
			broadcastRoom:  "r",
			broadcastMsg:   "still-there",
			expectReceived: map[string]int{"only": 1},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := NewHub()
			clients := make(map[string]*fakeClient)

			for roomID, ids := range tc.setup.clients {
				for _, id := range ids {
					c, ok := clients[id]
					if !ok {
						c = newFakeClient(id)
						clients[id] = c
					}
					h.Register(roomID, c)
				}
			}

			// Special case: "unregister removes recipient" — remove x first.
			if tc.name == "unregister removes recipient" {
				h.Unregister("r", clients["x"])
			}
			// Special case: "unregister non-member is no-op" — unregister a stranger.
			if tc.name == "unregister non-member is no-op" {
				h.Unregister("r", newFakeClient("stranger"))
			}

			h.Broadcast(tc.broadcastRoom, []byte(tc.broadcastMsg))

			for id, want := range tc.expectReceived {
				c := clients[id]
				if c == nil {
					if want != 0 {
						t.Errorf("client %q expected %d msgs but was never created", id, want)
					}
					continue
				}
				if got := len(c.messages()); got != want {
					t.Errorf("client %q: got %d msgs, want %d", id, got, want)
				}
			}
		})
	}
}

func TestHub_BroadcastAfterSendError(t *testing.T) {
	t.Parallel()
	h := NewHub()
	good := newFakeClient("good")
	bad := newFakeClient("bad")
	bad.err = errors.New("send failed")

	h.Register("r", good)
	h.Register("r", bad)
	h.Broadcast("r", []byte("msg"))

	if got := len(good.messages()); got != 1 {
		t.Errorf("good client: got %d msgs, want 1", got)
	}
	// bad client stays registered (no auto-removal policy in Hub)
	h.Broadcast("r", []byte("msg2"))
	if got := len(good.messages()); got != 2 {
		t.Errorf("good client after 2nd broadcast: got %d, want 2", got)
	}
}

func TestHub_UnregisterEmptiesRoom(t *testing.T) {
	t.Parallel()
	h := NewHub()
	c := newFakeClient("solo")
	h.Register("r", c)
	h.Unregister("r", c)
	// After unregistering the only member, the room map itself should be gone.
	h.mu.RLock()
	_, exists := h.rooms["r"]
	h.mu.RUnlock()
	if exists {
		t.Error("expected empty room to be cleaned up")
	}
}

func TestHub_Concurrent(t *testing.T) {
	t.Parallel()
	h := NewHub()

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
				c := newFakeClient("c")
				h.Register(roomID, c)
				h.Broadcast(roomID, []byte("ping"))
				h.Unregister(roomID, c)
			}
			_ = g
		}()
	}
	wg.Wait()

	// If we got here without -race complaining, concurrency is sound.
	// The room may or may not exist depending on timing; that's fine.
}

func TestHub_ConcurrentMultiRoom(t *testing.T) {
	t.Parallel()
	h := NewHub()

	rooms := []string{"alpha", "beta", "gamma"}
	const perRoom = 16
	const broadcasts = 50

	// Seed each room with persistent clients.
	allClients := make(map[string][]*fakeClient)
	for _, r := range rooms {
		for i := 0; i < perRoom; i++ {
			c := newFakeClient(fmt.Sprintf("%s-%d", r, i))
			h.Register(r, c)
			allClients[r] = append(allClients[r], c)
		}
	}

	var wg sync.WaitGroup
	for _, r := range rooms {
		r := r
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < broadcasts; i++ {
				h.Broadcast(r, []byte("wave"))
			}
		}()
	}
	wg.Wait()

	for _, r := range rooms {
		for _, c := range allClients[r] {
			if got := len(c.messages()); got != broadcasts {
				t.Errorf("room %s client %s: got %d msgs, want %d", r, c.id, got, broadcasts)
			}
		}
	}
}
