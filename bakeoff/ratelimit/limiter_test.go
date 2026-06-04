package ratelimit

import (
	"math/rand/v2"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeClock implements Clock with a manual advance. It does not move
// on its own; tests call Advance to move it forward.
type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func newFakeClock(start time.Time) *fakeClock { return &fakeClock{now: start} }

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

func TestLimiter_AllowUnderWindow(t *testing.T) {
	start := time.Unix(1_700_000_000, 0)
	tests := []struct {
		name     string
		max      int
		window   time.Duration
		offsets  []time.Duration // offsets from clock start (cumulative deltas)
		expected []bool
	}{
		{
			name:     "max=1 allows one then denies",
			max:      1,
			window:   time.Second,
			offsets:  []time.Duration{0, 0, 0},
			expected: []bool{true, false, false},
		},
		{
			name:     "max=3 allows three then denies",
			max:      3,
			window:   time.Second,
			offsets:  []time.Duration{0, 0, 0, 0, 0},
			expected: []bool{true, true, true, false, false},
		},
		{
			name:    "oldest hit falls out of window and one more is allowed",
			max:     2,
			window:  time.Second,
			offsets: []time.Duration{0, 0, 0, 1100 * time.Millisecond, 0, 0},
			// 0:   allow (1/2)
			// 0:   allow (2/2)
			// 0:   deny
			// +1.1s: oldest (t=0) is now out of window, allow (1/2)
			// 0:   allow (2/2)
			// 0:   deny
			expected: []bool{true, true, false, true, true, false},
		},
		{
			name:     "exactly at window boundary: still in window",
			max:      1,
			window:   time.Second,
			offsets:  []time.Duration{0, 999 * time.Millisecond, 1001 * time.Millisecond},
			// 0:        allow
			// +999ms:   at window-1ms, still inside, deny
			// +1001ms:  at window+1ms, oldest is out, allow
			expected: []bool{true, false, true},
		},
		{
			name:     "first call always allowed (basic sanity)",
			max:      5,
			window:   time.Minute,
			offsets:  []time.Duration{0},
			expected: []bool{true},
		},
		{
			name:     "max=0 always denies",
			max:      0,
			window:   time.Second,
			offsets:  []time.Duration{0, 0, 0},
			expected: []bool{false, false, false},
		},
		{
			name:     "max=3 within large window: cap is total, not per-window",
			max:      3,
			window:   time.Hour,
			offsets:  []time.Duration{0, 100 * time.Millisecond, 100 * time.Millisecond, 100 * time.Millisecond, 100 * time.Millisecond},
			expected: []bool{true, true, true, false, false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clk := newFakeClock(start)
			l := NewLimiter(tt.max, tt.window, clk)
			var last time.Duration
			for i, off := range tt.offsets {
				delta := off - last
				if delta > 0 {
					clk.Advance(delta)
				}
				last = off
				if got := l.Allow("alice"); got != tt.expected[i] {
					t.Errorf("call %d (offset %v): Allow = %v, want %v",
						i, off, got, tt.expected[i])
				}
			}
		})
	}
}

// TestLimiter_PerIDBudget is split out because alternating ids is
// awkward in the cumulative-offset table above.
func TestLimiter_PerIDBudget(t *testing.T) {
	clk := newFakeClock(time.Unix(1_700_000_000, 0))
	l := NewLimiter(1, time.Second, clk)
	if got := l.Allow("alice"); got != true {
		t.Errorf("alice[0] = %v, want true", got)
	}
	if got := l.Allow("bob"); got != true {
		t.Errorf("bob[0] = %v, want true", got)
	}
	if got := l.Allow("alice"); got != false {
		t.Errorf("alice[1] = %v, want false (alice used her 1/1)", got)
	}
	if got := l.Allow("bob"); got != false {
		t.Errorf("bob[1] = %v, want false (bob used his 1/1)", got)
	}
}

func TestLimiter_DefaultClockIsReal(t *testing.T) {
	l := NewLimiter(1, time.Hour, nil)
	if !l.Allow("alice") {
		t.Errorf("first call with default real clock should allow")
	}
	if l.Allow("alice") {
		t.Errorf("second call within an hour should deny")
	}
}

// TestLimiter_Concurrent is a -race-friendly stress test. Many
// goroutines call Allow on shared ids. The per-id cap must hold
// exactly (no id can be allowed more than `max` times, because the
// window never closes during the test).
func TestLimiter_Concurrent(t *testing.T) {
	const (
		nGoroutines = 32
		callsEach   = 1000
		idCount     = 8
	)
	max := 50
	window := time.Hour // large enough that nothing falls out
	clk := newFakeClock(time.Unix(1_700_000_000, 0))
	l := NewLimiter(max, window, clk)

	var allowed atomic.Int64
	var wg sync.WaitGroup
	for g := 0; g < nGoroutines; g++ {
		wg.Add(1)
		go func(seed uint64) {
			defer wg.Done()
			r := rand.New(rand.NewPCG(seed, seed+0x9E3779B97F4A7C15))
			for i := 0; i < callsEach; i++ {
				id := "id-" + strconv.Itoa(r.IntN(idCount))
				if l.Allow(id) {
					allowed.Add(1)
				}
			}
		}(uint64(g + 1))
	}
	wg.Wait()

	// The per-id cap is `max`. The window never closes, so the total
	// number of allowed calls must be exactly max * idCount — not
	// more (cap violated) and not less (test logic wrong).
	want := int64(max * idCount)
	if got := allowed.Load(); got != want {
		t.Errorf("allowed = %d, want exactly %d (per-id cap with a window that never closes)", got, want)
	}
}
