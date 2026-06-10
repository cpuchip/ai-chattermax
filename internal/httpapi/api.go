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
	mux.HandleFunc("GET /api/servers/{id}", a.getServer)
	mux.HandleFunc("GET /api/servers/{id}/rooms", a.listRooms)
	mux.HandleFunc("POST /api/servers/{id}/rooms", a.createRoom)
	mux.HandleFunc("GET /api/servers/{id}/registry", a.registry)
	mux.HandleFunc("GET /api/servers/{id}/personas", a.listPersonas)
	mux.HandleFunc("POST /api/servers/{id}/personas", a.createPersona)
	mux.HandleFunc("PATCH /api/personas/{id}", a.updatePersona)
	mux.HandleFunc("DELETE /api/personas/{id}", a.deletePersona)
	mux.HandleFunc("GET /api/personas/{id}/keys", a.listPersonaKeys)
	mux.HandleFunc("POST /api/personas/{id}/keys", a.mintKey)
	mux.HandleFunc("DELETE /api/personas/{id}/keys/{keyId}", a.revokePersonaKey)
	mux.HandleFunc("GET /api/personas/{id}/grants", a.listPersonaGrants)
	mux.HandleFunc("POST /api/personas/{id}/grants", a.grantPersona)
	mux.HandleFunc("DELETE /api/personas/{id}/grants/{roomId}", a.revokePersonaGrant)
	mux.HandleFunc("GET /api/rooms/{id}/messages", a.roomMessages)
	mux.HandleFunc("GET /api/rooms/{id}/search", a.roomSearch)
	mux.HandleFunc("GET /api/notifications", a.listNotifications)
	mux.HandleFunc("POST /api/notifications/read", a.readNotifications)
	mux.HandleFunc("GET /api/dms", a.listMyDMs)
	mux.HandleFunc("POST /api/dms", a.openDM)
	mux.HandleFunc("GET /api/dms/{id}/messages", a.dmMessages)
	mux.HandleFunc("DELETE /api/dms/{id}", a.deleteDM)
}

// PersonaRoomsHandler is authed by a PERSONA KEY (not the user cookie): a host
// presents its key and gets the persona's granted rooms, so it can subscribe to
// all of them and a model can see its own access. Registered as a public route
// (it does its own auth). Key via ?key= or "Authorization: Bearer <key>".
func (a *API) PersonaRoomsHandler(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		key = strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	}
	p, ok, err := a.store.ValidatePersonaKey(r.Context(), strings.TrimSpace(key))
	if err != nil || !ok {
		writeErr(w, 401, "invalid persona key")
		return
	}
	rooms, err := a.store.PersonaRooms(r.Context(), p.ID)
	if err != nil {
		writeErr(w, 500, "could not load rooms")
		return
	}
	writeJSON(w, 200, map[string]any{
		"persona": map[string]string{"slug": p.Slug, "displayName": p.DisplayName, "respondPolicy": p.RespondPolicy},
		"rooms":   orEmpty(rooms),
	})
}

// listNotifications returns the caller's latest mention alerts.
func (a *API) listNotifications(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFrom(r.Context())
	ns, err := a.store.ListNotifications(r.Context(), u.ID, 50)
	if err != nil {
		writeErr(w, 500, "could not load notifications")
		return
	}
	writeJSON(w, 200, orEmpty(ns))
}

// readNotifications marks the given ids read; with an empty body or no ids it
// marks everything read.
func (a *API) readNotifications(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFrom(r.Context())
	var in struct {
		IDs []string `json:"ids"`
	}
	_ = json.NewDecoder(r.Body).Decode(&in) // empty body = mark all
	if err := a.store.MarkNotificationsRead(r.Context(), u.ID, in.IDs); err != nil {
		writeErr(w, 500, "could not mark notifications read")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// PersonaDMsHandler is authed by a PERSONA KEY: a host gets the persona's DM
// threads so it can subscribe to them. Public route (does its own key auth).
func (a *API) PersonaDMsHandler(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		key = strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	}
	p, ok, err := a.store.ValidatePersonaKey(r.Context(), strings.TrimSpace(key))
	if err != nil || !ok {
		writeErr(w, 401, "invalid persona key")
		return
	}
	dms, err := a.store.PersonaDMs(r.Context(), p.ID)
	if err != nil {
		writeErr(w, 500, "could not load dms")
		return
	}
	writeJSON(w, 200, map[string]any{"dms": orEmpty(dms)})
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

func (a *API) getServer(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFrom(r.Context())
	id := r.PathValue("id")
	role, ok := a.member(w, r, id, u.ID)
	if !ok {
		return
	}
	sv, err := a.store.GetServer(r.Context(), id)
	if err != nil {
		writeErr(w, 404, "server not found")
		return
	}
	// Only owners/admins see the invite token.
	if role != "owner" && role != "admin" {
		sv.JoinToken = ""
	}
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

// --- direct messages --------------------------------------------------------

func (a *API) listMyDMs(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFrom(r.Context())
	dms, err := a.store.ListDMsForUser(r.Context(), u.ID)
	if err != nil {
		writeErr(w, 500, "could not list dms")
		return
	}
	writeJSON(w, 200, orEmpty(dms))
}

// openDM finds-or-creates a 1:1 DM with a persona (must be dm-enabled, same
// server) or another user (both members of the given server).
func (a *API) openDM(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFrom(r.Context())
	var in struct {
		Kind      string `json:"kind"`
		PersonaID string `json:"personaId"`
		UserID    string `json:"userId"`
		ServerID  string `json:"serverId"`
	}
	if !decode(w, r, &in) {
		return
	}
	switch in.Kind {
	case "user_persona":
		p, err := a.store.GetPersona(r.Context(), in.PersonaID)
		if err != nil {
			writeErr(w, 404, "persona not found")
			return
		}
		if _, ok := a.member(w, r, p.ServerID, u.ID); !ok {
			return
		}
		dm, err := a.store.OpenDMWithPersona(r.Context(), u.ID, in.PersonaID)
		if err != nil {
			writeErr(w, 400, err.Error())
			return
		}
		writeJSON(w, 200, dm)
	case "user_user":
		if _, ok := a.member(w, r, in.ServerID, u.ID); !ok {
			return
		}
		if _, ok, _ := a.store.ServerRole(r.Context(), in.ServerID, in.UserID); !ok {
			writeErr(w, 400, "that user is not in this server")
			return
		}
		dm, err := a.store.OpenDMWithUser(r.Context(), in.ServerID, u.ID, in.UserID)
		if err != nil {
			writeErr(w, 400, err.Error())
			return
		}
		writeJSON(w, 200, dm)
	default:
		writeErr(w, 400, "kind must be user_persona or user_user")
	}
}

func (a *API) dmMessages(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFrom(r.Context())
	dmID := r.PathValue("id")
	if ok, _ := a.store.UserCanAccessDM(r.Context(), dmID, u.ID); !ok {
		writeErr(w, 403, "no access to this conversation")
		return
	}
	msgs, err := a.store.ListDMMessages(r.Context(), dmID, 100)
	if err != nil {
		writeErr(w, 500, "could not load messages")
		return
	}
	writeJSON(w, 200, orEmptyMsgs(msgs))
}

// deleteDM removes a DM thread (and its messages) — a participant may close it.
func (a *API) deleteDM(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFrom(r.Context())
	dmID := r.PathValue("id")
	if ok, _ := a.store.UserCanAccessDM(r.Context(), dmID, u.ID); !ok {
		writeErr(w, 403, "no access to this conversation")
		return
	}
	if err := a.store.DeleteDM(r.Context(), dmID); err != nil {
		writeErr(w, 500, "could not delete conversation")
		return
	}
	w.WriteHeader(http.StatusNoContent)
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

// canManagePersona loads a persona and confirms the user may manage it (its
// owner, or a server owner/admin). Writes the error + returns ok=false otherwise.
func (a *API) canManagePersona(w http.ResponseWriter, r *http.Request, personaID string) (store.Persona, bool) {
	u, _ := auth.UserFrom(r.Context())
	p, err := a.store.GetPersona(r.Context(), personaID)
	if err != nil {
		writeErr(w, 404, "persona not found")
		return store.Persona{}, false
	}
	role, ok := a.member(w, r, p.ServerID, u.ID)
	if !ok {
		return store.Persona{}, false
	}
	if p.OwnerUserID != u.ID && role != "owner" && role != "admin" {
		writeErr(w, 403, "not allowed to manage this persona")
		return store.Persona{}, false
	}
	return p, true
}

// updatePersona patches mutable persona fields (currently dmEnabled).
func (a *API) updatePersona(w http.ResponseWriter, r *http.Request) {
	p, ok := a.canManagePersona(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	var in struct {
		DMEnabled     *bool   `json:"dmEnabled"`
		RespondPolicy *string `json:"respondPolicy"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.DMEnabled != nil {
		if err := a.store.SetPersonaDMEnabled(r.Context(), p.ID, *in.DMEnabled); err != nil {
			writeErr(w, 500, "could not update persona")
			return
		}
	}
	if in.RespondPolicy != nil {
		switch *in.RespondPolicy {
		case "all", "mentioned", "judgment":
		default:
			writeErr(w, 400, "respondPolicy must be all, mentioned, or judgment")
			return
		}
		if err := a.store.SetPersonaRespondPolicy(r.Context(), p.ID, *in.RespondPolicy); err != nil {
			writeErr(w, 500, "could not update persona")
			return
		}
	}
	np, err := a.store.GetPersona(r.Context(), p.ID)
	if err != nil {
		writeErr(w, 500, "could not reload persona")
		return
	}
	writeJSON(w, 200, np)
}

// deletePersona soft-deletes a persona (status=disabled): it disappears from
// listings, its keys stop working, and its history is preserved. Owner/admin only.
func (a *API) deletePersona(w http.ResponseWriter, r *http.Request) {
	p, ok := a.canManagePersona(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	if err := a.store.DisablePersona(r.Context(), p.ID); err != nil {
		writeErr(w, 500, "could not delete persona")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// listPersonaKeys returns a persona's keys (metadata only — never the raw key).
func (a *API) listPersonaKeys(w http.ResponseWriter, r *http.Request) {
	p, ok := a.canManagePersona(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	keys, err := a.store.ListPersonaKeys(r.Context(), p.ID)
	if err != nil {
		writeErr(w, 500, "could not list keys")
		return
	}
	writeJSON(w, 200, orEmpty(keys))
}

// revokePersonaKey soft-revokes a key so it stops validating.
func (a *API) revokePersonaKey(w http.ResponseWriter, r *http.Request) {
	p, ok := a.canManagePersona(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	if err := a.store.RevokePersonaKey(r.Context(), p.ID, r.PathValue("keyId")); err != nil {
		writeErr(w, 500, "could not revoke key")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// listPersonaGrants returns the rooms a persona is granted into.
func (a *API) listPersonaGrants(w http.ResponseWriter, r *http.Request) {
	p, ok := a.canManagePersona(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	rooms, err := a.store.PersonaRooms(r.Context(), p.ID)
	if err != nil {
		writeErr(w, 500, "could not list grants")
		return
	}
	writeJSON(w, 200, orEmpty(rooms))
}

// revokePersonaGrant removes a persona's grant to a room.
func (a *API) revokePersonaGrant(w http.ResponseWriter, r *http.Request) {
	p, ok := a.canManagePersona(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	if err := a.store.RevokePersonaRoom(r.Context(), p.ID, r.PathValue("roomId")); err != nil {
		writeErr(w, 500, "could not revoke grant")
		return
	}
	w.WriteHeader(http.StatusNoContent)
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
