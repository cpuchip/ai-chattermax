// Package ratelimit provides a per-participant hard rate ceiling
// with an injectable clock for testing.
package ratelimit

import (
	"sort"
	"sync"
	"time"
)

// Clock is an injectable time source.
type Clock interface {
	Now() time.Time
}

// RealClock uses the system clock.
type RealClock struct{}

func (RealClock) Now() time.Time { return time.Now() }

// Limiter enforces a maximum number of actions per participant
// within a rolling time window.
type Limiter struct {
	mu      sync.Mutex
	actions map[string][]time.Time
	window  time.Duration
	max     int
	clock   Clock
}

// NewLimiter returns a Limiter that allows at most max actions per
// window duration.  The injected clock is used for all time checks;
// pass a RealClock for production or a fake clock for tests.
func NewLimiter(window time.Duration, max int, clock Clock) *Limiter {
	return &Limiter{
		actions: make(map[string][]time.Time),
		window:  window,
		max:     max,
		clock:   clock,
	}
}

// Allow reports whether the participant identified by id is within
// their rate limit for the current window.  If allowed, the call
// counts against the limit.  Safe for concurrent use.
func (l *Limiter) Allow(id string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.clock.Now()
	cutoff := now.Add(-l.window)

	timestamps := l.actions[id]

	// Prune expired entries.  The slice is sorted by construction
	// (monotonic clock), so binary search finds the first entry
	// still within the window.
	idx := sort.Search(len(timestamps), func(i int) bool {
		return !timestamps[i].Before(cutoff)
	})
	timestamps = timestamps[idx:]

	if len(timestamps) < l.max {
		timestamps = append(timestamps, now)
		l.actions[id] = timestamps
		return true
	}

	l.actions[id] = timestamps
	return false
}
