package transcript

import "sync"

// memoryStore is a concurrency-safe in-memory implementation of Store.
type memoryStore struct {
	mu   sync.Mutex
	room map[string][]Message
}

// NewMemoryStore creates a new in-memory Store.
func NewMemoryStore() *memoryStore {
	return &memoryStore{
		room: make(map[string][]Message),
	}
}

// Append saves a message to the store.
func (s *memoryStore) Append(msg Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.room[msg.RoomID] = append(s.room[msg.RoomID], msg)
	return nil
}

// Replay returns messages for a room in insertion order.
// Returns an empty slice for unknown or empty rooms.
func (s *memoryStore) Replay(roomID string) ([]Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	msgs := s.room[roomID]
	out := make([]Message, len(msgs))
	copy(out, msgs)
	return out, nil
}
