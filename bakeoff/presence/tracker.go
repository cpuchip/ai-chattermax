// Package presence provides room-scoped participant tracking with
// support for Human and Persona kinds, Thinking status, and roster queries.
package presence

import (
	"sync"
	"time"
)

// Kind distinguishes types of participants in a room.
type Kind string

const (
	Human   Kind = "human"
	Persona Kind = "persona"
)

// IsHuman returns true if the kind is Human.
func (k Kind) IsHuman() bool { return k == Human }

// IsPersona returns true if the kind is Persona.
func (k Kind) IsPersona() bool { return k == Persona }

// Status represents a participant's availability status.
type Status string

const (
	StatusOnline  Status = "online"
	StatusAway    Status = "away"
	StatusBusy    Status = "busy"
	StatusOffline Status = "offline"
)

// Participant holds presence information for one room member.
type Participant struct {
	ID        string
	Kind      Kind
	Thinking bool
	Status    Status
	JoinedAt  time.Time
}

// Tracker provides room-scoped participant tracking. All methods are
// concurrency-safe.
type Tracker struct {
	mu     sync.RWMutex
	rooms  map[string]map[string]Participant // roomID → id → Participant
}

// NewTracker creates a new Tracker.
func NewTracker() *Tracker {
	return &Tracker{
		rooms: make(map[string]map[string]Participant),
	}
}

// Join adds a participant to a room. If the participant is already in the room,
// their Kind and JoinedAt are updated; Thinking and Status are reset to defaults.
func (t *Tracker) Join(roomID, id string, kind Kind) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.rooms[roomID] == nil {
		t.rooms[roomID] = make(map[string]Participant)
	}
	t.rooms[roomID][id] = Participant{
		ID:       id,
		Kind:     kind,
		Status:   StatusOnline,
		JoinedAt: time.Now(),
	}
}

// Leave removes a participant from a room. It is safe to call for a
// participant that is not in the room (no-op).
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

// SetThinking sets the Thinking flag for a persona in a room.
// If the participant is not in the room or is not a Persona, it is a no-op.
func (t *Tracker) SetThinking(roomID, id string, thinking bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if room, ok := t.rooms[roomID]; ok {
		if p, ok := room[id]; ok && p.Kind == Persona {
			p.Thinking = thinking
			room[id] = p
		}
	}
}

// SetStatus updates the Status of a participant in a room.
// If the participant is not in the room, it is a no-op.
func (t *Tracker) SetStatus(roomID, id string, status Status) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if room, ok := t.rooms[roomID]; ok {
		if p, ok := room[id]; ok {
			p.Status = status
			room[id] = p
		}
	}
}

// Roster returns the list of participants in a room, in no particular order.
// Returns an empty slice (not nil) for a room with no participants.
func (t *Tracker) Roster(roomID string) []Participant {
	t.mu.RLock()
	defer t.mu.RUnlock()
	room, ok := t.rooms[roomID]
	if !ok {
		return []Participant{}
	}
	result := make([]Participant, 0, len(room))
	for _, p := range room {
		result = append(result, p)
	}
	return result
}

// RoomIDs returns the IDs of all rooms that have at least one participant.
func (t *Tracker) RoomIDs() []string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	result := make([]string, 0, len(t.rooms))
	for id := range t.rooms {
		result = append(result, id)
	}
	return result
}