package gateway

import "testing"

func newTestClient(id string) *Client {
	return &Client{
		send: make(chan []byte, 8),
		who:  Participant{ID: id, Name: id, Kind: "human"},
		subs: make(map[string]bool),
	}
}

func drained(c *Client) bool {
	select {
	case <-c.send:
		return true
	default:
		return false
	}
}

func TestHubSubscribeRosterBroadcast(t *testing.T) {
	h := NewHub()
	a, b := newTestClient("a"), newTestClient("b")
	h.register(a)
	h.register(b)
	h.subscribe(a, "r1")
	h.subscribe(b, "r1")

	roster := h.roster("r1")
	if len(roster) != 2 {
		t.Fatalf("roster = %d, want 2", len(roster))
	}

	// broadcast to r1 except a → only b receives.
	h.broadcast("r1", []byte(`{"x":1}`), a)
	if drained(a) {
		t.Error("sender a should not receive its own broadcast")
	}
	if !drained(b) {
		t.Error("b should have received the broadcast")
	}
}

func TestHubSubscribeIdempotent(t *testing.T) {
	h := NewHub()
	a := newTestClient("a")
	h.register(a)
	if !h.subscribe(a, "r1") {
		t.Error("first subscribe should report true")
	}
	if h.subscribe(a, "r1") {
		t.Error("re-subscribe should report false")
	}
	if got := len(h.roster("r1")); got != 1 {
		t.Errorf("roster deduped = %d, want 1", got)
	}
}

func TestHubUnregisterCleansChannels(t *testing.T) {
	h := NewHub()
	a, b := newTestClient("a"), newTestClient("b")
	h.register(a)
	h.register(b)
	h.subscribe(a, "r1")
	h.subscribe(b, "r1")

	chans := h.unregister(a)
	if len(chans) != 1 || chans[0] != "r1" {
		t.Fatalf("unregister returned %v, want [r1]", chans)
	}
	roster := h.roster("r1")
	if len(roster) != 1 || roster[0].ID != "b" {
		t.Errorf("after unregister roster = %v, want only b", roster)
	}
	// a broadcast no longer reaches a.
	h.broadcast("r1", []byte("x"), nil)
	if drained(a) {
		t.Error("unregistered client should not receive broadcasts")
	}
	if !drained(b) {
		t.Error("b should still receive")
	}
}

func TestEnqueueAfterCloseIsSafe(t *testing.T) {
	c := newTestClient("a")
	c.closeSend()
	// must not panic (closed guard).
	c.enqueue([]byte("x"))
}
