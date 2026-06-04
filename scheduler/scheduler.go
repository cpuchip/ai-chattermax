package scheduler

import (
	"sync"
	"time"
)

// Scheduler manages round-robin turn-taking and a per-participant rate ceiling.
type Scheduler struct {
	clock func() time.Time
	mu    sync.Mutex

	// turn-taking state
	participants []string
	cursor       int

	// rate-ceiling state
	window     time.Duration
	maxActions int
	timestamps map[string][]time.Time
}

// New creates a Scheduler with the given clock, rolling window, and per-participant
// action cap. A maxActions of zero means no actions are ever allowed.
func New(clock func() time.Time, window time.Duration, maxActions int) *Scheduler {
	return &Scheduler{
		clock:      clock,
		window:     window,
		maxActions: maxActions,
		timestamps: make(map[string][]time.Time),
	}
}

// AddParticipant appends a new ID to the rotation if it is not already present.
func (s *Scheduler) AddParticipant(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, existing := range s.participants {
		if existing == id {
			return
		}
	}
	s.participants = append(s.participants, id)
}

// RemoveParticipant removes an ID from the rotation.
func (s *Scheduler) RemoveParticipant(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, existing := range s.participants {
		if existing != id {
			continue
		}
		s.participants = append(s.participants[:i], s.participants[i+1:]...)
		// If the cursor now points past the end, wrap it back.
		if s.cursor >= len(s.participants) {
			s.cursor = 0
		}
		return
	}
}

// NextTurn returns the next participant in round-robin order and advances the
// cursor. It returns the empty string when no participants are registered.
func (s *Scheduler) NextTurn() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.participants) == 0 {
		return ""
	}
	id := s.participants[s.cursor]
	s.cursor = (s.cursor + 1) % len(s.participants)
	return id
}

// Allow permits an action for id only if the participant has fewer than maxActions
// recorded within the rolling window. On success the current timestamp is recorded.
func (s *Scheduler) Allow(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.clock()
	cutoff := now.Add(-s.window)

	entries := s.timestamps[id]
	pruned := make([]time.Time, 0, len(entries))
	for _, t := range entries {
		if t.After(cutoff) || t.Equal(cutoff) {
			pruned = append(pruned, t)
		}
	}

	if len(pruned) >= s.maxActions {
		s.timestamps[id] = pruned
		return false
	}

	pruned = append(pruned, now)
	s.timestamps[id] = pruned
	return true
}
