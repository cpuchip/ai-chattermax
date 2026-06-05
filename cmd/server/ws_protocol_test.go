package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cpuchip/ai-chattermax/presence"
	"github.com/cpuchip/ai-chattermax/room"
	"github.com/cpuchip/ai-chattermax/scheduler"
	"github.com/cpuchip/ai-chattermax/transcript"
	"github.com/gorilla/websocket"
)

// TestWS_AttributionNoEchoAndReplay drives the real WS handler with two live
// clients to prove the AX3-2 fix end-to-end: messages arrive attributed
// ({sender,body}), the sender does NOT get an echo of its own message, and a
// late joiner receives the history on connect.
func TestWS_AttributionNoEchoAndReplay(t *testing.T) {
	hub := room.NewHub()
	sched := scheduler.New(time.Now, time.Hour, 100)
	store := transcript.NewMemoryStore()
	tracker := presence.NewTracker()
	srv := httptest.NewServer(newMux(hub, sched, store, tracker))
	defer srv.Close()

	wsBase := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws/r1?id="
	dial := func(id string) *websocket.Conn {
		c, _, err := websocket.DefaultDialer.Dial(wsBase+id, nil)
		if err != nil {
			t.Fatalf("dial %s: %v", id, err)
		}
		return c
	}

	// waitRoster blocks until the room has at least n participants — the handler
	// joins the tracker right after registering with the hub, so this guarantees
	// a client is registered before we rely on it receiving a broadcast.
	waitRoster := func(n int) {
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			resp, err := http.Get(srv.URL + "/roster/r1")
			if err == nil {
				var r []json.RawMessage
				_ = json.NewDecoder(resp.Body).Decode(&r)
				resp.Body.Close()
				if len(r) >= n {
					return
				}
			}
			time.Sleep(20 * time.Millisecond)
		}
		t.Fatalf("roster never reached %d participants", n)
	}

	alice := dial("alice")
	defer alice.Close()
	bob := dial("bob")
	defer bob.Close()
	waitRoster(2)

	if err := alice.WriteMessage(websocket.TextMessage, []byte("hello room")); err != nil {
		t.Fatalf("alice write: %v", err)
	}

	// bob receives an ATTRIBUTED envelope.
	bob.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, raw, err := bob.ReadMessage()
	if err != nil {
		t.Fatalf("bob read: %v", err)
	}
	var wm wireMessage
	if err := json.Unmarshal(raw, &wm); err != nil {
		t.Fatalf("bob got non-JSON %q: %v", raw, err)
	}
	if wm.Sender != "alice" || wm.Body != "hello room" {
		t.Fatalf("bob envelope = %+v, want sender=alice body=\"hello room\"", wm)
	}

	// alice must NOT receive an echo of her own message.
	alice.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	if _, _, err := alice.ReadMessage(); err == nil {
		t.Fatal("alice received an echo of her own message (self-echo not suppressed)")
	}

	// A late joiner gets the history replayed. bob's receipt above proves the
	// message is already stored, so there's no race here.
	carol := dial("carol")
	defer carol.Close()
	carol.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, raw2, err := carol.ReadMessage()
	if err != nil {
		t.Fatalf("carol replay read: %v", err)
	}
	var wm2 wireMessage
	if err := json.Unmarshal(raw2, &wm2); err != nil {
		t.Fatalf("carol got non-JSON %q: %v", raw2, err)
	}
	if wm2.Sender != "alice" || wm2.Body != "hello room" {
		t.Fatalf("carol replay = %+v, want the stored message", wm2)
	}
}
