package ratelimit

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeClock returns a deterministic clock that starts at t0 and advances
// by step on each call.
type fakeClock struct {
	t0   time.Time
	step time.Duration
	calls int64
}

func (fc *fakeClock) Now() time.Time {
	c := atomic.AddInt64(&fc.calls, 1)
	return fc.t0.Add(fc.step * time.Duration(c-1))
}

func TestLimiterUnderLimit(t *testing.T) {
	fc := &fakeClock{t0: time.Now(), step: 0}
	l := New(3, time.Minute, fc.Now)

	for i := 0; i < 3; i++ {
		if !l.Allow("alice") {
			t.Fatalf("expected allow at call %d", i+1)
		}
	}
}

func TestLimiterAtLimit(t *testing.T) {
	fc := &fakeClock{t0: time.Now(), step: 0}
	l := New(2, time.Minute, fc.Now)

	if !l.Allow("alice") {
		t.Fatal("expected first allow")
	}
	if !l.Allow("alice") {
		t.Fatal("expected second allow")
	}
	if l.Allow("alice") {
		t.Fatal("expected third deny")
	}
}

func TestLimiterWindowExpiry(t *testing.T) {
	t0 := time.Now()
	fc := &fakeClock{t0: t0, step: 0}
	l := New(2, time.Minute, fc.Now)

	l.Allow("alice")
	l.Allow("alice")
	if l.Allow("alice") {
		t.Fatal("expected deny at limit")
	}

	// Advance clock past the window.
	fc.step = 2 * time.Minute
	if !l.Allow("alice") {
		t.Fatal("expected allow after window expiry")
	}
}

func TestLimiterPerIDIsolation(t *testing.T) {
	fc := &fakeClock{t0: time.Now(), step: 0}
	l := New(1, time.Minute, fc.Now)

	if !l.Allow("alice") {
		t.Fatal("expected alice allow")
	}
	if !l.Allow("bob") {
		t.Fatal("expected bob allow")
	}
	if l.Allow("alice") {
		t.Fatal("expected alice deny")
	}
}

func TestLimiterSweep(t *testing.T) {
	t0 := time.Now()
	fc := &fakeClock{t0: t0, step: 0}
	l := New(10, time.Minute, fc.Now)

	l.Allow("alice")
	l.Allow("bob")

	fc.step = 2 * time.Minute // all events expired
	l.Sweep()

	// After sweep, both ids should be gone and Allow should succeed again.
	if !l.Allow("alice") {
		t.Fatal("expected alice allow after sweep")
	}
	if !l.Allow("bob") {
		t.Fatal("expected bob allow after sweep")
	}
}

func TestLimiterNilClock(t *testing.T) {
	l := New(1, time.Hour, nil)
	if !l.Allow("alice") {
		t.Fatal("expected allow with nil clock")
	}
	if l.Allow("alice") {
		t.Fatal("expected deny with nil clock")
	}
}

func TestLimiterZeroWindow(t *testing.T) {
	fc := &fakeClock{t0: time.Now(), step: 0}
	l := New(1, 0, fc.Now)

	if !l.Allow("alice") {
		t.Fatal("expected allow with zero window")
	}
	if l.Allow("alice") {
		t.Fatal("expected deny with zero window — all events are at the same instant")
	}
}

func TestLimiterConcurrent(t *testing.T) {
	fc := &fakeClock{t0: time.Now(), step: 0}
	l := New(10, time.Minute, fc.Now)

	const goroutines = 100
	const attempts = 50

	var allowed int64
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < attempts; j++ {
				if l.Allow("shared") {
					atomic.AddInt64(&allowed, 1)
				}
			}
		}()
	}
	wg.Wait()

	if allowed != 10 {
		t.Fatalf("expected exactly 10 allows, got %d", allowed)
	}
}

func TestLimiterConcurrentPerID(t *testing.T) {
	fc := &fakeClock{t0: time.Now(), step: 0}
	l := New(5, time.Minute, fc.Now)

	const ids = 20
	const goroutinesPerID = 10
	const attempts = 20

	var wg sync.WaitGroup
	results := make(map[string]*int64)
	for i := 0; i < ids; i++ {
		id := string(rune('a' + i))
		v := int64(0)
		results[id] = &v
		for j := 0; j < goroutinesPerID; j++ {
			wg.Add(1)
			go func(id string, counter *int64) {
				defer wg.Done()
				for k := 0; k < attempts; k++ {
					if l.Allow(id) {
						atomic.AddInt64(counter, 1)
					}
				}
			}(id, &v)
		}
	}
	wg.Wait()

	for id, counter := range results {
		if *counter != 5 {
			t.Fatalf("id %s expected 5 allows, got %d", id, *counter)
		}
	}
}
