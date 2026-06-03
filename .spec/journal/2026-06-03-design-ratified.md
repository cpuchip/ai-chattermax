---
date: 2026-06-03
title: Design ratified — five questions answered, chosen as coder-v2's flagship build
---

# Design ratified (2026-06-03)

Six days after genesis, and after the pg-ai-stewards substrate gained a working **coding capability** (it built FizzBuzz, then a real WebSocket chat-room server, autonomously — see this repo's `experiments/coder-proof-websocket-room/`), Michael brought ai-chattermax back and ratified the design. The coding capability changed the calculus: part of why this was set down at the 2026-05-23 Sabbath was that building it by hand was a lot — now the substrate can build it.

## The five questions, answered

Walked each via AskUserQuestion:

- **Q1 — Wire format:** WebSocket + turn-taking scheduler for the room; MCP for personas calling their home server's tools; A2A NOT adopted (the room is the message bus). *(working bet ratified)*
- **Q2 — Persona identity:** **the substrate owns personas natively.** pg-ai-stewards gains a first-class persona concept; ai-chattermax references substrate personas. Stronger than the original "chat owns identity" bet — tighter integration, and it means the build spans two repos (substrate schema + the chat server).
- **Q3 — Classifier gate:** Brain's tools-disabled gate-eval pattern, run on a **cheap opencode_go subscription model** (no per-message metered cost; fits the substrate dispatch + $-cap discipline). Not haiku.
- **Q4 — Pacing:** **the persona self-drives** its pace + quiet-period work — **within a hard ceiling the room enforces.** The orchestrating agent flagged that pure persona-self-pacing has the weakest runaway guarantee and the MVP criterion is "human-readable pace," so the room keeps a hard backstop (persona pacing = judgment, the cap = ground-truth floor).
- **Q5 — Credentials:** persona sub-tokens stored on the host, validated out-of-band, never raw in model context — same principle as coder-v2's GitHub token.

## Scope decision

**Ratified + build** — deliberately, as the stress test for the substrate's new coding ability. The substrate builds ai-chattermax incrementally as a **large multi-PR project**, with Michael + the orchestrating agent **monitoring and raising issues** as they surface (build failures, model fumbles on novel code, security seams). That feedback is the point. Mosiah 4:27 stays loaded — watch the other cycle threads.

## How it gets built

Via **coder-v2** (ratified same day in pg-ai-stewards): the coder works inside this repo and lands work as reviewable PRs — clone (substrate, fine-grained PAT scoped to allow-listed repos), agent edits/builds/tests + commits locally in the sandbox (token never in the sandbox), substrate pushes the branch + opens the PR, Michael reviews + merges (the Hinge). The 9-item build plan in `chat-server-design.md` is the decomposition.

## Carry-forward

- The **substrate persona concept** (build-plan item 6) needs its own quick ratification before building — it's substrate schema (a `stewards.personas` table + room handshake + sub-token minting).
- Build waits on **coder-v2 being built** in pg-ai-stewards.
- The room skeleton (item 1) has a working seed already: `experiments/coder-proof-websocket-room/`.
- Deployment to chat.ibeco.me is later + Hinge-gated (coder-v2's deferred Dokploy territory).
