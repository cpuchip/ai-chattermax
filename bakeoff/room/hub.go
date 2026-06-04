// Package room provides a concurrency-safe Hub for room-scoped message broadcast.
// It is transport-agnostic: clients implement the Client interface with Send and ID.
package room

import "sync"

// Client is the transport-agnostic interface for a chat participant.
// Implementations must be safe for concurrent calls to Send.
type Client interface {
	// ID returns a unique identifier for this client.
	ID() string
	// Send delivers a message to the client. Implementations should not block
	// indefinitely; the Hub will drop slow clients that return errors.
	Send([]byte) error
}

// Hub provides room-scoped broadcast. All state mutations flow through a
// single goroutine (Run), so no additional locking is needed for the room maps.
// Clients that return errors from Send are automatically removed from the room.
type Hub struct {
	mu       sync.RWMutex
	started  bool

	rooms    map[string]map[string]Client // roomID → clientID → Client
	ops      chan op
}

// op represents an operation sent to the Hub's Run loop.
type op struct {
	kind     opKind
	roomID   string
	client   Client
	message  []byte
	result   chan struct{} // signaled when the op is processed
}

type opKind int

const (
	opRegister opKind = iota
	opUnregister
	opBroadcast
	opRooms
)

// NewHub creates a new Hub. Call Run() in a goroutine to start processing.
func NewHub() *Hub {
	return &Hub{
		rooms: make(map[string]map[string]Client),
		ops:   make(chan op, 64),
	}
}

// Run starts the Hub's event loop. It blocks until the Hub is stopped.
// Call this in a separate goroutine: go hub.Run().
func (h *Hub) Run() {
	for o := range h.ops {
		switch o.kind {
		case opRegister:
			if h.rooms[o.roomID] == nil {
				h.rooms[o.roomID] = make(map[string]Client)
			}
			h.rooms[o.roomID][o.client.ID()] = o.client
			if o.result != nil {
				o.result <- struct{}{}
			}

		case opUnregister:
			if clients, ok := h.rooms[o.roomID]; ok {
				delete(clients, o.client.ID())
				if len(clients) == 0 {
					delete(h.rooms, o.roomID)
				}
			}
			if o.result != nil {
				o.result <- struct{}{}
			}

		case opBroadcast:
			clients := h.rooms[o.roomID]
			// Collect clients to drop (can't modify map during iteration)
			var toDrop []string
			for id, c := range clients {
				if err := c.Send(o.message); err != nil {
					toDrop = append(toDrop, id)
				}
			}
			for _, id := range toDrop {
				delete(clients, id)
			}
			if len(clients) == 0 {
				delete(h.rooms, o.roomID)
			}
			if o.result != nil {
				o.result <- struct{}{}
			}

		case opRooms:
			if o.result != nil {
				o.result <- struct{}{}
			}
		}
	}
}

// Stop shuts down the Hub's event loop.
func (h *Hub) Stop() {
	close(h.ops)
}

// Register adds a client to a room.
func (h *Hub) Register(roomID string, c Client) {
	h.ops <- op{kind: opRegister, roomID: roomID, client: c}
}

// RegisterSync registers a client and blocks until processed.
func (h *Hub) RegisterSync(roomID string, c Client) {
	done := make(chan struct{})
	h.ops <- op{kind: opRegister, roomID: roomID, client: c, result: done}
	<-done
}

// Unregister removes a client from a room. No-op if the client is not in the room.
func (h *Hub) Unregister(roomID string, c Client) {
	h.ops <- op{kind: opUnregister, roomID: roomID, client: c}
}

// UnregisterSync removes a client and blocks until processed.
func (h *Hub) UnregisterSync(roomID string, c Client) {
	done := make(chan struct{})
	h.ops <- op{kind: opUnregister, roomID: roomID, client: c, result: done}
	<-done
}

// Broadcast sends a message to all clients in a room by calling Client.Send.
// Clients that return errors from Send are automatically dropped.
func (h *Hub) Broadcast(roomID string, message []byte) {
	h.ops <- op{kind: opBroadcast, roomID: roomID, message: message}
}

// BroadcastSync broadcasts and blocks until processed.
func (h *Hub) BroadcastSync(roomID string, message []byte) {
	done := make(chan struct{})
	h.ops <- op{kind: opBroadcast, roomID: roomID, message: message, result: done}
	<-done
}

// Rooms returns the IDs of all rooms that currently have at least one client.
func (h *Hub) Rooms() []string {
	done := make(chan struct{})
	h.ops <- op{kind: opRooms, result: done}
	<-done

	h.mu.RLock()
	defer h.mu.RUnlock()
	result := make([]string, 0, len(h.rooms))
	for id := range h.rooms {
		result = append(result, id)
	}
	return result
}