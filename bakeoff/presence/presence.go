package presence

import (
	"sync"
)

// Kind distinguishes human participants from AI personas.
type Kind string

const (
	Human   Kind = "human"
	Persona Kind = "persona"
)

// Participant represents a single participant in a room.
type Participant struct {
	ID       string
	Kind     Kind
	Thinking bool // true when a persona is actively processing
}

// Tracker maintains room-scoped presence information.
type Tracker struct {
	mu      sync.RWMutex
	rooms   map[string]map[string]Participant // roomID -> participantID -> Participant
}

// NewTracker creates an empty Tracker.
func NewTracker() *Tracker {
	return &Tracker{
		rooms: make(map[string]map[string]Participant),
	}
}

// Join adds or updates a participant in a room.
func (t *Tracker) Join(roomID string, id string, kind Kind) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.rooms[roomID] == nil {
		t.rooms[roomID] = make(map[string]Participant)
	}
	t.rooms[roomID][id] = Participant{ID: id, Kind: kind}
}

// Leave removes a participant from a room.
func (t *Tracker) Leave(roomID string, id string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if participants, ok := t.rooms[roomID]; ok {
		delete(participants, id)
		if len(participants) == 0 {
			delete(t.rooms, roomID)
		}
	}
}

// SetThinking sets the Thinking flag for a participant in a room.
func (t *Tracker) SetThinking(roomID string, id string, thinking bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if participants, ok := t.rooms[roomID]; ok {
		if p, ok := participants[id]; ok {
			p.Thinking = thinking
			participants[id] = p
		}
	}
}

// SetKind sets the Kind for a participant in a room.
func (t *Tracker) SetKind(roomID string, id string, kind Kind) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if participants, ok := t.rooms[roomID]; ok {
		if p, ok := participants[id]; ok {
			p.Kind = kind
			participants[id] = p
		}
	}
}

// Roster returns a copy slice of all participants in the given room.
func (t *Tracker) Roster(roomID string) []Participant {
	t.mu.RLock()
	defer t.mu.RUnlock()

	participants, ok := t.rooms[roomID]
	if !ok {
		return nil
	}

	result := make([]Participant, 0, len(participants))
	for _, p := range participants {
		result = append(result, p)
	}
	return result
}
