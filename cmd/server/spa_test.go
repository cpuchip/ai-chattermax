package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestWithSPA(t *testing.T) {
	api := http.NewServeMux()
	api.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	api.HandleFunc("GET /roster/{room}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("roster"))
	})
	api.HandleFunc("GET /ws/{room}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ws"))
	})

	staticFS := fstest.MapFS{
		"index.html": {
			Data: []byte("<html><body>spa</body></html>"),
		},
		"vite.svg": {
			Data: []byte("svg"),
		},
	}

	handler := withSPA(api, staticFS)

	t.Run("GET / returns index.html", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}
		if !strings.Contains(rr.Body.String(), "spa") {
			t.Errorf("expected body to contain 'spa', got %q", rr.Body.String())
		}
		ct := rr.Header().Get("Content-Type")
		if ct != "text/html; charset=utf-8" {
			t.Errorf("expected Content-Type text/html, got %q", ct)
		}
	})

	t.Run("GET /room/lobby falls back to index.html", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/room/lobby", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}
		if !strings.Contains(rr.Body.String(), "spa") {
			t.Errorf("expected body to contain 'spa', got %q", rr.Body.String())
		}
	})

	t.Run("GET /healthz returns JSON", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}
		ct := rr.Header().Get("Content-Type")
		if ct != "application/json" {
			t.Errorf("expected Content-Type application/json, got %q", ct)
		}
		var body map[string]string
		if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
			t.Fatalf("failed to unmarshal body: %v", err)
		}
		if body["status"] != "ok" {
			t.Errorf("expected status ok, got %q", body["status"])
		}
	})

	t.Run("GET /roster/any-room passes through", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/roster/any-room", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}
		if rr.Body.String() != "roster" {
			t.Errorf("expected body 'roster', got %q", rr.Body.String())
		}
	})

	t.Run("GET /ws/lobby passes through", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/ws/lobby", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}
		if rr.Body.String() != "ws" {
			t.Errorf("expected body 'ws', got %q", rr.Body.String())
		}
	})

	t.Run("GET /vite.svg serves static file", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/vite.svg", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}
		if rr.Body.String() != "svg" {
			t.Errorf("expected body 'svg', got %q", rr.Body.String())
		}
	})
}
