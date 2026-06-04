package room

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// testClient implements Client for tests. Send pushes to an observable channel.
type testClient struct {
	id   string
	send chan []byte
	mu   sync.Mutex
	err  error // if non-nil, Send returns this error
}

func (c *testClient) ID() string { return c.id }
func (c *testClient) Send(b []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.err != nil {
		return c.err
	}
	c.send <- b
	return nil
}

func newTestClient(id string) *testClient {
	return &testClient{id: id, send: make(chan []byte, 512)}
}

// failClient always returns an error from Send, simulating a disconnected client.
type failClient struct {
	id string
}

func (c *failClient) ID() string        { return c.id }
func (c *failClient) Send(_ []byte) error { return errClientFail }

var errClientFail = error(nil)
func init() { errClientFail = &clientFailError{} }

type clientFailError struct{}

func (e *clientFailError) Error() string { return "client send failed" }

func startHub() *Hub {
	h := NewHub()
	go h.Run()
	return h
}

// ---- Table-driven tests ----

func TestRegisterAndBroadcast(t *testing.T) {
	h := startHub()
	defer h.Stop()

	c1 := newTestClient("alice")
	c2 := newTestClient("bob")

	h.RegisterSync("room1", c1)
	h.RegisterSync("room1", c2)

	msg := []byte("hello room1")
	h.BroadcastSync("room1", msg)

	for _, c := range []*testClient{c1, c2} {
		select {
		case got := <-c.send:
			if string(got) != string(msg) {
				t.Errorf("%s: expected %q, got %q", c.id, msg, got)
			}
		case <-time.After(time.Second):
			t.Fatalf("%s: did not receive broadcast within 1s", c.id)
		}
	}
}

func TestRoomIsolation(t *testing.T) {
	h := startHub()
	defer h.Stop()

	c1 := newTestClient("alice")
	c2 := newTestClient("bob")

	h.RegisterSync("room1", c1)
	h.RegisterSync("room2", c2)

	h.BroadcastSync("room1", []byte("only room1"))

	// alice should get it
	select {
	case got := <-c1.send:
		if string(got) != "only room1" {
			t.Errorf("alice: expected 'only room1', got %q", got)
		}
	case <-time.After(time.Second):
		t.Fatal("alice: did not receive room1 broadcast")
	}

	// bob should NOT get it
	select {
	case <-c2.send:
		t.Fatal("bob: received broadcast from wrong room")
	case <-time.After(50 * time.Millisecond):
		// expected: no message
	}

	h.BroadcastSync("room2", []byte("only room2"))

	select {
	case got := <-c2.send:
		if string(got) != "only room2" {
			t.Errorf("bob: expected 'only room2', got %q", got)
		}
	case <-time.After(time.Second):
		t.Fatal("bob: did not receive room2 broadcast")
	}
}

func TestUnregister(t *testing.T) {
	h := startHub()
	defer h.Stop()

	c1 := newTestClient("alice")
	c2 := newTestClient("bob")

	h.RegisterSync("room1", c1)
	h.RegisterSync("room1", c2)

	h.UnregisterSync("room1", c1)

	h.BroadcastSync("room1", []byte("after leave"))

	// alice should NOT get it
	select {
	case <-c1.send:
		t.Fatal("alice: received broadcast after unregister")
	case <-time.After(50 * time.Millisecond):
		// expected
	}

	// bob should get it
	select {
	case got := <-c2.send:
		if string(got) != "after leave" {
			t.Errorf("bob: expected 'after leave', got %q", got)
		}
	case <-time.After(time.Second):
		t.Fatal("bob: did not receive broadcast after alice left")
	}
}

func TestUnregisterNonexistent(t *testing.T) {
	h := startHub()
	defer h.Stop()

	c := newTestClient("nobody")
	// Unregister a client that was never registered — should not panic.
	h.UnregisterSync("room1", c)
}

func TestDoubleRegister(t *testing.T) {
	h := startHub()
	defer h.Stop()

	c := newTestClient("alice")
	h.RegisterSync("room1", c)
	// Register again — replaces the old entry.
	h.RegisterSync("room1", c)

	h.BroadcastSync("room1", []byte("double"))

	// Should receive exactly one message.
	select {
	case <-c.send:
		// expected
	case <-time.After(time.Second):
		t.Fatal("alice: did not receive broadcast after double register")
	}

	select {
	case <-c.send:
		t.Fatal("alice: received duplicate broadcast")
	case <-time.After(50 * time.Millisecond):
		// expected: no second message
	}
}

func TestBroadcastToEmptyRoom(t *testing.T) {
	h := startHub()
	defer h.Stop()

	// Broadcasting to a room with no clients should not panic.
	h.BroadcastSync("empty-room", []byte("nobody here"))
}

func TestRooms(t *testing.T) {
	h := startHub()
	defer h.Stop()

	c1 := newTestClient("alice")
	c2 := newTestClient("bob")

	h.RegisterSync("room1", c1)
	h.RegisterSync("room2", c2)

	rooms := h.Rooms()
	if len(rooms) != 2 {
		t.Fatalf("expected 2 rooms, got %d: %v", len(rooms), rooms)
	}
	seen := map[string]bool{}
	for _, r := range rooms {
		seen[r] = true
	}
	if !seen["room1"] || !seen["room2"] {
		t.Fatalf("expected room1 and room2, got %v", rooms)
	}

	h.UnregisterSync("room1", c1)
	rooms = h.Rooms()
	if len(rooms) != 1 || rooms[0] != "room2" {
		t.Fatalf("expected only room2, got %v", rooms)
	}
}

func TestSlowClientDropped(t *testing.T) {
	h := startHub()
	defer h.Stop()

	fast := newTestClient("fast")
	fail := &failClient{id: "failing"}

	h.RegisterSync("room1", fast)
	h.RegisterSync("room1", fail)

	h.BroadcastSync("room1", []byte("msg1"))

	// fast should get the message
	select {
	case got := <-fast.send:
		if string(got) != "msg1" {
			t.Errorf("fast: expected 'msg1', got %q", got)
		}
	case <-time.After(time.Second):
		t.Fatal("fast: did not receive broadcast")
	}

	// The failing client should have been dropped from the room.
	// Send another broadcast — only fast should receive it.
	h.BroadcastSync("room1", []byte("msg2"))

	select {
	case got := <-fast.send:
		if string(got) != "msg2" {
			t.Errorf("fast: expected 'msg2', got %q", got)
		}
	case <-time.After(time.Second):
		t.Fatal("fast: did not receive second broadcast")
	}
}

// ---- Concurrent tests ----

func TestConcurrentRegisterBroadcast(t *testing.T) {
	h := startHub()
	defer h.Stop()

	const numClients = 50
	const numMessages = 100

	clients := make([]*testClient, numClients)
	for i := range clients {
		clients[i] = newTestClient("client-" + string(rune('A'+i%26)) + string(rune('0'+i/26)))
	}

	// Register all clients concurrently
	var wg sync.WaitGroup
	for _, c := range clients {
		wg.Add(1)
		go func(cl *testClient) {
			defer wg.Done()
			h.RegisterSync("room1", cl)
		}(c)
	}
	wg.Wait()

	// Broadcast concurrently
	var bwg sync.WaitGroup
	for i := 0; i < numMessages; i++ {
		bwg.Add(1)
		go func() {
			defer bwg.Done()
			h.Broadcast("room1", []byte("concurrent-msg"))
		}()
	}
	bwg.Wait()

	// Give time for all broadcasts to process
	time.Sleep(200 * time.Millisecond)

	// Each client should receive at least some messages
	totalReceived := 0
	for _, c := range clients {
	drain:
		for {
			select {
			case <-c.send:
				totalReceived++
			default:
				break drain
			}
		}
	}
	t.Logf("Total messages received across %d clients: %d (sent %d)", numClients, totalReceived, numMessages)
}

func TestConcurrentMultiRoom(t *testing.T) {
	h := startHub()
	defer h.Stop()

	const numRooms = 10
	const clientsPerRoom = 10

	type roomClient struct {
		roomID string
		client *testClient
	}
	allClients := make([]roomClient, 0, numRooms*clientsPerRoom)
	for r := 0; r < numRooms; r++ {
		roomID := string(rune('A' + r))
		for c := 0; c < clientsPerRoom; c++ {
			cl := newTestClient(roomID + "-" + string(rune('0'+c)))
			allClients = append(allClients, roomClient{roomID, cl})
		}
	}

	// Register all clients concurrently
	var wg sync.WaitGroup
	for _, rc := range allClients {
		wg.Add(1)
		go func(roomID string, cl *testClient) {
			defer wg.Done()
			h.RegisterSync(roomID, cl)
		}(rc.roomID, rc.client)
	}
	wg.Wait()

	// Broadcast to each room
	for r := 0; r < numRooms; r++ {
		roomID := string(rune('A' + r))
		h.BroadcastSync(roomID, []byte("msg-"+roomID))
	}

	// Each client should receive exactly its room's message
	for r := 0; r < numRooms; r++ {
		roomID := string(rune('A' + r))
		for c := 0; c < clientsPerRoom; c++ {
			cl := allClients[r*clientsPerRoom+c].client
			select {
			case got := <-cl.send:
				if string(got) != "msg-"+roomID {
					t.Errorf("%s: expected 'msg-%s', got %q", cl.id, roomID, got)
				}
			case <-time.After(time.Second):
				t.Fatalf("%s: did not receive broadcast", cl.id)
			}
		}
	}
}

func TestConcurrentUnregister(t *testing.T) {
	h := startHub()
	defer h.Stop()

	const numClients = 100
	clients := make([]*testClient, numClients)
	for i := range clients {
		clients[i] = newTestClient("client-" + string(rune('0'+i%10)) + string(rune('A'+i/10)))
		h.RegisterSync("room1", clients[i])
	}

	// Unregister all concurrently
	var wg sync.WaitGroup
	for _, c := range clients {
		wg.Add(1)
		go func(cl *testClient) {
			defer wg.Done()
			h.Unregister("room1", cl)
		}(c)
	}
	wg.Wait()

	// Give time for unregistrations to process
	time.Sleep(200 * time.Millisecond)

	// Broadcast should reach no one
	h.BroadcastSync("room1", []byte("nobody"))
	time.Sleep(100 * time.Millisecond)
}

func TestHighContentionBroadcast(t *testing.T) {
	h := startHub()
	defer h.Stop()

	const numClients = 20
	clients := make([]*testClient, numClients)
	for i := range clients {
		clients[i] = newTestClient("c-" + string(rune('A'+i)))
		h.RegisterSync("room1", clients[i])
	}

	const numBroadcasts = 500
	var sent atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < numBroadcasts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			h.Broadcast("room1", []byte("burst"))
			sent.Add(1)
		}()
	}
	wg.Wait()

	// Give time for all broadcasts to be processed
	time.Sleep(500 * time.Millisecond)

	// Count total received messages across all clients
	totalReceived := 0
	for _, c := range clients {
	drain:
		for {
			select {
			case <-c.send:
				totalReceived++
			default:
				break drain
			}
		}
	}

	t.Logf("Sent %d broadcasts, %d total received across %d clients", sent.Load(), totalReceived, numClients)
}