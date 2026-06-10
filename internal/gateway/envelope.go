// Package gateway is the single multiplexed WebSocket per client: one connection
// carries every channel (room) the client is subscribed to. It generalizes the
// earlier per-room socket (AX3-2) into a typed envelope with a channel field.
package gateway

import "github.com/cpuchip/ai-chattermax/internal/store"

// Client→server frame types: "subscribe", "message", "history", "typing",
// "reaction". Server→client frame types: "ready", "subscribed", "message",
// "history", "presence", "reaction", "error".
type clientFrame struct {
	Type     string   `json:"type"`
	Channel  string   `json:"channel,omitempty"`
	Channels []string `json:"channels,omitempty"`
	Body     string   `json:"body,omitempty"`
	Limit    int      `json:"limit,omitempty"`
	// Reaction fields (type == "reaction").
	MessageID string `json:"messageId,omitempty"`
	Emoji     string `json:"emoji,omitempty"`
	Op        string `json:"op,omitempty"` // add | remove
}

// Participant is a presence/roster entry (deduped by ID across connections).
type Participant struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Kind   string `json:"kind"` // human | persona
	Avatar string `json:"avatar,omitempty"`
}

// Outbound frame shapes.
type readyFrame struct {
	Type    string      `json:"type"` // "ready"
	Session Participant `json:"session"`
}

type messageFrame struct {
	Type    string        `json:"type"` // "message"
	Channel string        `json:"channel"`
	Message store.Message `json:"message"`
}

type historyFrame struct {
	Type     string          `json:"type"` // "history"
	Channel  string          `json:"channel"`
	Messages []store.Message `json:"messages"`
}

type presenceFrame struct {
	Type    string        `json:"type"`            // "presence"
	Channel string        `json:"channel"`
	State   string        `json:"state,omitempty"` // join | leave (for a single who)
	Who     *Participant  `json:"who,omitempty"`
	Roster  []Participant `json:"roster,omitempty"` // full snapshot (on subscribe)
}

type typingFrame struct {
	Type    string `json:"type"` // "typing"
	Channel string `json:"channel"`
	Who     string `json:"who"`
}

type reactionFrame struct {
	Type      string      `json:"type"` // "reaction"
	Channel   string      `json:"channel"`
	MessageID string      `json:"messageId"`
	Emoji     string      `json:"emoji"`
	Op        string      `json:"op"` // add | remove
	Who       Participant `json:"who"`
}

type errorFrame struct {
	Type    string `json:"type"` // "error"
	Message string `json:"message"`
}
