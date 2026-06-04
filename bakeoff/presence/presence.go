// Package presence tracks which participants are in which rooms.
//
// A participant is either a human or an AI persona.  The tracker records
// a freeform status string and a "thinking" flag for each participant.
// All methods are safe for concurrent use.
package presence

import "sync"

// Kind discriminates human participants from AI personas.
type Kind int

const (
	KindHuman   Kind = iota + 1
	KindPersona
)

// String returns a human-readable label for the kind.
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

// Entry holds the presence state of one participant in one room.
type Entry struct {
	ID       string
	Kind     Kind
	Thinking bool
	Status   string
}

// RosterEntry is the public snapshot returned by Roster.
type RosterEntry struct {
	ID       string
	Kind     Kind
	Thinking bool
	Status   string
}

// Tracker tracks room presence.  Zero value is ready to use.
type Tracker struct {
	mu   sync.RWMutex
	byID map[string]map[string]*Entry // roomID -> participantID -> entry
}

// room returns the per-room map, allocating if needed.  Caller must hold t.mu (write lock).
func (t *Tracker) room(roomID string) map[string]*Entry {
	if t.byID == nil {
		t.byID = make(map[string]map[string]*Entry)
	}
	m, ok := t.byID[roomID]
	if !ok {
		m = make(map[string]*Entry)
		t.byID[roomID] = m
	}
	return m
}

// Join adds or updates a participant in a room.  If the participant is
// already present, only the Kind is updated — Thinking and Status are
// left unchanged.  Safe for concurrent use.
func (t *Tracker) Join(roomID, id string, kind Kind) {
	t.mu.Lock()
	defer t.mu.Unlock()

	entry, ok := t.room(roomID)[id]
	if ok {
		entry.Kind = kind
		return
	}
	t.room(roomID)[id] = &Entry{ID: id, Kind: kind}
}

// Leave removes a participant from a room.  Silent no-op if the
// participant is not in that room.  Safe for concurrent use.
func (t *Tracker) Leave(roomID, id string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	m := t.byID[roomID]
	if m == nil {
		return
	}
	delete(m, id)
	if len(m) == 0 {
		delete(t.byID, roomID)
	}
}

// SetThinking sets the Thinking flag for a participant.  No-op if the
// participant is not in the room.
func (t *Tracker) SetThinking(roomID, id string, thinking bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	e := t.entry(roomID, id)
	if e == nil {
		return
	}
	e.Thinking = thinking
}

// SetStatus sets the Status string for a participant.  No-op if the
// participant is not in the room.
func (t *Tracker) SetStatus(roomID, id, status string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	e := t.entry(roomID, id)
	if e == nil {
		return
	}
	e.Status = status
}

// entry returns a participant's entry, or nil.  Caller must hold t.mu.
func (t *Tracker) entry(roomID, id string) *Entry {
	if t.byID == nil {
		return nil
	}
	m := t.byID[roomID]
	if m == nil {
		return nil
	}
	return m[id]
}

// Roster returns a snapshot of every participant currently in roomID.
// The returned slice is a copy — the caller may hold or mutate it safely.
func (t *Tracker) Roster(roomID string) []RosterEntry {
	t.mu.RLock()
	defer t.mu.RUnlock()

	m := t.byID[roomID]
	if m == nil {
		return nil
	}
	out := make([]RosterEntry, 0, len(m))
	for _, e := range m {
		out = append(out, RosterEntry{
			ID:       e.ID,
			Kind:     e.Kind,
			Thinking: e.Thinking,
			Status:   e.Status,
		})
	}
	return out
}
