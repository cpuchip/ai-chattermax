// Package ratelimit provides a per-participant hard rate ceiling with a
// rolling window. A Limiter answers Allow(id) by recording the call
// time and granting the request iff fewer than max calls have been
// recorded within the last window. The clock is injectable so tests
// can advance time deterministically.
package ratelimit

import (
	"sync"
	"time"
)

// Clock returns the current time. It is a 1-method interface so tests
// can substitute a fake clock without touching time.Now.
type Clock interface {
	Now() time.Time
}

// realClock returns wall-clock time.
type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

// Limiter is a per-id rolling-window rate limiter. The zero value is
// not usable; obtain one with NewLimiter. A Limiter is safe for
// concurrent use by any number of goroutines.
type Limiter struct {
	max    int
	window time.Duration
	clock  Clock
	mu     sync.Mutex
	hits   map[string][]time.Time
}

// NewLimiter returns a Limiter that allows up to max actions per id
// within any window of length d. If clock is nil, a real wall clock is
// used.
//
// Edge cases follow directly from the algorithm: max<=0 always denies;
// window<=0 never lets old hits fall out, so each id is allowed max
// times total and then denied forever.
func NewLimiter(max int, d time.Duration, clock Clock) *Limiter {
	if clock == nil {
		clock = realClock{}
	}
	return &Limiter{
		max:    max,
		window: d,
		clock:  clock,
		hits:   make(map[string][]time.Time),
	}
}

// Allow records an attempt by id and returns true iff fewer than max
// attempts have been recorded for id within the last window. Allow
// itself never errors and never blocks.
func (l *Limiter) Allow(id string) bool {
	now := l.clock.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	hits := l.hits[id]
	if l.window > 0 {
		cutoff := now.Add(-l.window)
		drop := 0
		for drop < len(hits) && hits[drop].Before(cutoff) {
			drop++
		}
		if drop > 0 {
			hits = hits[drop:]
		}
	}
	if l.max <= 0 || len(hits) >= l.max {
		l.hits[id] = hits
		return false
	}
	hits = append(hits, now)
	l.hits[id] = hits
	return true
}
