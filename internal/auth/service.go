package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/cpuchip/ai-chattermax/internal/store"
)

const cookieName = "chattermax_session"
const cookieMaxAge = 30 * 24 * 60 * 60 // 30 days, seconds

type ctxKey int

const userKey ctxKey = 0

// Service wires the authenticator + store + cookie policy into HTTP handlers.
type Service struct {
	store        *store.Store
	auth         Authenticator
	cookieDomain string
	cookieSecure bool
}

// NewService builds the auth service.
func NewService(st *store.Store, a Authenticator, cookieDomain string, cookieSecure bool) *Service {
	return &Service{store: st, auth: a, cookieDomain: cookieDomain, cookieSecure: cookieSecure}
}

// Mode reports the authenticator mode ("dev" | "ibeco") — surfaced to the client
// so the login UI knows whether to show a name field.
func (s *Service) Mode() string { return s.auth.Mode() }

// UserFrom returns the authenticated user from the request context.
func UserFrom(ctx context.Context) (store.User, bool) {
	u, ok := ctx.Value(userKey).(store.User)
	return u, ok
}

// Required is middleware that rejects unauthenticated requests (401).
func (s *Service) Required(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, ok := s.userFromCookie(r)
		if !ok {
			writeErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userKey, u)))
	})
}

func (s *Service) userFromCookie(r *http.Request) (store.User, bool) {
	c, err := r.Cookie(cookieName)
	if err != nil || c.Value == "" {
		return store.User{}, false
	}
	u, ok, err := s.store.SessionUser(r.Context(), c.Value)
	if err != nil || !ok {
		return store.User{}, false
	}
	return u, true
}

// LoginHandler resolves identity (dev name or ibeco cookie), upserts the user,
// mints a platform session, and sets the session cookie.
func (s *Service) LoginHandler(w http.ResponseWriter, r *http.Request) {
	id, err := s.auth.Authenticate(r)
	if err != nil {
		code := http.StatusUnauthorized
		if !errors.Is(err, ErrUnauthenticated) {
			code = http.StatusBadGateway // ibeco unreachable, etc.
		}
		writeErr(w, code, "login failed")
		return
	}
	user, err := s.store.UpsertUserBySubject(r.Context(), id.Subject, id.DisplayName, id.Email, id.AvatarURL)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not establish user")
		return
	}
	token, err := s.store.CreateSession(r.Context(), user.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not create session")
		return
	}
	s.setSessionCookie(w, token)
	writeJSON(w, http.StatusOK, user)
}

// LogoutHandler revokes the session and clears the cookie.
func (s *Service) LogoutHandler(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(cookieName); err == nil && c.Value != "" {
		_ = s.store.DeleteSession(r.Context(), c.Value)
	}
	s.clearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

// MeHandler returns the authenticated user (behind Required).
func (s *Service) MeHandler(w http.ResponseWriter, r *http.Request) {
	u, ok := UserFrom(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	writeJSON(w, http.StatusOK, u)
}

func (s *Service) setSessionCookie(w http.ResponseWriter, token string) {
	// RFC 6265 §5.3: when a Domain is set, a previously host-only cookie of the
	// same name is a DIFFERENT cookie and would shadow ours — evict it first.
	if s.cookieDomain != "" {
		http.SetCookie(w, &http.Cookie{
			Name: cookieName, Value: "", Path: "/", MaxAge: -1,
			HttpOnly: true, Secure: s.cookieSecure, SameSite: http.SameSiteLaxMode,
		})
	}
	http.SetCookie(w, &http.Cookie{
		Name: cookieName, Value: token, Path: "/", Domain: s.cookieDomain,
		MaxAge: cookieMaxAge, HttpOnly: true, Secure: s.cookieSecure,
		SameSite: http.SameSiteLaxMode, Expires: time.Now().Add(cookieMaxAge * time.Second),
	})
}

func (s *Service) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: cookieName, Value: "", Path: "/", Domain: s.cookieDomain, MaxAge: -1,
		HttpOnly: true, Secure: s.cookieSecure, SameSite: http.SameSiteLaxMode,
	})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}
