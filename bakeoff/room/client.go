package room

// Client is a transport-agnostic chat client.
type Client interface {
	ID() string
	Send(data []byte) error
}
