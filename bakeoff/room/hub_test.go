package room

import (
	"fmt"
	"sync"
	"testing"
)

// fakeClient is an in-memory Client for testing.
type fakeClient struct {
	id       string
	messages [][]byte
	mu       sync.Mutex
	failNext bool
}

func newFakeClient(id string) *fakeClient {
	return &fakeClient{id: id}
}

func (c *fakeClient) ID() string {
	return c.id
}

func (c *fakeClient) Send(data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.failNext {
		c.failNext = false
		return fmt.Errorf("send failed")
	}
	c.messages = append(c.messages, data)
	return nil
}

func (c *fakeClient) Messages() [][]byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([][]byte, len(c.messages))
	copy(out, c.messages)
	return out
}

func TestHub_RegisterAndBroadcast(t *testing.T) {
	h := NewHub()
	c1 := newFakeClient("c1")
	c2 := newFakeClient("c2")

	h.Register("room-a", c1)
	h.Register("room-a", c2)

	h.Broadcast("room-a", []byte("hello"))

	if got := len(c1.Messages()); got != 1 {
		t.Fatalf("c1 expected 1 message, got %d", got)
	}
	if got := len(c2.Messages()); got != 1 {
		t.Fatalf("c2 expected 1 message, got %d", got)
	}
}

func TestHub_CrossRoomIsolation(t *testing.T) {
	h := NewHub()
	cA := newFakeClient("cA")
	cB := newFakeClient("cB")

	h.Register("room-a", cA)
	h.Register("room-b", cB)

	h.Broadcast("room-a", []byte("only-a"))

	if got := len(cA.Messages()); got != 1 {
		t.Fatalf("cA expected 1 message, got %d", got)
	}
	if got := len(cB.Messages()); got != 0 {
		t.Fatalf("cB expected 0 messages, got %d", got)
	}
}

func TestHub_Unregister(t *testing.T) {
	h := NewHub()
	c1 := newFakeClient("c1")

	h.Register("room-a", c1)
	h.Unregister("room-a", c1)
	h.Broadcast("room-a", []byte("after-unregister"))

	if got := len(c1.Messages()); got != 0 {
		t.Fatalf("c1 expected 0 messages after unregister, got %d", got)
	}
}

func TestHub_BroadcastDropsFailedClients(t *testing.T) {
	h := NewHub()
	c1 := newFakeClient("c1")
	c2 := newFakeClient("c2")
	c2.failNext = true

	h.Register("room-a", c1)
	h.Register("room-a", c2)

	h.Broadcast("room-a", []byte("msg"))

	if got := len(c1.Messages()); got != 1 {
		t.Fatalf("c1 expected 1 message, got %d", got)
	}

	// c2 should have been dropped; broadcast again and c1 still gets it, c2 doesn't
	h.Broadcast("room-a", []byte("msg2"))
	if got := len(c1.Messages()); got != 2 {
		t.Fatalf("c1 expected 2 messages, got %d", got)
	}
	if got := len(c2.Messages()); got != 0 {
		t.Fatalf("c2 expected 0 messages (dropped), got %d", got)
	}
}

func TestHub_ConcurrentStress(t *testing.T) {
	h := NewHub()
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
				clientID := fmt.Sprintf("client-%d-%d", g, j)
				client := newFakeClient(clientID)
				h.Register(roomID, client)
				h.Broadcast(roomID, []byte("hello"))
				h.Unregister(roomID, client)
			}
		}(i)
	}
	wg.Wait()
}
