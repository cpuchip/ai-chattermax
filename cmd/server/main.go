package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

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

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("websocket upgrade error: %v", err)
			return
		}
		defer conn.Close()

		client := &wsClient{id: clientID, conn: conn}

		hub.Register(roomID, client)
		tracker.Join(clientID, presence.Human)
		sched.AddParticipant(clientID)
		defer func() {
			hub.Unregister(roomID, client)
			tracker.Leave(clientID)
			sched.RemoveParticipant(clientID)
		}()

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

			hub.Broadcast(roomID, msg)
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

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
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
