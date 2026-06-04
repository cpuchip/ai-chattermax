// Package room provides a concurrency-safe, multi-room broadcast hub.
// The Hub manages client registration and message distribution by room,
// using a channel-driven dispatcher goroutine so all state mutation is
// serialized. Fan-out to individual clients is done via goroutine so a
// slow Send() never stalls the dispatcher.
package room

import (
	"errors"
	"sync"
)

// Client is the minimal interface a transport layer must satisfy to
// participate in a room.  It is intentionally small so implementations
// (WebSocket, mock, etc.) stay decoupled from package room.
type Client interface {
	ID() string
	Send([]byte) error
}

// Hub maintains a set of rooms, each holding a set of clients.
type Hub struct {
	register   chan registration
	unregister chan registration
	broadcast  chan broadcastMsg

	// mu guards rooms; only the dispatcher goroutine reads/writes.
	mu    sync.Mutex
	rooms map[string]map[string]Client
}

type registration struct {
	roomID string
	client Client
}

type broadcastResult struct {
	count int
	err   error
}

type broadcastMsg struct {
	roomID string
	data   []byte
	done   chan broadcastResult
}

// New creates a new Hub and starts its internal dispatcher goroutine.
func New() *Hub {
	h := &Hub{
		register:   make(chan registration),
		unregister: make(chan registration),
		broadcast:  make(chan broadcastMsg),
		rooms:      make(map[string]map[string]Client),
	}
	go h.run()
	return h
}

// Register adds a client to the named room. The call blocks until the
// dispatcher goroutine has processed the request.
func (h *Hub) Register(roomID string, client Client) {
	h.register <- registration{roomID: roomID, client: client}
}

// Unregister removes a client from the named room. The call blocks until
// the dispatcher goroutine has processed the request.
func (h *Hub) Unregister(roomID string, client Client) {
	h.unregister <- registration{roomID: roomID, client: client}
}

// Broadcast sends data to every client currently in roomID and returns
// the number of successful sends and a joined error of any failures.
func (h *Hub) Broadcast(roomID string, data []byte) (int, error) {
	done := make(chan broadcastResult, 1)
	h.broadcast <- broadcastMsg{roomID: roomID, data: data, done: done}
	res := <-done
	return res.count, res.err
}

// run is the single dispatcher goroutine.
func (h *Hub) run() {
	for {
		select {
		case reg := <-h.register:
			h.registerClient(reg)
		case reg := <-h.unregister:
			h.unregisterClient(reg)
		case msg := <-h.broadcast:
			msg.done <- h.broadcastToRoom(msg)
		}
	}
}

func (h *Hub) registerClient(reg registration) {
	h.mu.Lock()
	defer h.mu.Unlock()

	r, ok := h.rooms[reg.roomID]
	if !ok {
		r = make(map[string]Client)
		h.rooms[reg.roomID] = r
	}
	r[reg.client.ID()] = reg.client
}

func (h *Hub) unregisterClient(reg registration) {
	h.mu.Lock()
	defer h.mu.Unlock()

	r, ok := h.rooms[reg.roomID]
	if !ok {
		return
	}
	delete(r, reg.client.ID())
	if len(r) == 0 {
		delete(h.rooms, reg.roomID)
	}
}

func (h *Hub) broadcastToRoom(msg broadcastMsg) broadcastResult {
	h.mu.Lock()
	room, ok := h.rooms[msg.roomID]
	if !ok {
		h.mu.Unlock()
		return broadcastResult{count: 0, err: nil}
	}
	clients := make([]Client, 0, len(room))
	for _, c := range room {
		clients = append(clients, c)
	}
	h.mu.Unlock()

	var (
		sent   int
		errs   []error
		errMu  sync.Mutex
		wg     sync.WaitGroup
	)

	for _, c := range clients {
		wg.Add(1)
		go func(client Client) {
			defer wg.Done()
			if err := client.Send(msg.data); err != nil {
				errMu.Lock()
				errs = append(errs, err)
				errMu.Unlock()
			} else {
				errMu.Lock()
				sent++
				errMu.Unlock()
			}
		}(c)
	}
	wg.Wait()

	var joinedErr error
	if len(errs) > 0 {
		joinedErr = errors.Join(errs...)
	}
	return broadcastResult{count: sent, err: joinedErr}
}
