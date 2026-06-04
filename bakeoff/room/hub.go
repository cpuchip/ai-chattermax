// Package room provides a concurrency-safe, transport-agnostic broadcast
// hub for multi-room chat. A Hub maps a roomID to a set of Clients, each
// of which exposes only the two methods needed to send to it: an
// identifier and a non-blocking send. The Hub is safe for concurrent use
// by any number of goroutines.
package room

import "sync"

// Client is the transport-agnostic interface a Hub uses to reach a
// single participant. ID must be unique within a room and stable for
// the client's lifetime in the hub. Send must not block indefinitely:
// the Hub will drop a client whose Send returns an error, and a Send
// that blocks will block the Hub. Implementations should buffer or
// otherwise keep Send non-blocking.
type Client interface {
	// ID returns a stable identifier for this client. Two clients in
	// the same room must have distinct IDs.
	ID() string
	// Send delivers message to the client. A non-nil error causes the
	// Hub to drop the client from its room on the same Broadcast call.
	// Send may retain a reference to message; callers that intend to
	// reuse the underlying buffer should copy before passing it in.
	Send(message []byte) error
}

// Hub is a multi-room broadcast hub. The zero value is not usable;
// obtain one with NewHub. A Hub is safe for concurrent use.
type Hub struct {
	mu    sync.Mutex
	rooms map[string]map[string]Client
}

// NewHub returns an empty Hub ready for use.
func NewHub() *Hub {
	return &Hub{rooms: make(map[string]map[string]Client)}
}

// Register adds client to the room identified by roomID. If roomID does
// not yet exist it is created. If a client with the same ID is already
// registered in the room, it is replaced. Register is a no-op for a
// nil client.
func (h *Hub) Register(roomID string, client Client) {
	if client == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	room, ok := h.rooms[roomID]
	if !ok {
		room = make(map[string]Client)
		h.rooms[roomID] = room
	}
	room[client.ID()] = client
}

// Unregister removes client from the room identified by roomID. It is a
// no-op if the client is not currently registered or if the room does
// not exist. If the room becomes empty as a result, the room itself is
// removed. Unregister is a no-op for a nil client.
func (h *Hub) Unregister(roomID string, client Client) {
	if client == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	room, ok := h.rooms[roomID]
	if !ok {
		return
	}
	delete(room, client.ID())
	if len(room) == 0 {
		delete(h.rooms, roomID)
	}
}

// Broadcast delivers message to every client currently registered in
// the room identified by roomID. Clients whose Send returns a non-nil
// error are dropped from the room before Broadcast returns. Broadcast
// is a no-op for an unknown or empty room. The Hub takes a defensive
// copy of message so callers may safely reuse their buffer after the
// call returns.
func (h *Hub) Broadcast(roomID string, message []byte) {
	body := append([]byte(nil), message...)
	h.mu.Lock()
	defer h.mu.Unlock()
	room, ok := h.rooms[roomID]
	if !ok {
		return
	}
	for id, client := range room {
		if err := client.Send(body); err != nil {
			delete(room, id)
		}
	}
	if len(room) == 0 {
		delete(h.rooms, roomID)
	}
}

// Len returns the number of clients currently registered in roomID.
func (h *Hub) Len(roomID string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.rooms[roomID])
}

// Rooms returns the IDs of all rooms that currently have at least one
// registered client. The order is unspecified.
func (h *Hub) Rooms() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]string, 0, len(h.rooms))
	for id := range h.rooms {
		out = append(out, id)
	}
	return out
}
