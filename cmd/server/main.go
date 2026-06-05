package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/cpuchip/ai-chattermax"
	"github.com/cpuchip/ai-chattermax/presence"
	"github.com/cpuchip/ai-chattermax/room"
	"github.com/cpuchip/ai-chattermax/scheduler"
	"github.com/cpuchip/ai-chattermax/transcript"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var clientIDCounter atomic.Int64

func generateClientID() string {
	return fmt.Sprintf("client-%d", clientIDCounter.Add(1))
}

// kindFromQuery maps the optional ?kind= query param to a presence kind. A
// persona-host connects with kind=persona (or agent) so the roster shows it as
// an agent rather than a human; anything else defaults to human.
func kindFromQuery(raw string) presence.Kind {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "persona", "agent":
		return presence.Persona
	default:
		return presence.Human
	}
}

// withSPA wraps an API handler with static-file serving and SPA history fallback.
// API routes (/healthz, /roster/, /ws/) are passed through untouched.
func withSPA(api http.Handler, staticFS fs.FS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// Pass through API and WebSocket routes
		if path == "/healthz" || strings.HasPrefix(path, "/roster/") || strings.HasPrefix(path, "/ws/") {
			api.ServeHTTP(w, r)
			return
		}

		// Try to serve the exact file
		cleanPath := strings.TrimPrefix(path, "/")
		if cleanPath == "" {
			cleanPath = "index.html"
		}
		f, err := staticFS.Open(cleanPath)
		if err == nil {
			defer f.Close()
			stat, err := f.Stat()
			if err == nil && !stat.IsDir() {
				http.ServeContent(w, r, cleanPath, stat.ModTime(), f.(io.ReadSeeker))
				return
			}
		}

		// Fallback to index.html for SPA routes
		f, err = staticFS.Open("index.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		stat, err := f.Stat()
		if err != nil {
			http.NotFound(w, r)
			return
		}
		http.ServeContent(w, r, "index.html", stat.ModTime(), f.(io.ReadSeeker))
	})
}

// wsClient wraps a websocket connection to implement room.Client.
type wsClient struct {
	id   string
	conn *websocket.Conn
	mu   sync.Mutex
}

func (c *wsClient) ID() string {
	return c.id
}

func (c *wsClient) Send(msg []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn.WriteMessage(websocket.TextMessage, msg)
}

// wireMessage is the JSON envelope broadcast to clients so every message carries
// its sender. The frontend (useChat) and persona-host both parse {sender, body};
// ts is included for ordering.
type wireMessage struct {
	Sender string `json:"sender"`
	Body   string `json:"body"`
	TS     string `json:"ts"`
}

// wireBytes encodes a transcript message as the on-wire JSON envelope. On the
// (practically impossible) marshal error it falls back to the raw body so a
// message is never silently dropped.
func wireBytes(m transcript.Message) []byte {
	b, err := json.Marshal(wireMessage{
		Sender: m.Sender,
		Body:   m.Body,
		TS:     m.Timestamp.UTC().Format(time.RFC3339),
	})
	if err != nil {
		return []byte(m.Body)
	}
	return b
}

func newMux(hub *room.Hub, sched *scheduler.Scheduler, store transcript.Store, tracker *presence.Tracker) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	mux.HandleFunc("GET /roster/{room}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		roster := tracker.Roster()
		if err := json.NewEncoder(w).Encode(roster); err != nil {
			log.Printf("roster encode error: %v", err)
		}
	})

	mux.HandleFunc("GET /ws/{room}", func(w http.ResponseWriter, r *http.Request) {
		roomID := r.PathValue("room")
		clientID := r.URL.Query().Get("id")
		if clientID == "" {
			clientID = generateClientID()
		}
		kind := kindFromQuery(r.URL.Query().Get("kind"))

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("websocket upgrade error: %v", err)
			return
		}
		defer conn.Close()

		client := &wsClient{id: clientID, conn: conn}

		hub.Register(roomID, client)
		tracker.Join(clientID, kind)
		sched.AddParticipant(clientID)
		defer func() {
			hub.Unregister(roomID, client)
			tracker.Leave(clientID)
			sched.RemoveParticipant(clientID)
		}()

		// Replay room history to the joiner so it has context: humans see the
		// backlog, and a persona reads recent turns before it speaks.
		if history, err := store.Replay(roomID); err != nil {
			log.Printf("replay error: %v", err)
		} else {
			for _, m := range history {
				_ = client.Send(wireBytes(m))
			}
		}

		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
					log.Printf("websocket read error: %v", err)
				}
				break
			}

			if !sched.Allow(clientID) {
				continue // silently drop over-ceiling message
			}

			tmsg := transcript.Message{
				RoomID:    roomID,
				Sender:    clientID,
				Body:      string(msg),
				Timestamp: time.Now(),
			}
			if err := store.Append(tmsg); err != nil {
				log.Printf("store append error: %v", err)
				continue
			}

			// Broadcast the attributed envelope to everyone EXCEPT the sender —
			// the sender's client already shows its own message optimistically,
			// so echoing it back would duplicate it.
			hub.BroadcastExcept(roomID, clientID, wireBytes(tmsg))
		}
	})

	return mux
}

func main() {
	hub := room.NewHub()
	sched := scheduler.New(time.Now, 1*time.Minute, 10)
	store := transcript.NewMemoryStore()
	tracker := presence.NewTracker()

	mux := newMux(hub, sched, store, tracker)

	staticFS, err := static.FS()
	if err != nil {
		log.Fatalf("failed to create static fs: %v", err)
	}
	handler := withSPA(mux, staticFS)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: handler,
	}

	// Graceful shutdown
	idleConnsClosed := make(chan struct{})
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		<-sigCh

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("server shutdown error: %v", err)
		}
		close(idleConnsClosed)
	}()

	log.Printf("Server starting on :%s", port)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("ListenAndServe: %v", err)
	}
	<-idleConnsClosed
	log.Println("Server stopped")
}
