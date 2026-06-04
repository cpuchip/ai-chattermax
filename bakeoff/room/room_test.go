package room

import (
	"errors"
	"sync"
	"testing"
)

// stubClient is a test implementation of Client.
type stubClient struct {
	id      string
	mu      sync.Mutex
	msgs    [][]byte
	sendErr error
	// ch is an optional channel to signal each received message
	ch chan []byte
}

func newStub(id string) *stubClient {
	return &stubClient{id: id}
}

func newStubWithChan(id string) *stubClient {
	return &stubClient{id: id, ch: make(chan []byte, 100)}
}

func (c *stubClient) ID() string { return c.id }

func (c *stubClient) Send(msg []byte) error {
	c.mu.Lock()
	c.msgs = append(c.msgs, append([]byte(nil), msg...))
	err := c.sendErr
	c.mu.Unlock()
	if c.ch != nil {
		c.ch <- msg
	}
	return err
}

func (c *stubClient) received() [][]byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([][]byte, len(c.msgs))
	copy(out, c.msgs)
	return out
}

func (c *stubClient) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.msgs)
}

func TestHubRegisterAndBroadcast(t *testing.T) {
	tests := []struct {
		name       string
		roomID     string
		setup      func(h *Hub) []*stubClient
		broadcasts [][]byte
		wantCounts []int // per-client expected message count
	}{
		{
			name:   "single client single broadcast",
			roomID: "room-a",
			setup: func(h *Hub) []*stubClient {
				c := newStub("c1")
				h.Register("room-a", c)
				return []*stubClient{c}
			},
			broadcasts: [][]byte{[]byte("hello")},
			wantCounts:  []int{1},
		},
		{
			name:   "two clients single broadcast",
			roomID: "room-a",
			setup: func(h *Hub) []*stubClient {
				c1 := newStub("c1")
				c2 := newStub("c2")
				h.Register("room-a", c1)
				h.Register("room-a", c2)
				return []*stubClient{c1, c2}
			},
			broadcasts: [][]byte{[]byte("hello")},
			wantCounts:  []int{1, 1},
		},
		{
			name:   "broadcast to empty room",
			roomID: "empty",
			setup: func(h *Hub) []*stubClient {
				return nil
			},
			broadcasts: [][]byte{[]byte("nobody")},
			wantCounts:  nil,
		},
		{
			name:   "room isolation",
			roomID: "room-a",
			setup: func(h *Hub) []*stubClient {
				c1 := newStub("c1")
				c2 := newStub("c2")
				h.Register("room-a", c1)
				h.Register("room-b", c2)
				return []*stubClient{c1, c2}
			},
			broadcasts: [][]byte{[]byte("only-a")},
			wantCounts:  []int{1, 0}, // c1 gets it, c2 does not
		},
		{
			name:   "multiple broadcasts",
			roomID: "room-a",
			setup: func(h *Hub) []*stubClient {
				c := newStub("c1")
				h.Register("room-a", c)
				return []*stubClient{c}
			},
			broadcasts: [][]byte{[]byte("m1"), []byte("m2"), []byte("m3")},
			wantCounts:  []int{3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHub()
			defer h.Close()

			clients := tt.setup(h)

			for _, msg := range tt.broadcasts {
				h.Broadcast(tt.roomID, msg)
			}

			// Give the run loop time to process (unbuffered channel, sequential)
			// Use a final broadcast to a dummy room as a sync point.
			h.Broadcast("__sync__", nil)

			for i, c := range clients {
				got := c.count()
				want := tt.wantCounts[i]
				if got != want {
					t.Errorf("client %s: got %d messages, want %d", c.id, got, want)
				}
			}
		})
	}
}

func TestHubUnregister(t *testing.T) {
	t.Run("unregistered client not reached by broadcast", func(t *testing.T) {
		h := NewHub()
		defer h.Close()

		c := newStub("c1")
		h.Register("room-a", c)
		h.Unregister("room-a", c)
		h.Broadcast("room-a", []byte("hello"))
		h.Broadcast("__sync__", nil)

		if c.count() != 0 {
			t.Errorf("unregistered client received %d messages, want 0", c.count())
		}
	})

	t.Run("unregister non-present client is silent no-op", func(t *testing.T) {
		h := NewHub()
		defer h.Close()

		c := newStub("c1")
		h.Unregister("room-a", c) // never registered
		h.Broadcast("__sync__", nil)

		// No panic, no error
	})

	t.Run("unregister from wrong room leaves client in original room", func(t *testing.T) {
		h := NewHub()
		defer h.Close()

		c := newStub("c1")
		h.Register("room-a", c)
		h.Unregister("room-b", c) // wrong room
		h.Broadcast("room-a", []byte("hello"))
		h.Broadcast("__sync__", nil)

		if c.count() != 1 {
			t.Errorf("client in wrong-room unregister: got %d, want 1", c.count())
		}
	})
}

func TestHubSendErrorDropsClient(t *testing.T) {
	h := NewHub()
	defer h.Close()

	c1 := newStub("c1")
	c2 := newStub("c2")
	c2.sendErr = errors.New("gone")

	h.Register("room-a", c1)
	h.Register("room-a", c2)

	h.Broadcast("room-a", []byte("hello"))

	// Second broadcast should only reach c1
	h.Broadcast("room-a", []byte("world"))
	h.Broadcast("__sync__", nil)

	if c1.count() != 2 {
		t.Errorf("healthy client: got %d messages, want 2", c1.count())
	}
	if c2.count() != 1 {
		t.Errorf("faulty client: got %d messages, want 1 (dropped after first Send error)", c2.count())
	}
}

func TestHubClose(t *testing.T) {
	h := NewHub()
	h.Close()

	// Second close must not panic
	h.Close()
}

func TestHubConcurrent(t *testing.T) {
	h := NewHub()
	defer h.Close()

	const numClients = 20
	const numBroadcasts = 100

	clients := make([]*stubClient, numClients)
	for i := range numClients {
		clients[i] = newStub("c")
		h.Register("room", clients[i])
	}

	// Sync: register a final client and wait for registration to settle using a broadcast.
	h.Broadcast("__sync__", nil)

	var wg sync.WaitGroup
	wg.Add(numBroadcasts)

	for i := range numBroadcasts {
		go func(n int) {
			defer wg.Done()
			h.Broadcast("room", []byte{byte(n)})
		}(i)
	}

	wg.Wait()

	// Wait for all broadcasts to be processed.
	h.Broadcast("__sync__", nil)

	total := 0
	for _, c := range clients {
		total += c.count()
	}

	// Each of numBroadcasts should reach all numClients clients.
	expected := numBroadcasts * numClients
	if total != expected {
		t.Errorf("total messages received = %d, want %d", total, expected)
	}
}
