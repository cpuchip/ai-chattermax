package room

import (
	"sync"
)

// Client is a transport-agnostic participant in a chat room.
type Client interface {
	ID() string
	Send([]byte) error
}

// Hub manages room-scoped client registration and broadcast.
type Hub struct {
	mu      sync.RWMutex
	rooms   map[string]map[string]Client // roomID -> clientID -> Client
}

// NewHub creates an empty Hub.
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

	if clients, ok := h.rooms[roomID]; ok {
		delete(clients, c.ID())
		if len(clients) == 0 {
			delete(h.rooms, roomID)
		}
	}
}

// Broadcast sends a message to every client in a room.
// Clients that fail to receive the message are dropped from the room.
func (h *Hub) Broadcast(roomID string, message []byte) {
	h.mu.RLock()
	clients, ok := h.rooms[roomID]
	if !ok {
		h.mu.RUnlock()
		return
	}
	// Snapshot the clients to avoid holding the lock during Send.
	snapshot := make([]Client, 0, len(clients))
	for _, c := range clients {
		snapshot = append(snapshot, c)
	}
	h.mu.RUnlock()

	var dropped []string
	for _, c := range snapshot {
		if err := c.Send(message); err != nil {
			dropped = append(dropped, c.ID())
		}
	}

	if len(dropped) > 0 {
		h.mu.Lock()
		if clients, ok := h.rooms[roomID]; ok {
			for _, id := range dropped {
				delete(clients, id)
			}
			if len(clients) == 0 {
				delete(h.rooms, roomID)
			}
		}
		h.mu.Unlock()
	}
}
