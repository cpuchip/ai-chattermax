// Package ratelimit provides a per-participant hard rate ceiling using
// a rolling window.  An injectable clock makes tests deterministic.
package ratelimit

import (
	"sync"
	"time"
)

// Limiter enforces a hard ceiling of N actions per rolling window for
// each participant ID.
type Limiter struct {
	n      int
	window time.Duration
	now    func() time.Time

	mu     sync.Mutex
	events map[string][]time.Time // id -> timestamps of recent allowed actions
}

// New creates a Limiter that allows at most limit actions per window.
// If clock is nil, time.Now is used.
func New(limit int, window time.Duration, clock func() time.Time) *Limiter {
	if clock == nil {
		clock = time.Now
	}
	return &Limiter{
		n:      limit,
		window: window,
		now:    clock,
		events: make(map[string][]time.Time),
	}
}

// Allow reports whether participant id may perform an action right now.
// If allowed, the current timestamp is recorded so it counts toward the
// limit until it falls outside the window.
func (l *Limiter) Allow(id string) bool {
	now := l.now()
	cutoff := now.Add(-l.window)

	l.mu.Lock()
	defer l.mu.Unlock()

	events := l.events[id]

	// Drop expired timestamps.
	keep := 0
	for _, t := range events {
		if !t.Before(cutoff) {
			events[keep] = t
			keep++
		}
	}
	events = events[:keep]

	if len(events) >= l.n {
		l.events[id] = events
		return false
	}

	events = append(events, now)
	l.events[id] = events
	return true
}

// Sweep removes participant entries that have no events inside the
// current window.  Callers can invoke this periodically to control
// memory growth.  Sweep is safe for concurrent use.
func (l *Limiter) Sweep() {
	now := l.now()
	cutoff := now.Add(-l.window)

	l.mu.Lock()
	defer l.mu.Unlock()

	for id, events := range l.events {
		keep := 0
		for _, t := range events {
			if !t.Before(cutoff) {
				events[keep] = t
				keep++
			}
		}
		if keep == 0 {
			delete(l.events, id)
		} else {
			l.events[id] = events[:keep]
		}
	}
}
