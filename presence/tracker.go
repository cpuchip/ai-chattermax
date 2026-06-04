package presence

import (
	"sort"
	"sync"
)

// Tracker manages a concurrency-safe roster of present participants.
type Tracker struct {
	mu    sync.Mutex
	peers map[string]Participant
}

// NewTracker creates a new Tracker.
func NewTracker() *Tracker {
	return &Tracker{
		peers: make(map[string]Participant),
	}
}

// Join marks a participant as present. If the participant already exists,
// their kind is updated and they are set to Online.
func (t *Tracker) Join(id string, kind Kind) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.peers[id] = Participant{
		ID:     id,
		Kind:   kind,
		Online: true,
		Idle:   false,
		Thinking: false,
	}
}

// Leave removes a participant from the tracker.
func (t *Tracker) Leave(id string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	delete(t.peers, id)
}

// SetIdle updates the idle flag for a participant. It is a no-op if the
// participant is not present.
func (t *Tracker) SetIdle(id string, idle bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	p, ok := t.peers[id]
	if !ok {
		return
	}
	p.Idle = idle
	t.peers[id] = p
}

// SetThinking updates the thinking flag for a participant. It is a no-op if
// the participant is not present.
func (t *Tracker) SetThinking(id string, thinking bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	p, ok := t.peers[id]
	if !ok {
		return
	}
	p.Thinking = thinking
	t.peers[id] = p
}

// Roster returns a stable snapshot of all present participants sorted by ID.
func (t *Tracker) Roster() []Participant {
	t.mu.Lock()
	defer t.mu.Unlock()

	out := make([]Participant, 0, len(t.peers))
	for _, p := range t.peers {
		out = append(out, p)
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})

	return out
}
