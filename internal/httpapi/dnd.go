// D&D proxy (DH-4): the frontend's /char panel and HP chips call these; the
// server forwards to the dnd-tools service so the API key never reaches the
// browser. Access = room membership; edits = the sheet's player or a room
// admin.
package httpapi

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/cpuchip/ai-chattermax/internal/auth"
)

func (a *API) registerDND(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/dnd/rooms/{id}/characters", a.dndRoomCharacters)
	mux.HandleFunc("GET /api/dnd/rooms/{id}/me", a.dndMyCharacter)
	mux.HandleFunc("GET /api/dnd/rooms/{id}/characters/{name}", a.dndCharacter)
	mux.HandleFunc("PATCH /api/dnd/rooms/{id}/characters/{name}", a.dndPatchCharacter)
	mux.HandleFunc("GET /api/dnd/rooms/{id}/campaign", a.dndRoomCampaign)
	mux.HandleFunc("PUT /api/dnd/rooms/{id}/campaign", a.dndBindCampaign)
	mux.HandleFunc("GET /api/dnd/campaigns", a.dndCampaigns)
}

// dndRoomCampaign reports the room's bound campaign (the D&D switch state) —
// 404 when unbound, which is the frontend's "D&D off" signal.
func (a *API) dndRoomCampaign(w http.ResponseWriter, r *http.Request) {
	roomID, ok := a.dndRoomAccess(w, r)
	if !ok {
		return
	}
	name, err := a.dnd.CampaignByRoom(r.Context(), roomID)
	if err != nil {
		writeErr(w, http.StatusNotFound, "no campaign bound")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"campaign": name})
}

// dndBindCampaign binds/unbinds the room's campaign (room admins) — the
// Settings panel path; /dnd enable and /campaign are the chat paths.
func (a *API) dndBindCampaign(w http.ResponseWriter, r *http.Request) {
	roomID, ok := a.dndRoomAccess(w, r)
	if !ok {
		return
	}
	u, _ := auth.UserFrom(r.Context())
	if admin, _ := a.store.UserIsRoomAdmin(r.Context(), roomID, u.ID); !admin {
		writeErr(w, http.StatusForbidden, "room admins only")
		return
	}
	var body struct {
		Campaign string `json:"campaign"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, `body must be {"campaign":"<name>"} (empty unbinds)`)
		return
	}
	name, err := a.dnd.BindRoom(r.Context(), roomID, body.Campaign)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"campaign": name, "bound": body.Campaign})
}

// dndCampaigns lists campaigns (for the Settings bind picker).
func (a *API) dndCampaigns(w http.ResponseWriter, r *http.Request) {
	if !a.dnd.Enabled() {
		writeErr(w, http.StatusNotImplemented, "dnd-tools is not configured on this server")
		return
	}
	cs, err := a.dnd.Campaigns(r.Context())
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, cs)
}

// dndRoomAccess gates a request on the integration + room membership and
// returns the room id.
func (a *API) dndRoomAccess(w http.ResponseWriter, r *http.Request) (string, bool) {
	if !a.dnd.Enabled() {
		writeErr(w, http.StatusNotImplemented, "dnd-tools is not configured on this server")
		return "", false
	}
	u, _ := auth.UserFrom(r.Context())
	roomID := r.PathValue("id")
	if ok, _ := a.store.UserCanAccessRoom(r.Context(), roomID, u.ID); !ok {
		writeErr(w, http.StatusForbidden, "no access to this room")
		return "", false
	}
	return roomID, true
}

func writeRaw(w http.ResponseWriter, raw json.RawMessage) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(raw)
}

func (a *API) dndRoomCharacters(w http.ResponseWriter, r *http.Request) {
	roomID, ok := a.dndRoomAccess(w, r)
	if !ok {
		return
	}
	raw, err := a.dnd.RoomCharacters(r.Context(), roomID)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeRaw(w, raw)
}

func (a *API) dndMyCharacter(w http.ResponseWriter, r *http.Request) {
	roomID, ok := a.dndRoomAccess(w, r)
	if !ok {
		return
	}
	u, _ := auth.UserFrom(r.Context())
	raw, err := a.dnd.PlayerCharacterRaw(r.Context(), roomID, u.DisplayName)
	if err != nil {
		writeErr(w, http.StatusNotFound, err.Error())
		return
	}
	writeRaw(w, raw)
}

func (a *API) dndCharacter(w http.ResponseWriter, r *http.Request) {
	roomID, ok := a.dndRoomAccess(w, r)
	if !ok {
		return
	}
	campaign, err := a.dnd.CampaignByRoom(r.Context(), roomID)
	if err != nil {
		writeErr(w, http.StatusNotFound, err.Error())
		return
	}
	raw, err := a.dnd.CharacterByName(r.Context(), r.PathValue("name"), campaign)
	if err != nil {
		writeErr(w, http.StatusNotFound, err.Error())
		return
	}
	writeRaw(w, raw)
}

func (a *API) dndPatchCharacter(w http.ResponseWriter, r *http.Request) {
	roomID, ok := a.dndRoomAccess(w, r)
	if !ok {
		return
	}
	u, _ := auth.UserFrom(r.Context())
	campaign, err := a.dnd.CampaignByRoom(r.Context(), roomID)
	if err != nil {
		writeErr(w, http.StatusNotFound, err.Error())
		return
	}
	name := r.PathValue("name")

	// Edit gate: your own sheet, or room admin for any sheet.
	raw, err := a.dnd.CharacterByName(r.Context(), name, campaign)
	if err != nil {
		writeErr(w, http.StatusNotFound, err.Error())
		return
	}
	var ch struct {
		Player string `json:"player"`
	}
	_ = json.Unmarshal(raw, &ch)
	if ch.Player != u.DisplayName {
		if admin, _ := a.store.UserIsRoomAdmin(r.Context(), roomID, u.ID); !admin {
			writeErr(w, http.StatusForbidden, "only the sheet's player (or a room admin) can edit it")
			return
		}
	}

	patch, err := io.ReadAll(io.LimitReader(r.Body, 256<<10))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "could not read patch body")
		return
	}
	updated, err := a.dnd.Patch(r.Context(), name, campaign, patch)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeRaw(w, updated)
}
