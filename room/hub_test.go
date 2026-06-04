package room

import (
	"reflect"
	"sync"
	"testing"
)

// fakeClient is an in-memory Client for testing.
type fakeClient struct {
	id       string
	mu       sync.Mutex
	messages [][]byte
}

func newFakeClient(id string) *fakeClient {
	return &fakeClient{id: id}
}

func (fc *fakeClient) ID() string {
	return fc.id
}

func (fc *fakeClient) Send(message []byte) error {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	fc.messages = append(fc.messages, append([]byte(nil), message...))
	return nil
}

func (fc *fakeClient) Messages() [][]byte {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	out := make([][]byte, len(fc.messages))
	for i, m := range fc.messages {
		out[i] = append([]byte(nil), m...)
	}
	return out
}

func TestHub_RegisterAndBroadcast(t *testing.T) {
	tests := []struct {
		name        string
		setup       func() (*Hub, *fakeClient, *fakeClient)
		broadcastTo string
		message     []byte
		wantA       [][]byte
		wantB       [][]byte
	}{
		{
			name: "broadcast sends to all clients in room",
			setup: func() (*Hub, *fakeClient, *fakeClient) {
				h := NewHub()
				a := newFakeClient("a")
				b := newFakeClient("b")
				h.Register("r1", a)
				h.Register("r1", b)
				return h, a, b
			},
			broadcastTo: "r1",
			message:     []byte("hello"),
			wantA:       [][]byte{[]byte("hello")},
			wantB:       [][]byte{[]byte("hello")},
		},
		{
			name: "client in another room does not receive",
			setup: func() (*Hub, *fakeClient, *fakeClient) {
				h := NewHub()
				a := newFakeClient("a")
				b := newFakeClient("b")
				h.Register("r1", a)
				h.Register("r2", b)
				return h, a, b
			},
			broadcastTo: "r1",
			message:     []byte("hello"),
			wantA:       [][]byte{[]byte("hello")},
			wantB:       [][]byte{},
		},
		{
			name: "unregister removes client from room",
			setup: func() (*Hub, *fakeClient, *fakeClient) {
				h := NewHub()
				a := newFakeClient("a")
				b := newFakeClient("b")
				h.Register("r1", a)
				h.Register("r1", b)
				h.Unregister("r1", a)
				return h, a, b
			},
			broadcastTo: "r1",
			message:     []byte("hello"),
			wantA:       [][]byte{},
			wantB:       [][]byte{[]byte("hello")},
		},
		{
			name: "broadcast to empty room does nothing",
			setup: func() (*Hub, *fakeClient, *fakeClient) {
				h := NewHub()
				a := newFakeClient("a")
				b := newFakeClient("b")
				return h, a, b
			},
			broadcastTo: "r1",
			message:     []byte("hello"),
			wantA:       [][]byte{},
			wantB:       [][]byte{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, a, b := tt.setup()
			h.Broadcast(tt.broadcastTo, tt.message)

			if got := a.Messages(); !reflect.DeepEqual(got, tt.wantA) {
				t.Errorf("client A messages = %v, want %v", got, tt.wantA)
			}
			if got := b.Messages(); !reflect.DeepEqual(got, tt.wantB) {
				t.Errorf("client B messages = %v, want %v", got, tt.wantB)
			}
		})
	}
}

func TestHub_ConcurrentAccess(t *testing.T) {
	h := NewHub()
	fc := newFakeClient("c1")

	// Exercise concurrent register, unregister, and broadcast.
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(3)
		go func() {
			defer wg.Done()
			h.Register("room", fc)
		}()
		go func() {
			defer wg.Done()
			h.Unregister("room", fc)
		}()
		go func() {
			defer wg.Done()
			h.Broadcast("room", []byte("msg"))
		}()
	}
	wg.Wait()

	// Verify we don't panic or deadlock.
}
