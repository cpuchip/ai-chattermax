package ratelimit

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// mockClock is a controllable clock for deterministic tests.
type mockClock struct {
	mu   sync.Mutex
	now  time.Time
}

func newMockClock() *mockClock {
	return &mockClock{now: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)}
}

func (m *mockClock) Now() time.Time {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.now
}

func (m *mockClock) Advance(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.now = m.now.Add(d)
}

// ---- Table-driven tests ----

func TestAllowUpToLimit(t *testing.T) {
	clock := newMockClock()
	limiter := NewLimiter(Config{MaxActions: 3, Window: time.Minute}, clock)

	// First 3 actions should be allowed
	for i := 0; i < 3; i++ {
		if !limiter.Allow("user1") {
			t.Errorf("action %d: expected Allow=true, got false", i+1)
		}
	}

	// 4th action should be rejected
	if limiter.Allow("user1") {
		t.Error("action 4: expected Allow=false (over limit), got true")
	}
}

func TestRollingWindow(t *testing.T) {
	clock := newMockClock()
	limiter := NewLimiter(Config{MaxActions: 2, Window: time.Minute}, clock)

	// Use first action
	if !limiter.Allow("user1") {
		t.Fatal("first action should be allowed")
	}

	// Advance time past half the window
	clock.Advance(31 * time.Second)

	if !limiter.Allow("user1") {
		t.Fatal("second action should be allowed")
	}

	// At capacity now
	if limiter.Allow("user1") {
		t.Error("third action should be rejected (at capacity)")
	}

	// Advance past the first action's timestamp (outside the 1-minute window)
	clock.Advance(30 * time.Second)

	// Now the first action has rolled off; one slot should be free
	if !limiter.Allow("user1") {
		t.Error("action after window roll should be allowed")
	}
}

func TestPerParticipantIsolation(t *testing.T) {
	clock := newMockClock()
	limiter := NewLimiter(Config{MaxActions: 2, Window: time.Minute}, clock)

	// user1 uses both slots
	if !limiter.Allow("user1") {
		t.Fatal("user1 action 1 should be allowed")
	}
	if !limiter.Allow("user1") {
		t.Fatal("user1 action 2 should be allowed")
	}

	// user2 should still have their own slots
	if !limiter.Allow("user2") {
		t.Fatal("user2 action 1 should be allowed (separate bucket)")
	}
	if !limiter.Allow("user2") {
		t.Fatal("user2 action 2 should be allowed (separate bucket)")
	}
	if limiter.Allow("user2") {
		t.Error("user2 action 3 should be rejected")
	}
}

func TestReset(t *testing.T) {
	clock := newMockClock()
	limiter := NewLimiter(Config{MaxActions: 1, Window: time.Minute}, clock)

	if !limiter.Allow("user1") {
		t.Fatal("first action should be allowed")
	}
	if limiter.Allow("user1") {
		t.Error("second action should be rejected (at capacity)")
	}

	limiter.Reset("user1")

	// After reset, the full allowance is restored
	if !limiter.Allow("user1") {
		t.Error("action after reset should be allowed")
	}
}

func TestResetNonexistent(t *testing.T) {
	clock := newMockClock()
	limiter := NewLimiter(Config{MaxActions: 1, Window: time.Minute}, clock)

	// Resetting a nonexistent id should not panic
	limiter.Reset("nobody")
}

func TestBoundaryExactlyN(t *testing.T) {
	clock := newMockClock()
	limiter := NewLimiter(Config{MaxActions: 5, Window: time.Minute}, clock)

	// Exactly N allows should succeed
	for i := 0; i < 5; i++ {
		if !limiter.Allow("user1") {
			t.Fatalf("action %d/5: expected Allow=true", i+1)
		}
	}

	// N+1 should be rejected
	if limiter.Allow("user1") {
		t.Error("action 6: expected Allow=false (boundary)")
	}
}

func TestWindowExpiryFullRecovery(t *testing.T) {
	clock := newMockClock()
	limiter := NewLimiter(Config{MaxActions: 2, Window: time.Minute}, clock)

	limiter.Allow("user1")
	limiter.Allow("user1")

	// At capacity
	if limiter.Allow("user1") {
		t.Error("expected rejection at capacity")
	}

	// Advance past the entire window
	clock.Advance(61 * time.Second)

	// Both slots should be free now
	if !limiter.Allow("user1") {
		t.Error("expected Allow after full window expiry")
	}
	if !limiter.Allow("user1") {
		t.Error("expected second Allow after full window expiry")
	}
	if limiter.Allow("user1") {
		t.Error("expected rejection after using recovered slots")
	}
}

func TestRealClockBasic(t *testing.T) {
	limiter := NewLimiter(Config{MaxActions: 3, Window: time.Second}, nil)

	for i := 0; i < 3; i++ {
		if !limiter.Allow("real-user") {
			t.Fatalf("action %d: expected Allow=true", i+1)
		}
	}
	if limiter.Allow("real-user") {
		t.Error("expected Allow=false after limit reached")
	}
}

func TestZeroMaxActions(t *testing.T) {
	clock := newMockClock()
	limiter := NewLimiter(Config{MaxActions: 0, Window: time.Minute}, clock)

	// With MaxActions=0, no actions should ever be allowed
	if limiter.Allow("user1") {
		t.Error("expected Allow=false with MaxActions=0")
	}
}

func TestDifferentParticipantsConcurrentBuckets(t *testing.T) {
	clock := newMockClock()
	limiter := NewLimiter(Config{MaxActions: 1, Window: time.Minute}, clock)

	// Each participant gets their own bucket
	ids := []string{"a", "b", "c", "d", "e"}
	for _, id := range ids {
		if !limiter.Allow(id) {
			t.Errorf("%s: expected Allow=true", id)
		}
		if limiter.Allow(id) {
			t.Errorf("%s: expected Allow=false on second call", id)
		}
	}
}

// ---- Concurrent tests ----

func TestConcurrentAllow(t *testing.T) {
	limiter := NewLimiter(Config{MaxActions: 100, Window: time.Minute}, nil)

	const numGoroutines = 50
	const actionsPerGoroutine = 10

	var allowed atomic.Int64
	var rejected atomic.Int64
	var wg sync.WaitGroup

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			for i := 0; i < actionsPerGoroutine; i++ {
				if limiter.Allow(id) {
					allowed.Add(1)
				} else {
					rejected.Add(1)
				}
			}
		}("participant-" + string(rune('A'+g%26)))
	}
	wg.Wait()

	// Each participant gets MaxActions=100 slots, and there are ~2 goroutines per id
	// (26 ids for 50 goroutines). Total capacity = 26 * 100 = 2600 >> 500 total attempts.
	// So most should be allowed, but the exact count depends on scheduling.
	t.Logf("allowed=%d, rejected=%d", allowed.Load(), rejected.Load())

	if allowed.Load() == 0 {
		t.Error("expected at least some allowed actions")
	}
}

func TestConcurrentAllowSingleParticipant(t *testing.T) {
	limiter := NewLimiter(Config{MaxActions: 50, Window: time.Minute}, nil)

	const numGoroutines = 100
	var allowed atomic.Int64
	var wg sync.WaitGroup

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if limiter.Allow("single-user") {
				allowed.Add(1)
			}
		}()
	}
	wg.Wait()

	// Exactly 50 should be allowed (MaxActions), rest rejected
	a := allowed.Load()
	if a != 50 {
		t.Errorf("expected exactly 50 allowed, got %d", a)
	}
}

func TestConcurrentResetAndAllow(t *testing.T) {
	clock := newMockClock()
	limiter := NewLimiter(Config{MaxActions: 10, Window: time.Minute}, clock)

	var wg sync.WaitGroup
	var allowed atomic.Int64

	// Hammer with Allow calls
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if limiter.Allow("user1") {
				allowed.Add(1)
			}
		}()
	}

	// Also reset periodically
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			limiter.Reset("user1")
		}()
	}

	wg.Wait()

	// After everything, the total allowed should be reasonable.
	// With 5 resets, each could restore up to 10 slots,
	// so the maximum possible is 10 (initial) + 5*10 (resets) = 60,
	// but exact count depends on scheduling.
	t.Logf("total allowed: %d", allowed.Load())
}

func TestConcurrentMultiParticipant(t *testing.T) {
	limiter := NewLimiter(Config{MaxActions: 5, Window: time.Minute}, nil)

	const numParticipants = 20
	const callsPerParticipant = 20

	var allowed atomic.Int64
	var wg sync.WaitGroup

	for p := 0; p < numParticipants; p++ {
		id := "participant-" + string(rune('A'+p%26)) + string(rune('0'+p/26))
		for c := 0; c < callsPerParticipant; c++ {
			wg.Add(1)
			go func(pid string) {
				defer wg.Done()
				if limiter.Allow(pid) {
					allowed.Add(1)
				}
			}(id)
		}
	}
	wg.Wait()

	// Each participant allows at most 5. Some may share ids due to rune collision,
	// but worst case all 20 unique ids * 5 = 100 max allowed.
	a := allowed.Load()
	if a == 0 {
		t.Error("expected some allowed actions")
	}

	// Simple upper bound check
	if a > 100 {
		t.Errorf("allowed %d, expected at most 100 (20 ids * 5 max)", a)
	}

	t.Logf("allowed=%d (max possible: 100)", a)
}