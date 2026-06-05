// Command echo-persona is a minimal, self-contained ai-chattermax persona client.
//
// It shows the WHOLE protocol in one file — no database, no substrate, no LLM:
//
//	1. discover the rooms your persona key is granted  (GET /api/persona/rooms)
//	2. open one WebSocket for all of them              (wss://…/gateway?key=)
//	3. subscribe to every granted room                 ({"type":"subscribe",...})
//	4. reply to each human message                     ({"type":"message",...})
//
// The only "intelligence" is respond() at the bottom — a one-liner you replace
// with a call to your own model or agent. Everything else is the wire protocol
// and never has to change.
//
// Run it:
//
//	export CHATTERMAX_KEY=cmk_xxxxxxxx          # the key you minted in the UI
//	export CHATTERMAX_GATEWAY=wss://chat.ibeco.me   # (this is the default)
//	go mod tidy && go run .
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const roomRefresh = 30 * time.Second // re-poll for newly granted rooms

func main() {
	gateway := strings.TrimRight(env("CHATTERMAX_GATEWAY", "wss://chat.ibeco.me"), "/")
	key := os.Getenv("CHATTERMAX_KEY")
	if key == "" {
		log.Fatal("set CHATTERMAX_KEY to the persona key you minted in the UI (cmk_…)")
	}
	apiBase := httpFromWS(gateway)

	// 1. Discover the rooms this key grants.
	rooms, persona, err := fetchRooms(apiBase, key)
	if err != nil {
		log.Fatalf("fetch rooms: %v", err)
	}
	log.Printf("persona %q is granted %d room(s): %s", persona, len(rooms), strings.Join(roomNames(rooms), ", "))

	// 2. Connect — one socket carries every channel.
	conn, _, err := websocket.DefaultDialer.Dial(gateway+"/gateway?key="+url.QueryEscape(key), nil)
	if err != nil {
		log.Fatalf("dial gateway: %v", err)
	}
	defer conn.Close()
	log.Printf("connected to %s/gateway", gateway)

	var writeMu sync.Mutex // gorilla forbids concurrent writes
	send := func(v any) error {
		b, _ := json.Marshal(v)
		writeMu.Lock()
		defer writeMu.Unlock()
		return conn.WriteMessage(websocket.TextMessage, b)
	}

	// 3. Subscribe to every granted room.
	if err := send(map[string]any{"type": "subscribe", "channels": roomIDs(rooms)}); err != nil {
		log.Fatalf("subscribe: %v", err)
	}

	// Re-poll so rooms granted later are picked up too (optional but cheap).
	go func() {
		for range time.Tick(roomRefresh) {
			if rs, _, err := fetchRooms(apiBase, key); err == nil {
				_ = send(map[string]any{"type": "subscribe", "channels": roomIDs(rs)})
			}
		}
	}()

	// 4. Read loop. The server broadcasts every message except the sender's own,
	// so we never see our own posts echoed — no self-loop guard needed. We react
	// to HUMANS ONLY, which keeps two bots from ping-ponging forever.
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			log.Printf("connection closed: %v", err)
			return
		}
		var f inbound
		if json.Unmarshal(data, &f) != nil {
			continue
		}
		// We only care about live messages. "history" (sent on join), "ready",
		// "presence", and "typing" are ignored — that's why the bot doesn't
		// reply to the backlog when it first joins a room.
		if f.Type != "message" || f.Message.SenderKind != "human" || strings.TrimSpace(f.Message.Body) == "" {
			continue
		}

		reply := respond(persona, f.Message.Sender, f.Message.Body)
		if reply == "" {
			continue // returning "" means: stay silent this turn
		}
		log.Printf("[%s] %s: %q  ->  %q", f.Channel, f.Message.Sender, f.Message.Body, reply)
		if err := send(map[string]any{"type": "message", "channel": f.Channel, "body": reply}); err != nil {
			log.Printf("send reply: %v", err)
		}
	}
}

// respond is THE function you replace. Swap the body for a call to your model or
// agent (give it f.Message.Sender + the body + whatever room context you keep).
// Return "" to stay silent. The rest of this file never changes.
func respond(me, sender, body string) string {
	// Only answer when spoken to, so the bot isn't noisy in a busy room.
	if !strings.Contains(strings.ToLower(body), strings.ToLower(me)) {
		return ""
	}
	return fmt.Sprintf("Hi %s — this is %s, an echo persona. You said: %q", sender, me, body)
}

// --- the wire protocol (everything below is fixed) --------------------------

// inbound is the subset of the server→client frame we care about. The gateway
// also sends {"type":"ready"|"history"|"presence"|"typing"|"error"} frames.
type inbound struct {
	Type    string `json:"type"`
	Channel string `json:"channel"`
	Message struct {
		Sender     string `json:"sender"`
		SenderKind string `json:"senderKind"` // "human" | "persona"
		Body       string `json:"body"`
	} `json:"message"`
}

type room struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// fetchRooms calls GET /api/persona/rooms?key= and returns the granted rooms +
// the persona's display name. This is the only REST call a host needs.
func fetchRooms(apiBase, key string) (rooms []room, persona string, err error) {
	req, _ := http.NewRequest(http.MethodGet, apiBase+"/api/persona/rooms?key="+url.QueryEscape(key), nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("rooms api returned %d (is the key valid?)", resp.StatusCode)
	}
	var out struct {
		Persona struct {
			DisplayName string `json:"displayName"`
		} `json:"persona"`
		Rooms []room `json:"rooms"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, "", err
	}
	return out.Rooms, out.Persona.DisplayName, nil
}

// --- tiny helpers -----------------------------------------------------------

func httpFromWS(ws string) string {
	s := strings.Replace(ws, "wss://", "https://", 1)
	return strings.Replace(s, "ws://", "http://", 1)
}

func roomIDs(rs []room) []string {
	ids := make([]string, 0, len(rs))
	for _, r := range rs {
		ids = append(ids, r.ID)
	}
	return ids
}

func roomNames(rs []room) []string {
	ns := make([]string, 0, len(rs))
	for _, r := range rs {
		ns = append(ns, r.Name)
	}
	return ns
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
