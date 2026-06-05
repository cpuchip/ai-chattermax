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

## STAGE 1 COMPLETE — 2026-06-05 (built overnight, deploy = Michael's Hinge)

All S1.1–S1.14 done except the production deploy, which is **PR #21** (merge to
`main` = Dokploy auto-deploy). The platform is built, tested, e2e-proven, and the
prod Docker image is boot-verified in ibeco config. The one thing I could not
verify alone — a real ibeco.me login (needs Michael's `becoming_session` +
cross-domain cookie) — is the PR's single pre-merge check.

## Decisions made (defaults chosen while Michael slept — review these)
- **Postgres 18 + pgcrypto** (join tokens). FTS via generated tsvector + GIN.
- **Auth**: platform runs its OWN `chattermax_session` after a `dev` name-login or
  the `ibeco` `GET /api/me` handshake. RFC 6265 §5.3 host-only eviction included.
- **Split persona model**: platform owns membership + mints the key; pg-ai-stewards
  owns the mind. Persona-host adapted to the gateway (humans-only via senderKind).
- **Gateway**: broadcast-except-sender (AX3-2 carried), history-on-join, ping/pong
  keepalive, drop-on-full-buffer for slow clients.
- **Onboarding (v1)**: every login auto-joins the demo "Tavern Keep" server.
- **Dev keys**: seeded ONLY in dev mode → prod ships no known credential.
- **Deploy = Hinge**: did NOT auto-deploy; can't verify ibeco login unattended,
  won't swap the live site onto an unverified auth path. PR #21.
- **DEFERRED (flagged)**: per-message rate ceiling (runaway backstop) not yet
  re-wired into the gateway; DMs/sub-personas/multi-server-join-UI = later stages.

## Gotchas hit (for next time)
- Windows: `pkill -f` does NOT kill the .exe; use `taskkill //F //IM name.exe`.
  (Cost me a confused "stale embed" detour — it was an old process holding the port.)
- `gen_random_bytes` needs `CREATE EXTENSION pgcrypto`; `gen_random_uuid` is core.
- pgx uuid columns reject `''` — never pass an empty string for a uuid filter.
- Transient empty kimi completion surfaced as a turn error (handled); the model
  path is healthy (verified via a direct substrate probe).

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
