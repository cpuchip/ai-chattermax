package room

import (
	"sync"
)

// Hub manages room-scoped client registration and broadcast.
type Hub struct {
	mu     sync.RWMutex
	rooms  map[string]map[string]Client // roomID -> clientID -> Client
}

// NewHub creates a new Hub.
func NewHub() *Hub {
	return &Hub{
		rooms: make(map[string]map[string]Client),
	}
}

// Register adds a client to a room.
func (h *Hub) Register(roomID string, client Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.rooms[roomID] == nil {
		h.rooms[roomID] = make(map[string]Client)
	}
	h.rooms[roomID][client.ID()] = client
}

// Unregister removes a client from a room.
func (h *Hub) Unregister(roomID string, client Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if clients, ok := h.rooms[roomID]; ok {
		delete(clients, client.ID())
		if len(clients) == 0 {
			delete(h.rooms, roomID)
		}
	}
}

// Broadcast sends a message to all clients in a room.
// Clients that fail to receive are dropped from the room.
func (h *Hub) Broadcast(roomID string, message []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()

	clients, ok := h.rooms[roomID]
	if !ok {
		return
	}

	for id, client := range clients {
		if err := client.Send(message); err != nil {
			delete(clients, id)
		}
	}

	if len(clients) == 0 {
		delete(h.rooms, roomID)
	}
}
