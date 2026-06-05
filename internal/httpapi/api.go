// Package httpapi is the REST surface over the platform store. Every handler
// runs behind the auth middleware (the user is in the request context).
package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/cpuchip/ai-chattermax/internal/auth"
	"github.com/cpuchip/ai-chattermax/internal/store"
)

// API holds the dependencies for the REST handlers.
type API struct {
	store    *store.Store
	authMode string
}

// New builds the API.
func New(st *store.Store, authMode string) *API {
	return &API{store: st, authMode: authMode}
}

// Register attaches the (authenticated) REST routes to a mux. The caller wraps
// these with the auth.Required middleware; /api/config is registered separately
// (public) by main.
func (a *API) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/servers", a.listServers)
	mux.HandleFunc("POST /api/servers", a.createServer)
	mux.HandleFunc("POST /api/servers/join", a.joinServer)
	mux.HandleFunc("GET /api/servers/{id}/rooms", a.listRooms)
	mux.HandleFunc("POST /api/servers/{id}/rooms", a.createRoom)
	mux.HandleFunc("GET /api/servers/{id}/registry", a.registry)
	mux.HandleFunc("GET /api/servers/{id}/personas", a.listPersonas)
	mux.HandleFunc("POST /api/servers/{id}/personas", a.createPersona)
	mux.HandleFunc("POST /api/personas/{id}/keys", a.mintKey)
	mux.HandleFunc("POST /api/personas/{id}/grants", a.grantPersona)
	mux.HandleFunc("GET /api/rooms/{id}/messages", a.roomMessages)
	mux.HandleFunc("GET /api/rooms/{id}/search", a.roomSearch)
}

// ConfigHandler is public — tells the client which auth mode is in effect.
func (a *API) ConfigHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"authMode": a.authMode})
}

// --- servers ----------------------------------------------------------------

func (a *API) listServers(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFrom(r.Context())
	servers, err := a.store.ListServersForUser(r.Context(), u.ID)
	if err != nil {
		writeErr(w, 500, "could not list servers")
		return
	}
	writeJSON(w, 200, orEmpty(servers))
}

func (a *API) createServer(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFrom(r.Context())
	var in struct{ Name, Slug string }
	if !decode(w, r, &in) {
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		writeErr(w, 400, "name is required")
		return
	}
	slug := slugify(firstNonEmpty(in.Slug, in.Name))
	sv, err := a.store.CreateServer(r.Context(), slug, in.Name, u.ID)
	if err != nil {
		writeErr(w, 400, "could not create server (slug may be taken)")
		return
	}
	writeJSON(w, 201, sv)
}

func (a *API) joinServer(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFrom(r.Context())
	var in struct{ Token string }
	if !decode(w, r, &in) {
		return
	}
	sv, err := a.store.ServerByJoinToken(r.Context(), strings.TrimSpace(in.Token))
	if err != nil {
		writeErr(w, 404, "invalid join link")
		return
	}
	if err := a.store.AddServerMember(r.Context(), sv.ID, u.ID, "member"); err != nil {
		writeErr(w, 500, "could not join")
		return
	}
	sv.JoinToken = "" // don't leak the token to a plain member
	writeJSON(w, 200, sv)
}

// --- rooms ------------------------------------------------------------------

func (a *API) listRooms(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFrom(r.Context())
	serverID := r.PathValue("id")
	if _, ok := a.member(w, r, serverID, u.ID); !ok {
		return
	}
	rooms, err := a.store.ListRoomsForUser(r.Context(), serverID, u.ID)
	if err != nil {
		writeErr(w, 500, "could not list rooms")
		return
	}
	writeJSON(w, 200, orEmpty(rooms))
}

func (a *API) createRoom(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFrom(r.Context())
	serverID := r.PathValue("id")
	role, ok := a.member(w, r, serverID, u.ID)
	if !ok {
		return
	}
	if role != "owner" && role != "admin" {
		writeErr(w, 403, "only owners/admins create rooms")
		return
	}
	var in struct{ Name, Slug, Visibility, Topic string }
	if !decode(w, r, &in) {
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		writeErr(w, 400, "name is required")
		return
	}
	vis := in.Visibility
	if vis != "private" {
		vis = "public"
	}
	room, err := a.store.CreateRoom(r.Context(), serverID, slugify(firstNonEmpty(in.Slug, in.Name)), in.Name, vis, in.Topic, u.ID)
	if err != nil {
		writeErr(w, 400, "could not create room (slug may be taken)")
		return
	}
	if vis == "private" {
		_ = a.store.AddRoomMember(r.Context(), room.ID, u.ID, "moderator")
	}
	writeJSON(w, 201, room)
}

func (a *API) roomMessages(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFrom(r.Context())
	roomID := r.PathValue("id")
	if ok, _ := a.store.UserCanAccessRoom(r.Context(), roomID, u.ID); !ok {
		writeErr(w, 403, "no access to this room")
		return
	}
	msgs, err := a.store.ListRoomMessages(r.Context(), roomID, 100)
	if err != nil {
		writeErr(w, 500, "could not load messages")
		return
	}
	writeJSON(w, 200, orEmptyMsgs(msgs))
}

func (a *API) roomSearch(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFrom(r.Context())
	roomID := r.PathValue("id")
	if ok, _ := a.store.UserCanAccessRoom(r.Context(), roomID, u.ID); !ok {
		writeErr(w, 403, "no access to this room")
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeJSON(w, 200, []store.Message{})
		return
	}
	msgs, err := a.store.SearchRoomMessages(r.Context(), roomID, q, 50)
	if err != nil {
		writeErr(w, 500, "search failed")
		return
	}
	writeJSON(w, 200, orEmptyMsgs(msgs))
}

// --- personas ---------------------------------------------------------------

func (a *API) listPersonas(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFrom(r.Context())
	serverID := r.PathValue("id")
	if _, ok := a.member(w, r, serverID, u.ID); !ok {
		return
	}
	personas, err := a.store.ListPersonasForServer(r.Context(), serverID)
	if err != nil {
		writeErr(w, 500, "could not list personas")
		return
	}
	writeJSON(w, 200, orEmpty(personas))
}

func (a *API) createPersona(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFrom(r.Context())
	serverID := r.PathValue("id")
	if _, ok := a.member(w, r, serverID, u.ID); !ok {
		return
	}
	var in struct{ Slug, DisplayName, HostRef, AvatarURL string }
	if !decode(w, r, &in) {
		return
	}
	in.DisplayName = strings.TrimSpace(in.DisplayName)
	if in.DisplayName == "" {
		writeErr(w, 400, "displayName is required")
		return
	}
	p, err := a.store.CreatePersona(r.Context(), serverID, u.ID, slugify(firstNonEmpty(in.Slug, in.DisplayName)), in.DisplayName, in.AvatarURL, in.HostRef)
	if err != nil {
		writeErr(w, 400, "could not create persona (slug may be taken)")
		return
	}
	writeJSON(w, 201, p)
}

func (a *API) mintKey(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFrom(r.Context())
	personaID := r.PathValue("id")
	p, err := a.store.GetPersona(r.Context(), personaID)
	if err != nil {
		writeErr(w, 404, "persona not found")
		return
	}
	if p.OwnerUserID != u.ID {
		writeErr(w, 403, "only the persona's owner can mint a key")
		return
	}
	var in struct{ Label string }
	_ = decodeOptional(r, &in)
	raw, err := a.store.MintPersonaKey(r.Context(), personaID, in.Label)
	if err != nil {
		writeErr(w, 500, "could not mint key")
		return
	}
	// Shown ONCE — the raw key is never retrievable again.
	writeJSON(w, 201, map[string]string{"key": raw, "personaId": personaID})
}

func (a *API) grantPersona(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFrom(r.Context())
	personaID := r.PathValue("id")
	p, err := a.store.GetPersona(r.Context(), personaID)
	if err != nil {
		writeErr(w, 404, "persona not found")
		return
	}
	role, ok := a.member(w, r, p.ServerID, u.ID)
	if !ok {
		return
	}
	if p.OwnerUserID != u.ID && role != "owner" && role != "admin" {
		writeErr(w, 403, "not allowed to grant this persona")
		return
	}
	var in struct{ RoomID string }
	if !decode(w, r, &in) {
		return
	}
	room, err := a.store.GetRoom(r.Context(), in.RoomID)
	if err != nil || room.ServerID != p.ServerID {
		writeErr(w, 400, "room not in this server")
		return
	}
	if err := a.store.GrantPersonaRoom(r.Context(), personaID, in.RoomID, u.ID); err != nil {
		writeErr(w, 500, "could not grant")
		return
	}
	writeJSON(w, 200, map[string]string{"personaId": personaID, "roomId": in.RoomID})
}

// --- registry ---------------------------------------------------------------

type registryMember struct {
	store.Member
	Personas []store.Persona `json:"personas"`
}

func (a *API) registry(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFrom(r.Context())
	serverID := r.PathValue("id")
	if _, ok := a.member(w, r, serverID, u.ID); !ok {
		return
	}
	members, err := a.store.ListServerMembers(r.Context(), serverID)
	if err != nil {
		writeErr(w, 500, "could not load registry")
		return
	}
	personas, err := a.store.ListPersonasForServer(r.Context(), serverID)
	if err != nil {
		writeErr(w, 500, "could not load registry")
		return
	}
	byOwner := map[string][]store.Persona{}
	for _, p := range personas {
		byOwner[p.OwnerUserID] = append(byOwner[p.OwnerUserID], p)
	}
	out := make([]registryMember, 0, len(members))
	for _, m := range members {
		out = append(out, registryMember{Member: m, Personas: orEmpty(byOwner[m.UserID])})
	}
	writeJSON(w, 200, out)
}

// --- helpers ----------------------------------------------------------------

// member returns the user's role in a server, writing 403 + false if not a member.
func (a *API) member(w http.ResponseWriter, r *http.Request, serverID, userID string) (string, bool) {
	role, ok, err := a.store.ServerRole(r.Context(), serverID, userID)
	if err != nil || !ok {
		writeErr(w, 403, "not a member of this server")
		return "", false
	}
	return role, true
}

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = slugRe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		s = "untitled"
	}
	if len(s) > 48 {
		s = s[:48]
	}
	return s
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(v); err != nil {
		writeErr(w, 400, "invalid request body")
		return false
	}
	return true
}

func decodeOptional(r *http.Request, v any) error {
	if r.Body == nil {
		return nil
	}
	return json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(v)
}

// orEmpty returns a non-nil slice so JSON encodes [] rather than null.
func orEmpty[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

func orEmptyMsgs(s []store.Message) []store.Message { return orEmpty(s) }

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}
