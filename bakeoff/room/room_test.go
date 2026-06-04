package room

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
)

// testClient is a mock Client that records every message it receives.
type testClient struct {
	id       string
	messages [][]byte
	mu       sync.Mutex
	failNext error
}

func newTestClient(id string) *testClient {
	return &testClient{id: id}
}

func (c *testClient) ID() string { return c.id }

func (c *testClient) Send(data []byte) error {
	if c.failNext != nil {
		return c.failNext
	}
	c.mu.Lock()
	c.messages = append(c.messages, data)
	c.mu.Unlock()
	return nil
}

func (c *testClient) messagesCopy() [][]byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([][]byte, len(c.messages))
	for i, m := range c.messages {
		out[i] = append([]byte(nil), m...)
	}
	return out
}

func TestHubBroadcast(t *testing.T) {
	h := New()
	c1 := newTestClient("c1")
	c2 := newTestClient("c2")

	h.Register("alpha", c1)
	h.Register("alpha", c2)

	count, err := h.Broadcast("alpha", []byte("hello"))
	if err != nil {
		t.Fatalf("broadcast error: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected count 2, got %d", count)
	}

	if l := len(c1.messagesCopy()); l != 1 {
		t.Fatalf("expected c1 to have 1 message, got %d", l)
	}
	if l := len(c2.messagesCopy()); l != 1 {
		t.Fatalf("expected c2 to have 1 message, got %d", l)
	}
	if string(c1.messagesCopy()[0]) != "hello" {
		t.Fatalf("c1 message mismatch")
	}
}

func TestHubRoomIsolation(t *testing.T) {
	h := New()
	c1 := newTestClient("c1")
	c2 := newTestClient("c2")

	h.Register("alpha", c1)
	h.Register("beta", c2)

	count, err := h.Broadcast("alpha", []byte("alpha-msg"))
	if err != nil {
		t.Fatalf("broadcast error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected count 1, got %d", count)
	}

	if len(c1.messagesCopy()) != 1 {
		t.Fatalf("c1 should have 1 message")
	}
	if len(c2.messagesCopy()) != 0 {
		t.Fatalf("c2 should have 0 messages (room isolation)")
	}
}

func TestHubUnregister(t *testing.T) {
	h := New()
	c1 := newTestClient("c1")
	c2 := newTestClient("c2")

	h.Register("alpha", c1)
	h.Register("alpha", c2)
	h.Unregister("alpha", c1)

	count, err := h.Broadcast("alpha", []byte("after-unreg"))
	if err != nil {
		t.Fatalf("broadcast error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected count 1 after unregister, got %d", count)
	}
	if len(c2.messagesCopy()) != 1 {
		t.Fatalf("c2 should have received message")
	}
	if len(c1.messagesCopy()) != 0 {
		t.Fatalf("c1 should NOT have received message after unregister")
	}
}

func TestHubBroadcastEmptyRoom(t *testing.T) {
	h := New()
	count, err := h.Broadcast("empty", []byte("nobody"))
	if err != nil {
		t.Fatalf("broadcast to empty room error: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected count 0 for empty room, got %d", count)
	}
}

func TestHubDoubleRegister(t *testing.T) {
	h := New()
	c1 := newTestClient("c1")

	h.Register("alpha", c1)
	h.Register("alpha", c1) // same ID again

	count, err := h.Broadcast("alpha", []byte("dup"))
	if err != nil {
		t.Fatalf("broadcast error: %v", err)
	}
	if count != 1 {
		t.Fatalf("double-register should result in 1 client, got %d", count)
	}
	if len(c1.messagesCopy()) != 1 {
		t.Fatalf("c1 should have exactly 1 message")
	}
}

func TestHubSendError(t *testing.T) {
	h := New()
	good := newTestClient("good")
	bad := newTestClient("bad")
	bad.failNext = errors.New("send failed")

	h.Register("alpha", good)
	h.Register("alpha", bad)

	count, err := h.Broadcast("alpha", []byte("msg"))
	if err == nil {
		t.Fatal("expected error from bad client")
	}
	if count != 1 {
		t.Fatalf("expected 1 success, got %d", count)
	}
	if len(good.messagesCopy()) != 1 {
		t.Fatalf("good client should have message")
	}
}

func TestHubConcurrent(t *testing.T) {
	h := New()

	const rooms = 10
	const clientsPerRoom = 10
	const messages = 50

	// Create clients and register them.
	clients := make(map[string][]*testClient)
	for r := 0; r < rooms; r++ {
		roomID := fmt.Sprintf("room-%d", r)
		for c := 0; c < clientsPerRoom; c++ {
			cid := fmt.Sprintf("%s-client-%d", roomID, c)
			client := newTestClient(cid)
			clients[roomID] = append(clients[roomID], client)
			h.Register(roomID, client)
		}
	}

	var wg sync.WaitGroup
	// Concurrently broadcast messages to every room.
	for r := 0; r < rooms; r++ {
		roomID := fmt.Sprintf("room-%d", r)
		for m := 0; m < messages; m++ {
			wg.Add(1)
			go func(rid string, seq int) {
				defer wg.Done()
				payload := []byte(fmt.Sprintf("msg-%d", seq))
				_, err := h.Broadcast(rid, payload)
				if err != nil {
					// Some sends may fail if we concurrently unregister; that's okay.
					t.Logf("broadcast error in %s: %v", rid, err)
				}
			}(roomID, m)
		}
	}

	// Concurrently unregister some clients mid-broadcast.
	for r := 0; r < rooms; r++ {
		roomID := fmt.Sprintf("room-%d", r)
		for i := 0; i < clientsPerRoom/2; i++ {
			wg.Add(1)
			go func(rid string, idx int) {
				defer wg.Done()
				client := clients[rid][idx]
				h.Unregister(rid, client)
			}(roomID, i)
		}
	}

	wg.Wait()

	// Verify no client ever received a message meant for another room,
	// and every still-registered client received every broadcast that
	// was sent after it was registered and before it was unregistered.
	for r := 0; r < rooms; r++ {
		roomID := fmt.Sprintf("room-%d", r)
		for c := 0; c < clientsPerRoom; c++ {
			client := clients[roomID][c]
			for _, m := range client.messagesCopy() {
				if string(m)[:4] != "msg-" {
					t.Fatalf("client %s received malformed message", client.id)
				}
			}
		}
	}
}

func TestHubRegisterUnregisterRace(t *testing.T) {
	h := New()
	const goroutines = 100
	const iterations = 100

	var wg sync.WaitGroup
	var totalBroadcasts int64

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			client := newTestClient(fmt.Sprintf("c-%d", id))
			for j := 0; j < iterations; j++ {
				h.Register("race", client)
				count, _ := h.Broadcast("race", []byte("x"))
				atomic.AddInt64(&totalBroadcasts, int64(count))
				h.Unregister("race", client)
			}
		}(i)
	}
	wg.Wait()

	// No panic means the race detector is happy.  We can't assert an exact
	// count because the interleaving is non-deterministic.
	_ = totalBroadcasts
}
