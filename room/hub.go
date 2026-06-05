package room

import (
	"sync"
)

// Client is a transport-agnostic participant in a room.
type Client interface {
	ID() string
	Send(message []byte) error
}

// Hub manages room membership and broadcasts messages.
type Hub struct {
	mu    sync.Mutex
	rooms map[string]map[string]Client // roomID -> clientID -> Client
}

// NewHub creates a new Hub.
func NewHub() *Hub {
	return &Hub{
		rooms: make(map[string]map[string]Client),
	}
}

// Register adds a client to a room.
func (h *Hub) Register(roomID string, c Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.rooms[roomID] == nil {
		h.rooms[roomID] = make(map[string]Client)
	}
	h.rooms[roomID][c.ID()] = c
}

// Unregister removes a client from a room.
func (h *Hub) Unregister(roomID string, c Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	clients, ok := h.rooms[roomID]
	if !ok {
		return
	}

	delete(clients, c.ID())
	if len(clients) == 0 {
		delete(h.rooms, roomID)
	}
}

// Broadcast sends a message to all clients currently in the room.
func (h *Hub) Broadcast(roomID string, message []byte) {
	h.BroadcastExcept(roomID, "", message)
}

// BroadcastExcept sends a message to every client in the room except the one
// whose ID equals exceptID. Used so a sender does not receive an echo of its own
// message (the client renders its own optimistically). exceptID == "" sends to all.
func (h *Hub) BroadcastExcept(roomID, exceptID string, message []byte) {
	h.mu.Lock()
	clients, ok := h.rooms[roomID]
	if !ok || len(clients) == 0 {
		h.mu.Unlock()
		return
	}

	// Copy client references to avoid holding the lock during I/O.
	copied := make([]Client, 0, len(clients))
	for _, c := range clients {
		if c.ID() == exceptID {
			continue
		}
		copied = append(copied, c)
	}
	h.mu.Unlock()

	for _, c := range copied {
		_ = c.Send(message)
	}
}
