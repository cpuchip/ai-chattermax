// Package presence tracks room-scoped participant state.
// It distinguishes human participants from AI personas and exposes
// a "thinking" flag for personas.  All lookups are room-scoped:
// a participant in room A is invisible to room B's Roster.
package presence

import (
	"sort"
	"sync"
	"time"
)

// Kind distinguishes human participants from AI personas.
type Kind int

const (
	KindHuman Kind = iota
	KindPersona
)

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

// Participant is a snapshot of a participant in a room.
type Participant struct {
	ID       string
	Kind     Kind
	Name     string
	Thinking bool
	JoinedAt time.Time
	LastSeen time.Time
}

// Tracker maintains room-scoped presence information.
type Tracker struct {
	mu     sync.RWMutex
	rooms  map[string]map[string]*Participant // roomID -> participantID -> participant
}

// New creates a new Tracker.
func New() *Tracker {
	return &Tracker{
		rooms: make(map[string]map[string]*Participant),
	}
}

// Join records that participant id of kind has entered roomID.
// If the participant is already present, only LastSeen is updated.
func (t *Tracker) Join(roomID, id string, kind Kind) {
	t.mu.Lock()
	defer t.mu.Unlock()

	r, ok := t.rooms[roomID]
	if !ok {
		r = make(map[string]*Participant)
		t.rooms[roomID] = r
	}

	now := time.Now()
	if p, ok := r[id]; ok {
		p.LastSeen = now
		return
	}

	r[id] = &Participant{
		ID:       id,
		Kind:     kind,
		JoinedAt: now,
		LastSeen: now,
	}
}

// Leave removes participant id from roomID.  If the room becomes empty it
// is pruned from the tracker.
func (t *Tracker) Leave(roomID, id string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	r, ok := t.rooms[roomID]
	if !ok {
		return
	}
	delete(r, id)
	if len(r) == 0 {
		delete(t.rooms, roomID)
	}
}

// SetThinking sets the Thinking flag for participant id in roomID.
// If the participant is not present, this is a no-op.
func (t *Tracker) SetThinking(roomID, id string, thinking bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	r, ok := t.rooms[roomID]
	if !ok {
		return
	}
	if p, ok := r[id]; ok {
		p.Thinking = thinking
		p.LastSeen = time.Now()
	}
}

// Touch updates LastSeen for participant id in roomID.
// If the participant is not present, this is a no-op.
func (t *Tracker) Touch(roomID, id string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	r, ok := t.rooms[roomID]
	if !ok {
		return
	}
	if p, ok := r[id]; ok {
		p.LastSeen = time.Now()
	}
}

// Roster returns a sorted slice of participants in roomID.
// The returned slice is owned by the caller; mutations do not affect
// the tracker.  If roomID has no participants, an empty slice is returned.
func (t *Tracker) Roster(roomID string) []*Participant {
	t.mu.RLock()
	defer t.mu.RUnlock()

	r, ok := t.rooms[roomID]
	if !ok {
		return []*Participant{}
	}

	out := make([]*Participant, 0, len(r))
	for _, p := range r {
		// Return a shallow copy so the caller can't mutate internal state.
		copyP := *p
		out = append(out, &copyP)
	}

	// Deterministic ordering for tests.
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})

	return out
}
