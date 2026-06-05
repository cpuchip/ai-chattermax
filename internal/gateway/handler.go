package gateway

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
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
		who = Participant{ID: u.ID, Name: u.DisplayName, Kind: "human", Avatar: u.AvatarURL}
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
			h.sendHistory(c, f.Channel, f.Limit)
		case "typing":
			if f.Channel != "" {
				h.hub.broadcast(f.Channel, marshal(typingFrame{Type: "typing", Channel: f.Channel, Who: c.who.Name}), c)
			}
		}
	}
}

func (h *Handler) handleSubscribe(c *Client, channel string, human *store.User, persona *store.Persona) {
	if channel == "" || !h.canAccess(c, channel, human, persona) {
		return
	}
	if !h.hub.subscribe(c, channel) {
		return // already subscribed
	}
	// History + roster snapshot to the new subscriber, then announce to others.
	h.sendHistory(c, channel, historyOnJoin)
	c.enqueue(marshal(presenceFrame{Type: "presence", Channel: channel, Roster: h.hub.roster(channel)}))
	h.hub.broadcast(channel, marshal(presenceFrame{Type: "presence", Channel: channel, State: "join", Who: &c.who}), c)
}

func (h *Handler) handleMessage(c *Client, f clientFrame, human *store.User, persona *store.Persona) {
	if f.Channel == "" || f.Body == "" || !h.canAccess(c, f.Channel, human, persona) {
		return
	}
	ctx := context.Background()
	var (
		msg store.Message
		err error
	)
	switch {
	case human != nil:
		msg, err = h.store.InsertRoomUserMessage(ctx, f.Channel, human.ID, f.Body)
	case persona != nil:
		msg, err = h.store.InsertRoomPersonaMessage(ctx, f.Channel, persona.ID, nil, f.Body)
	}
	if err != nil {
		log.Printf("gateway persist message: %v", err)
		c.enqueue(marshal(errorFrame{Type: "error", Message: "could not send message"}))
		return
	}
	// Broadcast to everyone in the channel except the sender (the sender's UI
	// shows its own message optimistically — AX3-2 carried forward).
	h.hub.broadcast(f.Channel, marshal(messageFrame{Type: "message", Channel: f.Channel, Message: msg}), c)
}

func (h *Handler) sendHistory(c *Client, channel string, limit int) {
	if channel == "" {
		return
	}
	if limit <= 0 {
		limit = historyOnJoin
	}
	msgs, err := h.store.ListRoomMessages(context.Background(), channel, limit)
	if err != nil {
		log.Printf("gateway history: %v", err)
		return
	}
	c.enqueue(marshal(historyFrame{Type: "history", Channel: channel, Messages: msgs}))
}

// canAccess checks whether the connection may read/post in a channel.
func (h *Handler) canAccess(c *Client, channel string, human *store.User, persona *store.Persona) bool {
	ctx := context.Background()
	switch {
	case human != nil:
		ok, err := h.store.UserCanAccessRoom(ctx, channel, human.ID)
		return err == nil && ok
	case persona != nil:
		ok, err := h.store.PersonaCanAccessRoom(ctx, persona.ID, channel)
		return err == nil && ok
	}
	return false
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
