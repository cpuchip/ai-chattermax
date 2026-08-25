// Package gateway is the single multiplexed WebSocket per client: one connection
// carries every channel (room) the client is subscribed to. It generalizes the
// earlier per-room socket (AX3-2) into a typed envelope with a channel field.
package gateway

import "github.com/cpuchip/ai-chattermax/internal/store"

// Client→server frame types: "subscribe", "message", "history", "typing",
// "reaction", "mood". Server→client frame types: "ready", "message", "history",
// "presence", "typing", "reaction", "mood", "cast", "program", "initiative",
// "notification", "error".
//
// There is NO "subscribed" ack frame and there never was — an earlier draft of
// this comment listed one and it survived here long enough to reach a
// cross-stack protocol review (claks treaty v0.1, 2026-08-25) as a genuine
// ambiguity. Subscribe is acknowledged by its effects: a "history" frame and a
// roster-bearing "presence" frame. This comment is now the wire contract's
// inventory; if a frame type is added, add it here in the same change.
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
	// Mood field (type == "mood").
	Mood string `json:"mood,omitempty"`
	// Echo (type == "message"): opt in to receiving your own message's
	// broadcast frame — the authoritative server-assigned id/ts — instead of
	// relying on optimistic rendering. Added for agent clients that need to
	// correlate their own sends (claks treaty Q3, ruled by Michael 2026-08-25;
	// chillacks converged on the same opt-in independently). Default stays
	// false: the browser UI renders optimistically and never sets it.
	Echo bool `json:"echo,omitempty"`
	// Cast attribution (type == "message", personas only): speak as a named
	// cast member — auto-created on first use (DH-2).
	SubPersona string `json:"subPersona,omitempty"`
}

// Participant is a presence/roster entry (deduped by ID across connections).
type Participant struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Kind   string `json:"kind"` // human | persona
	Avatar string `json:"avatar,omitempty"`
	Mood   string `json:"mood,omitempty"` // emoji status (humans, REM-3)
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

type notificationFrame struct {
	Type         string             `json:"type"` // "notification"
	Notification store.Notification `json:"notification"`
}

type moodFrame struct {
	Type    string      `json:"type"` // "mood"
	Channel string      `json:"channel"`
	Who     Participant `json:"who"`
}

type castFrame struct {
	Type    string             `json:"type"` // "cast"
	Channel string             `json:"channel"`
	Cast    []store.SubPersona `json:"cast"`
}

// programFrame announces a holodeck session boundary (DH-4): persona hosts
// write their campaign notes and rotate sessions on "archive"; "resume"
// reopens play on fresh sessions seeded from the campaign log.
type programFrame struct {
	Type    string `json:"type"` // "program"
	Channel string `json:"channel"`
	Op      string `json:"op"` // archive | resume
	By      string `json:"by"`
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
