package presence

// Kind indicates whether a participant is a human user or an AI persona.
type Kind string

const (
	Human   Kind = "human"
	Persona Kind = "persona"
)

// Participant represents the presence state of a single participant.
type Participant struct {
	ID       string
	Kind     Kind
	Online   bool
	Idle     bool
	Thinking bool
}
