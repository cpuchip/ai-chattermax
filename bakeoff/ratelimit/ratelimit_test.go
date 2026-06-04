package ratelimit

import (
	"sync"
	"testing"
	"time"
)

// fakeClock provides a deterministic, manually-advanced clock for tests.
type fakeClock struct {
	mu    sync.Mutex
	now   time.Time
}

func newFakeClock(start time.Time) *fakeClock {
	return &fakeClock{now: start}
}

func (f *fakeClock) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now
}

func (f *fakeClock) Add(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = f.now.Add(d)
}

func TestLimiterAllowsUpToCapacity(t *testing.T) {
	clk := newFakeClock(time.Now())
	lim := NewLimiterWithClock(3, time.Minute, clk)

	if !lim.Allow("alice") {
		t.Fatal("expected first Allow to be true")
	}
	if !lim.Allow("alice") {
		t.Fatal("expected second Allow to be true")
	}
	if !lim.Allow("alice") {
		t.Fatal("expected third Allow to be true")
	}
	if lim.Allow("alice") {
		t.Fatal("expected fourth Allow to be false")
	}
}

func TestLimiterResetsAfterWindowPasses(t *testing.T) {
	clk := newFakeClock(time.Now())
	lim := NewLimiterWithClock(2, time.Minute, clk)

	if !lim.Allow("bob") {
		t.Fatal("expected first Allow to be true")
	}
	if !lim.Allow("bob") {
		t.Fatal("expected second Allow to be true")
	}
	if lim.Allow("bob") {
		t.Fatal("expected third Allow to be false")
	}

	clk.Add(time.Minute + time.Second)

	if !lim.Allow("bob") {
		t.Fatal("expected Allow after window to be true")
	}
}

func TestLimiterSlidingWindow(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	clk := newFakeClock(start)
	lim := NewLimiterWithClock(3, time.Minute, clk)

	// Three actions at t=0.
	lim.Allow("charlie")
	lim.Allow("charlie")
	lim.Allow("charlie")

	if lim.Allow("charlie") {
		t.Fatal("expected fourth Allow to be false")
	}

	// Advance 30 seconds; the first three are still inside the window.
	clk.Add(30 * time.Second)
	if lim.Allow("charlie") {
		t.Fatal("expected Allow after 30s to still be false")
	}

	// Advance another 31 seconds so the initial actions slide out.
	clk.Add(31 * time.Second)
	if !lim.Allow("charlie") {
		t.Fatal("expected Allow after window slides to be true")
	}
}

func TestLimiterZeroCapacity(t *testing.T) {
	clk := newFakeClock(time.Now())
	lim := NewLimiterWithClock(0, time.Minute, clk)

	if lim.Allow("dave") {
		t.Fatal("expected zero-capacity limiter to reject everything")
	}
}

func TestLimiterIsolatedPerID(t *testing.T) {
	clk := newFakeClock(time.Now())
	lim := NewLimiterWithClock(1, time.Minute, clk)

	if !lim.Allow("eve") {
		t.Fatal("expected first eve Allow to be true")
	}
	if !lim.Allow("frank") {
		t.Fatal("expected first frank Allow to be true")
	}
	if lim.Allow("eve") {
		t.Fatal("expected second eve Allow to be false")
	}
	if lim.Allow("frank") {
		t.Fatal("expected second frank Allow to be false")
	}
}

func TestLimiterConcurrentStress(t *testing.T) {
	clk := newFakeClock(time.Now())
	lim := NewLimiterWithClock(100, time.Minute, clk)

	const goroutines = 100
	const attempts = 500

	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := string(rune('A' + id%26))
			for i := 0; i < attempts; i++ {
				lim.Allow(key)
			}
		}(g)
	}
	wg.Wait()

	// Only the first 100 per key should have been allowed.
	roster := []string{"A", "B", "C", "D", "E"}
	for _, key := range roster {
		// Drain and count.
		var count int
		for i := 0; i < attempts; i++ {
			if lim.Allow(key) {
				count++
			}
		}
		if count != 0 {
			// After 100+ Allow calls under the same window, none should succeed
			// unless we advance the clock, which we haven't.
			t.Fatalf("expected no further Allows for %s, got %d", key, count)
		}
	}
}

func TestLimiterConcurrentWithClockAdvance(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	clk := newFakeClock(start)
	lim := NewLimiterWithClock(10, time.Second, clk)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				lim.Allow("shared")
				clk.Add(200 * time.Millisecond)
			}
		}()
	}
	wg.Wait()
}
