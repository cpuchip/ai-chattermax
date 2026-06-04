package presence

import (
	"sort"
	"sync"
)

// RoomID identifies a room.
type RoomID string

// ID uniquely identifies a participant within a room.
type ID string

// Kind distinguishes human participants from AI personas.
type Kind string

const (
	HumanKind   Kind = "human"
	PersonaKind Kind = "persona"
)

// Status represents a participant's presence status.
type Status string

const (
	Online Status = "online"
	Idle   Status = "idle"
)

// Participant represents a single human or AI persona.
type Participant struct {
	ID       ID
	Name     string
	Kind     Kind
	Status   Status
	Thinking bool // meaningful only when Kind == PersonaKind
}

// Roster is an immutable point-in-time snapshot of participants in a room.
type Roster struct {
	Participants []Participant
}

// Tracker maintains per-room presence state protected by a mutex.
type Tracker struct {
	mu           sync.RWMutex
	participants map[RoomID]map[ID]Participant
}

// NewTracker creates an initialized Tracker.
func NewTracker() *Tracker {
	return &Tracker{
		participants: make(map[RoomID]map[ID]Participant),
	}
}

func (t *Tracker) ensureRoom(roomID RoomID) {
	if t.participants[roomID] == nil {
		t.participants[roomID] = make(map[ID]Participant)
	}
}

// Join adds or updates a participant in a room.
func (t *Tracker) Join(roomID RoomID, p Participant) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ensureRoom(roomID)
	t.participants[roomID][p.ID] = p
}

// Leave removes a participant from a room.
func (t *Tracker) Leave(roomID RoomID, id ID) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if room, ok := t.participants[roomID]; ok {
		delete(room, id)
		if len(room) == 0 {
			delete(t.participants, roomID)
		}
	}
}

// SetStatus updates a participant's presence status in a room.
func (t *Tracker) SetStatus(roomID RoomID, id ID, status Status) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if room, ok := t.participants[roomID]; ok {
		if p, ok := room[id]; ok {
			p.Status = status
			room[id] = p
		}
	}
}

// SetThinking updates the thinking flag for a persona in a room. No-op for humans.
func (t *Tracker) SetThinking(roomID RoomID, id ID, thinking bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if room, ok := t.participants[roomID]; ok {
		if p, ok := room[id]; ok && p.Kind == PersonaKind {
			p.Thinking = thinking
			room[id] = p
		}
	}
}

// Roster returns a deep-copied, immutable snapshot of participants in a room.
func (t *Tracker) Roster(roomID RoomID) Roster {
	t.mu.RLock()
	defer t.mu.RUnlock()

	roster := Roster{
		Participants: make([]Participant, 0),
	}

	if room, ok := t.participants[roomID]; ok {
		roster.Participants = make([]Participant, 0, len(room))
		for _, p := range room {
			roster.Participants = append(roster.Participants, p)
		}
		sort.Slice(roster.Participants, func(i, j int) bool {
			return roster.Participants[i].ID < roster.Participants[j].ID
		})
	}

	return roster
}
