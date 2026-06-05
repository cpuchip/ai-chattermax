// Package auth resolves a request to a user. Login establishes identity once
// (dev name-login locally, or the ibeco.me handshake in prod), then the platform
// runs its own session cookie for subsequent requests.
package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Identity is the external identity resolved at login.
type Identity struct {
	Subject     string // stable: "becoming:<id>" or "dev:<name>"
	DisplayName string
	Email       string
	AvatarURL   string
}

// ErrUnauthenticated means no valid identity could be resolved from the request.
var ErrUnauthenticated = errors.New("unauthenticated")

// Authenticator resolves the external identity for a login request.
type Authenticator interface {
	Authenticate(r *http.Request) (Identity, error)
	Mode() string
}

// NewAuthenticator returns the dev or ibeco authenticator for the given mode.
func NewAuthenticator(mode, ibecoBaseURL string) Authenticator {
	if mode == "ibeco" {
		return ibecoAuth{baseURL: strings.TrimRight(ibecoBaseURL, "/"), client: &http.Client{Timeout: 8 * time.Second}}
	}
	return devAuth{}
}

// --- dev: name login (local only) -------------------------------------------

type devAuth struct{}

func (devAuth) Mode() string { return "dev" }

func (devAuth) Authenticate(r *http.Request) (Identity, error) {
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	if name == "" {
		var body struct {
			Name string `json:"name"`
		}
		if r.Body != nil {
			_ = json.NewDecoder(io.LimitReader(r.Body, 4<<10)).Decode(&body)
		}
		name = strings.TrimSpace(body.Name)
	}
	if name == "" {
		return Identity{}, fmt.Errorf("%w: a display name is required in dev login", ErrUnauthenticated)
	}
	return Identity{Subject: "dev:" + strings.ToLower(name), DisplayName: name}, nil
}

// --- ibeco: borrow the becoming session -------------------------------------

type ibecoAuth struct {
	baseURL string
	client  *http.Client
}

func (ibecoAuth) Mode() string { return "ibeco" }

// Authenticate reads the becoming_session cookie and resolves identity via the
// becoming server's GET /api/me (server-to-server, forwarding the cookie).
func (a ibecoAuth) Authenticate(r *http.Request) (Identity, error) {
	cookie, err := r.Cookie("becoming_session")
	if err != nil || cookie.Value == "" {
		return Identity{}, fmt.Errorf("%w: no becoming_session cookie", ErrUnauthenticated)
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, a.baseURL+"/api/me", nil)
	if err != nil {
		return Identity{}, err
	}
	req.AddCookie(&http.Cookie{Name: "becoming_session", Value: cookie.Value})
	resp, err := a.client.Do(req)
	if err != nil {
		return Identity{}, fmt.Errorf("ibeco /api/me: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Identity{}, fmt.Errorf("%w: ibeco /api/me returned %d", ErrUnauthenticated, resp.StatusCode)
	}
	var me struct {
		ID     int64  `json:"id"`
		Email  string `json:"email"`
		Name   string `json:"name"`
		Avatar string `json:"avatar_url"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&me); err != nil {
		return Identity{}, fmt.Errorf("decode ibeco /api/me: %w", err)
	}
	name := me.Name
	if name == "" && me.Email != "" {
		name = strings.Split(me.Email, "@")[0]
	}
	if name == "" {
		name = "user"
	}
	return Identity{
		Subject:     fmt.Sprintf("becoming:%d", me.ID),
		DisplayName: name,
		Email:       me.Email,
		AvatarURL:   me.Avatar,
	}, nil
}
