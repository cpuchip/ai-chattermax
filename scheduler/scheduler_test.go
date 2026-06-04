package scheduler

import (
	"sync"
	"testing"
	"time"
)

// fakeClock provides a controllable, frozen time source for deterministic tests.
type fakeClock struct {
	t time.Time
}

func newFakeClock(t time.Time) *fakeClock {
	return &fakeClock{t: t}
}

func (fc *fakeClock) Now() time.Time {
	return fc.t
}

func (fc *fakeClock) Add(d time.Duration) {
	fc.t = fc.t.Add(d)
}

func TestScheduler_NextTurn_RoundRobin(t *testing.T) {
	fc := newFakeClock(time.Now())
	s := New(fc.Now, time.Minute, 3)

	s.AddParticipant("a")
	s.AddParticipant("b")
	s.AddParticipant("c")

	want := []string{"a", "b", "c", "a", "b"}
	for i, w := range want {
		if got := s.NextTurn(); got != w {
			t.Errorf("turn %d = %q, want %q", i, got, w)
		}
	}
}

func TestScheduler_AddRemove_MidRotation(t *testing.T) {
	fc := newFakeClock(time.Now())
	s := New(fc.Now, time.Minute, 3)

	s.AddParticipant("a")
	s.AddParticipant("b")
	s.AddParticipant("c")

	// Consume "a".
	if got := s.NextTurn(); got != "a" {
		t.Fatalf("first turn = %q, want a", got)
	}

	// Remove "b" while cursor points at index 1 (b).
	s.RemoveParticipant("b")

	// NextTurn should return "c", then "a".
	want := []string{"c", "a", "c"}
	for i, w := range want {
		if got := s.NextTurn(); got != w {
			t.Errorf("turn after remove %d = %q, want %q", i, got, w)
		}
	}

	// Re-add "b" and verify it joins the rotation at the end.
	s.AddParticipant("b")
	if got := s.NextTurn(); got != "a" {
		t.Errorf("turn after re-add = %q, want a", got)
	}

	// Remove "c" (now at index 1); the element at index 1 becomes "b".
	s.RemoveParticipant("c")
	if got := s.NextTurn(); got != "b" {
		t.Errorf("turn after removing c = %q, want b", got)
	}
	if got := s.NextTurn(); got != "a" {
		t.Errorf("turn after removing c (2) = %q, want a", got)
	}
}

func TestScheduler_Allow_RateCeiling(t *testing.T) {
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	fc := newFakeClock(base)
	window := 5 * time.Minute
	cap := 3
	s := New(fc.Now, window, cap)

	id := "user1"

	// Allow exactly cap actions.
	for i := 0; i < cap; i++ {
		if !s.Allow(id) {
			t.Fatalf("action %d should be allowed", i+1)
		}
	}

	// Next action should be blocked.
	if s.Allow(id) {
		t.Fatal("action over cap should be blocked")
	}

	// Advance clock just shy of the window — still blocked.
	fc.Add(window - time.Second)
	if s.Allow(id) {
		t.Fatal("action still within window should be blocked")
	}

	// Advance clock past the window so all recorded actions expire.
	fc.Add(2 * time.Second)
	if !s.Allow(id) {
		t.Fatal("action after window roll should be allowed")
	}

	// We can allow up to cap-1 more actions before blocking again.
	for i := 0; i < cap-1; i++ {
		if !s.Allow(id) {
			t.Fatalf("refill action %d should be allowed", i+2)
		}
	}
	if s.Allow(id) {
		t.Fatal("action after refill cap should be blocked again")
	}
}

func TestScheduler_Allow_MultipleParticipants(t *testing.T) {
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	fc := newFakeClock(base)
	s := New(fc.Now, time.Minute, 2)

	// One participant hitting the cap does not affect another.
	if !s.Allow("alice") {
		t.Fatal("alice 1 should be allowed")
	}
	if !s.Allow("alice") {
		t.Fatal("alice 2 should be allowed")
	}
	if s.Allow("alice") {
		t.Fatal("alice 3 should be blocked")
	}

	if !s.Allow("bob") {
		t.Fatal("bob 1 should be allowed")
	}
	if !s.Allow("bob") {
		t.Fatal("bob 2 should be allowed")
	}
	if s.Allow("bob") {
		t.Fatal("bob 3 should be blocked")
	}
}

func TestScheduler_AddParticipant_Dedup(t *testing.T) {
	fc := newFakeClock(time.Now())
	s := New(fc.Now, time.Minute, 3)

	s.AddParticipant("a")
	s.AddParticipant("a")
	s.AddParticipant("b")

	want := []string{"a", "b", "a"}
	for i, w := range want {
		if got := s.NextTurn(); got != w {
			t.Errorf("turn %d = %q, want %q", i, got, w)
		}
	}
}

func TestScheduler_NextTurn_Empty(t *testing.T) {
	fc := newFakeClock(time.Now())
	s := New(fc.Now, time.Minute, 3)

	if got := s.NextTurn(); got != "" {
		t.Errorf("empty rotation = %q, want empty string", got)
	}
}

func TestScheduler_ConcurrentAccess(t *testing.T) {
	fc := newFakeClock(time.Now())
	s := New(fc.Now, time.Minute, 10)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(4)
		go func() {
			defer wg.Done()
			s.AddParticipant("x")
		}()
		go func() {
			defer wg.Done()
			s.RemoveParticipant("x")
		}()
		go func() {
			defer wg.Done()
			_ = s.NextTurn()
		}()
		go func() {
			defer wg.Done()
			_ = s.Allow("x")
		}()
	}
	wg.Wait()

	// We verify only that we do not panic or deadlock.
}
