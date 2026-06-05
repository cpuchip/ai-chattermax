# ai-chattermax — Active Context

> **2026-06-04 (#7 v1 LIVE) — A PERSONA TALKS IN THE ROOM.** The turn loop works end-to-end: a human posts, `DM Assistant` reads it and replies in character as a roster participant (`Kind=persona`). Cognition is real substrate dispatch in pg-ai-stewards — persona-host drives a `persona-turn` pipeline per turn straight from its pgxpool (spawn at turn-zero → consult each later turn; the **path-check resolved: pure SQL, no MCP plumbing**). Built: `extension/r7-persona-turn.sql` (persona agent + persona-turn pipeline + auto-verify) + `cmd/persona-host/{dispatch,turnloop,autojoin}.go`. ai-chattermax's part = honor `?kind=` on WS connect (persona vs Human; `5601164` → auto-deploy). **e2e proven** against the live substrate (~$0.014/turn); the SILENCE gate (model judges whether to speak) + humans-only filter + @mention hint all work. **Replay-on-join (AX3-2) saved the day** — the human msg landed before the persona finished connecting (startup race → reconnect), the room replayed it, the persona answered. v1 scope (ratified): triggers #1 Reactive + #2 Addressed, humans-only. **v2 (deferred):** the aliveness layer (#3 pacing/shy-vs-quick, #4 ambient cron, #5 pipeline hooks, persona↔persona arbitration) + per-persona system-prompt/model_override + UI agent badges + JWT-verify on connect. **NEXT:** #11 moderation, #12 D&D MVP (2 personas + human DM). Substrate journal: `../pg-ai-stewards/.spec/journal/2026-06-04-persona-turn-loop-r7.md`.

> **2026-06-04 — BACKEND + UI LIVE; persona-host (#6) built; AX3-2 attributed protocol SHIPPED.** Big movement since the design ratified. **AX1:** the substrate's coder built the backend core autonomously (Ammon night) on `agent/night-build` → Michael's Hinge merge to main. **#9 Vue UI:** built by the coder (PR #20), full-context-verified + merged → **`chat.ibeco.me` serves the SPA**, auto-deploy on push proven. **#6 substrate persona concept:** built by me as the **`cmd/persona-host` Go SIDECAR** in pg-ai-stewards (NOT the extension) — `persona_host` schema, ed25519/EdDSA JWT mint, `/pubkey` + `/personas` + `/join` handshake, `dm-assistant`+`npc-ally` seeded, live-verified + log-leak clean. **AX3-2 (this session):** fixed the WS protocol — messages now broadcast as a JSON envelope `{sender, body, ts}` to everyone EXCEPT the sender (no more `sender:'unknown'` + self-echo dup), and the room **replays history on join**. Server-side only; frontend already fit. Live two-client WS integration test proves it; shipped `83490bd` → live. **NEXT — #7 the persona turn loop:** persona-host connects to a room as a WS client, holds a **persistent substrate session per (persona, room)** (spawn at join, `consult_subagent` each turn — context accumulates), and posts its turns. **Turn model is an 'aliveness' layer, not just reactive** (Michael's call, no answers yet → needs its own design pass): reactive-with-judgment + @addressing (room/personal/group) + delayed/timed/'thinking-out-loud' cadence + pipeline-state hooks + pileup-avoidance with multiple agents. Then #11 moderation, #12 D&D MVP. Open: presence should tag personas as agents (not Human); token-verify on connect still deferred (hardening).

> **2026-06-03 — DESIGN RATIFIED + chosen as coder-v2's flagship build.** The five questions are answered (see `.spec/proposals/chat-server-design.md`): Q1 WebSocket + turn-scheduler room, MCP for persona tools, A2A deferred · **Q2 the SUBSTRATE owns personas natively** (pg-ai-stewards gains a first-class persona concept — chat references it; bigger commit than the original bet, = substrate schema work too) · Q3 classifier gate on a cheap opencode_go model (tools-off, no per-msg $) · **Q4 persona self-paces *within a hard room-enforced ceiling*** (the runaway backstop — flagged + added to the bet) · Q5 sub-tokens never in raw model context (same principle as coder-v2's GitHub token). Scope decision: **ratified + build** — it's the deliberate stress test for the substrate's new coding ability, built incrementally as a large multi-PR project with the orchestrating agent monitoring + raising issues. Built via **coder-v2** (`../pg-ai-stewards/.spec/proposals/substrate-coding-capability-v2.md`: work-in-repo + commit-local + substrate-pushes-PRs, fine-grained PAT scoped to allow-listed repos). The `experiments/coder-proof-websocket-room/` artifact (substrate-built, working) is the seed for build-plan item 1 (the room skeleton). **Mosiah 4:27 still loaded** — watch whether this crowds the other cycle threads. **NOT yet built** — waits on coder-v2 being built + the substrate-persona sub-batch ratified.

> **2026-05-28 — Genesis.** Michael revived the chat-with-repos seed (set down at the 2026-05-23 Sabbath as work-scope-adjacent, not workspace ambition) and created `projects/ai-chattermax/` with `LICENSE` (MIT), `.gitignore` (Go-flavored), `README.md` ("a hostable chat room for humans and AI."), and pushed an initial commit to `github.com/cpuchip/ai-chattermax`. Today's design session captured: A2A vs MCP vs chat-protocol distinction, persona-vs-server-identity question, MVP binding question (D&D with 2 personas + human DM), classifier-as-prompt-gate pattern (reuses Brain's tools_disabled=true gate-eval discipline), rate-cap-with-quiet-periods architecture. Agent granted commit+push stewardship parallel to marsfield.org. Workspace `.mind/active.md` updated; auto-memory entries written. **This is design work, NOT yet a ratified fourth thread.** Mosiah 4:27 evidence-test still loaded — at next Sabbath, decide whether ai-chattermax joins the cycle, displaces one of the existing three threads (substrate Council ②, teaching Episode 2, 1828 finish), or stays design-only another week.

---

## Priorities

1. ★ **Write the design proposal.** `.spec/proposals/chat-server-design.md` — stub created; needs the five open-question answers and the MVP slice scoped tight. Working binding question: *can two AI personas, hosted by different MCP servers, collaborate on a D&D session with a human DM, without spam or prompt injection or token runaway, in a chat room that uses ibeco.me login?*
2. **Decide persona/server boundary.** Working bet: ai-chattermax owns persona identity; persona-host servers (pg-ai-stewards, etc.) expose tool endpoints the personas call. Test this against the substrate's existing pipeline shape before ratifying.
3. **Decide protocol.** Working bet: WebSocket + turn-taking scheduler for the room; MCP for personas calling out to their home server; A2A stays unbuilt until something demands it.
4. **Coordinate with webster-v2 MCP work** (Sabbath thread 3). If chat-persona-exposure reuses webster-v2's server pattern, both threads benefit. Don't design either in isolation.

## In Flight

| Item | Status | Ref |
|------|--------|-----|
| Project scaffold (CLAUDE.md, .mind/, .spec/, journal) | ✅ shipped 2026-05-28 | this file + repo root |
| Design proposal — chat-server-design.md | 🔨 stub created, five open questions named | `.spec/proposals/chat-server-design.md` |
| Genesis session journal | ✅ written 2026-05-28 | `.spec/journal/2026-05-28-genesis-and-design-session.md` |

## Deferred / Paused

- **Code.** Nothing built until design proposal ratified AND next-Sabbath cycle decision lands.
- **A2A protocol adoption.** Stays unbuilt until something concrete demands it.
- **Deployment to chat.ibeco.me / chat.cpuchip.net.** Same — needs design first, then ratification, then build, then deploy.

## Key Facts

- Repo: `github.com/cpuchip/ai-chattermax`
- License: MIT
- Stack (provisional): Go backend
- First deploy target: `chat.ibeco.me` (borrowing becoming/ibeco.me login session via `COOKIE_DOMAIN=.ibeco.me`)
- Long-term home: `chat.cpuchip.net` (when that domain has auth)
- First persona-host: pg-ai-stewards (2 working personas — research, science-researcher)
- MVP scope: D&D with 1 human DM, 1 AI DM-assistant persona, 1 AI NPC persona

## Cross-Domain Auth Gotcha (read before wiring auth)

The 2026-05-27 ibeco.me login loop (`074e769`) hit RFC 6265 §5.3 — a host-only cookie and a Domain-scoped cookie with the same name are TWO different cookies. Existing users with pre-COOKIE_DOMAIN cookies got trapped. Fix included a host-only eviction emission whenever `CookieDomain` is set. If chat.ibeco.me borrows the becoming session, it inherits that same scope and the same trap. Verify against the four-fix sweep before assuming auth "just works."

Workspace journal: `../../.spec/journal/2026-05-27-ibeco-login-loop-and-secret-leak.md`.

## Secret Handling (read before any persona credential storage)

Same 2026-05-27 session — Dokploy `application.one` query exposed `POSTGRES_PASSWORD` into model context despite the skill's WARNING. Reactive filtering proved insufficient. ai-chattermax's persona sub-tokens are EXACTLY the same shape of failure mode — credentials minted by a server, scoped to a persona, must never enter model context as raw values. Design the credential-handling path with this in mind from day one.
