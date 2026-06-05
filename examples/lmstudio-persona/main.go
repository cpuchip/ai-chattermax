// Command lmstudio-persona is echo-persona extended to think with a real local
// model. It's the same ~150-line gateway client, but respond() now calls an
// OpenAI-compatible endpoint — LM Studio (http://localhost:1234) by default —
// so the persona's replies come from qwen3.6-27b (or any model you've loaded)
// instead of a canned echo.
//
// This is the "now make it actually think" step after examples/echo-persona.
// Everything about the chat protocol is identical; the only new part is the
// callModel() function and a small per-room history so the model has context.
//
// Run it:
//
//	# 1. Load a model in LM Studio and start its local server (port 1234).
//	# 2. Mint a persona key in the chat UI (Settings → New persona → Grant + mint).
//	export CHATTERMAX_KEY=cmk_xxxxxxxx
//	export CHATTERMAX_GATEWAY=wss://chat.ibeco.me   # default
//	export LMSTUDIO_MODEL="qwen/qwen3.6-27b"        # default; any loaded model id
//	go mod tidy && go run .
//
// Note: qwen3.6 is a *reasoning* model — it spends a few hundred tokens thinking
// before it answers, so LMSTUDIO_MAX_TOKENS defaults to 2000 to leave room. A
// non-reasoning model (e.g. a gemma or mistral instruct) is faster and cheaper.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	roomRefresh   = 30 * time.Second
	historyTurns  = 10 // per-room messages kept as model context
	defaultPrompt = "You are a warm, concise persona in a group chat. Reply in 1-3 short, natural sentences, in character. Don't narrate your reasoning."
)

func main() {
	gateway := strings.TrimRight(env("CHATTERMAX_GATEWAY", "wss://chat.ibeco.me"), "/")
	key := os.Getenv("CHATTERMAX_KEY")
	if key == "" {
		log.Fatal("set CHATTERMAX_KEY to the persona key you minted in the UI (cmk_…)")
	}
	bot := &bot{
		lmURL:     strings.TrimRight(env("LMSTUDIO_URL", "http://localhost:1234"), "/"),
		model:     env("LMSTUDIO_MODEL", "qwen/qwen3.6-27b"),
		maxTokens: envInt("LMSTUDIO_MAX_TOKENS", 2000),
		prompt:    env("PERSONA_PROMPT", defaultPrompt),
		history:   map[string][]chatMsg{},
		http:      &http.Client{Timeout: 3 * time.Minute},
	}
	apiBase := httpFromWS(gateway)

	// 1. Discover the rooms this key grants.
	rooms, persona, err := fetchRooms(apiBase, key)
	if err != nil {
		log.Fatalf("fetch rooms: %v", err)
	}
	bot.me = persona
	log.Printf("persona %q granted %d room(s): %s — backed by %s on %s",
		persona, len(rooms), strings.Join(roomNames(rooms), ", "), bot.model, bot.lmURL)

	// 2. Connect — one socket for all rooms.
	conn, _, err := websocket.DefaultDialer.Dial(gateway+"/gateway?key="+url.QueryEscape(key), nil)
	if err != nil {
		log.Fatalf("dial gateway: %v", err)
	}
	defer conn.Close()

	var writeMu sync.Mutex
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
	go func() {
		for range time.Tick(roomRefresh) {
			if rs, _, err := fetchRooms(apiBase, key); err == nil {
				_ = send(map[string]any{"type": "subscribe", "channels": roomIDs(rs)})
			}
		}
	}()

	// 4. Read loop. One turn at a time (the model call blocks the loop, which is
	// fine for an example — replies never interleave). Humans only; the server
	// never echoes our own posts back to us.
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
		switch f.Type {
		case "history": // backlog on join — remember it, don't reply to it
			for _, m := range f.Messages {
				bot.remember(f.Channel, m.Sender, m.SenderKind, m.Body)
			}
		case "message":
			if f.Message.SenderKind != "human" || strings.TrimSpace(f.Message.Body) == "" {
				bot.remember(f.Channel, f.Message.Sender, f.Message.SenderKind, f.Message.Body)
				continue
			}
			bot.remember(f.Channel, f.Message.Sender, "human", f.Message.Body)
			reply := bot.respond(f.Channel, f.Message.Sender, f.Message.Body)
			if reply == "" {
				continue
			}
			log.Printf("[%s] %s: %q  ->  %q", f.Channel, f.Message.Sender, f.Message.Body, reply)
			bot.remember(f.Channel, bot.me, "persona", reply)
			if err := send(map[string]any{"type": "message", "channel": f.Channel, "body": reply}); err != nil {
				log.Printf("send reply: %v", err)
			}
		}
	}
}

// --- the bot: the only part that differs from echo-persona -------------------

type bot struct {
	me        string
	lmURL     string
	model     string
	maxTokens int
	prompt    string
	history   map[string][]chatMsg // per-channel context (read-loop owned, no lock)
	http      *http.Client
}

type chatMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// remember appends a turn to the channel's bounded history.
func (b *bot) remember(channel, sender, kind, body string) {
	if strings.TrimSpace(body) == "" {
		return
	}
	role := "user"
	if kind == "persona" && sender == b.me {
		role = "assistant"
	}
	// Prefix the speaker so the model can tell who's who in a group chat.
	content := body
	if role == "user" {
		content = sender + ": " + body
	}
	h := append(b.history[channel], chatMsg{Role: role, Content: content})
	if len(h) > historyTurns {
		h = h[len(h)-historyTurns:]
	}
	b.history[channel] = h
}

// respond gates on being addressed (so a reasoning model isn't invoked on every
// line) and then asks the local model. Returns "" to stay silent. THIS is where
// your own agent would go — swap callModel() for whatever you like.
func (b *bot) respond(channel, sender, body string) string {
	if !strings.Contains(strings.ToLower(body), strings.ToLower(b.me)) {
		return "" // only answer when spoken to, by name
	}
	msgs := append([]chatMsg{{Role: "system", Content: b.systemPrompt()}}, b.history[channel]...)
	out, err := b.callModel(msgs)
	if err != nil {
		log.Printf("model error: %v", err)
		return ""
	}
	return out
}

func (b *bot) systemPrompt() string {
	return fmt.Sprintf("You are %q. %s", b.me, b.prompt)
}

// callModel POSTs an OpenAI-compatible chat completion to LM Studio.
func (b *bot) callModel(msgs []chatMsg) (string, error) {
	reqBody, _ := json.Marshal(map[string]any{
		"model":       b.model,
		"messages":    msgs,
		"temperature": 0.6,
		"max_tokens":  b.maxTokens, // reasoning models need headroom (see file header)
		"stream":      false,
	})
	resp, err := b.http.Post(b.lmURL+"/v1/chat/completions", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("LM Studio returned %d (is a model loaded + server started?)", resp.StatusCode)
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}
	if out.Choices[0].FinishReason == "length" {
		log.Printf("warning: hit max_tokens before finishing — raise LMSTUDIO_MAX_TOKENS")
	}
	// Some reasoning models leak a <think>…</think> block into content; strip it.
	return stripThink(out.Choices[0].Message.Content), nil
}

var thinkRe = regexp.MustCompile(`(?s)^\s*<think>.*?</think>\s*`)

func stripThink(s string) string { return strings.TrimSpace(thinkRe.ReplaceAllString(s, "")) }

// --- the wire protocol (identical to echo-persona) --------------------------

type inbound struct {
	Type     string `json:"type"`
	Channel  string `json:"channel"`
	Message  msg    `json:"message"`
	Messages []msg  `json:"messages"`
}

type msg struct {
	Sender     string `json:"sender"`
	SenderKind string `json:"senderKind"` // "human" | "persona"
	Body       string `json:"body"`
}

type room struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

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

func httpFromWS(ws string) string {
	return strings.Replace(strings.Replace(ws, "wss://", "https://", 1), "ws://", "http://", 1)
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

func envInt(k string, def int) int {
	if v := os.Getenv(k); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
