package room

import (
	"errors"
	"sync"
	"testing"
)

// fakeClient is a test double implementing Client.
type fakeClient struct {
	id       string
	mu       sync.Mutex
	sent     [][]byte
	failNext bool
}

func (f *fakeClient) ID() string { return f.id }

func (f *fakeClient) Send(b []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failNext {
		f.failNext = false
		return errors.New("send error")
	}
	f.sent = append(f.sent, append([]byte(nil), b...))
	return nil
}

func TestHubRegisterAndBroadcast(t *testing.T) {
	h := NewHub()
	c1 := &fakeClient{id: "c1"}
	c2 := &fakeClient{id: "c2"}

	h.Register("r1", c1)
	h.Register("r1", c2)

	h.Broadcast("r1", []byte("hello"))

	if len(c1.sent) != 1 || string(c1.sent[0]) != "hello" {
		t.Fatalf("c1 expected hello, got %v", c1.sent)
	}
	if len(c2.sent) != 1 || string(c2.sent[0]) != "hello" {
		t.Fatalf("c2 expected hello, got %v", c2.sent)
	}
}

func TestHubBroadcastIsRoomScoped(t *testing.T) {
	h := NewHub()
	c1 := &fakeClient{id: "c1"}
	c2 := &fakeClient{id: "c2"}

	h.Register("r1", c1)
	h.Register("r2", c2)

	h.Broadcast("r1", []byte("room1-only"))

	if len(c1.sent) != 1 || string(c1.sent[0]) != "room1-only" {
		t.Fatalf("c1 expected room1-only, got %v", c1.sent)
	}
	if len(c2.sent) != 0 {
		t.Fatalf("c2 expected no messages, got %v", c2.sent)
	}
}

func TestHubUnregister(t *testing.T) {
	h := NewHub()
	c1 := &fakeClient{id: "c1"}

	h.Register("r1", c1)
	h.Unregister("r1", c1)
	h.Broadcast("r1", []byte("after-unregister"))

	if len(c1.sent) != 0 {
		t.Fatalf("c1 expected no messages after unregister, got %v", c1.sent)
	}
}

func TestHubUnregisterUnknownRoomIsSafe(t *testing.T) {
	h := NewHub()
	c1 := &fakeClient{id: "c1"}

	// Should not panic.
	h.Unregister("nonexistent", c1)
}

func TestHubBroadcastDropsFailingClient(t *testing.T) {
	h := NewHub()
	c1 := &fakeClient{id: "c1", failNext: true}
	c2 := &fakeClient{id: "c2"}

	h.Register("r1", c1)
	h.Register("r1", c2)

	h.Broadcast("r1", []byte("msg"))

	if len(c2.sent) != 1 {
		t.Fatalf("c2 expected 1 message, got %d", len(c2.sent))
	}

	// c1 was dropped; subsequent broadcasts should not reach it.
	h.Broadcast("r1", []byte("msg2"))
	if len(c1.sent) != 0 {
		t.Fatalf("c1 expected 0 messages after drop, got %d", len(c1.sent))
	}
}

func TestHubBroadcastEmptyRoomIsSafe(t *testing.T) {
	h := NewHub()
	// Should not panic.
	h.Broadcast("empty", []byte("msg"))
}

func TestHubConcurrentStress(t *testing.T) {
	h := NewHub()
	const rooms = 10
	const clientsPerRoom = 50
	const iterations = 100

	var wg sync.WaitGroup
	for r := 0; r < rooms; r++ {
		roomID := string(rune('a' + r))
		for c := 0; c < clientsPerRoom; c++ {
			client := &fakeClient{id: string(rune('A' + c)) + "-" + roomID}
			h.Register(roomID, client)
		}
	}

	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func(iter int) {
			defer wg.Done()
			for r := 0; r < rooms; r++ {
				roomID := string(rune('a' + r))
				h.Broadcast(roomID, []byte("iter"))
			}
		}(i)
	}

	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func(iter int) {
			defer wg.Done()
			for r := 0; r < rooms; r++ {
				roomID := string(rune('a' + r))
				c := &fakeClient{id: "dynamic-" + string(rune('0'+iter%10))}
				h.Register(roomID, c)
				h.Unregister(roomID, c)
			}
		}(i)
	}

	wg.Wait()
}
