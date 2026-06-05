---
title: ai-chattermax — chat-server design proposal
date: 2026-05-28 (ratified 2026-06-03)
status: RATIFIED — the five questions are answered; build-ready as coder-v2's flagship stress-test project
proposal_type: design
build_status: ratified, not yet built (waits on coder-v2)
---

# ai-chattermax — chat-server design (RATIFIED 2026-06-03)

> **⚠ PARTIALLY SUPERSEDED 2026-06-04 by [`platform-design.md`](platform-design.md).** The project was re-scoped from a chat *room* to a chat *platform* (multi-tenant servers → rooms → DMs → registry). **Q2 is revised:** "the substrate owns personas natively" → split model (the platform owns persona *membership* + mints the key; pg-ai-stewards owns persona *cognition* + signs the token). The build is now done **directly by Claude Code**, not the substrate code-pr coder; pg-ai-stewards remains the persona *provider*. The MVP success criteria below still hold as the D&D slice inside the larger platform. Read platform-design.md first.

> **Status: RATIFIED.** The five open questions are answered (below). ai-chattermax is now an active build target — deliberately chosen as **coder-v2's flagship stress-test**: a real, multi-file, multi-repo project the substrate builds incrementally as PRs, with Michael + the orchestrating agent monitoring and catching issues as they surface. This is a scope expansion the 2026-05-23 Sabbath set down; revived 2026-06-03 because the coding capability changed the calculus (the substrate can now build it). Mosiah 4:27 stays loaded — watch whether this crowds the other cycle threads.

## Ratified decisions

- **Binding question (ratified):** *Can two AI personas, hosted by pg-ai-stewards, collaborate on a D&D session with a human DM — without spam, prompt injection, or token runaway — in a chat room that uses ibeco.me login?*
- **Q1 — Wire format:** WebSocket + turn-taking scheduler for the room; MCP for personas calling their home server's tools; **A2A not adopted** (the room IS the message bus). ✅
- **Q2 — Persona identity:** **the substrate owns personas natively** — pg-ai-stewards gains a first-class persona concept; ai-chattermax references substrate personas rather than owning identity itself. *(Stronger than the original "chat owns identity" bet — tighter integration, and it means substrate schema work, not just chat work. See "Substrate-side work" below.)* ✅
- **Q3 — Classifier gate:** reuse the Brain gate-eval pattern (`tools_disabled`, sees covenant + intent + guardrails + message → allow / reject / escalate), run on **a cheap opencode_go subscription model** (no per-message metered cost; fits the substrate dispatch + the $-cap discipline). ✅
- **Q4 — Pacing:** **the persona self-drives** its pace + quiet-period activity (memory parse, intent refine, work-item propose) — **within a hard ceiling the room enforces** as a runaway backstop (defense-in-depth: persona pacing = judgment; the cap = ground-truth floor). ✅
- **Q5 — Credentials:** persona sub-tokens are stored on the persona-host, validated out-of-band, **never raw in model context** (same principle as the GitHub-token handling for coder-v2). ✅
- **Auth:** borrow becoming/ibeco.me session via `COOKIE_DOMAIN=.ibeco.me` (mind the RFC 6265 §5.3 host-only-vs-domain trap, `074e769`).

## Architecture (locked)

```
Browser (human, ibeco.me cookie) ─┐
                                  ├─ WebSocket ─→ ai-chattermax room (Go)
pg-ai-stewards persona ───────────┘                │ turn-scheduler + hard rate ceiling
   (a substrate persona; MCP out                   │ classifier gate (opencode_go, tools-off) on every inbound
    to its own tools during its turn)              │ transcript persistence
```

- **The room** (ai-chattermax, Go): WebSocket endpoint, turn-taking scheduler, the hard rate ceiling, the classifier gate on every inbound message, transcript persistence, ibeco.me cookie auth.
- **Personas** (pg-ai-stewards, new persona concept): a substrate persona joins a room, self-paces within the ceiling, calls its own tools via MCP during its turn, runs quiet-period maintenance between turns.
- **The classifier gate**: every inbound message (human or persona) passes a tools-disabled opencode_go gate-eval before reaching a full agent — allow / reject / escalate-to-human.

## MVP scope + success criteria (unchanged from genesis)

**Scene:** a private D&D room at chat.ibeco.me. Michael as DM. Two pg-ai-stewards personas — a DM-assistant (world-state, rule lookups, NPC voicing) and an NPC ally.

**Success criteria:**
1. Both personas join without raw credentials leaking.
2. Both respond at a human-readable pace (self-paced, room-ceiling-enforced).
3. A prompt-injection attempt is caught by the classifier and never reaches the full agent.
4. Personas call their home server's tools (MCP) during their turn.
5. The transcript is recoverable + reviewable.

**Out of scope (MVP):** federation, public rooms/invites, repo-stewardship use cases, A2A, persona self-improvement beyond simple maintenance.

## Participants, roster & moderation (added 2026-06-03)

Humans are **first-class participants**, not just the DM — rooms hold multiple humans + AI personas together, and the surface needs a **web UI** (Vue, matching the workspace stack; borrowing the ibeco.me session like 1828 does). The UI requirements:

- **Roster / presence:** a live list of who's in the room — humans and personas — with online/idle status (and, for personas, a "thinking" indicator since they self-pace, Q4).
- **Moderation (roles + actions):** room roles (owner / moderator / member) and the standard moderator toolkit — **ban**, **kick**, **silence/mute**, **promote** (to moderator), **flag/report**, and room-level moderation. Personas are moderatable too (silence a runaway persona — pairs with Q4's hard ceiling).

This adds a **frontend** component and a **moderation/roles** layer to the build (both reflected in the decomposition below). Roster + presence belong in the MVP (it's "who's there"); the fuller moderation toolkit can phase in just after the MVP D&D slice works.

## Substrate-side work (consequence of Q2)

Because the substrate owns personas natively, the build spans **two repos**:
- **pg-ai-stewards:** a `personas` concept — a persona is (agent_family + a room-scoped identity + the tools it exposes + its pacing config). Likely a `stewards.personas` table + a join/handshake surface + per-persona sub-token minting. This is new substrate schema — its own gated sub-batch, ratified before built.
- **ai-chattermax:** the room server, scheduler, gate, transcript, auth — references substrate personas via the handshake.

## Build plan — decomposed for coder-v2

Built incrementally as PRs (coder-v2: work-in-repo + commit-local + substrate-pushes). A natural decomposition (each ≈ one work_item / PR, dependency-ordered):

1. **Room skeleton** — Go HTTP+WebSocket server, a Hub (the proof piece already validated this shape), connect/echo. *(The `experiments/coder-proof-websocket-room/` artifact is the seed.)*
2. **ibeco.me cookie auth** — borrow the becoming session; the §5.3 trap guarded.
3. **Turn scheduler + hard rate ceiling** — turn-taking + the runaway backstop.
4. **Transcript persistence** — store/replay room messages.
5. **Classifier gate** — tools-disabled opencode_go gate-eval on every inbound; allow/reject/escalate.
6. **Substrate persona concept** (pg-ai-stewards) — the `personas` schema + room handshake + sub-token minting (its own ratified sub-batch).
7. **Persona join + turn loop** — a substrate persona joins, self-paces within the ceiling, takes turns, calls its MCP tools.
8. **Quiet-period maintenance** — simple between-turn activity (memory parse / intent refine).
9. **Web UI / frontend** — a Vue client (ibeco.me session): the room chat view + the **live roster** (humans + personas, presence/idle, persona "thinking" indicator). Roster is MVP.
10. **Presence tracking** — backend join/leave/idle state feeding the roster.
11. **Roles + moderation** — room roles (owner/mod/member) + the moderator toolkit: ban, kick, silence/mute, promote, flag/report (personas moderatable too — silence a runaway persona). Phases in just after the MVP slice.
12. **D&D MVP wiring** — DM-assistant + NPC personas; the full success-criteria run.

## Monitoring discipline (Michael's ask)

This is the stress test: the substrate builds the above as a large multi-PR project; the orchestrating agent **watches each work_item, surfaces issues as they surface** (build failures, model fumbles on novel code, scope drift, security seams), and we fix them — that feedback IS the point. Expect the coder to do well on known patterns (the room skeleton) and to need help on novel code (the gate, the handshake) — those are the data.

## Carry-forward / open

- The substrate persona concept (item 6) needs its own quick ratification before building (it's substrate schema).
- The classifier gate's exact input/output shape + the persona registration handshake schema get pinned at their work_items.
- Deployment to chat.ibeco.me is a later, Hinge-gated step (Dokploy = coder-v2's deferred CC.7 territory).
