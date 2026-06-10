# D&D on Chattermax — the Holodeck program

**Status:** RATIFIED 2026-06-10 — all four decisions as recommended: unified
server-side dice for all senders; sub-persona cast with display/cognition
decoupling; dnd-tools greenlit (public Go MCP twin on SRD 5.2 + Open5e);
Phase 1 (D1+D2+D3) building now under stewardship.
**Binding question:** What does it take for someone to walk into a prep room,
cook a campaign and characters with a DM persona, get an alert when the
holodeck opens, and play a real multi-session D&D campaign with AI-run NPCs and
PCs — without spawning a zoo of registered personas or burning the budget?

The vision (Michael, verbatim intent): hop into a prep room — the entrance to
Holodeck-3 — chat with the DM and PC personas, have them cook a campaign and
PCs for you, then the "program" readies, Holodeck-3 opens, you get an alert.
Archive and resume sessions for multi-part play. Multiple holodecks for
concurrent campaigns.

## What already exists (verified 2026-06-10)

The morning's REM arc built most of the table infrastructure without naming it:

- **Expressiveness:** room_say beats + mood emoji, typing indicator, 👀 eyes on
  the message being worked, reactions (the 🎲-then-narrate pattern is live).
- **Routing:** @mention parsing, notifications (the "program ready" alert
  mechanism), respond_policy (all | mentioned | judgment) — what keeps a
  multi-NPC room from answering everything.
- **`sub_personas` table** — the platform anticipated this on day one
  ("Room-scoped identity for a persona — D&D PC in-character vs OOC. v2 UI;
  schema now"): `(persona_id, room_id, display_name, prompt_override)`, message
  attribution (`messages.sub_persona_id`) and resolved rendering
  (`COALESCE(sp.display_name, …)`) already wired. ⚠ Constraint to loosen:
  `UNIQUE (persona_id, room_id)` allows ONE alias per room — the cast vision
  needs many.
- **Substrate:** fiction agent family + `dm-assistant`/`npc-ally`/`dmbot`
  persona rows seeded in persona_host; per-channel sessions; §7 faceted
  self-notes ({persona}/{room}-scoped durable memory = the campaign-log
  mechanism); cost caps + budgets.
- **Rooms are cheap:** Holodeck-3 already exists; "multiple holodecks" is just
  rooms — per-channel sessions already isolate them.

## The external landscape (researched 2026-06-10)

- **D&D Beyond: NO public API.** Forum mods state it flatly ("There is no
  officially supported public facing API"); the undocumented character-JSON
  endpoint was actively blocked for tools in 2019; the 2026 roadmap (rebuilt
  game platform, Quickbuilder, DM tools) is all their own surface — no
  developer access announced. **Verdict: not a foundation. Don't build on it.**
- **SRD 5.2.1 is Creative Commons (CC-BY-4.0), irrevocably** — and SRD 5.1
  likewise. Both the 5e and 5.5e rulesets are legally clean to build on with
  attribution.
- **Open5e** (api.open5e.com): mature open-source REST API over 22+ source
  books including `srd-2014` and `srd-2024` — spells, monsters, classes, items,
  conditions. An MIT-licensed community MCP server already wraps it
  (Mnehmos/open5e.mcp).
- **Verdict for #3:** reference data is a solved problem (Open5e); what nobody
  offers is OUR characters' state. `dnd-tools` = character + game state on the
  SRD ruleset, consuming Open5e for reference data. A D&D-Beyond *importer*
  (user-supplied JSON export) can be a later nice-to-have; an API integration
  cannot.

---

## Track D1 — persona↔persona triggers (the lifeblood)

Today a turn fires only on human messages. With a DM persona and a Party
persona, persona↔persona triggers are not a feature — they're the game: DM
narrates → PC reacts → DM resolves.

**Design:**
- A persona message triggers another persona's turn **only when it @mentions
  it** (or name-addresses, same `isAddressed`). No ambient persona-to-persona
  turns — that's the spam vector.
- **Hop budget (the ping-pong guard):** each persona-triggered turn carries a
  hop counter (host-side, per channel). Default budget **3**; a human message
  resets it to 0. At budget, the trigger is note()'d but no turn fires — the
  table waits for a human. The DM ending its narration with "@party-bard, your
  move" spends one hop; the bard's reply mentioning the DM spends the second;
  the DM's resolution the third; then it's the humans' table again.
- Eyes/typing/beats all apply to persona-triggered turns unchanged (the loop
  doesn't care who triggered).
- Host-only change (handle() + maybeStartTurn + a `hops` field on
  channelState). No platform or substrate change.

## Track D2 — dice (one implementation, everyone rolls in the open)

**Unified server-side rolling:** the chattermax server intercepts
command-shaped message bodies. `/roll 2d6+3` from ANY sender — human or
persona — is rolled server-side (crypto/rand) and persisted as the result:

> 🎲 **Grimble** rolled `2d6+3` → [4, 2] + 3 = **9**

- One implementation, one fairness story: the server rolls, in the room, in
  history. Nobody — human or model — can fudge. (The same honesty discipline as
  read-before-quoting: the model never invents numbers it claims are rolls.)
- Personas roll by posting `/roll …` (via room_say mid-turn or as the answer) —
  no substrate tool needed; the DM is *prompted* to roll dice in the open.
- Syntax v1: `NdM[+/-K]`, advantage/disadvantage (`adv`/`dis`), multiple
  groups later. Parser unit-tested.
- Implemented as the first **slash command** (D3's framework) — they ship
  together.

## Track D3 — slash commands + autocomplete

- **Server:** a command registry — `GET /api/commands` returns
  `[{name, args, help}]`; `handleMessage` routes bodies starting with `/` to
  the command processor (unknown command → error frame to sender, not a posted
  message). v1 commands: `/roll`, `/mood 😎` (alias for the picker), `/me
  narrates…` (italic emote — D&D flavor, trivial).
- **Frontend composer autocomplete:** one popup component, two triggers —
  - `/` at line start → command list (from the registry, with arg hints),
  - `@` anywhere → mention completion over the room roster + members
    (**this finishes REM-3**: mentions become typeable without memorizing
    names; also the syntax personas are told to use).
  - Keyboard: ↑↓ select, Tab/Enter complete, Esc dismiss.
- Registry-driven so dnd-tools commands (`/char`, `/archive`…) appear later
  without frontend changes.

## Track D4 — the cast system (sub-personas, Michael's #5)

**The ratifying principle: display identity is decoupled from cognition.**
What the room sees (a named cast member) is a `sub_persona`; what thinks is
whatever the host wires behind it — and we can rewire per-NPC without the room
noticing. That's the "adaptable as we play" requirement, made structural.

- **Platform:** loosen `sub_personas` UNIQUE to
  `(persona_id, room_id, display_name)`; CRUD via persona-key auth (the DM's
  host mints/retires its own cast) + owner REST; the persona WS `message` frame
  gains `subPersona` so a post lands attributed (rendering already works);
  room_say gains `as` → outbox carries it → drainer passes it through.
  **Roster shows the cast nested under its owner**: ◆ DM Assistant ▸ Grimble
  the shopkeep, Lady Vex, 3× goblin — the "DM has these NPCs in the room"
  coordination view Michael described.
- **Cognition (host/substrate), adaptable per NPC:**
  - *Facet mode (default):* minor NPCs (shopkeeps, mobs, color) are voiced by
    the DM's own session — one model turn can speak as several cast members via
    multiple room_say-as calls. Cheapest; one mind playing many parts, which is
    literally what a DM is.
  - *Session mode:* a major NPC (the villain, a quest-giver with secrets) gets
    its OWN substrate session + its own §7 notes — real separate memory,
    "tightly controlled persona inside pg-ai-stewards" — while still posting
    through the same registered persona's connection as a sub-persona.
  - Promotion path: an NPC starts as a facet and gets promoted to a session
    when it earns it. The room never sees the rewiring.
- **Cost note:** this is the persona-count AND budget answer — one gateway
  connection, one respond_policy, one turn per trigger, many voices.

## Track D5 — the Party persona (Michael's #6)

Same machinery, pointed at PCs: one registered **Party** persona whose cast is
the agent-run player characters. Each PC = sub-persona + **own substrate
session + own §7 faceted notes** (PCs deserve real independent memory — they're
session-mode by default, unlike NPCs). Two registered personas per holodeck —
**DM + Party** — exactly the shape Michael sketched. A human player joining
just… plays; their PC sheet still lives in dnd-tools (D6), whether a human or
an agent runs it.

DM and Party converse via D1 triggers; respond_policy `mentioned` keeps it
orderly; the hop budget keeps it from running away.

## Track D6 — dnd-tools (new public repo)

`github.com/cpuchip/dnd-tools` at `projects/dnd-tools/` — public, MIT, the
webster/strongs twin pattern: a Go MCP server, embedded/SQLite storage,
cross-compiled into the substrate bridge like strongs was.

- **Owns:** character state (sheets on the SRD 5.2 ruleset: abilities, class,
  level, HP, inventory, spell slots), campaign/party records, character ops
  (`char_create`, `char_get`, `char_update`, `char_levelup`, `char_roll` —
  "roll perception for Grimble" applies the sheet's modifiers), session
  archive/resume records (D7).
- **Does NOT own:** reference data — spells/monsters/items come from Open5e
  (`srd-2014` + `srd-2024` slugs), cached locally for speed/offline. Attribution
  per CC-BY-4.0 in the README.
- **Surfaces:** MCP (substrate personas: the DM looks up monsters, the Party
  levels a PC) + a small HTTP JSON API (the chattermax frontend renders a
  sheet; slash commands like `/char show Grimble` call through).
- **Character builder UX:** conversational-first — the Party persona interviews
  you in the prep room and drives `char_create` (that's the holodeck flow, not
  a form). A frontend sheet *viewer* first; a form-based builder only if the
  conversational path leaves gaps.
- D&D Beyond import: deferred indefinitely (no API); design the character
  model so a user-supplied JSON export could map in later.

## Track D7 — the holodeck program flow (Michael's vision, assembled)

With D1–D6, the flow is composition, not new architecture:

1. **Prep room** (`#holodeck-3-entrance`): DM + Party personas granted;
   policies `judgment`. You chat the campaign into existence — the DM persona
   uses fiction-family agents (+ the workspace's fiction/story craft distilled
   into its prompts — the "eval our skills and pull them in" item) and
   `start_brainstorm` for world-building; the Party persona interviews you and
   builds PCs via dnd-tools.
2. **Program ready:** the DM persona @mentions you — the REM-3 notification IS
   the holodeck-door chime. (v1: the room pre-exists and the DM says it's open;
   a persona-driven room-create/grant tool is a later flourish.)
3. **Play** (`#holodeck-3`): D1 triggers + D4/D5 cast + D2 dice + §7 campaign
   notes. Eyes/typing/beats make the table feel live.
4. **Archive/resume:** `/archive` → the DM summarizes the session into the
   campaign log (dnd-tools record + §7 notes), session rotates (which also
   resets context cost — and is the planned mitigation for the SILENCE-row
   accumulation watch item). `/resume` → new session seeded from the log.
   Multi-part play = archive/resume; concurrent campaigns = more holodeck
   rooms (already isolated per-channel).

## Phasing + cost guards

| Phase | Tracks | Where |
|---|---|---|
| **1 (next build arc)** | D1 triggers + D2 dice + D3 slash/autocomplete | host + chattermax |
| **2** | D4 cast + D5 Party | chattermax + host + substrate |
| **3** | D6 dnd-tools scaffold + char ops + Open5e cache | new repo + bridge |
| **4** | D7 flow: prep-room prompts, /archive + /resume, first campaign | composition |

Cost guards carried throughout: hop budget (D1), respond_policy `mentioned`
for the Party, one-persona-many-voices (D4), substrate per-work-item caps +
daily buckets already in place. A rough kimi-tier session estimate goes in
Phase 4's first-campaign report; small-model NPCs (qwen + CT2 context tools)
are the cost lever if needed.

## Decision points for ratification

1. **Dice:** unified server-side `/roll` for all senders (recommended) vs a
   separate substrate roll tool for personas.
2. **Cast principle:** one registered persona + many sub-personas, display
   decoupled from cognition (facet ↔ session per NPC, promotable) — vs real
   registered personas per NPC.
3. **dnd-tools:** greenlight the public repo now (Go MCP twin, SRD 5.2 CC-BY,
   Open5e reference data, DDB-import deferred) — vs wait until after the first
   played session.
4. **Phase 1 authorization:** build D1+D2+D3 now under existing stewardship.
