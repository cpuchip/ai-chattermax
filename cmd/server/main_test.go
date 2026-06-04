package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/cpuchip/ai-chattermax/presence"
	"github.com/cpuchip/ai-chattermax/room"
	"github.com/cpuchip/ai-chattermax/scheduler"
	"github.com/cpuchip/ai-chattermax/transcript"
)

func TestHealthzHandler(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	ct := rr.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type %q, got %q", "application/json", ct)
	}

	var body map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to unmarshal response body: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("expected body status \"ok\", got %q", body["status"])
	}
}

// fakeClient is an in-memory room.Client for testing message handling.
type fakeClient struct {
	id   string
	msgs chan []byte
}

func newFakeClient(id string, bufSize int) *fakeClient {
	return &fakeClient{
		id:   id,
		msgs: make(chan []byte, bufSize),
	}
}

func (c *fakeClient) ID() string {
	return c.id
}

func (c *fakeClient) Send(msg []byte) error {
	c.msgs <- append([]byte(nil), msg...)
	return nil
}

func TestRosterEndpoint(t *testing.T) {
	hub := room.NewHub()
	sched := scheduler.New(time.Now, time.Minute, 10)
	store := transcript.NewMemoryStore()
	tracker := presence.NewTracker()

	mux := newMux(hub, sched, store, tracker)

	tracker.Join("alice", presence.Human)
	tracker.Join("bob", presence.Persona)

	req := httptest.NewRequest(http.MethodGet, "/roster/any-room", nil)
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	ct := rr.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type %q, got %q", "application/json", ct)
	}

	var roster []presence.Participant
	if err := json.Unmarshal(rr.Body.Bytes(), &roster); err != nil {
		t.Fatalf("failed to unmarshal roster: %v", err)
	}

	want := []presence.Participant{
		{ID: "alice", Kind: presence.Human, Online: true, Idle: false, Thinking: false},
		{ID: "bob", Kind: presence.Persona, Online: true, Idle: false, Thinking: false},
	}
	if !reflect.DeepEqual(roster, want) {
		t.Errorf("roster = %v, want %v", roster, want)
	}
}

func TestInboundMessageHandling(t *testing.T) {
	tests := []struct {
		name       string
		sendCount  int
		maxActions int
		wantStored int
		wantSent   int
	}{
		{
			name:       "allowed message is stored and broadcast",
			sendCount:  1,
			maxActions: 2,
			wantStored: 1,
			wantSent:   1,
		},
		{
			name:       "over ceiling message is dropped",
			sendCount:  3,
			maxActions: 2,
			wantStored: 2,
			wantSent:   2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			roomID := "test-room"
			clientID := "client-1"

			hub := room.NewHub()
			sched := scheduler.New(time.Now, time.Hour, tt.maxActions)
			store := transcript.NewMemoryStore()
			tracker := presence.NewTracker()

			client := newFakeClient(clientID, 10)
			hub.Register(roomID, client)
			tracker.Join(clientID, presence.Human)
			sched.AddParticipant(clientID)

			payload := []byte("hello world")

			for i := 0; i < tt.sendCount; i++ {
				if sched.Allow(clientID) {
					msg := transcript.Message{
						RoomID:    roomID,
						Sender:    clientID,
						Body:      string(payload),
						Timestamp: time.Now(),
					}
					if err := store.Append(msg); err != nil {
						t.Fatalf("store.Append error: %v", err)
					}
					hub.Broadcast(roomID, payload)
				}
				// else: silently drop
			}

			msgs, err := store.Replay(roomID)
			if err != nil {
				t.Fatalf("store.Replay error: %v", err)
			}
			if len(msgs) != tt.wantStored {
				t.Errorf("stored messages = %d, want %d", len(msgs), tt.wantStored)
			}

			received := 0
			done := false
			for !done {
				select {
				case <-client.msgs:
					received++
				default:
					done = true
				}
			}
			if received != tt.wantSent {
				t.Errorf("broadcast messages received = %d, want %d", received, tt.wantSent)
			}
		})
	}
}
