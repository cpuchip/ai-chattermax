// Package presence provides a room-scoped, concurrency-safe tracker for
// participants (humans and personas) within a council session.
package presence

import "sync"

// Kind indicates whether a participant is a human or a persona.
type Kind string

const (
	KindHuman   Kind = "human"
	KindPersona Kind = "persona"
)

// Participant represents a single presence entry in a room.
type Participant struct {
	ID       string
	Kind     Kind
	Thinking bool
}

// room holds the participants for a single room.
type room struct {
	participants map[string]*Participant
}

// Tracker is a concurrency-safe registry of participants keyed by room.
// The zero value is ready to use.
type Tracker struct {
	mu    sync.RWMutex
	rooms map[string]*room
}

// Join adds a participant to the given room. If the participant already
// exists, it is replaced with the new kind and thinking state is reset
// to false.
func (t *Tracker) Join(roomID, id string, kind Kind) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.rooms == nil {
		t.rooms = make(map[string]*room)
	}

	r, ok := t.rooms[roomID]
	if !ok {
		r = &room{participants: make(map[string]*Participant)}
		t.rooms[roomID] = r
	}

	r.participants[id] = &Participant{
		ID:   id,
		Kind: kind,
	}
}

// Leave removes a participant from the given room. If the room becomes
// empty, it is deleted from the tracker.
func (t *Tracker) Leave(roomID, id string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	r, ok := t.rooms[roomID]
	if !ok {
		return
	}

	delete(r.participants, id)
	if len(r.participants) == 0 {
		delete(t.rooms, roomID)
	}
}

// SetThinking updates the thinking flag for a participant in a room.
// If the room or participant does not exist, it is a no-op.
func (t *Tracker) SetThinking(roomID, id string, thinking bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	r, ok := t.rooms[roomID]
	if !ok {
		return
	}

	p, ok := r.participants[id]
	if !ok {
		return
	}

	p.Thinking = thinking
}

// Roster returns a snapshot of all participants currently in the room.
// The returned slice is a copy; the caller may safely retain it.
func (t *Tracker) Roster(roomID string) []Participant {
	t.mu.RLock()
	defer t.mu.RUnlock()

	r, ok := t.rooms[roomID]
	if !ok {
		return nil
	}

	out := make([]Participant, 0, len(r.participants))
	for _, p := range r.participants {
		// Return a value copy, not a pointer to internal state.
		out = append(out, *p)
	}

	// Deterministic ordering for testability.
	// Sort by ID to give a stable order.
	for i := 0; i < len(out)-1; i++ {
		for j := i + 1; j < len(out); j++ {
			if out[i].ID > out[j].ID {
				out[i], out[j] = out[j], out[i]
			}
		}
	}

	return out
}
