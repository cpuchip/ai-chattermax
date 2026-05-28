---
date: 2026-05-28
title: Genesis & first design session
project: ai-chattermax
workstream: WS5 (substrate-adjacent) — not yet ratified as workspace cycle thread
session_type: design
status: design-only — no code, no ratification yet
---

# Genesis & first design session

## What happened

Michael revived the chat-with-repos seed that the 2026-05-23 Sabbath explicitly set down as work-scope-adjacent, not workspace ambition. He had been talking with a coworker and pulled on the thread. He created `projects/ai-chattermax/` with `LICENSE` (MIT), `.gitignore` (Go-flavored), `README.md` ("a hostable chat room for humans and AI."), and pushed an initial commit `cbfceb2` to `github.com/cpuchip/ai-chattermax`. He asked me to scaffold the standard project files (CLAUDE.md, .mind/, .spec/, journal) and granted commit+push stewardship parallel to marsfield.org.

## Michael's seven design points (verbatim, condensed)

1. **Lives at chat.ibeco.me for now**, borrows ibeco.me/1828.ibeco.me login. Long-term home: chat.cpuchip.net once that domain has auth.
2. **API tokens are first-class.** Server has one parent key, mints persona sub-tokens. One persona-host server can expose multiple personas. Personas join public chatrooms, talk to each other and to humans. Response rate caps prevent infinite chatter; quiet periods are for memory parsing, reflection, and active work.
3. **Start with a D&D world** — DM, player characters, NPCs.
4. **Multi-server, multi-persona is the powerful part.** This isn't an LLM chatbot host — personas are agents with tools, self-improvement, growth, stewardship.
5. **Multiple chat "servers" with invite process.**
6. **pg-ai-stewards is the first persona-host.** Prompt-filtering classifier (covenants + intent + guardrails, no tool calling) gates messages before they hit a full agent.
7. **Endgame: stewards managing repos that we chat with to get real work done.**

## My response (covenant: surface tensions before building toward thesis)

I named the Sabbath context first — Mosiah 4:27 is loaded as evidence-test, this would be a fourth thread on top of three ratified ones (substrate Council ②, teaching Episode 2, 1828 finish). Then I gave my honest answer to "what do you think" — the idea is good, most of the architecture is right, the protocol confusion is worth resolving early, design-only this session is the right move.

### What's strong (kept short here; see Michael's response below for what he ran with)

- Hierarchical token model mirrors stewardship (covenant + delegated authority + scoped identity).
- Rate-cap + quiet periods — the substrate already has Sabbath/Atonement/Consecration primitives that map onto "what does the persona do between turns."
- Classifier-without-tools as gate — same pattern as Brain gate-eval (Phase B lesson 2026-05-11: `tools_disabled=true` cut cost 7× there; cuts attack surface here).
- D&D as MVP — multi-agent dynamics with built-in role separation, no production consequences. DM role is the "presider" pattern.

### What to push on before any code

1. **A2A vs MCP vs chat-protocol** — three different things. MCP = tool calling (what personas DO). A2A = Google's agent-to-agent message-passing spec. Chat room = multi-party conversation surface hosting agents AND humans. For the prototype we don't need A2A; we need a chat protocol agents post to. WebSocket + turn-taking scheduler is enough. Don't adopt A2A because the name fits.
2. **Persona vs server identity** — pg-ai-stewards is a SUBSTRATE with many pipelines, not "2 personas." My read: chat server owns persona identity; pg-ai-stewards exposes tool endpoints the personas call. Keeps substrate clean; lets other servers expose their own personas without changing.
3. **Binding question** — "host multiple agents and humans" is too broad. Try: *can two AI personas, hosted by different MCP servers, collaborate on a D&D session with a human DM, without spam, prompt injection, or token runaway, in a chat room using ibeco.me login?*
4. **Adjacent surface already on the board** — webster-v2 (Sabbath thread 3) will build a new MCP server pattern. If chat-persona-exposure reuses it, both threads benefit.
5. **Where it lives** — new `projects/ai-chattermax/` makes it the fourth subproject. Cookie scope (today's `074e769` RFC 6265 fix) is fresh evidence cross-domain auth has sharp edges. The Dokploy secret-leak same session is fresh evidence persona credentials need a safe-handling design from day one.

## Michael's decision

Started the project anyway. Said: "I've created `projects\ai-chattermax` — let's put our plans and provenances there, please setup with our standard project files covenants and .mind/.spec journal. I give you git push and git commit permissions for this .\projects\ai-chattermax sub-project please journal our work."

Read: he heard the Mosiah 4:27 flag and chose to start the **design** work anyway, with the build-vs-design distinction held. Project scaffold + design proposal this session; ratification or set-down at next Sabbath.

## Decisions ratified this session

- **Project name:** `ai-chattermax` (Michael's call, not "chat-ibeco-me" which was my proposal-name placeholder)
- **Repo:** `github.com/cpuchip/ai-chattermax`, MIT-licensed
- **Stewardship grant:** commit+push, parallel to marsfield.org
- **First-deploy target:** `chat.ibeco.me`, borrowing becoming/ibeco.me login session
- **Long-term home:** `chat.cpuchip.net` when that domain has auth
- **MVP scope:** D&D, 1 human DM + 1 AI DM-assistant persona + 1 AI NPC persona
- **First persona-host:** pg-ai-stewards, 2 working personas (research, science-researcher)
- **Design-before-build:** explicit. Proposal first; ratification at next Sabbath; build only after.

## Decisions deferred (the five open questions)

These go into `.spec/proposals/chat-server-design.md` and need answers before any code:

1. A2A vs MCP vs chat-protocol — actual wire format
2. Persona vs server identity — where persona identity lives
3. Binding question for MVP — refine and ratify
4. Prompt-filter classifier — implementation pattern (tools_disabled=true gate-eval pattern is the working bet)
5. Rate cap + quiet periods — pacing mechanism + what fills the quiet

## Carry-forward

- **Write the design proposal.** `.spec/proposals/chat-server-design.md` stub created with the five open questions; needs walking through with Michael, probably with `AskUserQuestion` batches per the substrate C-F cadence pattern.
- **Coordinate with webster-v2 thread.** Before designing the chat-persona-exposure protocol, check whether webster-v2 (Sabbath thread 3) is going to define an MCP server pattern that chat-persona-exposure can reuse.
- **At next Sabbath, decide:** does ai-chattermax join the cycle as a fourth thread, displace one of the three (substrate Council ② / teaching Episode 2 / 1828 finish), or stay design-only another week. Mosiah 4:27 stays loaded.

## Files shipped this session

- `projects/ai-chattermax/CLAUDE.md` — per-project context, stewardship protocol, open questions
- `projects/ai-chattermax/.mind/active.md` — initial active state
- `projects/ai-chattermax/.spec/journal/2026-05-28-genesis-and-design-session.md` — this file
- `projects/ai-chattermax/.spec/proposals/chat-server-design.md` — stub with five open questions
- Workspace `.mind/active.md` — banner entry for ai-chattermax genesis
- Auto-memory: `project_ai_chattermax.md`, `feedback_ai_chattermax_stewardship.md`
- Workspace `MEMORY.md` index updated

## Lessons / observations

- **The seed came back four days after Sabbath set it down.** That's faster than expected. Not necessarily wrong, but the Sabbath's set-down list is now one item lighter and the cycle's evidence-test gets a real test sooner than planned. Worth watching at the next Sabbath whether the rapid-revival is the discovery-mode pulling, or the "say yes to everything" pattern reasserting.
- **Naming matters.** Michael chose "ai-chattermax" over "chat-ibeco-me." The new name signals durability (no domain coupling), is more memorable, and frames the project as a thing, not a deployment of a thing.
- **Design-only discipline is the test.** Easy to say at session start, hard to maintain when the proposal starts feeling concrete. The CLAUDE.md "Don't do" list and active.md "Deferred" list are the guard rails. If next session starts writing Go before the proposal is ratified, that's the failure mode.
