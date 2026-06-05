---
date: 2026-06-05
title: "Platform Stage 1 — room → multi-tenant chat platform (overnight build)"
---

## What happened

Michael ratified the platform reframe and handed me an Ammon night: "build this all out while I sleep… full stewardship." So ai-chattermax went from a single in-memory room to a **multi-tenant chat platform** — servers → rooms → personas → registry — on Postgres 18, built directly (not via the substrate coder; pg-ai-stewards stays the persona *mind*).

Stage 1, all on branch `platform-stage-1`, gated commits:
- **Schema + migrations** (Postgres 18, FTS via generated tsvector + GIN; pgcrypto for join tokens).
- **db + config**: pgxpool with boot retry, embedded idempotent migration runner.
- **store**: users/sessions, servers/members, rooms, personas + key mint/validate (the split model), messages with resolved sender + FTS.
- **auth**: an `Authenticator` interface — `dev` name-login (local) + `ibeco` (borrows the becoming session via `GET /api/me`, then runs the platform's own `chattermax_session`; RFC 6265 §5.3 eviction).
- **REST**: servers/rooms/personas/registry/messages/search + persona key mint + room grants.
- **gateway**: one multiplexed `/gateway` WS — typed envelope, dual auth (cookie→human, `?key=`→persona), per-channel routing, broadcast-except-sender, history-on-join, presence, ping/pong. Generalizes AX3-2.
- **Vue shell**: Discord/Slack three-pane (server rail, channels + personas, attributed transcript with ◆ agent badges, roster) + login + create server/room + mint-key + grant.
- **seed**: "Tavern Keep" demo (D&D rooms + dm-assistant/npc-ally); dev keys only in dev mode.
- **persona-host adaptation** (in pg-ai-stewards): a gateway client that dials `/gateway?key=`, humans-only via the envelope's `senderKind`, cognition unchanged.
- **tests**: gateway/auth/httpapi unit + a DB-gated store integration test; all green, vet clean.

## How it was verified

Layer by layer against a real Postgres 18: boot/migrate/seed; login→me→servers→rooms→registry; the gateway (persona-key + cookie auth, persistence, history replay, presence). Then the **full e2e**: a human posts → DM Assistant replies in character via the substrate, attributed as a persona. Then the **browser** (login → shell → transcript with agent badges). Then the **production Docker image** in `AUTH_MODE=ibeco` config: builds, boots, serves the SPA, `/api/config`=ibeco, cookie-less login → clean 401, no dev keys.

## The deploy decision (held for the Hinge)

I did **not** auto-deploy. The one thing I can't verify alone is a real ibeco.me login (needs Michael's `becoming_session` + the `.ibeco.me` cross-domain cookie). I won't replace the live site onto an unverified auth path — if it failed, chat.ibeco.me would be a login wall. So: **PR #21** is the deploy Hinge. Merge to `main` = Dokploy auto-deploy, after Michael confirms his login. Everything else is proven.

## Carry-forward

- **Deploy** = merge PR #21 (verify ibeco login first; compose adds a self-contained postgres:18 service).
- **Stage 2**: My Personas polish, create-room UX (currently `prompt()`), FTS search UI, room membership.
- **Stage 3**: multi-server join-link UX + the registry view surfacing.
- **Stage 4**: DMs (incl. DM-your-persona — the CT2 surface).
- **Stage 5**: sub-personas, multi-room grants, full moderation.
- **Restore the per-message rate ceiling** in the gateway (the runaway backstop the old scheduler gave; dropped in the rewrite — personas bounded by substrate cost caps + SILENCE for now).
- Decide whether `frontend/dist` should stay tracked (Docker rebuilds it; could gitignore).
