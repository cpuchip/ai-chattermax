# Platform build — progress (resumable)

> Live build log for the platform-design.md Stage 1 (and beyond). Updated as each
> sub-step lands so a context-compaction can resume without re-deriving. Branch:
> `platform-stage-1`. Builder: Claude Code (overnight stewardship, 2026-06-04).

## Operating rules (overnight)
- Gated commits, tests at each gate, never yield with a broken build.
- Local `docker compose` e2e MUST be green before any deploy. Deploy to chat.ibeco.me
  only if confident it won't break the live site; else leave for Michael's morning Hinge.
- Sensible defaults for every open question — recorded in "Decisions made" below.
- Only substrate touch: persona-host turn loop → gateway+key. Nothing destructive.

## Decisions made (defaults chosen while Michael sleeps — review these)
- (fill in as I go)

## Stage 1 checklist
- [ ] S1.1 schema + migrations (Postgres 18, FTS)
- [ ] S1.2 go.mod deps + config + db (pgxpool + migration runner)
- [ ] S1.3 store: repositories (users/servers/rooms/members/personas/keys/messages)
- [ ] S1.4 auth: Authenticator (dev + ibeco), sessions, middleware, /api/me, login/logout
- [ ] S1.5 REST API: servers/rooms/members/personas/history/search
- [ ] S1.6 gateway WS: hub + envelope + subscribe/message/history/presence, persisted
- [ ] S1.7 persona key validation on gateway connect
- [ ] S1.8 seed: 1 server + D&D rooms + 2 personas + a dev key
- [ ] S1.9 frontend: shell (rail/channels/room/roster) + router + login + gateway client
- [ ] S1.10 frontend: My Personas (mint key) basic
- [ ] S1.11 persona-host adaptation (turn loop → gateway + key)
- [ ] S1.12 local docker-compose e2e green (login → see rooms → post → persona replies)
- [ ] S1.13 deploy decision + (if green) deploy + verify, else leave for Hinge
- [ ] S1.14 memory + morning report

## Log
- 2026-06-04: spec ratified; branch `platform-stage-1`; auth resolved (becoming `GET /api/me`,
  cookie `becoming_session`, opaque server-side session → ai-chattermax calls ibeco.me/api/me
  at login then runs its own session). Starting S1.1.
