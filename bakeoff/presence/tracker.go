// Package presence tracks the per-room roster of a multi-room chat:
// which human or persona participants are in which room, their status,
// and (for personas) whether they are currently thinking.
package presence

import (
	"errors"
	"sort"
	"sync"
)

// Kind identifies whether a participant is a real human or a synthetic
// persona (e.g. an AI assistant or NPC).
type Kind int

const (
	// KindHuman is a real human participant.
	KindHuman Kind = iota
	// KindPersona is a synthetic persona (AI, NPC, bot, etc.).
	KindPersona
)

// String returns a human-readable name for the kind.
func (k Kind) String() string {
	switch k {
	case KindHuman:
		return "human"
	case KindPersona:
		return "persona"
	default:
		return "unknown"
	}
}

// Participant describes a single occupant of a room.
type Participant struct {
	ID       string
	Kind     Kind
	Status   string
	Thinking bool
}

// ErrNotInRoom is returned by status setters when the named id is not
// currently a participant in the named room.
var ErrNotInRoom = errors.New("presence: participant not in room")

// Tracker is a room-scoped presence registry. The zero value is not
// usable; obtain one with New. A Tracker is safe for concurrent use.
type Tracker struct {
	mu    sync.RWMutex
	rooms map[string]map[string]Participant
}

// New returns an empty Tracker.
func New() *Tracker {
	return &Tracker{rooms: make(map[string]map[string]Participant)}
}

// Join adds (or updates) id in roomID with the given kind. If id was
// already a participant in the room, its entry is replaced: kind and
// status are overwritten and Thinking is reset to false. (Callers that
// want to preserve Thinking should call SetThinking after Join.)
func (t *Tracker) Join(roomID, id string, kind Kind) {
	t.mu.Lock()
	defer t.mu.Unlock()
	room, ok := t.rooms[roomID]
	if !ok {
		room = make(map[string]Participant)
		t.rooms[roomID] = room
	}
	room[id] = Participant{ID: id, Kind: kind}
}

// Leave removes id from roomID. It is a no-op if the room does not
// exist or id is not in the room. Empty rooms are deleted.
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

// SetStatus updates the Status field of id in roomID. It returns
// ErrNotInRoom if id is not currently a participant in roomID.
func (t *Tracker) SetStatus(roomID, id, status string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	room, ok := t.rooms[roomID]
	if !ok {
		return ErrNotInRoom
	}
	p, ok := room[id]
	if !ok {
		return ErrNotInRoom
	}
	p.Status = status
	room[id] = p
	return nil
}

// SetThinking sets the Thinking flag of id in roomID. It returns
// ErrNotInRoom if id is not currently a participant in roomID. The
// flag has no defined meaning for KindHuman participants; callers may
// use or ignore it as they see fit.
func (t *Tracker) SetThinking(roomID, id string, thinking bool) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	room, ok := t.rooms[roomID]
	if !ok {
		return ErrNotInRoom
	}
	p, ok := room[id]
	if !ok {
		return ErrNotInRoom
	}
	p.Thinking = thinking
	room[id] = p
	return nil
}

// Roster returns a sorted-by-ID snapshot of the participants currently
// in roomID. The returned slice is freshly allocated; callers may
// mutate it freely. Roster of an unknown or empty room returns nil.
func (t *Tracker) Roster(roomID string) []Participant {
	t.mu.RLock()
	defer t.mu.RUnlock()
	room, ok := t.rooms[roomID]
	if !ok {
		return nil
	}
	out := make([]Participant, 0, len(room))
	for _, p := range room {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Rooms returns the IDs of all rooms that currently have at least one
// participant. The order is unspecified.
func (t *Tracker) Rooms() []string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := make([]string, 0, len(t.rooms))
	for id := range t.rooms {
		out = append(out, id)
	}
	return out
}
