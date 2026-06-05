package auth

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDevAuth(t *testing.T) {
	a := NewAuthenticator("dev", "")
	// JSON body name
	r := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(`{"name":"Bob"}`))
	id, err := a.Authenticate(r)
	if err != nil || id.Subject != "dev:bob" || id.DisplayName != "Bob" {
		t.Fatalf("dev json login = %+v, err=%v", id, err)
	}
	// query name
	r2 := httptest.NewRequest("POST", "/api/auth/login?name=Alice", nil)
	id2, err := a.Authenticate(r2)
	if err != nil || id2.DisplayName != "Alice" {
		t.Fatalf("dev query login = %+v, err=%v", id2, err)
	}
	// empty → error
	r3 := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(`{}`))
	if _, err := a.Authenticate(r3); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("empty dev login should be unauthenticated, got %v", err)
	}
}

func TestIbecoAuth(t *testing.T) {
	becoming := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/me" {
			http.NotFound(w, r)
			return
		}
		if c, err := r.Cookie("becoming_session"); err != nil || c.Value != "good-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"id":42,"name":"Mara","email":"mara@example.com"}`))
	}))
	defer becoming.Close()

	a := NewAuthenticator("ibeco", becoming.URL)

	// valid cookie → identity
	r := httptest.NewRequest("POST", "/api/auth/login", nil)
	r.AddCookie(&http.Cookie{Name: "becoming_session", Value: "good-token"})
	id, err := a.Authenticate(r)
	if err != nil || id.Subject != "becoming:42" || id.DisplayName != "Mara" || id.Email != "mara@example.com" {
		t.Fatalf("ibeco login = %+v, err=%v", id, err)
	}

	// no cookie → unauthenticated
	if _, err := a.Authenticate(httptest.NewRequest("POST", "/api/auth/login", nil)); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("no-cookie should be unauthenticated, got %v", err)
	}

	// bad cookie → becoming 401 → unauthenticated
	rBad := httptest.NewRequest("POST", "/api/auth/login", nil)
	rBad.AddCookie(&http.Cookie{Name: "becoming_session", Value: "nope"})
	if _, err := a.Authenticate(rBad); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("bad cookie should be unauthenticated, got %v", err)
	}
}
