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

// handleProgram runs /archive and /resume — the holodeck session boundary.
// Personas and room admins may run it; the broadcast `program` frame tells
// persona hosts to close out / rotate their sessions.
func (h *Handler) handleProgram(c *Client, channel, op string, human *store.User, persona *store.Persona) (string, bool) {
	allowed := persona != nil
	if human != nil {
		if ok, err := h.store.UserIsRoomAdmin(context.Background(), channel, human.ID); err == nil && ok {
			allowed = true
		}
	}
	if !allowed {
		return h.dndErr(c, "/"+op+" is for room admins (or personas)")
	}
	h.hub.broadcast(channel, marshal(programFrame{Type: "program", Channel: channel, Op: op, By: c.who.Name}), nil)
	if op == "archive" {
		return "📼 **Program archived** — the holodeck dims. Sessions will be summarized and rotated; /resume reopens the program.", false
	}
	return "▶️ **Program resumed** — the holodeck hums back to life. Personas, check the campaign log (dnd_campaign_get) before improvising.", false
}
