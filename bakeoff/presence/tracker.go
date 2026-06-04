// Package presence tracks room-scoped participants with typed kinds
// (human vs persona) and optional status metadata.
package presence

import "sync"

// Kind distinguishes humans from AI personas in a room.
type Kind string

const (
	KindHuman   Kind = "human"
	KindPersona Kind = "persona"
)

// Participant is a snapshot of one participant's presence in a room.
type Participant struct {
	ID       string
	Kind     Kind
	Thinking bool   // personas only: currently composing a response
	Status   string // free-form status text
}

// Tracker maintains room-scoped participant rosters. A zero Tracker is not
// valid; use NewTracker.
type Tracker struct {
	mu    sync.RWMutex
	rooms map[string]map[string]*Participant
}

// NewTracker returns an initialised Tracker.
func NewTracker() *Tracker {
	return &Tracker{rooms: make(map[string]map[string]*Participant)}
}

// Join adds a participant to a room. Joining the same (roomID, id) twice
// is idempotent and does not overwrite the existing record.
func (t *Tracker) Join(roomID, id string, kind Kind) {
	t.mu.Lock()
	defer t.mu.Unlock()

	room, ok := t.rooms[roomID]
	if !ok {
		room = make(map[string]*Participant)
		t.rooms[roomID] = room
	}
	if _, exists := room[id]; exists {
		return
	}
	room[id] = &Participant{ID: id, Kind: kind}
}

// Leave removes a participant from a room. If the room becomes empty it is
// deleted. Leaving a room the participant is not in is a no-op.
func (t *Tracker) Leave(roomID, id string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	room, ok := t.rooms[roomID]
	if !ok {
		return
	}
	delete(room, id)
	if len(room) == 0 {
		delete(t.rooms, roomID)
	}
}

// SetThinking marks a persona participant as currently thinking (or not).
// It is a no-op if the participant is not in the room.
func (t *Tracker) SetThinking(roomID, id string, thinking bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	room, ok := t.rooms[roomID]
	if !ok {
		return
	}
	p, ok := room[id]
	if !ok {
		return
	}
	p.Thinking = thinking
}

// SetStatus sets free-form status text for a participant. It is a no-op if
// the participant is not in the room.
func (t *Tracker) SetStatus(roomID, id, status string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	room, ok := t.rooms[roomID]
	if !ok {
		return
	}
	p, ok := room[id]
	if !ok {
		return
	}
	p.Status = status
}

// Roster returns a snapshot of all participants in a room. The returned
// slice contains copies, so callers may mutate them freely.
func (t *Tracker) Roster(roomID string) []Participant {
	t.mu.RLock()
	defer t.mu.RUnlock()

	room, ok := t.rooms[roomID]
	if !ok || len(room) == 0 {
		return nil
	}
	out := make([]Participant, 0, len(room))
	for _, p := range room {
		out = append(out, *p) // value copy
	}
	return out
}
