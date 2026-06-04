package ratelimit

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// fakeClock provides deterministic time control for tests.
type fakeClock struct {
	t time.Time
}

func (f *fakeClock) Now() time.Time {
	return f.t
}

func (f *fakeClock) Add(d time.Duration) {
	f.t = f.t.Add(d)
}

func TestLimiter_AllowAndDeny(t *testing.T) {
	clk := &fakeClock{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	lim := NewLimiterWithClock(3, time.Minute, clk)

	if !lim.Allow("u1") {
		t.Fatal("expected first Allow to be true")
	}
	if !lim.Allow("u1") {
		t.Fatal("expected second Allow to be true")
	}
	if !lim.Allow("u1") {
		t.Fatal("expected third Allow to be true")
	}
	if lim.Allow("u1") {
		t.Fatal("expected fourth Allow to be false")
	}
}

func TestLimiter_WindowRollReallow(t *testing.T) {
	clk := &fakeClock{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	lim := NewLimiterWithClock(2, time.Minute, clk)

	lim.Allow("u1")
	lim.Allow("u1")
	if lim.Allow("u1") {
		t.Fatal("expected third Allow to be false")
	}

	// Advance past the window
	clk.Add(2 * time.Minute)

	if !lim.Allow("u1") {
		t.Fatal("expected Allow after window roll to be true")
	}
}

func TestLimiter_BoundaryAtExactCutoff(t *testing.T) {
	clk := &fakeClock{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	lim := NewLimiterWithClock(1, time.Minute, clk)

	lim.Allow("u1")
	clk.Add(time.Minute)

	// At exactly one minute, the old entry is at the cutoff (not after cutoff)
	if !lim.Allow("u1") {
		t.Fatal("expected Allow at exact cutoff to be true")
	}
}

func TestLimiter_DifferentIDsIndependent(t *testing.T) {
	clk := &fakeClock{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	lim := NewLimiterWithClock(1, time.Minute, clk)

	if !lim.Allow("u1") {
		t.Fatal("expected u1 first Allow to be true")
	}
	if !lim.Allow("u2") {
		t.Fatal("expected u2 first Allow to be true")
	}
}

func TestLimiter_ConcurrentStress(t *testing.T) {
	clk := &fakeClock{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	lim := NewLimiterWithClock(10, time.Minute, clk)

	const goroutines = 50
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			id := fmt.Sprintf("user-%d", g%5)
			for j := 0; j < 100; j++ {
				_ = lim.Allow(id)
			}
		}(i)
	}
	wg.Wait()
}
