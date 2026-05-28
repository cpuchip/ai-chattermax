# ai-chattermax — Active Context

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
