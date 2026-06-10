package gateway

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Inline commands (DH-1/D8 follow-up): /roll and /init +N execute in the
// MIDDLE of a message, expanded in place — the natural table voice:
//
//	"I lunge at the goblin! /roll 1d20+5"
//	  → "I lunge at the goblin! 🎲 `1d20+5` → [12] +5 = **17**"
//
// This is also what lets personas play: Starlet's "/init +2 — may the dice
// favor the fabulous!" rolled nothing (and she invented a result) when
// commands were start-of-message only. Mutating commands (start/next/end/
// add/remove) stay start-of-message — they're deliberate actions, not prose.
var (
	inlineRollRe = regexp.MustCompile(`(?i)/roll\s+(\d*d(?:\d+|%)(?:\s*[+-]\s*\d+)?(?:\s+(?:adv|dis)\b)?)(\s*\[[^\[\]]{1,200}\])?`)
	inlineInitRe = regexp.MustCompile(`(?i)/init\s+([+-]\d+)\b(\s*\[[^\[\]]{1,200}\])?`)
)

// inlineComment renders a captured "[flavor]" group as " — *flavor*".
func inlineComment(raw string) string {
	c := strings.TrimSpace(raw)
	if c == "" {
		return ""
	}
	return " — *" + strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(c, "["), "]")) + "*"
}

const maxInlinePerMessage = 3

// expandInline executes inline /roll and /init tokens, returning the expanded
// body and whether anything changed. Unparseable or out-of-context tokens are
// left as literal prose — a mid-sentence mention never blocks the message.
func (h *Handler) expandInline(ctx context.Context, c *Client, channel, kind, body string) (string, bool) {
	changed, count := false, 0

	body = inlineRollRe.ReplaceAllStringFunc(body, func(tok string) string {
		if count >= maxInlinePerMessage {
			return tok
		}
		m := inlineRollRe.FindStringSubmatch(tok)
		out, err := rollCommand(m[1])
		if err != nil {
			return tok
		}
		count++
		changed = true
		// Compact inline form: drop the "rolled" word from the block form.
		return strings.Replace(out, "🎲 rolled ", "🎲 ", 1) + inlineComment(m[2])
	})

	if kind == "room" {
		body = inlineInitRe.ReplaceAllStringFunc(body, func(tok string) string {
			if count >= maxInlinePerMessage {
				return tok
			}
			r, ok, err := h.store.ActiveInitiative(ctx, channel)
			if err != nil || !ok {
				return tok // no round running — leave the prose alone
			}
			m := inlineInitRe.FindStringSubmatch(tok)
			mod, _ := strconv.Atoi(m[1])
			roll := rollDie(20)
			total := roll + mod
			if err := h.store.UpsertInitiativeEntry(ctx, r.ID, c.who.Name, mod, roll, total); err != nil {
				return tok
			}
			h.broadcastInitiative(ctx, channel)
			count++
			changed = true
			return fmt.Sprintf("⚔️ initiative [%d] %+d = **%d**", roll, mod, total) + inlineComment(m[2])
		})
	}
	return body, changed
}
