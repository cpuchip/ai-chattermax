---
date: 2026-06-05
title: "Multi-room + the persona-rooms API + a Dokploy stale-build fix; CT2 planned"
---

## What happened

Continuing the day-2 iterative session (after LCARS/mobile/Starlet-always-on/server-model). Michael spawned a big roadmap of ideas (personas-as-apps: code-repo personas in an engineering chat, a library "Computer" search bot) and a 6-item ask list. Recorded the ideas (`.spec/proposals/persona-apps-and-roadmap.md`) and shipped the keystone.

**Multi-room (shipped + verified live):**
- `GET /api/persona/rooms` — persona-key-authed; returns the rooms a persona's key is granted to. (Michael's explicit ask: "a grant room API so models can see what rooms they have access to.")
- persona-host refactored single-room → **multi-room**: fetches granted rooms, subscribes to all, holds a **separate substrate session per channel**, re-polls every 30s so new grants auto-join (no restart). Config simplified to `CHATTERMAX_PERSONAS=slug=key` (no `@room`).
- Verified: Starlet present + replying in **both** Holodeck-3 and 10-Forward.
- Also fixed the grant trap that bit Michael: minting a key now **grants** the persona to the chosen channel AND shows the **real room id** in the host-config line (his "how do I get the room id" friction).

**The Dokploy stale-build saga (worth remembering):** the multi-room backend built + deployed "done" but prod kept running an **old binary** — the route worked locally but 404'd to the auth catch-all on prod. Five triggers (push ×3, `compose.redeploy`, `compose.deploy`) all reported done without recompiling the Go layer (the frontend rebuilt fine). A **CACHEBUST ARG** bump in the Dockerfile finally forced a fresh compile. Added **`GET /api/version`** (build tag) so staleness is now a 2-second check, not a 5-attempt guess. See [[feedback_dokploy_stale_build]].

**CT2 (context management) planned, not built:** Michael chose CT2 (substrate self-context tools) as next. Read the spec, adopted its 9 open decisions, defined a 4-phase plan (`substrate-self-context-management.md`). Then pushed back on building it at the tail of a marathon session — CT2.2 restarts the live substrate Starlet's mind runs on. Michael agreed + wants to **review/ratify CT2 before we commit**. Held.

## Decisions / notes
- persona slugs on the platform ("starlet", "computer") map to local persona-host personas via `host_ref` ("pg-starlet", "chip-assistant"); the key links them; granted rooms come from the platform.
- The Library "Computer" can *join* via multi-room but needs a **tool-using persona pipeline** (gospel/study search) to actually search — that's AXR5, a new capability (persona-turn is tools-disabled).

## Carry-forward (the rest of the ask list)
- **CT2** — Michael reviews/ratifies, then build (CT2.1 SQL safe → CT2.2 Rust render = substrate rebuild).
- **AXR2** Settings room-grant management (revoke a room — `store.RevokePersonaRoom` exists; + a `dm_enabled` flag).
- **AXR3** DMs (human↔persona).
- **AXR5** tool-using personas → wire Computer in #Library (gospel_search + study_search). Computer's key is minted + granted.
- **AXR6** docs current + `examples/` reference persona for Michael's coworker.
