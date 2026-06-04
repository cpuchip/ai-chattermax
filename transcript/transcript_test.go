package transcript

import (
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestMemoryStore_AppendAndReplay(t *testing.T) {
	tests := []struct {
		name   string
		msgs   []Message
		roomID string
		want   []Message
	}{
		{
			name: "append then replay returns messages in order",
			msgs: []Message{
				{RoomID: "r1", Sender: "alice", Body: "hello", Timestamp: time.Unix(1, 0)},
				{RoomID: "r1", Sender: "bob", Body: "world", Timestamp: time.Unix(2, 0)},
			},
			roomID: "r1",
			want: []Message{
				{RoomID: "r1", Sender: "alice", Body: "hello", Timestamp: time.Unix(1, 0)},
				{RoomID: "r1", Sender: "bob", Body: "world", Timestamp: time.Unix(2, 0)},
			},
		},
		{
			name:   "replay unknown room returns empty slice",
			msgs:   []Message{{RoomID: "r1", Sender: "alice", Body: "hello", Timestamp: time.Unix(1, 0)}},
			roomID: "r2",
			want:   []Message{},
		},
		{
			name:   "replay empty store returns empty slice",
			msgs:   nil,
			roomID: "r1",
			want:   []Message{},
		},
		{
			name: "isolation between rooms",
			msgs: []Message{
				{RoomID: "r1", Sender: "alice", Body: "a", Timestamp: time.Unix(1, 0)},
				{RoomID: "r2", Sender: "bob", Body: "b", Timestamp: time.Unix(2, 0)},
			},
			roomID: "r1",
			want: []Message{
				{RoomID: "r1", Sender: "alice", Body: "a", Timestamp: time.Unix(1, 0)},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewMemoryStore()
			for _, msg := range tt.msgs {
				if err := s.Append(msg); err != nil {
					t.Fatalf("Append() error = %v", err)
				}
			}
			got, err := s.Replay(tt.roomID)
			if err != nil {
				t.Fatalf("Replay() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Replay() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMemoryStore_ConcurrentAccess(t *testing.T) {
	s := NewMemoryStore()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_ = s.Append(Message{
				RoomID:    "room",
				Sender:    "user",
				Body:      "msg",
				Timestamp: time.Now(),
			})
		}()
		go func() {
			defer wg.Done()
			_, _ = s.Replay("room")
		}()
	}
	wg.Wait()

	msgs, err := s.Replay("room")
	if err != nil {
		t.Fatalf("Replay() error = %v", err)
	}
	if len(msgs) != 100 {
		t.Errorf("got %d messages, want 100", len(msgs))
	}
}
