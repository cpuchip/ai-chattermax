package ratelimit

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeClock is an injectable, mutable clock for deterministic tests.
type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func newFakeClock(t time.Time) *fakeClock { return &fakeClock{now: t} }

func (f *fakeClock) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now
}

func (f *fakeClock) Advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = f.now.Add(d)
}

func TestLimiter_TableDriven(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		limit        int
		window       time.Duration
		calls        int
		advanceAfter int           // advance the clock after this many calls (0 = never)
		advanceBy    time.Duration
		wantAllowed  int
	}{
		{
			name:        "under limit: all allowed",
			limit:       3,
			window:      time.Second,
			calls:       2,
			wantAllowed: 2,
		},
		{
			name:        "at limit: all allowed",
			limit:       3,
			window:      time.Second,
			calls:       3,
			wantAllowed: 3,
		},
		{
			name:        "over limit: excess denied",
			limit:       3,
			window:      time.Second,
			calls:       5,
			wantAllowed: 3,
		},
		{
			name:         "window expiry: advances reset the budget",
			limit:        2,
			window:       time.Second,
			calls:        4,
			advanceAfter: 2,
			advanceBy:    2 * time.Second,
			wantAllowed:  4,
		},
		{
			name:         "partial window expiry: only some slots freed",
			limit:        3,
			window:       time.Second,
			calls:        5,
			advanceAfter: 3,
			advanceBy:    500 * time.Millisecond, // not enough to expire any
			wantAllowed:  3,
		},
		{
			name:        "limit of 1: single action then denied",
			limit:       1,
			window:      time.Minute,
			calls:       5,
			wantAllowed: 1,
		},
		{
			name:        "limit of 0: always denied",
			limit:       0,
			window:      time.Second,
			calls:       3,
			wantAllowed: 0,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			clk := newFakeClock(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))
			lim := New(tc.limit, tc.window, clk)

			allowed := 0
			for i := 0; i < tc.calls; i++ {
				if tc.advanceAfter > 0 && i == tc.advanceAfter {
					clk.Advance(tc.advanceBy)
				}
				if lim.Allow("user1") {
					allowed++
				}
			}

			if allowed != tc.wantAllowed {
				t.Errorf("got %d allowed, want %d", allowed, tc.wantAllowed)
			}
		})
	}
}

func TestLimiter_PerParticipant(t *testing.T) {
	t.Parallel()
	clk := newFakeClock(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))
	lim := New(2, time.Second, clk)

	// alice uses her quota
	if !lim.Allow("alice") {
		t.Error("alice: first call should be allowed")
	}
	if !lim.Allow("alice") {
		t.Error("alice: second call should be allowed")
	}
	if lim.Allow("alice") {
		t.Error("alice: third call should be denied")
	}

	// bob is independent
	if !lim.Allow("bob") {
		t.Error("bob: first call should be allowed")
	}
	if !lim.Allow("bob") {
		t.Error("bob: second call should be allowed")
	}
	if lim.Allow("bob") {
		t.Error("bob: third call should be denied")
	}
}

func TestLimiter_RollingWindow(t *testing.T) {
	t.Parallel()
	clk := newFakeClock(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))
	lim := New(2, time.Second, clk)

	// Use the full budget.
	lim.Allow("u")
	clk.Advance(400 * time.Millisecond)
	lim.Allow("u")

	// Now at limit. Advance enough that the first action falls out of the window.
	clk.Advance(700 * time.Millisecond) // 1.1s total: first action at t=0 is now > 1s old

	if !lim.Allow("u") {
		t.Error("expected first slot to be freed by rolling window")
	}
}

func TestLimiter_NilClockUsesRealTime(t *testing.T) {
	t.Parallel()
	lim := New(10, time.Second, nil)
	if !lim.Allow("x") {
		t.Error("first call with real clock should be allowed")
	}
}

func TestLimiter_Concurrent(t *testing.T) {
	t.Parallel()
	clk := newFakeClock(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))
	lim := New(100, time.Second, clk)

	const goroutines = 64
	const callsPerGoroutine = 50

	var wg sync.WaitGroup
	var allowed atomic.Int64

	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < callsPerGoroutine; i++ {
				if lim.Allow("shared") {
					allowed.Add(1)
				}
			}
		}()
	}
	wg.Wait()

	if n := allowed.Load(); n > 100 {
		t.Errorf("allowed %d actions, but limit is 100", n)
	}
}

func TestLimiter_ConcurrentMultiParticipant(t *testing.T) {
	t.Parallel()
	clk := newFakeClock(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))
	const perParticipantLimit = 50
	lim := New(perParticipantLimit, time.Second, clk)

	participants := []string{"alice", "bob", "carol", "dave"}
	const goroutinesPerParticipant = 8
	const callsPerGoroutine = 20

	counters := make(map[string]*atomic.Int64)
	for _, p := range participants {
		counters[p] = &atomic.Int64{}
	}

	var wg sync.WaitGroup
	for _, p := range participants {
		p := p
		ctr := counters[p]
		for g := 0; g < goroutinesPerParticipant; g++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for i := 0; i < callsPerGoroutine; i++ {
					if lim.Allow(p) {
						ctr.Add(1)
					}
				}
			}()
		}
	}
	wg.Wait()

	totalAttempts := goroutinesPerParticipant * callsPerGoroutine
	for p, ctr := range counters {
		n := ctr.Load()
		if n > perParticipantLimit {
			t.Errorf("participant %s: allowed %d, exceeds limit of %d", p, n, perParticipantLimit)
		}
		if n == int64(totalAttempts) {
			t.Errorf("participant %s: all %d calls allowed, expected some denials", p, totalAttempts)
		}
	}
}
