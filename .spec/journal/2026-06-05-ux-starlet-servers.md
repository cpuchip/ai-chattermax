---
date: 2026-06-05
title: "LCARS UI + mobile fix + Starlet always-on + server/invite model (all shipped live)"
---

## What happened

A fast iterative session with Michael on the live platform (post Stage-1 deploy). Four things shipped to chat.ibeco.me, each verified:

1. **LCARS theme + responsive layout.** Rebuilt the frontend on cpuchip.net's LCARS design system (palette, Antonio font, elbow/pill shapes). Desktop = the three-pane elbow frame; mobile collapses to a compact bar + slide-in drawers. Settings page for persona/key management; native LCARS dialogs replaced window.prompt. (commits up to `1e4f84a`.)

2. **Mobile composer fix** (`dec876a`). Michael's phone screenshot showed the composer cut off + covered by the keyboard — the `100vh` trap. Fixed with `100dvh` everywhere + `interactive-widget=resizes-content` on the viewport meta so the keyboard resizes the layout. Verified live (viewport meta present).

3. **Starlet — first persona always-on** (`1b24d41`). Michael created a "Starlet" persona in the UI, granted it to #main-game, minted a key. I wired it: created a local `pg-starlet` persona-host persona (old-Hollywood-glamour starship-lounge character — a placeholder Michael can retune), then made persona-host a **service in the substrate's own compose** (`extension/docker-compose.yaml`, `restart: unless-stopped`) — same compose because it needs the substrate DB (`pg:5432`) for cognition, which isn't network-exposed. Key lives in the gitignored `.env`. Verified: container connects + Starlet replies in character on prod. She survives restarts now.

4. **Server/membership model — own-server-on-signup + invite links** (`e42bfe5`). Michael hit the seam: everyone was auto-joined to one shared demo (my v1 shortcut). Reworked to his model (ratified Option A): a user who owns no server gets their own on login (idempotent); collaboration is via **invite links** (`chat.ibeco.me/?join=<token>`, auto-joins after login) surfaced in Settings with a member list; server-rail "+" → Create/Join dialog. Verified live (test account now owns "Claude Codetest's Server" with an invite token).

## Key decisions / notes

- persona-host belongs in the substrate compose, NOT a separate/remote one (needs the non-exposed substrate DB).
- Auto-create trigger = "owns no server" (so existing members of others, like Michael in Tavern Keep, still get their own board). Idempotent.
- Invite token is returned only to owner/admin (GET /api/servers/{id}).
- Dropped the demo auto-join; the seeded Tavern Keep + demo personas remain but nobody's auto-joined.

## Carry-forward

- **Starlet's home:** she's still bound to the seed's Tavern Keep #main-game (where Michael + claude-codetest are both already members). To move her into Michael's OWN (invitable) server: create a Starlet persona there → mint key → update persona-host `.env` CHATTERMAX_PERSONAS → restart the persona-host service.
- **Invite-by-username/email** (add an existing user directly) — deferred; the link covers the immediate need.
- Remaining stages: DMs (the CT2 surface), sub-personas, the per-message rate ceiling, moderation toolkit.
- A test ibeco account exists for verification: `claude-codetest@ibeco.me` (becoming id 8) — see [[feedback_test_in_prod_with_own_creds]].
