package gateway

import (
	"sync"

	"github.com/gorilla/websocket"
)

// Client is one live gateway connection.
type Client struct {
	hub    *Hub
	conn   *websocket.Conn
	send   chan []byte
	who    Participant
	mu     sync.Mutex
	closed bool
	subs   map[string]bool // channels this client is subscribed to
}

// Hub fans messages out to the clients subscribed to each channel.
type Hub struct {
	mu      sync.RWMutex
	clients map[*Client]bool
	subs    map[string]map[*Client]bool // channel -> subscribers
}

// NewHub builds an empty hub.
func NewHub() *Hub {
	return &Hub{
		clients: make(map[*Client]bool),
		subs:    make(map[string]map[*Client]bool),
	}
}

func (h *Hub) register(c *Client) {
	h.mu.Lock()
	h.clients[c] = true
	h.mu.Unlock()
}

// unregister removes a client from the hub and every channel, returning the
// channels it had been subscribed to (so the caller can emit presence leaves).
func (h *Hub) unregister(c *Client) []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients, c)
	var chans []string
	for ch := range c.subs {
		chans = append(chans, ch)
		if set := h.subs[ch]; set != nil {
			delete(set, c)
			if len(set) == 0 {
				delete(h.subs, ch)
			}
		}
	}
	return chans
}

// subscribe adds a client to a channel. Returns false if it was already subscribed.
func (h *Hub) subscribe(c *Client, channel string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if c.subs[channel] {
		return false
	}
	c.subs[channel] = true
	if h.subs[channel] == nil {
		h.subs[channel] = make(map[*Client]bool)
	}
	h.subs[channel][c] = true
	return true
}

// broadcast sends payload to every client subscribed to channel, except `except`.
func (h *Hub) broadcast(channel string, payload []byte, except *Client) {
	h.mu.RLock()
	subs := h.subs[channel]
	targets := make([]*Client, 0, len(subs))
	for c := range subs {
		if c != except {
			targets = append(targets, c)
		}
	}
	h.mu.RUnlock()
	for _, c := range targets {
		c.enqueue(payload)
	}
}

// roster returns the distinct participants subscribed to a channel (deduped by ID).
func (h *Hub) roster(channel string) []Participant {
	h.mu.RLock()
	defer h.mu.RUnlock()
	seen := make(map[string]bool)
	var out []Participant
	for c := range h.subs[channel] {
		if !seen[c.who.ID] {
			seen[c.who.ID] = true
			out = append(out, c.who)
		}
	}
	return out
}

// enqueue pushes a payload to the client's send buffer, dropping it if the
// buffer is full — a slow client must not block the hub. The closed-flag guard
// makes enqueue and closeSend mutually exclusive, so a broadcast can never send
// on a closed channel (the mutex-hub races the read pump's cleanup otherwise).
func (c *Client) enqueue(payload []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	select {
	case c.send <- payload:
	default:
		// Buffer full — drop this frame rather than block the hub.
	}
}

// closeSend closes the send channel exactly once so the write pump exits.
func (c *Client) closeSend() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.closed {
		c.closed = true
		close(c.send)
	}
}
