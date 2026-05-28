# ai-chattermax — Claude Code project context

ai-chattermax is a hostable chat room for humans and AI agents. Servers register, mint per-persona sub-tokens from one parent API key, and their personas join chat rooms alongside humans. Born 2026-05-28 from the chat-with-repos seed first set down at the 2026-05-23 Sabbath and brought back into design four days later.

This is its **own git repo** (`github.com/cpuchip/ai-chattermax`, public, MIT-licensed), nested in the `scripture-study` workspace at `projects/ai-chattermax/`.

## Identity

A multi-party chat surface where AI personas (hosted by repo-stewarding servers like pg-ai-stewards, 1828, becoming, scripture-book) and humans converse, collaborate on real work, and play. **Not an LLM chatbot host** — the personas are agents with their own memory, tools, and stewardship; the chat surface is the conversation layer they share with us.

First target deployment: `chat.ibeco.me`, borrowing the becoming/ibeco.me login session (the same cross-domain cookie path 1828.ibeco.me uses). Long-term home: `chat.cpuchip.net` when that domain has auth.

## Stack & shape (provisional, not yet ratified)

- **Backend:** Go (per the Go-flavored `.gitignore` Michael placed at genesis; matches the workspace's other servers — becoming, stewards-ui, gospel-engine-v2).
- **Auth:** borrow `becoming` session cookie via `COOKIE_DOMAIN=.ibeco.me` (mind the RFC 6265 §5.3 host-only-vs-domain-scoped trap that bit ibeco.me on 2026-05-27, commit `074e769`).
- **Protocol:** chat protocol (WebSocket + turn-taking scheduler) is the working assumption. MCP is what personas call OUT to (their own server's tools). A2A is explicitly NOT adopted yet — not until the chat protocol's shape is proven on a real session.
- **First MVP:** D&D — 1 human DM, 1 AI DM-assistant persona, 1 AI NPC persona. Low stakes, natural multi-agent dynamics, no production consequences.

These are the **opening positions**, not ratified decisions. The first design proposal (see below) walks them as questions.

## Where things are

| Need | Path |
|------|------|
| **Current state / in flight** | `.mind/active.md` |
| **First design proposal** (in progress) | `.spec/proposals/chat-server-design.md` |
| **Session journals** | `.spec/journal/` |
| **Workspace context** (parent) | `../../CLAUDE.md` + `../../.github/copilot-instructions.md` |
| **Sabbath where the seed was set down** | `../../.spec/sabbath/2026-05-23-the-arc-that-said-yes-to-everything.md` |

## Related work (in the parent `scripture-study` workspace)

- **pg-ai-stewards** — `../pg-ai-stewards/`. The first persona-host we plan to wire in. Hosts pipelines today; will expose 2 personas (working names: research, science-researcher). The persona/server boundary is one of the design proposal's open questions — does the substrate gain a persona concept, or does ai-chattermax own persona identity and just call substrate tools?
- **becoming / ibeco.me** — `../../scripts/becoming/`. Provides the login session we plan to borrow. The cross-domain cookie work is already done for 1828.ibeco.me.
- **1828-illuminated** — `../1828-illuminated/`. Reference for "subproject that borrows ibeco.me auth via `COOKIE_DOMAIN=.ibeco.me`."
- **webster-v2 MCP** (Sabbath thread 3) — not yet built. If chat-persona-exposure can reuse webster-v2's MCP server pattern, both threads benefit. Worth knowing before either is designed in isolation.
- **Auto-memory** — `~/.claude/projects/<workspace>/memory/` — see `project_ai_chattermax.md`, `feedback_ai_chattermax_stewardship.md`.

## Cycle context (where this fits in the workspace)

The 2026-05-23 Sabbath ratified **three threads** for the current cycle: substrate Council ②, teaching Episode 2, and 1828 finish (incl. webster-v2 MCP). The chat-with-repos prototype was explicitly held as a seed only, NOT one of the three threads. Mosiah 4:27 is loaded as evidence-test.

This project's existence is design work, not a ratified fourth thread. At the next Sabbath the question becomes: does ai-chattermax join the cycle, displace one of the three, or stay design-only for another week. Until then: proposals and journaling only. Build only after Sabbath ratification.

## Stewardship & working-session protocol

This directory is its own git repo, and the agent has **stewardship over it** — Michael owns the intent and vision, the agent owns the code within that intent (per the workspace covenant, `agent_commits_to.exercise_stewardship`). Granted explicitly 2026-05-28, parallel to the marsfield.org grant.

At the **end of every working session** on ai-chattermax:

1. **Journal** — write a session entry to `.spec/journal/` (`YYYY-MM-DD-short-title.md`, frontmatter + what happened + decisions + carry-forward; match the existing entries).
2. **Update `.mind/active.md`** — current state, in flight, next, date banner.
3. **Update proposals** — mark decisions ratified, surface new questions, move items to "shipped" as they land.
4. **If the session's work is complete** — `git commit` with a clear message, then `git push`. Normal pushes only, never force. Don't commit/push mid-task or with the build broken.

When in doubt about whether a change is a stewardship action or a scope-change that needs ratification: apply the boundary test from the covenant — "would Michael, if asked in advance, say 'yes, obviously do that'?" If yes, do it. If unsure, ask.

## Open questions (the proposal needs to answer these)

1. **A2A vs MCP vs chat-protocol** — what's the actual wire format? Working bet: chat-protocol (WS + turn scheduler) is the room; MCP is the per-server tool surface; A2A stays unbuilt until we need it.
2. **Persona vs server identity** — does pg-ai-stewards gain a persona concept, or does ai-chattermax own persona identity and call substrate tools? Working bet: chat owns persona identity.
3. **Binding question for MVP** — *can two AI personas, hosted by different MCP servers, collaborate on a D&D session with a human DM, without spam or prompt injection or token runaway, in a chat room that uses ibeco.me login?*
4. **Prompt-filter classifier** — what runs the gate? Working bet: same "tools_disabled=true" pattern that cut Brain gate-eval cost 7× (Phase B lesson, 2026-05-11). Cuts cost AND attack surface.
5. **Rate cap + quiet periods** — persona pacing so the room doesn't run away from humans. Quiet periods become "what does the persona do between turns" — memory parse, intent refine, work-item propose. The substrate's Sabbath/Atonement/Consecration primitives map here.

## Don't do

- Don't start building before the design proposal is ratified. The Sabbath set this down for a reason; the design pass is to test whether bringing it back is right.
- Don't adopt A2A because the name fits. It's a spec to satisfy; adopt only when something concrete demands it.
- Don't store persona credentials the way the 2026-05-27 Dokploy secret-leak showed us not to — bridge for inspection without putting raw values in model context.
- Don't commit/push with the build broken. Pushing main is a deploy.
