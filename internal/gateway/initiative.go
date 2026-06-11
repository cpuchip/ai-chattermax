package gateway

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"

	"github.com/cpuchip/ai-chattermax/internal/store"
)

type initiativeFrame struct {
	Type    string                `json:"type"` // "initiative"
	Channel string                `json:"channel"`
	Round   store.InitiativeRound `json:"round"`
}

var initMod = regexp.MustCompile(`^([+-]?\d+)$`)

// handleInitiative routes the /initiative | /init family (DH-1/D8). Anyone may
// start a round or roll themselves in; mutations (next/add/remove/end) are for
// the starter, server owner/admins, and personas (the DM persona IS the DM).
// Every successful action broadcasts the full panel state and posts a compact
// log-of-record message (the returned body).
func (h *Handler) handleInitiative(c *Client, channel, kind, args string, human *store.User, persona *store.Persona) (string, bool) {
	fail := func(msg string) (string, bool) {
		c.enqueue(marshal(errorFrame{Type: "error", Message: msg}))
		return "", true
	}
	if kind != "room" {
		return fail("initiative runs in rooms, not DMs")
	}
	ctx := context.Background()
	sub, rest, _ := strings.Cut(args, " ")
	rest = strings.TrimSpace(rest)

	switch strings.ToLower(sub) {
	case "start":
		var uid, pid *string
		if human != nil {
			uid = &human.ID
		} else if persona != nil {
			pid = &persona.ID
		}
		if _, err := h.store.StartInitiative(ctx, channel, uid, pid); err != nil {
			return fail(err.Error())
		}
		h.broadcastInitiative(ctx, channel)
		return "⚔️ **Roll for initiative!** Join with `/init +<mod>` — the server rolls d20+mod.", false

	case "next", "end", "remove", "add":
		r, ok, err := h.store.ActiveInitiative(ctx, channel)
		if err != nil || !ok {
			return fail("no initiative round is running — `/initiative start`")
		}
		if !h.canRunInitiative(ctx, r, human, persona) {
			return fail("only the round's starter, server admins, or personas can do that")
		}
		switch strings.ToLower(sub) {
		case "next":
			nr, err := h.store.AdvanceInitiative(ctx, channel)
			if err != nil {
				return fail(err.Error())
			}
			h.broadcastInitiative(ctx, channel)
			return fmt.Sprintf("⚔️ Round %d — **%s**'s turn", nr.Round, currentName(nr)), false
		case "end":
			er, err := h.store.EndInitiative(ctx, channel)
			if err != nil {
				return fail(err.Error())
			}
			h.broadcastInitiative(ctx, channel)
			return fmt.Sprintf("⚔️ Initiative ends (round %d)", er.Round), false
		case "remove":
			if rest == "" {
				return fail("usage: /init remove <name>")
			}
			gone, err := h.store.RemoveInitiativeEntry(ctx, r.ID, rest)
			if err != nil || !gone {
				return fail(fmt.Sprintf("%q isn't in the order", rest))
			}
			h.broadcastInitiative(ctx, channel)
			return fmt.Sprintf("⚔️ %s drops out of the order", rest), false
		default: // add
			name, mod := rest, 0
			if i := strings.LastIndex(rest, " "); i > 0 {
				if m := initMod.FindString(strings.TrimSpace(rest[i+1:])); m != "" {
					mod, _ = strconv.Atoi(m)
					name = strings.TrimSpace(rest[:i])
				}
			}
			if name == "" {
				return fail("usage: /init add <name> [+mod]")
			}
			return h.rollIn(ctx, c, channel, r.ID, name, mod, "%s joins the order")
		}

	default:
		// Self-join: /init +3, /init -1, /init 3 (the modifier is yours).
		if m := initMod.FindString(strings.TrimSpace(args)); m != "" {
			r, ok, err := h.store.ActiveInitiative(ctx, channel)
			if err != nil || !ok {
				return fail("no initiative round is running — `/initiative start`")
			}
			mod, _ := strconv.Atoi(m)
			return h.rollIn(ctx, c, channel, r.ID, c.who.Name, mod, "%s rolls initiative")
		}
		// Bare /init (DH-4/D8 tie-in): pull DEX from YOUR character sheet and
		// join under the character's name — the sheet is the truth.
		if strings.TrimSpace(args) == "" && h.dnd.Enabled() {
			r, ok, err := h.store.ActiveInitiative(ctx, channel)
			if err != nil || !ok {
				return fail("no initiative round is running — `/initiative start`")
			}
			ch, err := h.dnd.PlayerCharacter(ctx, channel, c.who.Name)
			if err != nil {
				return fail(err.Error() + " — or join with an explicit /init +<mod>")
			}
			res, err := h.dnd.Check(ctx, ch.Name, ch.Campaign, "initiative")
			if err != nil {
				return fail(err.Error())
			}
			return h.rollIn(ctx, c, channel, r.ID, ch.Name, res.Mod, "%s rolls initiative (DEX, from the sheet)")
		}
		return fail("usage: /initiative start · /init (your sheet's DEX) · /init +<mod> · /init add <name> [+mod] · /init next · /init remove <name> · /init end")
	}
}

// rollIn rolls d20+mod server-side, slots the combatant, broadcasts the panel.
func (h *Handler) rollIn(ctx context.Context, c *Client, channel, roundID, name string, mod int, verb string) (string, bool) {
	roll := rollDie(20)
	total := roll + mod
	if err := h.store.UpsertInitiativeEntry(ctx, roundID, name, mod, roll, total); err != nil {
		log.Printf("gateway initiative: %v", err)
		c.enqueue(marshal(errorFrame{Type: "error", Message: "could not record the roll"}))
		return "", true
	}
	h.broadcastInitiative(ctx, channel)
	out := fmt.Sprintf("⚔️ "+verb+": [%d]", name, roll)
	if mod != 0 {
		out += fmt.Sprintf(" %+d", mod)
	}
	return out + fmt.Sprintf(" = **%d**", total), false
}

// canRunInitiative: starter, server owner/admin, or any persona.
func (h *Handler) canRunInitiative(ctx context.Context, r store.InitiativeRound, human *store.User, persona *store.Persona) bool {
	if persona != nil {
		return true
	}
	if human == nil {
		return false
	}
	if r.StarterID == human.ID {
		return true
	}
	ok, err := h.store.UserIsRoomAdmin(ctx, r.RoomID, human.ID)
	return err == nil && ok
}

// broadcastInitiative pushes the full panel state (or the cleared state when
// no round is active) to everyone in the room — idempotent client patch.
func (h *Handler) broadcastInitiative(ctx context.Context, channel string) {
	r, ok, err := h.store.ActiveInitiative(ctx, channel)
	if err != nil {
		log.Printf("gateway initiative state: %v", err)
		return
	}
	if !ok {
		r = store.InitiativeRound{RoomID: channel, Active: false}
	}
	h.hub.broadcast(channel, marshal(initiativeFrame{Type: "initiative", Channel: channel, Round: r}), nil)
}

func currentName(r store.InitiativeRound) string {
	for _, e := range r.Entries {
		if e.ID == r.CurrentEntryID {
			return e.Name
		}
	}
	return "—"
}
