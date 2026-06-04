// Package ratelimit provides a per-participant hard rate ceiling with
// an injectable clock for deterministic testing.
package ratelimit

import (
	"sync"
	"time"
)

// Clock is the time source used by the Limiter. Inject a mock in tests
// for deterministic time advancement.
type Clock interface {
	Now() time.Time
}

// RealClock uses time.Now(). It is the default.
type RealClock struct{}

func (RealClock) Now() time.Time { return time.Now() }

// Config holds rate limiter parameters.
type Config struct {
	// MaxActions is the maximum number of actions allowed per Window.
	MaxActions int
	// Window is the rolling time window duration.
	Window time.Duration
}

// Limiter enforces a per-participant hard rate ceiling. Each participant
// (identified by id) gets up to MaxActions actions per rolling Window.
// When the ceiling is reached, Allow returns false until the window rolls
// forward.
//
// Limiter is concurrency-safe.
type Limiter struct {
	mu     sync.Mutex
	clock  Clock
	config Config
	window map[string][]time.Time // id → timestamps of actions within window
}

// NewLimiter creates a new Limiter with the given config and clock.
// If clock is nil, RealClock is used.
func NewLimiter(config Config, clock Clock) *Limiter {
	if clock == nil {
		clock = RealClock{}
	}
	return &Limiter{
		clock:  clock,
		config: config,
		window: make(map[string][]time.Time),
	}
}

// Allow records an action for the given id and returns true if the action
// is within the rate limit. Returns false if the participant has exceeded
// MaxActions in the current rolling window.
func (l *Limiter) Allow(id string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.clock.Now()
	windowStart := now.Add(-l.config.Window)

	// Prune expired timestamps.
	timestamps := l.window[id]
	valid := timestamps[:0]
	for _, ts := range timestamps {
		if ts.After(windowStart) {
			valid = append(valid, ts)
		}
	}

	if len(valid) >= l.config.MaxActions {
		l.window[id] = valid
		return false
	}

	l.window[id] = append(valid, now)
	return true
}

// Reset removes all recorded actions for the given id, immediately
// restoring their full allowance. Useful for test cleanup or
// explicit rate-limit lifting.
func (l *Limiter) Reset(id string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.window, id)
}