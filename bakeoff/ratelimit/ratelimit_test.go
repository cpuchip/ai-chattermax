package ratelimit

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeClock is a manually-steppable clock for deterministic tests.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func newFakeClock(t time.Time) *fakeClock {
	return &fakeClock{t: t}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

func (c *fakeClock) set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = t
}

func TestAllowBasic(t *testing.T) {
	clock := newFakeClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	lim := NewLimiter(time.Second, 3, clock)

	// First 3 calls allowed.
	if !lim.Allow("a") {
		t.Error("call 1: want true, got false")
	}
	if !lim.Allow("a") {
		t.Error("call 2: want true, got false")
	}
	if !lim.Allow("a") {
		t.Error("call 3: want true, got false")
	}

	// 4th call within same window must be denied.
	if lim.Allow("a") {
		t.Error("call 4: want false, got true")
	}
}

func TestAllowAfterWindow(t *testing.T) {
	clock := newFakeClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	lim := NewLimiter(time.Second, 2, clock)

	lim.Allow("a") // t=0
	lim.Allow("a") // t=0 — exhausted

	if lim.Allow("a") {
		t.Error("call 3 at t=0: want false (exhausted)")
	}

	// Advance past the window.  The first call at t=0 is now expired.
	clock.advance(1100 * time.Millisecond) // t=1.1s — only the second call (also at t=0) is still in window? No — both calls at t=0 are now older than 1s.
	// Actually both are at t=0, window=1s, cutoff = now-window = 0.1s. So both t=0 timestamps are before cutoff. Slots open.
	if !lim.Allow("a") {
		t.Error("call at t=1.1s: want true (window slid past both t=0 calls)")
	}
}

func TestAllowSlidingWindow(t *testing.T) {
	clock := newFakeClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	lim := NewLimiter(2*time.Second, 2, clock)

	lim.Allow("a")                               // t=0s
	clock.advance(500 * time.Millisecond)         // t=0.5s
	lim.Allow("a")                               // t=0.5s — now exhausted
	clock.advance(600 * time.Millisecond)         // t=1.1s — window=[-0.9s, 1.1s]; first call at 0s is still in window, second at 0.5s too — still exhausted
	if lim.Allow("a") {
		t.Error("t=1.1s: want false (both calls still in window)")
	}

	clock.advance(1 * time.Second) // t=2.1s — window=[0.1s, 2.1s]; first call at 0s expired, second at 0.5s expired
	if !lim.Allow("a") {
		t.Error("t=2.1s: want true (window slid past both calls)")
	}
}

func TestSeparateIDs(t *testing.T) {
	clock := newFakeClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	lim := NewLimiter(time.Second, 1, clock)

	if !lim.Allow("a") {
		t.Error("a: want true")
	}
	if !lim.Allow("b") {
		t.Error("b: want true (separate limit)")
	}
	if lim.Allow("a") {
		t.Error("a second call: want false")
	}
}

func TestZeroMax(t *testing.T) {
	clock := newFakeClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	lim := NewLimiter(time.Second, 0, clock)

	if lim.Allow("a") {
		t.Error("max=0: want false")
	}
}

func TestMaxOne(t *testing.T) {
	clock := newFakeClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	lim := NewLimiter(time.Second, 1, clock)

	if !lim.Allow("a") {
		t.Error("first: want true")
	}
	if lim.Allow("a") {
		t.Error("second: want false")
	}

	// After window passes, allowed again.
	clock.advance(2 * time.Second)
	if !lim.Allow("a") {
		t.Error("after window: want true")
	}
}

func TestPruneLargeHistory(t *testing.T) {
	// Simulate a participant that was very active, then went idle
	// for a long time.  The slice should be pruned to empty.
	clock := newFakeClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	lim := NewLimiter(time.Second, 100, clock)

	// Exhaust the limit with 100 calls at t=0.
	for i := 0; i < 100; i++ {
		if !lim.Allow("a") {
			t.Fatalf("call %d unexpectedly denied", i+1)
		}
	}
	if lim.Allow("a") {
		t.Fatal("101st call should be denied")
	}

	// Advance far past the window.
	clock.advance(1 * time.Hour)

	// Now all 100 should be pruned, and we can make 100 fresh calls.
	for i := 0; i < 100; i++ {
		if !lim.Allow("a") {
			t.Fatalf("call %d after prune: unexpectedly denied", i+1)
		}
	}
}

func TestConcurrent(t *testing.T) {
	clock := newFakeClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	lim := NewLimiter(time.Second, 5, clock)

	const numGoroutines = 50
	var allowed atomic.Int32

	var wg sync.WaitGroup
	for range numGoroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if lim.Allow("a") {
				allowed.Add(1)
			}
		}()
	}
	wg.Wait()

	if n := allowed.Load(); n > 5 {
		t.Errorf("allowed %d out of 50, want ≤ 5", n)
	}
}

func TestConcurrentMultipleIDs(t *testing.T) {
	clock := newFakeClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	lim := NewLimiter(time.Second, 3, clock)

	const numIDs = 10
	const numGoroutinesPerID = 20

	var allowed [numIDs]atomic.Int32

	var wg sync.WaitGroup
	for id := range numIDs {
		for range numGoroutinesPerID {
			wg.Add(1)
			go func(ident string, idx int) {
				defer wg.Done()
				if lim.Allow(ident) {
					allowed[idx].Add(1)
				}
			}(string(rune('a'+id)), id)
		}
	}
	wg.Wait()

	for i := range numIDs {
		if n := allowed[i].Load(); n > 3 {
			t.Errorf("id %c: allowed %d out of %d, want ≤ 3", 'a'+i, n, numGoroutinesPerID)
		}
	}
}

func TestRealClock(t *testing.T) {
	var c RealClock
	now := c.Now()
	if now.IsZero() {
		t.Error("RealClock.Now() returned zero time")
	}
}
