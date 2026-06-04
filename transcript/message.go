package transcript

import "time"

// Message represents a single chat message in a room.
type Message struct {
	RoomID    string
	Sender    string
	Body      string
	Timestamp time.Time
}

// Store persists and retrieves messages by room.
type Store interface {
	Append(msg Message) error
	Replay(roomID string) ([]Message, error)
}
