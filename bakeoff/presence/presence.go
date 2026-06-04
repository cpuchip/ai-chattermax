package presence

import (
	"sync"
)

// Kind indicates whether a participant is a human or a persona.
type Kind int

const (
	Human Kind = iota
	Persona
)

// Participant represents a room participant.
type Participant struct {
	ID       string
	Kind     Kind
	Thinking bool
}

// Tracker manages room-scoped presence.
type Tracker struct {
	mu    sync.RWMutex
	rooms map[string]map[string]*Participant // roomID -> id -> *Participant
}

// NewTracker creates a new Tracker.
func NewTracker() *Tracker {
	return &Tracker{
		rooms: make(map[string]map[string]*Participant),
	}
}

// Join adds a participant to a room. If the participant already exists, it is overwritten.
func (t *Tracker) Join(roomID, id string, kind Kind) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.rooms[roomID] == nil {
		t.rooms[roomID] = make(map[string]*Participant)
	}
	t.rooms[roomID][id] = &Participant{ID: id, Kind: kind}
}

// Leave removes a participant from a room.
func (t *Tracker) Leave(roomID, id string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if room, ok := t.rooms[roomID]; ok {
		delete(room, id)
		if len(room) == 0 {
			delete(t.rooms, roomID)
		}
	}
}

// SetThinking sets the Thinking flag for a participant in a room.
func (t *Tracker) SetThinking(roomID, id string, thinking bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if room, ok := t.rooms[roomID]; ok {
		if p, ok := room[id]; ok {
			p.Thinking = thinking
		}
	}
}

// Roster returns a defensive copy of all participants in a room.
func (t *Tracker) Roster(roomID string) []Participant {
	t.mu.RLock()
	defer t.mu.RUnlock()

	room, ok := t.rooms[roomID]
	if !ok {
		return nil
	}

	out := make([]Participant, 0, len(room))
	for _, p := range room {
		out = append(out, *p)
	}
	return out
}
