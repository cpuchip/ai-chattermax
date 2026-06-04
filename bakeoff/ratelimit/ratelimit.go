package ratelimit

import (
	"sync"
	"time"
)

// Clock provides the current time. The real implementation uses time.Now;
// tests may inject a fake clock for deterministic behavior.
type Clock interface {
	Now() time.Time
}

// realClock is the default production implementation of Clock.
type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

// Limiter enforces a hard rate ceiling per participant using a sliding window.
type Limiter struct {
	mu       sync.RWMutex
	clock    Clock
	capacity int           // maximum actions per window
	window   time.Duration // length of the rolling window
	buckets  map[string][]time.Time // id -> timestamps of recent actions
}

// NewLimiter creates a Limiter with the given capacity and window duration.
// A zero capacity allows no actions.
func NewLimiter(capacity int, window time.Duration) *Limiter {
	return NewLimiterWithClock(capacity, window, realClock{})
}

// NewLimiterWithClock creates a Limiter with an injectable clock.
func NewLimiterWithClock(capacity int, window time.Duration, clock Clock) *Limiter {
	return &Limiter{
		clock:    clock,
		capacity: capacity,
		window:   window,
		buckets:  make(map[string][]time.Time),
	}
}

// Allow returns true if the action is within the rate limit for the given id.
// Once the capacity is reached, further calls return false until old actions
// fall outside the rolling window.
func (l *Limiter) Allow(id string) bool {
	if l.capacity <= 0 {
		return false
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.clock.Now()
	cutoff := now.Add(-l.window)

	// Evict timestamps that have fallen outside the window.
	times := l.buckets[id]
	var kept int
	for _, t := range times {
		if t.After(cutoff) {
			times[kept] = t
			kept++
		}
	}
	times = times[:kept]

	if len(times) >= l.capacity {
		l.buckets[id] = times
		return false
	}

	l.buckets[id] = append(times, now)
	return true
}
