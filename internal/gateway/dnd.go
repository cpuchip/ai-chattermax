// D&D slash commands (DH-4): sheet-aware /check, /save, /attack, /cast, /hp
// backed by a dnd-tools service, plus /archive + /resume program control.
// dnd-tools resolves WHAT to roll; the dice are rolled HERE (crypto/rand,
// posted in-room) — the same fairness story as /roll.
package gateway

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/cpuchip/ai-chattermax/internal/store"
)

// dndErr sends a command error back to the sender only.
func (h *Handler) dndErr(c *Client, msg string) (string, bool) {
	c.enqueue(marshal(errorFrame{Type: "error", Message: msg}))
	return "", true
}

// dndReady guards every D&D command behind a configured service.
func (h *Handler) dndReady(c *Client) bool {
	if h.dnd.Enabled() {
		return true
	}
	c.enqueue(marshal(errorFrame{Type: "error", Message: "D&D commands need a dnd-tools service configured (DND_URL) — ask the server owner"}))
	return false
}

// d20Line renders "🎲 [14] +5 = **19**" — one die, sheet modifier shown.
func d20Line(mod int) (string, int) {
	die := rollDie(20)
	total := die + mod
	out := fmt.Sprintf("🎲 [%d]", die)
	if mod != 0 {
		out += fmt.Sprintf(" %+d", mod)
	}
	return out + fmt.Sprintf(" = **%d**", total), total
}

// handleDndCheck runs /check <skill|ability> and /save <ability>.
func (h *Handler) handleDndCheck(c *Client, channel, args string, save bool) (string, bool) {
	if !h.dndReady(c) {
		return "", true
	}
	if args == "" {
		usage := "usage: /check <skill or ability>, e.g. /check stealth"
		if save {
			usage = "usage: /save <ability>, e.g. /save dex"
		}
		return h.dndErr(c, usage)
	}
	ctx := context.Background()
	ch, err := h.dnd.PlayerCharacter(ctx, channel, c.who.Name)
	if err != nil {
		return h.dndErr(c, err.Error())
	}
	check := args
	if save {
		check += " save"
	}
	res, err := h.dnd.Check(ctx, ch.Name, ch.Campaign, check)
	if err != nil {
		return h.dndErr(c, err.Error())
	}
	line, _ := d20Line(res.Mod)
	return fmt.Sprintf("**%s** — %s: %s *(%s)*", ch.Name, res.Label, line, res.Breakdown), false
}

// handleDndAttack runs /attack [target] [with <weapon>] — rolls to hit now;
// the DM adjudicates; the damage roll is handed back ready to post.
func (h *Handler) handleDndAttack(c *Client, channel, args string) (string, bool) {
	if !h.dndReady(c) {
		return "", true
	}
	target, weapon := args, ""
	if t, w, found := cutLastWith(args); found {
		target, weapon = t, w
	}
	ctx := context.Background()
	ch, err := h.dnd.PlayerCharacter(ctx, channel, c.who.Name)
	if err != nil {
		return h.dndErr(c, err.Error())
	}
	res, err := h.dnd.Attack(ctx, ch.Name, ch.Campaign, weapon, target)
	if err != nil {
		return h.dndErr(c, err.Error())
	}
	line, _ := d20Line(res.Result.ToHit)
	vs := ""
	if target != "" {
		vs = " " + target
	}
	out := fmt.Sprintf("⚔️ **%s** attacks%s with %s: %s to hit *(%s)*.",
		ch.Name, vs, res.Result.Weapon, line, res.Result.Breakdown)
	if res.Result.DamageRoll != "" {
		out += fmt.Sprintf("\nOn a hit: `%s`", res.Result.DamageRoll)
	}
	return out, false
}

// cutLastWith splits "the goblin with longsword" → ("the goblin", "longsword").
// The LAST " with " wins so targets may contain the word.
func cutLastWith(s string) (target, weapon string, found bool) {
	idx := strings.LastIndex(strings.ToLower(" "+s+" "), " with ")
	if idx < 0 {
		return s, "", false
	}
	idx-- // compensate the leading space pad
	return strings.TrimSpace(s[:max(idx, 0)]), strings.TrimSpace(s[min(idx+6, len(s)):]), true
}

// handleDndCast runs /cast <spell> [@slotLevel].
func (h *Handler) handleDndCast(c *Client, channel, args string) (string, bool) {
	if !h.dndReady(c) {
		return "", true
	}
	if args == "" {
		return h.dndErr(c, "usage: /cast <spell> [@slot], e.g. /cast fireball or /cast fireball @5")
	}
	spell := args
	slot := 0
	if i := strings.LastIndex(args, "@"); i >= 0 {
		if n, err := strconv.Atoi(strings.TrimSpace(args[i+1:])); err == nil {
			spell = strings.TrimSpace(args[:i])
			slot = n
		}
	}
	ctx := context.Background()
	ch, err := h.dnd.PlayerCharacter(ctx, channel, c.who.Name)
	if err != nil {
		return h.dndErr(c, err.Error())
	}
	res, err := h.dnd.Cast(ctx, ch.Name, ch.Campaign, spell, slot)
	if err != nil {
		return h.dndErr(c, err.Error())
	}
	var out string
	if res.Level == 0 {
		out = fmt.Sprintf("✨ **%s** casts **%s** (cantrip).", ch.Name, res.Spell)
	} else {
		out = fmt.Sprintf("✨ **%s** casts **%s** (level-%d slot — %d left).",
			ch.Name, res.Spell, res.SlotUsed, res.SlotsRemaining)
	}
	if res.DamageRoll != "" {
		if rolled, err := rollCommand(res.DamageRoll); err == nil {
			out += "\n" + rolled
		}
	}
	return out, false
}

// handleDndHP runs /hp <delta> [character] — own character by default; naming
// another sheet (the DM nudging a goblin) works for any room member, table-trust.
func (h *Handler) handleDndHP(c *Client, channel, args string) (string, bool) {
	if !h.dndReady(c) {
		return "", true
	}
	deltaStr, rest, _ := strings.Cut(args, " ")
	delta, err := strconv.Atoi(strings.TrimSpace(deltaStr))
	if err != nil || delta == 0 {
		return h.dndErr(c, "usage: /hp -5 (damage) or /hp +3 (healing), optionally /hp -5 Goblin")
	}
	ctx := context.Background()
	var name, campaign string
	if rest = strings.TrimSpace(rest); rest != "" {
		camp, err := h.dnd.CampaignByRoom(ctx, channel)
		if err != nil {
			return h.dndErr(c, err.Error())
		}
		name, campaign = rest, camp
	} else {
		ch, err := h.dnd.PlayerCharacter(ctx, channel, c.who.Name)
		if err != nil {
			return h.dndErr(c, err.Error())
		}
		name, campaign = ch.Name, ch.Campaign
	}
	res, err := h.dnd.HP(ctx, name, campaign, delta)
	if err != nil {
		return h.dndErr(c, err.Error())
	}
	icon := "💚"
	if delta < 0 {
		icon = "🩸"
	}
	return fmt.Sprintf("%s **%s**: %d/%d HP", icon, res.Character, res.HPCurrent, res.HPMax), false
}

// roomAdmin reports whether the sender may run room-scoped admin commands
// (personas count — the DM persona IS the DM).
func (h *Handler) roomAdmin(channel string, human *store.User, persona *store.Persona) bool {
	if persona != nil {
		return true
	}
	if human == nil {
		return false
	}
	ok, err := h.store.UserIsRoomAdmin(context.Background(), channel, human.ID)
	return err == nil && ok
}

// handleProgram runs /archive and /resume — the holodeck session boundary.
// Only rooms with a bound campaign have a "program"; the broadcast frame
// tells persona hosts to close out / rotate their sessions.
func (h *Handler) handleProgram(c *Client, channel, op string, human *store.User, persona *store.Persona) (string, bool) {
	if !h.dndReady(c) {
		return "", true
	}
	if !h.roomAdmin(channel, human, persona) {
		return h.dndErr(c, "/"+op+" is for room admins (or personas)")
	}
	campaign, err := h.dnd.CampaignByRoom(context.Background(), channel)
	if err != nil {
		return h.dndErr(c, "this room has no program — /dnd enable binds a campaign first")
	}
	h.hub.broadcast(channel, marshal(programFrame{Type: "program", Channel: channel, Op: op, By: c.who.Name}), nil)
	if op == "archive" {
		return "📼 **Program archived** — *" + campaign + "* dims. Sessions will be summarized and rotated; /resume reopens the program.", false
	}
	return "▶️ **Program resumed** — *" + campaign + "* hums back to life. Personas, check the campaign log (dnd_campaign_get) before improvising.", false
}

// handleDndToggle runs /dnd enable [name] and /dnd disable — the room's D&D
// switch IS the campaign binding (no separate flag to fall out of sync).
// Bare /dnd enable creates (or reuses) a campaign named after the room.
func (h *Handler) handleDndToggle(c *Client, channel, args string, human *store.User, persona *store.Persona) (string, bool) {
	if !h.dndReady(c) {
		return "", true
	}
	sub, rest, _ := strings.Cut(args, " ")
	rest = strings.TrimSpace(rest)
	ctx := context.Background()
	switch strings.ToLower(sub) {
	case "enable", "on":
		if !h.roomAdmin(channel, human, persona) {
			return h.dndErr(c, "/dnd enable is for room admins")
		}
		if name, err := h.dnd.CampaignByRoom(ctx, channel); err == nil {
			return h.dndErr(c, "this room is already playing **"+name+"** — /dnd disable first to switch")
		}
		name := rest
		if name == "" {
			name = h.roomCampaignName(channel)
		}
		bound, err := h.dnd.BindRoom(ctx, channel, name)
		if err != nil {
			return h.dndErr(c, err.Error())
		}
		h.broadcastDndState(channel)
		return "🎲 **D&D enabled** — this room now plays **" + bound + "**. Sheets, /attack, /check and friends are live; /char opens your sheet.", false
	case "disable", "off":
		if !h.roomAdmin(channel, human, persona) {
			return h.dndErr(c, "/dnd disable is for room admins")
		}
		was, err := h.dnd.BindRoom(ctx, channel, "")
		if err != nil {
			return h.dndErr(c, err.Error())
		}
		h.broadcastDndState(channel)
		return "🚪 **D&D disabled** — **" + was + "** is unbound (the campaign and its sheets are kept; /dnd enable " + was + " brings it back).", false
	default:
		return h.dndErr(c, "usage: /dnd enable [campaign name] · /dnd disable")
	}
}

// handleCampaign runs /campaign [bind <name> | unbind] — view for everyone,
// changes for room admins.
func (h *Handler) handleCampaign(c *Client, channel, args string, human *store.User, persona *store.Persona) (string, bool) {
	if !h.dndReady(c) {
		return "", true
	}
	sub, rest, _ := strings.Cut(args, " ")
	rest = strings.TrimSpace(rest)
	ctx := context.Background()
	switch strings.ToLower(sub) {
	case "":
		name, err := h.dnd.CampaignByRoom(ctx, channel)
		if err != nil {
			return h.dndErr(c, "no campaign is bound to this room — /dnd enable starts one")
		}
		return "🗺 This room plays **" + name + "**.", false
	case "bind":
		if rest == "" {
			return h.dndErr(c, "usage: /campaign bind <name>")
		}
		if !h.roomAdmin(channel, human, persona) {
			return h.dndErr(c, "/campaign bind is for room admins")
		}
		bound, err := h.dnd.BindRoom(ctx, channel, rest)
		if err != nil {
			return h.dndErr(c, err.Error())
		}
		h.broadcastDndState(channel)
		return "🗺 This room now plays **" + bound + "**.", false
	case "unbind":
		if !h.roomAdmin(channel, human, persona) {
			return h.dndErr(c, "/campaign unbind is for room admins")
		}
		was, err := h.dnd.BindRoom(ctx, channel, "")
		if err != nil {
			return h.dndErr(c, err.Error())
		}
		h.broadcastDndState(channel)
		return "🚪 **" + was + "** unbound from this room.", false
	default:
		return h.dndErr(c, "usage: /campaign · /campaign bind <name> · /campaign unbind")
	}
}

// roomCampaignName derives a default campaign name from the room ("Holodeck-3
// Campaign"); falls back to a generic name when the room can't be loaded.
func (h *Handler) roomCampaignName(channel string) string {
	if room, err := h.store.GetRoom(context.Background(), channel); err == nil && room.Name != "" {
		return room.Name + " Campaign"
	}
	return "New Campaign"
}

// broadcastDndState nudges subscribers to re-check the room's D&D binding
// (autocomplete gating + HP chips) via a program frame with op "state".
func (h *Handler) broadcastDndState(channel string) {
	h.hub.broadcast(channel, marshal(programFrame{Type: "program", Channel: channel, Op: "state"}), nil)
}
