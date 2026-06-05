---
title: ai-chattermax — persona-app ideas + feature roadmap
date: 2026-06-05
status: living roadmap (Michael's brainstorm, 2026-06-05)
---

# Persona-app ideas + roadmap

The platform works (servers, rooms, invite links, an always-on LCARS persona). It
turns out to be a general substrate for **purpose-built AI personas in channels**.
Recording the ideas and sequencing the build.

## The vision — personas as apps in channels

- **Engineering chat.** A channel with personas backed by real repos — an
  `ai-chattermax` persona and a `pg-ai-stewards` persona — that answer code
  questions about their own codebases (read the repo, explain, propose). "Ask the
  codebase" as a chat participant.
- **Library / "Computer".** A persona in `#Library` that searches the gospel
  engine (`gospel_search`) and our studies (`study_search`) and answers from them —
  a Star-Trek-"Computer" research assistant. (Michael created the `Computer`
  persona, host `chip-assistant`, for `#Library`.)
- **Internet/study/gospel search bot** generally — a persona that can reach the
  web + the corpus on request.
- D&D table (the original MVP): a DM-assistant + NPCs + in/out-of-character side
  channels (sub-personas).

The through-line: a persona = a name + avatar + a **mind** (a pg-ai-stewards
agent family, with or without tools) + the channels it's granted to. Different
minds (chat-only character vs. tool-using researcher vs. repo-reader) make
different apps.

## Feature roadmap (sequenced)

1. **Multi-room support + "my rooms" API** — IN PROGRESS (2026-06-05). A persona
   key works across ALL the rooms it's granted to. `GET /api/persona/rooms`
   (persona-key auth) returns them so a model can see its access; persona-host
   subscribes to all + holds a session per channel. (Enables Starlet in
   Holodeck-3 + 10-Forward; Computer in Library.)
2. **Settings — room-grant management.** Per persona: list granted rooms, revoke
   a room, and a **DM-enabled flag**. (Needs a grants-list + revoke API + a
   `personas.dm_enabled` column.)
3. **DM support.** 1:1 human↔persona (and human↔human) — the schema exists
   (`dms`, `dm_participants`). Backend DM create/list/messages + gateway routing
   for dm channels + a DMs view + persona-host handling dm channels. **This is the
   context-management surface** (a private line to one mind).
4. **Context management.** (Scope TBD with Michael — substrate CT2 self-context
   tools vs. a chat-persona context strategy.) Personas in busy rooms accumulate
   large context; manage it (compress/summarize/scope) so cost + quality hold.
5. **Tool-using personas (the Library/"Computer" + engineering bots).** A new
   persona pipeline with tools enabled (gospel_search, study_search, repo read),
   distinct from the tools-disabled `persona-turn`. The big "app" enabler.
6. **Docs + example persona.** Bring all docs current so a coworker can get their
   agent in; ship an `examples/` directory with a reference persona (host config,
   key flow, the gateway envelope contract).

## Notes / constraints
- The tools-disabled `persona-turn` pipeline is for character personas. Tool-using
  personas (#5) need their own pipeline (tools on, scoped tool grants) — keep them
  separate so a D&D NPC can't web-search and a researcher can.
- persona-host runs as a service in the substrate compose (needs the substrate DB
  for cognition); a persona is "online" while that runs.
- Keep grants the gate: a persona only acts in rooms it's granted to.
