package presence

import (
	"reflect"
	"sync"
	"testing"
)

func TestTracker_JoinLeave(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*Tracker)
		want  []Participant
	}{
		{
			name:  "empty tracker returns empty roster",
			setup: func(_ *Tracker) {},
			want:  []Participant{},
		},
		{
			name: "single participant",
			setup: func(tr *Tracker) {
				tr.Join("alice", Human)
			},
			want: []Participant{
				{ID: "alice", Kind: Human, Online: true, Idle: false, Thinking: false},
			},
		},
		{
			name: "multiple participants sorted by ID",
			setup: func(tr *Tracker) {
				tr.Join("charlie", Persona)
				tr.Join("alice", Human)
				tr.Join("bob", Human)
			},
			want: []Participant{
				{ID: "alice", Kind: Human, Online: true, Idle: false, Thinking: false},
				{ID: "bob", Kind: Human, Online: true, Idle: false, Thinking: false},
				{ID: "charlie", Kind: Persona, Online: true, Idle: false, Thinking: false},
			},
		},
		{
			name: "leave removes participant",
			setup: func(tr *Tracker) {
				tr.Join("alice", Human)
				tr.Join("bob", Human)
				tr.Leave("alice")
			},
			want: []Participant{
				{ID: "bob", Kind: Human, Online: true, Idle: false, Thinking: false},
			},
		},
		{
			name: "leave unknown participant is no-op",
			setup: func(tr *Tracker) {
				tr.Join("alice", Human)
				tr.Leave("unknown")
			},
			want: []Participant{
				{ID: "alice", Kind: Human, Online: true, Idle: false, Thinking: false},
			},
		},
		{
			name: "join updates existing participant kind",
			setup: func(tr *Tracker) {
				tr.Join("alice", Human)
				tr.SetIdle("alice", true)
				tr.SetThinking("alice", true)
				tr.Join("alice", Persona)
			},
			want: []Participant{
				{ID: "alice", Kind: Persona, Online: true, Idle: false, Thinking: false},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := NewTracker()
			tt.setup(tr)
			got := tr.Roster()
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Roster() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTracker_IdleTransitions(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*Tracker)
		want  []Participant
	}{
		{
			name: "set idle on present participant",
			setup: func(tr *Tracker) {
				tr.Join("alice", Human)
				tr.SetIdle("alice", true)
			},
			want: []Participant{
				{ID: "alice", Kind: Human, Online: true, Idle: true, Thinking: false},
			},
		},
		{
			name: "clear idle on present participant",
			setup: func(tr *Tracker) {
				tr.Join("alice", Human)
				tr.SetIdle("alice", true)
				tr.SetIdle("alice", false)
			},
			want: []Participant{
				{ID: "alice", Kind: Human, Online: true, Idle: false, Thinking: false},
			},
		},
		{
			name: "set idle on unknown participant is no-op",
			setup: func(tr *Tracker) {
				tr.Join("alice", Human)
				tr.SetIdle("unknown", true)
			},
			want: []Participant{
				{ID: "alice", Kind: Human, Online: true, Idle: false, Thinking: false},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := NewTracker()
			tt.setup(tr)
			got := tr.Roster()
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Roster() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTracker_ThinkingTransitions(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*Tracker)
		want  []Participant
	}{
		{
			name: "set thinking on present participant",
			setup: func(tr *Tracker) {
				tr.Join("bot", Persona)
				tr.SetThinking("bot", true)
			},
			want: []Participant{
				{ID: "bot", Kind: Persona, Online: true, Idle: false, Thinking: true},
			},
		},
		{
			name: "clear thinking on present participant",
			setup: func(tr *Tracker) {
				tr.Join("bot", Persona)
				tr.SetThinking("bot", true)
				tr.SetThinking("bot", false)
			},
			want: []Participant{
				{ID: "bot", Kind: Persona, Online: true, Idle: false, Thinking: false},
			},
		},
		{
			name: "set thinking on unknown participant is no-op",
			setup: func(tr *Tracker) {
				tr.Join("bot", Persona)
				tr.SetThinking("unknown", true)
			},
			want: []Participant{
				{ID: "bot", Kind: Persona, Online: true, Idle: false, Thinking: false},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := NewTracker()
			tt.setup(tr)
			got := tr.Roster()
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Roster() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTracker_RosterSnapshotIndependence(t *testing.T) {
	tr := NewTracker()
	tr.Join("alice", Human)
	tr.Join("bob", Persona)

	snapshot := tr.Roster()

	// Mutate internal state after taking the snapshot.
	tr.Leave("alice")
	tr.SetIdle("bob", true)
	tr.SetThinking("bob", true)

	want := []Participant{
		{ID: "alice", Kind: Human, Online: true, Idle: false, Thinking: false},
		{ID: "bob", Kind: Persona, Online: true, Idle: false, Thinking: false},
	}

	if !reflect.DeepEqual(snapshot, want) {
		t.Errorf("snapshot mutated after internal changes = %v, want %v", snapshot, want)
	}
}

func TestTracker_ConcurrentAccess(t *testing.T) {
	tr := NewTracker()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(5)
		go func(n int) {
			defer wg.Done()
			tr.Join("id", Human)
		}(i)
		go func(n int) {
			defer wg.Done()
			tr.Leave("id")
		}(i)
		go func(n int) {
			defer wg.Done()
			tr.SetIdle("id", true)
		}(i)
		go func(n int) {
			defer wg.Done()
			tr.SetThinking("id", true)
		}(i)
		go func(n int) {
			defer wg.Done()
			_ = tr.Roster()
		}(i)
	}
	wg.Wait()

	// We verify only that we do not panic or deadlock.
}
