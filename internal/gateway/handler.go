package gateway

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/cpuchip/ai-chattermax/internal/store"
	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 8 << 10
	sendBuffer     = 256
	historyOnJoin  = 50
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// UserResolver resolves the human behind a request (the session cookie).
type UserResolver func(r *http.Request) (store.User, bool)

// Handler upgrades /gateway connections and runs the per-client pumps.
type Handler struct {
	hub      *Hub
	store    *store.Store
	resolve  UserResolver
}

// NewHandler builds the gateway handler.
func NewHandler(hub *Hub, st *store.Store, resolve UserResolver) *Handler {
	return &Handler{hub: hub, store: st, resolve: resolve}
}

// ServeHTTP authenticates (session cookie → human, or ?key= → persona), upgrades
// to WebSocket, and runs the read/write pumps.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	who, human, persona, ok := h.authenticate(r)
	if !ok {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("gateway upgrade: %v", err)
		return
	}
	c := &Client{
		hub:  h.hub,
		conn: conn,
		send: make(chan []byte, sendBuffer),
		who:  who,
		subs: make(map[string]bool),
	}
	h.hub.register(c)
	c.enqueue(marshal(readyFrame{Type: "ready", Session: who}))

	go c.writePump()
	h.readPump(c, human, persona)
}

func (h *Handler) authenticate(r *http.Request) (who Participant, human *store.User, persona *store.Persona, ok bool) {
	if u, found := h.resolve(r); found {
		who = Participant{ID: u.ID, Name: u.DisplayName, Kind: "human", Avatar: u.AvatarURL, Mood: u.Mood}
		return who, &u, nil, true
	}
	if key := r.URL.Query().Get("key"); key != "" {
		p, found, err := h.store.ValidatePersonaKey(r.Context(), key)
		if err == nil && found {
			who = Participant{ID: p.ID, Name: p.DisplayName, Kind: "persona", Avatar: p.AvatarURL}
			return who, nil, &p, true
		}
	}
	return Participant{}, nil, nil, false
}

// readPump reads frames until the connection closes, then cleans up presence.
func (h *Handler) readPump(c *Client, human *store.User, persona *store.Persona) {
	defer func() {
		chans := h.hub.unregister(c)
		for _, ch := range chans {
			h.hub.broadcast(ch, marshal(presenceFrame{Type: "presence", Channel: ch, State: "leave", Who: &c.who}), c)
		}
		c.closeSend()
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		var f clientFrame
		if json.Unmarshal(data, &f) != nil {
			continue
		}
		switch f.Type {
		case "subscribe":
			for _, ch := range f.Channels {
				h.handleSubscribe(c, ch, human, persona)
			}
			if f.Channel != "" {
				h.handleSubscribe(c, f.Channel, human, persona)
			}
		case "message":
			h.handleMessage(c, f, human, persona)
		case "history":
			if kind, ok := h.channelKind(c, f.Channel, human, persona); ok {
				h.sendHistory(c, f.Channel, kind, f.Limit)
			}
		case "typing":
			if f.Channel != "" {
				h.hub.broadcast(f.Channel, marshal(typingFrame{Type: "typing", Channel: f.Channel, Who: c.who.Name}), c)
			}
		case "reaction":
			h.handleReaction(c, f, human, persona)
		case "mood":
			h.handleMood(c, f, human)
		}
	}
}

func (h *Handler) handleSubscribe(c *Client, channel string, human *store.User, persona *store.Persona) {
	if channel == "" {
		return
	}
	kind, ok := h.channelKind(c, channel, human, persona)
	if !ok {
		return
	}
	if !h.hub.subscribe(c, channel) {
		return // already subscribed
	}
	// History + roster snapshot to the new subscriber, then announce to others.
	h.sendHistory(c, channel, kind, historyOnJoin)
	c.enqueue(marshal(presenceFrame{Type: "presence", Channel: channel, Roster: h.hub.roster(channel)}))
	// Active initiative round (DH-1/D8) — the panel survives reloads.
	if kind == "room" {
		if r, ok, _ := h.store.ActiveInitiative(context.Background(), channel); ok {
			c.enqueue(marshal(initiativeFrame{Type: "initiative", Channel: channel, Round: r}))
		}
	}
	h.hub.broadcast(channel, marshal(presenceFrame{Type: "presence", Channel: channel, State: "join", Who: &c.who}), c)
}

func (h *Handler) handleMessage(c *Client, f clientFrame, human *store.User, persona *store.Persona) {
	if f.Channel == "" || f.Body == "" {
		return
	}
	kind, ok := h.channelKind(c, f.Channel, human, persona)
	if !ok {
		return
	}
	// Slash commands (DH-1/D3): same surface for humans and personas. A
	// command either transforms the body (e.g. /roll → the rolled result,
	// which persists + broadcasts normally) or consumes the message.
	transformed := false
	if strings.HasPrefix(f.Body, "/") {
		newBody, consumed := h.handleCommand(c, f, kind, human, persona)
		if consumed {
			return
		}
		f.Body = newBody
		transformed = true
	} else if strings.Contains(f.Body, "/roll ") || strings.Contains(f.Body, "/init ") {
		// Inline commands mid-message: "I lunge! /roll 1d20+5" rolls in place.
		if nb, changed := h.expandInline(context.Background(), c, f.Channel, kind, f.Body); changed {
			f.Body = nb
			transformed = true
		}
	}
	ctx := context.Background()
	var (
		msg store.Message
		err error
	)
	switch {
	case human != nil && kind == "dm":
		msg, err = h.store.InsertDMUserMessage(ctx, f.Channel, human.ID, f.Body)
	case human != nil:
		msg, err = h.store.InsertRoomUserMessage(ctx, f.Channel, human.ID, f.Body)
	case persona != nil && kind == "dm":
		msg, err = h.store.InsertDMPersonaMessage(ctx, f.Channel, persona.ID, f.Body)
	case persona != nil:
		msg, err = h.store.InsertRoomPersonaMessage(ctx, f.Channel, persona.ID, nil, f.Body)
	}
	if err != nil {
		log.Printf("gateway persist message: %v", err)
		c.enqueue(marshal(errorFrame{Type: "error", Message: "could not send message"}))
		return
	}
	// Broadcast to everyone in the channel except the sender (the sender's UI
	// shows its own message optimistically — AX3-2 carried forward). Command
	// results are the exception: the sender skipped optimistic rendering (the
	// raw "/roll …" isn't the message) and needs the authoritative result too.
	except := c
	if transformed {
		except = nil
	}
	h.hub.broadcast(f.Channel, marshal(messageFrame{Type: "message", Channel: f.Channel, Message: msg}), except)
	if kind == "room" {
		h.notifyMentions(f.Channel, msg)
	}
}

// notifyMentions resolves @tokens in a room message against the server's
// members, persists a notification per mentioned user, and pushes a live
// notification frame to their connections. Best-effort: failures log only.
func (h *Handler) notifyMentions(roomID string, msg store.Message) {
	ctx := context.Background()
	members, err := h.store.MembersForRoom(ctx, roomID)
	if err != nil {
		log.Printf("gateway mentions: members: %v", err)
		return
	}
	for _, uid := range store.MentionedUserIDs(msg.Body, msg.SenderID, members) {
		id, createdAt, err := h.store.CreateMentionNotification(ctx, uid, msg.ID, roomID)
		if err != nil {
			log.Printf("gateway mentions: %v", err)
			continue
		}
		snippet := msg.Body
		if len(snippet) > 120 {
			snippet = snippet[:120]
		}
		h.hub.sendToUser(uid, marshal(notificationFrame{Type: "notification", Notification: store.Notification{
			ID: id, Kind: "mention", RoomID: roomID, MessageID: msg.ID,
			From: msg.SenderName, Snippet: snippet, CreatedAt: createdAt,
		}}))
	}
}

// handleCommand routes a slash-prefixed body. Returns the (possibly
// transformed) body to persist, or consumed=true when the command produced no
// room message (mood set, or an error sent back to the sender only).
func (h *Handler) handleCommand(c *Client, f clientFrame, kind string, human *store.User, persona *store.Persona) (string, bool) {
	cmd, args, _ := strings.Cut(strings.TrimPrefix(f.Body, "/"), " ")
	args = strings.TrimSpace(args)
	switch strings.ToLower(cmd) {
	case "initiative", "init":
		return h.handleInitiative(c, f.Channel, kind, args, human, persona)
	case "roll":
		out, err := rollCommand(args)
		if err != nil {
			c.enqueue(marshal(errorFrame{Type: "error", Message: err.Error()}))
			return "", true
		}
		return out, false
	case "me":
		if args == "" {
			c.enqueue(marshal(errorFrame{Type: "error", Message: "usage: /me does something"}))
			return "", true
		}
		return "*" + c.who.Name + " " + args + "*", false
	case "mood":
		h.handleMood(c, clientFrame{Mood: args}, human)
		return "", true
	default:
		c.enqueue(marshal(errorFrame{Type: "error", Message: "unknown command /" + cmd + " — try /roll, /init, /me, /mood"}))
		return "", true
	}
}

// handleMood persists a human's roster mood and announces it to every channel
// the connection is subscribed to.
func (h *Handler) handleMood(c *Client, f clientFrame, human *store.User) {
	if human == nil || len(f.Mood) > 32 {
		return
	}
	if err := h.store.SetUserMood(context.Background(), human.ID, f.Mood); err != nil {
		log.Printf("gateway mood: %v", err)
		return
	}
	for _, ch := range h.hub.setMood(c, f.Mood) {
		h.hub.broadcast(ch, marshal(moodFrame{Type: "mood", Channel: ch, Who: c.who}), nil)
	}
}

// handleReaction validates and persists an emoji reaction, then broadcasts it to
// the whole channel (sender included — reactions are idempotent, so no optimistic
//-UI special case). The MessageInChannel guard stops cross-channel UUID guessing.
func (h *Handler) handleReaction(c *Client, f clientFrame, human *store.User, persona *store.Persona) {
	if f.Channel == "" || f.MessageID == "" || f.Emoji == "" || len(f.Emoji) > 32 {
		return
	}
	if f.Op != "add" && f.Op != "remove" {
		return
	}
	if _, ok := h.channelKind(c, f.Channel, human, persona); !ok {
		return
	}
	ctx := context.Background()
	if ok, err := h.store.MessageInChannel(ctx, f.MessageID, f.Channel); err != nil || !ok {
		return
	}
	var userID, personaID *string
	switch {
	case human != nil:
		userID = &human.ID
	case persona != nil:
		personaID = &persona.ID
	default:
		return
	}
	var err error
	if f.Op == "add" {
		err = h.store.AddReaction(ctx, f.MessageID, userID, personaID, f.Emoji)
	} else {
		err = h.store.RemoveReaction(ctx, f.MessageID, userID, personaID, f.Emoji)
	}
	if err != nil {
		log.Printf("gateway reaction: %v", err)
		return
	}
	h.hub.broadcast(f.Channel, marshal(reactionFrame{
		Type: "reaction", Channel: f.Channel, MessageID: f.MessageID,
		Emoji: f.Emoji, Op: f.Op, Who: c.who,
	}), nil)
}

func (h *Handler) sendHistory(c *Client, channel, kind string, limit int) {
	if channel == "" {
		return
	}
	if limit <= 0 {
		limit = historyOnJoin
	}
	var (
		msgs []store.Message
		err  error
	)
	if kind == "dm" {
		msgs, err = h.store.ListDMMessages(context.Background(), channel, limit)
	} else {
		msgs, err = h.store.ListRoomMessages(context.Background(), channel, limit)
	}
	if err != nil {
		log.Printf("gateway history: %v", err)
		return
	}
	c.enqueue(marshal(historyFrame{Type: "history", Channel: channel, Messages: msgs}))
}

// channelKind reports whether the connection may access a channel and whether it
// is a "room" or a "dm" (so the message + history paths route correctly).
func (h *Handler) channelKind(c *Client, channel string, human *store.User, persona *store.Persona) (string, bool) {
	ctx := context.Background()
	switch {
	case human != nil:
		if ok, _ := h.store.UserCanAccessRoom(ctx, channel, human.ID); ok {
			return "room", true
		}
		if ok, _ := h.store.UserCanAccessDM(ctx, channel, human.ID); ok {
			return "dm", true
		}
	case persona != nil:
		if ok, _ := h.store.PersonaCanAccessRoom(ctx, persona.ID, channel); ok {
			return "room", true
		}
		if ok, _ := h.store.PersonaCanAccessDM(ctx, channel, persona.ID); ok {
			return "dm", true
		}
	}
	return "", false
}

// writePump drains the send channel and keeps the connection alive with pings.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()
	for {
		select {
		case payload, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, payload); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func marshal(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		return []byte(`{"type":"error","message":"encode failed"}`)
	}
	return b
}
