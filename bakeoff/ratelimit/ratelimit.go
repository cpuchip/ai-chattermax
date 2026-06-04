package ratelimit

import (
	"sync"
	"time"
)

// Clock provides the current time; inject a fake in tests for determinism.
type Clock interface {
	Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

// Limiter enforces a hard per-participant rate ceiling using a sliding window.
type Limiter struct {
	clock        Clock
	window       time.Duration
	max          int
	mu           sync.Mutex
	participants map[string][]time.Time
}

// NewLimiter creates a new Limiter.
// max is the maximum number of actions allowed per window.
func NewLimiter(max int, window time.Duration) *Limiter {
	return NewLimiterWithClock(max, window, realClock{})
}

// NewLimiterWithClock creates a new Limiter with an injected clock.
func NewLimiterWithClock(max int, window time.Duration, clock Clock) *Limiter {
	return &Limiter{
		clock:        clock,
		window:       window,
		max:          max,
		participants: make(map[string][]time.Time),
	}
}

// Allow returns true if the action is within the rate limit for the given id.
// Evicts timestamps older than the window before checking.
func (l *Limiter) Allow(id string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.clock.Now()
	cutoff := now.Add(-l.window)

	timestamps := l.participants[id]
	// Evict old entries
	idx := 0
	for i, t := range timestamps {
		if t.After(cutoff) {
			idx = i
			break
		}
	}
	if idx > 0 {
		timestamps = timestamps[idx:]
	} else if len(timestamps) > 0 && (timestamps[0].Before(cutoff) || timestamps[0].Equal(cutoff)) {
		// All entries are old
		timestamps = timestamps[:0]
	}

	if len(timestamps) >= l.max {
		l.participants[id] = timestamps
		return false
	}

	l.participants[id] = append(timestamps, now)
	return true
}
