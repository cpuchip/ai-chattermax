package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestServeHealthzEmpty(t *testing.T) {
	hub := newHub()

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	serveHealthz(hub, rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", got)
	}

	var body struct {
		Clients int `json:"clients"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Clients != 0 {
		t.Errorf("expected clients=0, got %d", body.Clients)
	}
}

func TestServeHealthzWithClients(t *testing.T) {
	hub := newHub()
	hub.clients[&Client{hub: hub, send: make(chan []byte, 256)}] = true
	hub.clients[&Client{hub: hub, send: make(chan []byte, 256)}] = true
	hub.clients[&Client{hub: hub, send: make(chan []byte, 256)}] = true

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	serveHealthz(hub, rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var body struct {
		Clients int `json:"clients"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Clients != 3 {
		t.Errorf("expected clients=3, got %d", body.Clients)
	}
}

func TestClientCount(t *testing.T) {
	hub := newHub()
	if got := hub.ClientCount(); got != 0 {
		t.Errorf("empty hub: expected 0, got %d", got)
	}

	hub.clients[&Client{hub: hub, send: make(chan []byte, 256)}] = true
	hub.clients[&Client{hub: hub, send: make(chan []byte, 256)}] = true
	if got := hub.ClientCount(); got != 2 {
		t.Errorf("populated hub: expected 2, got %d", got)
	}
}
