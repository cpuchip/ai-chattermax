---
title: ai-chattermax — chat-server design proposal
date: 2026-05-28
status: STUB — open questions named, not yet answered or ratified
proposal_type: design
build_status: design-only, no code
---

# ai-chattermax — chat-server design proposal

> **Status: STUB.** This proposal names the five open questions raised at the 2026-05-28 genesis session. The next pass walks them with the user via `AskUserQuestion` batches (substrate C–F cadence). No code until ratified. No ratification until the next Sabbath or until Michael explicitly decides this displaces one of the existing three cycle threads.

## Provenance

The chat-with-repos idea was first surfaced at the 2026-05-23 Sabbath, where Michael explicitly set it down as work-scope-adjacent, not workspace ambition. Four days later (2026-05-28) he revived it after a coworker conversation, created `projects/ai-chattermax/`, and granted stewardship. Genesis session journal: `.spec/journal/2026-05-28-genesis-and-design-session.md`.

## Working binding question (needs ratification)

*Can two AI personas, hosted by different MCP servers, collaborate on a D&D session with a human DM, without spam, prompt injection, or token runaway, in a chat room that uses ibeco.me login?*

That's checkable. Three months from now we'd know if we got there.

## Working architecture (opening positions, not ratified)

- **Backend:** Go.
- **Auth:** borrow becoming/ibeco.me session cookie via `COOKIE_DOMAIN=.ibeco.me`. Mind the RFC 6265 §5.3 trap (074e769).
- **Wire format for the room:** WebSocket + turn-taking scheduler.
- **Wire format for personas → their home server:** MCP (the persona calls out for tools).
- **A2A protocol:** explicitly NOT adopted yet. Wait for a concrete demand.
- **Persona identity:** owned by ai-chattermax; persona-host servers expose tool endpoints, not persona identities.
- **MVP scope:** D&D — 1 human DM + 1 AI DM-assistant + 1 AI NPC, all hosted by pg-ai-stewards.

## The five open questions (need answers before code)

### Q1. A2A vs MCP vs chat-protocol — what's the actual wire format?

**Working bet:**
- Chat room itself: WebSocket + turn-taking scheduler.
- Persona → its home server: MCP (existing pattern).
- A2A: not adopted.

**What to test:** is there a real reason to adopt A2A now, or does the chat protocol cover all the agent-to-agent messaging the MVP needs (because they're all in the same room, the room IS the message bus)?

### Q2. Persona vs server identity — where does persona identity live?

**Working bet:** ai-chattermax owns persona identity. Persona-host servers (pg-ai-stewards, becoming, 1828, scripture-book) expose tool endpoints the personas call. Keeps the substrate clean; lets new persona-hosts plug in without refactoring.

**What to test:** can pg-ai-stewards expose its "research" and "science-researcher" personas via this boundary without growing a new substrate concept? Or does the substrate need to know about personas natively?

### Q3. Prompt-filter classifier — implementation pattern

**Working bet:** reuse the Brain gate-eval pattern. `tools_disabled=true` on the payload (Phase B lesson 2026-05-11 — cut cost 7× there; here it cuts attack surface). Classifier sees: covenant + intent + guardrails + the incoming message. Returns: allow / reject / escalate-to-human. No tool calling at this layer.

**What to test:** what model? haiku ($0.005/eval, hard-pinned in Brain) or something even cheaper? What's the cost-per-message budget?

### Q4. Rate cap + quiet periods — pacing + what fills the quiet

**Working bet:** persona response cap (max N msgs per minute, with backoff). Between turns, persona runs maintenance — memory parse, intent refine, work-item propose. The substrate's Sabbath/Atonement/Consecration primitives map directly here.

**What to test:** are the quiet-period activities driven by the persona itself, by the host server, or by a chat-side scheduler? How does the human know the persona is "thinking" vs offline?

### Q5. Credential handling — persona sub-token storage

**Working bet:** persona sub-tokens never enter model context as raw values. Stored on the persona-host server. The chat server validates them out-of-band. Same shape of failure mode as the 2026-05-27 Dokploy secret-leak — design the safe path from day one.

**What to test:** is there an MCP equivalent of "credential reference" that works across server boundaries, or does ai-chattermax need its own auth dance?

## MVP definition (refine before building)

**Scene:** a private D&D chat room at chat.ibeco.me. Michael as DM. Two pg-ai-stewards personas: one acting as DM-assistant (world-state, rule lookups, NPC voicing), one as an NPC ally character.

**Success criteria:**
- Both personas join the room without raw credentials leaking.
- Both respond at a human-readable pace (response cap + quiet periods enforced).
- A prompt-injection attempt against either persona is caught by the classifier and doesn't reach the full agent.
- Personas can call their home server's tools (MCP) during their turn.
- Session transcript is recoverable and reviewable.

**Out of scope for MVP:**
- Multiple chat servers / federation.
- Public rooms / invite process.
- Repo-stewardship use cases (the endgame, deliberately deferred).
- A2A protocol.
- Persona self-improvement during quiet periods (start with simple maintenance only).

## Cycle context

Three cycle threads are active per the 2026-05-23 Sabbath ratification: substrate Council ②, teaching Episode 2, 1828 finish (incl. webster-v2 MCP). ai-chattermax is NOT one of them. At the next Sabbath, decision is one of:

- (a) **Join the cycle as a fourth thread.** Mosiah 4:27 evidence: are the existing three still on pace? If not, this is the second confirmation of the "say yes to everything" pattern.
- (b) **Displace one of the three.** Most likely candidate: 1828 finish if webster-v2 MCP pattern can be designed jointly with chat-persona-exposure.
- (c) **Stay design-only another week.** Refine the proposal further, then re-evaluate.

## Carry-forward (what the next pass does)

1. Walk the five open questions with the user via `AskUserQuestion` (one batch per question or grouped per pair of related questions — substrate C-F cadence).
2. Coordinate with webster-v2 thread before designing the MCP exposure pattern.
3. Sketch the WebSocket message schema for D&D MVP (who can post, when, what fields).
4. Define the persona registration handshake (server presents parent key, ai-chattermax issues persona sub-tokens scoped to a room).
5. Define the classifier gate's exact input/output shape.
