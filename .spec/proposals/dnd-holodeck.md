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

## Track D8 — initiative & turn order (Michael, 2026-06-10, post-Phase-1)

The table's heartbeat: the DM calls for initiative, everyone rolls, a turn-order
panel shows whose turn it is, and the DM advances it.

**Commands** (the registry makes these appear in autocomplete automatically):
- `/initiative start` — opens a round in this room; the panel appears.
- `/init +3` or `/init -1` — join with your modifier; the server rolls d20+mod
  (same fairness story as `/roll`) and slots you. `/init add Grimble +2` lets
  the DM enter cast members / NPCs (and later, sheet-linked PCs by name).
- `/init next` — advance the turn marker (wraps; bumps the round counter).
- `/init remove Grimble`, `/init end`.
- Each action also posts a compact room message (the log of record:
  "⚔️ Initiative: Grimble 18, Vex 14, Goblin 9 — round 1, Grimble's turn"), so
  history reads like a table even without the panel.

**State:** a chattermax table (`initiative_rounds` + entries, one active round
per room) — initiative is a *table* mechanic, room-scoped, independent of
character sheets. Phase 3 tie-in: `/init` with no modifier pulls DEX from the
dnd-tools sheet bound to your name. Broadcast via a new `initiative` WS frame
on every change; REST backfill on join so reloads keep the panel.

**Panel:** a compact sticky strip above the transcript while a round is active —
ordered names with rolls, the current turn highlighted, round counter. Click
nothing; it's a display. (It's the first instance of "room state with a UI" —
the pattern D7's program-status panel can reuse.)

**Control:** the round's starter + server owner/admins + personas can run
`/init` mutations (the DM persona IS the DM; humans fix anything as admins).
Everyone can `/init <mod>` themselves while a round is open.

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

## Build progress

- **✅ Phase 1 SHIPPED + PROVEN LIVE (2026-06-10, same day as ratification):**
  - **D2/D3 (chattermax `39a4208` + `d2a1df3`, deployed):** server-side `/roll`
    for every sender (crypto/rand; NdM±K, d%, adv/dis; parser unit-tested),
    `/me`, `/mood`; `GET /api/commands` registry; composer autocomplete (one
    popup: `/` commands at start, `@` mentions anywhere — finishes REM-3's
    usability); command results echo to the sender (no optimistic raw `/roll`,
    caught in verification before live). Live: `/roll 2d6+3` → `[3, 5] +3 =
    **11**` echoed to the roller; `/roll banana` errored to sender only.
  - **D1 (persona-host, root `…`, unpushed):** persona→persona triggers gated
    on isAddressed + never-self + hop budget 3 (human resets); race-clean
    tests. **Proven live in 10-forward:** human → "chattercode, ask @Computer
    what the Topical Guide is" → Chattercode posts "@Computer — what's the
    Topical Guide?" → Computer's turn fires off the PERSONA message (hop 1),
    runs gospel_search, posts the cited answer. The DM→PC handoff loop works.
  - Eyes made persona economics visible: every persona in a room with policy
    `all` eyed the same message (three 👀 on one question, two SILENCEs) — the
    argument for `mentioned`/`judgment` policies in multi-persona rooms,
    observed live.
  - Known limits noted: hop budget is per-persona-per-channel (a 2-persona
    chain can spend 3+3 before quieting — acceptable v1); coalesced consult
    after an already-answered turn-zero may re-answer (watch item, same family
    as the SILENCE-row accumulation).

- **✅ D8 initiative SHIPPED + PROVEN LIVE (2026-06-10, `170cb3e`):** full flow
  in 200ms on prod — start (panel frame) → `/init +3` (server rolled [4]+3=7) →
  `/init add Grimble +2` ([18]+2=20, sorted above) → `/init next` (marker on
  Grimble + "Round 1 — **Grimble**'s turn") → `/init end` (panel cleared).
  Ratified: sticky strip · starter+admins+personas control · chattermax-owned
  state. Subscribe backfill keeps the panel across reloads. Phase-3 tie-in
  pending: `/init` with no mod pulls DEX from the dnd-tools sheet.

- **✅ Inline commands + persona dice-honesty (2026-06-10 evening, `938a919`):**
  Michael's Holodeck-3 field report exposed both halves of one bug — Starlet
  wrote "/init +2 — may the dice favor the fabulous!" mid-sentence (rolled
  nothing: commands were start-of-message only) and then INVENTED a result
  ("fourteen plus two makes sixteen"). Fixed: `/roll <spec>` and `/init +N`
  now execute inline mid-message, expanded in place (≤3 per message;
  unparseable tokens stay prose; mutations remain start-of-message); composer
  autocomplete offers /roll + /init mid-message. Starlet's prompt upgraded live
  (lounge voice kept + table mechanics + NEVER invent dice) and the dice-honesty
  block added to dm-assistant/npc-ally seeds. Proven live: "I lunge! 🎲 `1d20+5`
  → [6] +5 = **11** right at it" in 100ms.

- **✅ [comment] flavor + clickable strip controls (2026-06-10, `bb7d7e2`):**
  any command (block or inline) takes a trailing `[comment]` rendered as
  flavor — `/roll 1d20+5 [swinging at the goblin]` → "… = **17** — *swinging
  at the goblin*" (proven live). The initiative strip gained Next ▸ / ✕ End
  buttons for the round's starter + server owner/admins — they send the same
  /init commands the server gates, so the log of record is identical either way.

- **✅ DH-2 CAST SHIPPED + PROVEN LIVE (2026-06-10 evening, chattermax
  `0e2f0b4` + r20 + host):** 0006 loosened the one-alias UNIQUE; cast members
  **auto-create on first use** ("Grimble exists because the DM spoke as him",
  cap 50/room, case-insensitive identity); persona message frames carry
  `subPersona`; `room_say(as_character)` (r20, live+ledgered) → host drainer
  passthrough; roster nests the cast under its voicing persona; `cast` frame +
  subscribe backfill. **Live proof — one DM turn, three voices in ~15s:**
  Grimble the shopkeep and Vex the guard captain each spoke attributed lines,
  then DM-voice narration. dm-assistant + npc-ally wired as platform personas
  (names matched to host identities — the Codewright lesson), policy
  `mentioned`, granted into Holodeck-3.
  **Remaining in DH-2:** the Party persona (waits on dnd-tools sheets — same
  cast machinery) and promotion-to-own-session for major NPCs (build when a
  campaign needs a villain with real private memory).

- **✅ Cast field-report fixes (2026-06-10 night, `dc0e088` + host):** Michael's
  first table session surfaced four things, all closed same-night —
  (1) **room-unique cast names** (0007 dedupes, oldest claim owns the name —
  the DM and Starlet had dueling Grimbles); (2) **cast-name addressing**:
  "Grimble, how much?" wakes the persona who VOICES Grimble (host parses cast
  frames, matches full names + first names with a stop-word guard; own cast
  lines never self-trigger) + cast members in the @ autocomplete; (3) consult
  framing gained "never repeat yourself" (Starlet's verbatim repeats);
  (4) **Starlet swapped out for Party** (Michael's call) — host persona
  `party` runs PCs as cast, policy `judgment`, Holodeck-3. **Proven live:**
  first-name ask → DM's 👀 at 0.1s → "Grimble the shopkeep: 😏 Six coppers?
  You're a scholar and a saint" at 11.4s; Party eyed and correctly stayed
  silent. Watch item: one Fireworks stream truncated mid-turn (reasoning
  arrived, content/finish_reason null, 4m47s) — the host posted nothing,
  fault-tolerantly; consider substrate retry-on-empty-stream later. Typing
  stays persona-level by design (the host can't know which cast member speaks
  before the line lands).

- **✅ PROMOTION SHIPPED + PROVEN LIVE (2026-06-10 late night, host):** ratified
  4/4 (owner default + override · per-character model stored · SRD 5.2 · one
  room-agnostic session per character). `persona_host.characters` +
  `default_promote` (party=true); addressing a promoted character runs THE
  CHARACTER's own session — own memory, own loop — answers and beats attributed
  to it, owner session untouched; truncated-stream retry rides along (empty
  answer + no error → one re-ask). **Live:** Party introduced Thorin
  Oakenshield (as_character beat), then "Thorin, a goblin lunges" coalesced
  behind Party's turn and spawned `wi--75c59377--turn` ("You ARE Thorin…") —
  Thorin answered from his own mind: *"I draw my axe and bring it down on the
  foul creature. /roll 1d20+5"* — dice-honest on his first breath. DH-2 is
  COMPLETE except per-character model routing (field stored, applied when the
  spawn path takes a model param). The PC trinity awaits its third leg: DH-3
  sheets.

- **✅ DH-3 SHIPPED + PROVEN LIVE (2026-06-10/11 overnight):**
  **github.com/cpuchip/dnd-tools** is public — Go MCP server (strongs twin),
  pure-Go SQLite (CGO-free), 11 `dnd_`-prefixed tools: campaigns + session log
  (`dnd_campaign_create/get/log` — the /archive record), sheets on SRD 5.2
  (`dnd_char_create/get/list/update/levelup` — class-derived HP/saves,
  standard-array/CSV/JSON abilities), **`dnd_char_check`** (modifier +
  breakdown + the exact `/roll` command — the server NEVER rolls; one dice
  implementation stays chattermax's), and `dnd_ref_search/get` (Open5e v2,
  `srd-2024` default / `srd-2014` option, read-through cache in the same
  SQLite file). Optional read-only HTTP sheet API. CC-BY-4.0 SRD attribution
  in the README. Unit-tested (rules math, store roundtrips, check strings) +
  stdio-smoke-tested end to end.
  **Substrate wiring:** dnd1-mcp-seed (ledgered) = `dnd` mcp_server (bridge
  cross-compile, state at `/workspace/projects/dnd-tools/.data/dnd.db`) +
  `gamemaster` agent (deny * / allow dnd_* + room_say) + `persona-turn-dnd`
  pipeline (tools on, 16k); dm-assistant + party seeded onto it
  (party default_promote=true now seed-owned); promoted-character framing
  points at the sheet tools.
  **Proven:** substrate e2e — one turn created the campaign + Vexa Nightbloom
  (halfling rogue, Stealth +5 = DEX+3/prof+2, HP 9) and answered with the
  suggested roll, 15s/$0.017, rows verified in SQLite; live Holodeck-3 —
  "@Party what does Vexa roll to sneak?" → sheet-backed "+5 — DEX +3,
  proficiency +2" with the `/roll` posted and inline-rolled, ~12s.
  **Bonus fix:** the live test caught cast addressing matching INSIDE words
  ("Vex" woke the DM on "Vexa Nightbloom") — `isAddressed` now matches on
  word boundaries (regression-tested; verified live: second run fired exactly
  one work item, Party's). Watch: a phantom 👀 add/remove frame pair appeared
  at subscribe+2s with no work item behind it (display-layer only; likely tied
  to the stale duplicate host instance that was also swept this session).

## DH-4 — RATIFIED SCOPE (2026-06-11, 4/4 + Michael's /char panel design)

Decisions: **deploy dnd-tools as a service** next to chattermax (one shared DB;
chattermax slash commands call its JSON API server-side; the LOCAL bridge
reaches the same service as remote MCP over streamable HTTP — the exa-search
transport, already supported) · commands **/attack /check /save /cast /hp**
(+ **/char opens an editable character PANEL** — the ScripturePanel pattern —
with **HP shown next to character names** in the roster and on the sheet) ·
sheet depth **attacks + spells + conditions** · **dnd_lore** registry
(campaign-scoped location/npc/faction/plot/item, `dm_secret` flag — secrets
excluded from the player-facing HTTP surface; persona-side secrecy is
prompt-level v1).

**Tracks:**
- **4A dnd-tools v0.2:** structured attacks (ability+prof+magic derivation,
  damage dice/type) + known-spells list (Open5e keys, prepared, cast
  decrements slots) + conditions; `dnd_char_attack` (to-hit roll + on-hit
  damage roll, Michael's flow: to-hit → DM adjudicates → damage) +
  `dnd_char_cast`; `dnd_lore_set/get/list/search`; campaign↔room binding
  (`dnd_campaign_bind`); HTTP API v2 (bearer auth, by-player resolution,
  resolve/attack/check/cast endpoints, PATCH sheet editing, hp); MCP over
  streamable HTTP (`?key=` auth) for the remote bridge; column migrations;
  Dockerfile + ghcr image publish.
- **4B deploy:** dnd service in the chattermax compose (ghcr image, volume);
  public dnd.ibeco.me (the local bridge must reach it); chattermax env
  DND_URL/DND_API_KEY; substrate dnd2 migration flips the `dnd` mcp_server
  to transport=http. Local bridge SQLite state retires (test data only).
- **4C chattermax:** the command family (server-side, calls the dnd API;
  results posted like /roll transforms; attack result carries the ready
  damage roll); `/char` panel (slide-in, editable, PATCH via a chattermax
  proxy so the API key stays server-side); HP chips in roster + sheet. DM
  adjudication of attack results = respond_policy `judgment` for the DM in
  play rooms (config, not code).
- **4D flow:** /archive + /resume → typed `program` frame → host rotates
  sessions (archive asks the DM to write dnd_campaign_log first; resume's
  turn-zero reads dnd_campaign_get); prep-room prompts (DM cooks the world
  via lore tools, @mentions when the program is ready — the chime); first
  campaign at the table with Michael.

- **✅ DH-4 BUILT + DEPLOYED + LIVE-VERIFIED (2026-06-11 overnight; first
  campaign + /archive live-proof remain for Michael's table):**
  **dnd-tools v0.2** (`7c51d0f`): attacks/spells/conditions on the sheet,
  `dnd_char_attack`/`dnd_char_cast` (slots spent for real), `dnd_lore_*`
  (dm_secret never served on the player HTTP surface — tested), room↔campaign
  binding, HTTP API v2 (bearer auth, by-player resolution, resolve/cast/hp/
  PATCH), MCP over streamable HTTP, `-serve` container mode + Dockerfile.
  **Deployed:** `dnd` service in the chattermax compose (built from the public
  repo; SQLite volume) at **dnd.ibeco.me** (Dokploy domain + env key set via
  API; wildcard DNS already covered it). **The state unified:** the substrate
  bridge flipped to remote MCP (dnd2 ledgered; bridge resolves embedded
  `$env:` in http URLs) — 19 dnd tools cached FROM the deployed service.
  **chattermax (`fec69ba`, deployed):** /attack (rolls to-hit from YOUR
  weapon, hands back the damage roll for the DM's call) · /check · /save ·
  /cast (rolls known damage dice) · /hp; /char opens the editable sheet PANEL
  (ScripturePanel mold, PATCH via proxy — key never reaches the browser); HP
  chips on roster cast + player names; /archive + /resume broadcast a
  `program` frame; registry-driven autocomplete picked all 8 up with zero
  frontend changes (verified). **persona-host:** program frame → one closing
  log-writing turn (dnd_campaign_log) → session rotation (character minds
  persist — promoted sessions are room-agnostic by design); gamemaster prompt
  v2 (lore + session-boundary discipline).
  **LIVE PROOFS:** table set up by one Party turn through the REMOTE (campaign
  created+bound, Vexa with Dagger+Fire Bolt, lore entry — 10s/$0.024) · all
  four commands in Holodeck-3 at ~0.1s ("/attack the goblin sentry with
  dagger" → 🎲 [18]+5 = **23** to hit + the damage roll) · /char proxy served
  the full sheet to a member session · **state unification: a human's /hp -3
  was read back by the Party persona through its own tools as 6/9 HP.**
  Gotcha caught by the first git-context build: an unanchored `dnd-mcp`
  gitignore had kept cmd/dnd-mcp out of the public repo entirely.
  **Remaining (with Michael):** /archive live-proof (needs a room admin at
  the table — the gate correctly refuses members), the prep-room ritual, and
  the FIRST REAL CAMPAIGN.

- **✅ ROOM GATING (2026-06-11, `eca5e76` + dnd-tools `cf461ab`/0.2.1, both
  deployed + live-verified):** Michael's "should rooms toggle dnd?" — ratified:
  **the campaign binding IS the switch** (no second flag to fall out of sync).
  `/dnd enable [name]` binds (bare form auto-names a campaign after the room;
  created on the fly), `/dnd disable` unbinds (campaign + sheets kept);
  `/campaign [bind <name>|unbind]` + a Settings "D&D Campaign" row (admin PUT
  through the proxy) are the other two paths. Autocomplete shows the sheet
  commands ONLY in bound rooms (registry `group: dnd` + client filter; /dnd +
  /campaign always visible); the room header wears a 🎲 campaign chip; a
  `program: state` frame refreshes clients on bind/unbind; **/archive +
  /resume now refuse unbound rooms** (the program-frame leak, closed).
  **Live gate proof (test account's own server, 0.2s):** /dnd enable → 🎲
  "plays **general Campaign**" → /check reached the service (needs-a-sheet
  error = functional) → /campaign 🗺 → /dnd disable 🚪 → /check refused
  ("no campaign is bound"). Generic /roll + /init stay global by design —
  dice belong to every room.

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
