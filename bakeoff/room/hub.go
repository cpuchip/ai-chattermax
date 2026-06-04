// Package room provides a concurrency-safe, multi-room broadcast hub.
//
// The Hub is transport-agnostic: clients implement the Client interface
// (ID + Send) and the hub handles room-scoped registration and broadcast.
package room

import "sync"

// Client is the transport-agnostic interface for a connected participant.
type Client interface {
	// ID returns a unique identifier for this client.
	ID() string
	// Send delivers a message to the client. Implementations must be
	// safe for concurrent use.
	Send([]byte) error
}

// Hub manages room-scoped client membership and broadcast.
// A zero Hub is not valid; use NewHub.
type Hub struct {
	mu    sync.RWMutex
	rooms map[string]map[string]Client
}

// NewHub returns an initialised Hub.
func NewHub() *Hub {
	return &Hub{rooms: make(map[string]map[string]Client)}
}

// Register adds a client to a room. Registering the same client twice in
// the same room is a no-op.
func (h *Hub) Register(roomID string, c Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	room, ok := h.rooms[roomID]
	if !ok {
		room = make(map[string]Client)
		h.rooms[roomID] = room
	}
	room[c.ID()] = c
}

// Unregister removes a client from a room. If the room becomes empty it is
// deleted from the hub. Unregistering a client that is not in the room is
// a no-op.
func (h *Hub) Unregister(roomID string, c Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	room, ok := h.rooms[roomID]
	if !ok {
		return
	}
	delete(room, c.ID())
	if len(room) == 0 {
		delete(h.rooms, roomID)
	}
}

// Broadcast sends msg to every client currently registered in roomID.
// A client whose Send returns an error is left in the room — callers that
// want auto-removal on error should wrap Broadcast with their own policy.
// Broadcasting to an empty or unknown room is a no-op.
func (h *Hub) Broadcast(roomID string, msg []byte) {
	h.mu.RLock()
	room, ok := h.rooms[roomID]
	if !ok || len(room) == 0 {
		h.mu.RUnlock()
		return
	}
	// Snapshot under the read lock so Send calls happen without holding it.
	targets := make([]Client, 0, len(room))
	for _, c := range room {
		targets = append(targets, c)
	}
	h.mu.RUnlock()

	for _, c := range targets {
		_ = c.Send(msg)
	}
}
