// Package ratelimit provides per-participant rate limiting with a rolling
// window and an injectable clock for deterministic testing.
package ratelimit

import (
	"sync"
	"time"
)

// Clock abstracts time.Now for testing.
type Clock interface {
	Now() time.Time
}

// realClock wraps time.Now.
type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

// Limiter enforces a per-participant rate ceiling over a rolling window.
// A zero Limiter is not valid; use New.
type Limiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	clock   Clock
	actions map[string][]time.Time // id -> timestamps of allowed actions
}

// New returns a Limiter that allows up to limit actions per rolling window.
// If clock is nil, time.Now is used.
func New(limit int, window time.Duration, clock Clock) *Limiter {
	if clock == nil {
		clock = realClock{}
	}
	return &Limiter{
		limit:   limit,
		window:  window,
		clock:   clock,
		actions: make(map[string][]time.Time),
	}
}

// Allow checks whether the participant identified by id is allowed to
// perform an action. It returns true if the participant has not exceeded
// the limit within the rolling window, and records the action. It returns
// false if the limit has been reached.
func (l *Limiter) Allow(id string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.clock.Now()
	cutoff := now.Add(-l.window)

	// Prune old timestamps.
	timestamps := l.actions[id]
	var pruned []time.Time
	for _, ts := range timestamps {
		if ts.After(cutoff) {
			pruned = append(pruned, ts)
		}
	}

	// Check limit.
	if len(pruned) >= l.limit {
		l.actions[id] = pruned
		return false
	}

	// Record the new action.
	l.actions[id] = append(pruned, now)
	return true
}
