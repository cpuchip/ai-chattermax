// Package room provides a concurrency-safe multi-room chat hub.
//
// The Hub is transport-agnostic: clients implement the Client interface
// and are registered into named rooms. Broadcasts reach every client in
// the target room. A single select-loop goroutine owns all state mutation.
package room

import "sync"

// Client is a transport-agnostic participant in a chat room.
// ID must return a unique identifier for this client.
// Send delivers a message to the client; a non-nil error causes
// the Hub to drop the client from its room.
type Client interface {
	ID() string
	Send([]byte) error
}

type actionKind int

const (
	actionRegister actionKind = iota
	actionUnregister
	actionBroadcast
)

// action is a command sent to the Hub's run loop.
type action struct {
	kind   actionKind
	roomID string
	client Client
	msg    []byte
}

// Hub is a concurrency-safe multi-room chat hub.
// All state mutation happens in a single background goroutine,
// so the exported methods are safe for concurrent use.
type Hub struct {
	actions   chan action
	rooms     map[string]map[Client]struct{}
	done      chan struct{}
	closeOnce sync.Once
}

// NewHub creates a Hub and starts its background run loop.
func NewHub() *Hub {
	h := &Hub{
		actions: make(chan action),
		rooms:   make(map[string]map[Client]struct{}),
		done:    make(chan struct{}),
	}
	go h.run()
	return h
}

// run is the single select-loop goroutine.  All state mutation
// happens here, serialized by the actions channel.
func (h *Hub) run() {
	defer close(h.done)
	for a := range h.actions {
		switch a.kind {
		case actionRegister:
			if h.rooms[a.roomID] == nil {
				h.rooms[a.roomID] = make(map[Client]struct{})
			}
			h.rooms[a.roomID][a.client] = struct{}{}

		case actionUnregister:
			clients, ok := h.rooms[a.roomID]
			if !ok {
				continue
			}
			delete(clients, a.client)
			if len(clients) == 0 {
				delete(h.rooms, a.roomID)
			}

		case actionBroadcast:
			clients := h.rooms[a.roomID]
			for c := range clients {
				if err := c.Send(a.msg); err != nil {
					delete(clients, c)
				}
			}
			if len(clients) == 0 {
				delete(h.rooms, a.roomID)
			}
		}
	}
}

// Register adds client to roomID, creating the room if necessary.
func (h *Hub) Register(roomID string, client Client) {
	h.actions <- action{kind: actionRegister, roomID: roomID, client: client}
}

// Unregister removes client from roomID.  Silent no-op if the
// client is not in that room.
func (h *Hub) Unregister(roomID string, client Client) {
	h.actions <- action{kind: actionUnregister, roomID: roomID, client: client}
}

// Broadcast sends message to every client currently in roomID.
// Clients that return an error from Send are dropped.
func (h *Hub) Broadcast(roomID string, message []byte) {
	h.actions <- action{kind: actionBroadcast, roomID: roomID, msg: message}
}

// Close shuts down the background run loop and waits for it to finish.
// After Close returns the Hub is no longer usable.
func (h *Hub) Close() {
	h.closeOnce.Do(func() {
		close(h.actions)
	})
	<-h.done
}
