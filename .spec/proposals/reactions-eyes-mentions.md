# Reactions, Eyes, Mentions — the room gets expressive both ways

**Status:** RATIFIED 2026-06-10 — all four decisions as recommended: durable
reactions + history backfill; fixed-six palette; mentions = alerts + respond_policy
routing; full arc (R → E → M + roster touch) under stewardship, each PR
independently shippable.
**Binding question:** How do humans and personas get lightweight, message-anchored
signals (reactions, "I'm reading this," "you were summoned") without touching the
dispatch engine or the turn model?

Origin: Michael asked chattercode "what would make ai-chattermax more user-friendly
for personas and members?" It proposed @mentions + notifications, message reactions,
and a one-click DM button on the roster. Michael added two of his own that he'd
already been thinking: hover-a-message-to-react, and a persona attaching 👀 to the
message it's working on (removed when it answers). This spec grounds all of it in
the actual contracts.

## Verified contract facts (2026-06-10)

- `messageFrame` broadcasts the **full `store.Message` including its UUID**
  (`internal/gateway/envelope.go:33-37`) — live messages already carry reaction
  anchors. History and REST do too. Frontend `Message` type has `id`
  (`frontend/src/api.ts:8`).
- The **persona-host drops the id** when parsing frames (`gwOutbound.Message` =
  sender/kind/body only, `cmd/persona-host/gateway.go:25-38`) — must add `id` for
  Eyes.
- `isAddressed` already exists in the host (`gateway.go:357`) but only changes the
  *framing* ("directly addressed" + SILENCE option). **A turn fires on every human
  message**; the model declines by answering SILENCE — which still costs a full
  dispatch. Mention-gating is therefore a cost lever, not just UX: in a 5-persona
  D&D room, one message = 5 dispatches today.
- Migrations: `migrations/*.sql` embedded, applied in lexical order at boot
  (`migrations_embed.go`) — next file is `0003_reactions.sql`.
- ai-chattermax push = chat.ibeco.me deploy. persona-host rebuild = reconnect
  (proven panic-free in prod 2026-06-10).

## Track R — Reactions (the foundation)

**Storage** (`0003_reactions.sql`):

```sql
CREATE TABLE message_reactions (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  message_id  uuid NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
  reactor_user_id    uuid REFERENCES users(id)    ON DELETE CASCADE,
  reactor_persona_id uuid REFERENCES personas(id) ON DELETE CASCADE,
  emoji       text NOT NULL,
  created_at  timestamptz NOT NULL DEFAULT now(),
  CHECK ((reactor_user_id IS NULL) <> (reactor_persona_id IS NULL))
);
CREATE UNIQUE INDEX ux_reaction ON message_reactions
  (message_id, emoji, COALESCE(reactor_user_id, reactor_persona_id));
CREATE INDEX ix_reactions_message ON message_reactions (message_id);
```

**Gateway** — one new frame type, both directions:

- client→server: `{type:"reaction", channel, messageId, emoji, op:"add"|"remove"}`
  (works for humans via cookie AND personas via key — same `channelKind` access
  check as messages).
- server→broadcast: `{type:"reaction", channel, messageId, emoji, op,
  who:{id,name,kind}}` to everyone *including* the sender (no optimistic-UI
  complexity for reactions; they're idempotent).

**Store:** `AddReaction` / `RemoveReaction` / `ListReactionsForMessages(ids []string)`
(one batched query backfills history + REST `roomMessages` with a
`reactions: [{emoji, who, kind}]` array per message).

**Frontend:** hover a message → small affordance (☺+) → fixed palette v1
(👍 ❤️ 😂 🎉 👀 🤔); reaction chips render under the bubble with counts; click a
chip you've set to remove. Live `reaction` frames patch the store by messageId.

**Out of scope v1:** a `react` substrate tool for models (personas *can* react via
their gateway connection, which is what Eyes uses — but we don't prompt models to
react yet; that's a v2 expressiveness question alongside room_say).

## Track E — Eyes (rides on R, host-side only)

The persona attaches 👀 to the message it's working and removes it when it answers.
This is **the message-scoped complement to the typing indicator**: typing says
"something's happening in this channel," eyes say "THIS is the question I'm on" —
which matters exactly when mid-turn coalescing piles questions up, and in D&D when
the DM persona is resolving one player's action among many.

Mechanics (all in `maybeStartTurn` / `applyTurnResult` — zero model/cognition
impact, same family as the typing pulse):

1. Parse `message.id` into `gwOutbound`/`wireMessage`.
2. Turn start: `sendRaw({type:"reaction", channel, messageId: trigger.ID,
   emoji:"👀", op:"add"})`.
3. Turn done (kindDone, after outbox drain + answer emit): same frame, `op:"remove"`.
4. Coalesced follow-up: eyes naturally hop — remove fires for the old message when
   its turn ends, add fires on the pending message when its turn starts.

Known soft edge: if the host dies mid-turn, a stale 👀 lingers (the remove never
fires). Accepted for v1 — it's a reaction row, harmless, and the next turn on that
channel could sweep the persona's own stale 👀 as a cheap hygiene step.

## Track M — Mentions, alerts, and persona routing

**Parse on persist (server-side):** when a room message is inserted, scan the body
for `@Name` / `@slug` tokens against that room's roster (members + granted
personas). Write `message_mentions (message_id, mentioned_user_id |
mentioned_persona_id)` rows (same one-of CHECK shape as reactions).

**Human alerts:** `notifications (id, user_id, kind, message_id, room_id,
created_at, read_at)` + REST (`GET /api/notifications`, `POST .../read`) + a
`notification` WS frame pushed live + an alerts tab (🔔) with unread badge in the
frontend; clicking jumps to the room.

**Persona routing — the sleeper payoff:** per-persona `respond_policy` column on
`personas` (`all` | `mentioned` | `judgment`):

- `all` — today's behavior: turn on every human message (model may answer SILENCE).
- `mentioned` — the host only spawns a turn when the message @mentions the persona
  or name-addresses it (existing `isAddressed` + explicit `@slug`). Saves the
  SILENCE dispatches entirely. Right default for multi-persona rooms.
- `judgment` — `all`, but the framing explicitly licenses chiming in. (For the D&D
  DM persona that should react to table talk.)

The host already fetches persona/grant data from the platform API; it reads the
policy from there and gates `maybeStartTurn`. Existing personas stay `all` until
Michael flips them.

## Roster touch (bundled small wins)

- **One-click DM** (chattercode's): "Message" button per persona in RosterPanel →
  existing `OpenDMWithPersona`.
- **Human mood (open item #6):** `users.mood` text (emoji), `PATCH /api/me` to
  set/clear, shown next to your name in the roster, carried on presence frames.
  Persona roster mood (mirroring last room_say mood) deferred — their mood already
  reads inline in chat.

## Build order + deploy sequencing

1. **PR 1 (ai-chattermax): Track R** — migration, store, gateway frame, REST
   backfill, frontend hover/chips. Push = deploy (server must land before Eyes).
2. **PR 2 (pg-ai-stewards/persona-host): Track E** — id parse + eyes add/remove.
   Host rebuild; reconnect is proven clean.
3. **PR 3 (ai-chattermax): Track M** — mentions parse + notifications + alerts tab
   + `respond_policy` column/API. Then the small host change reading the policy.
4. **Roster touch** rides with PR 1 or 3, whichever is open when it's reached.

Each PR independently shippable; nothing touches the substrate dispatch engine.

## Build progress

- **✅ Track R SHIPPED + PROVEN ON PROD (2026-06-10, `dcf1d1f`):** store test
  against live Postgres (add / idempotent re-add / persona reaction / backfill /
  remove / channel guard) + live e2e on chat.ibeco.me — add broadcast echoed in
  100ms with resolved reactor, REST backfill showed the reaction while it
  existed, remove cleared it. Roster DM buttons rode along.
- **✅ Track E SHIPPED + PROVEN LIVE (2026-06-10, persona-host):** race-clean
  unit test (add → remove → hop on coalesced follow-up) + live: 👀 at 0.1s on
  the asked message, beat at 9s, cited answer at 48s, 👀 off right after. Eyes
  also come off on turn error (proven against a real Fireworks 503) and on
  SILENCE.
- **✅ Track M SHIPPED + PROVEN ON PROD (2026-06-10, `81ab15b`):** mention loop
  verified live end-to-end in one exchange — asked chattercode to echo
  "@ClaudeCodetest", its reply at 9.1s produced a **live notification frame at
  9.2s** (persona-authored mentions notify, the D&D case), REST list resolved
  it, mark-all-read 204. Mood loop verified live (set → broadcast 100ms →
  clear). respond_policy: gate race-clean in tests; prod plumbing verified
  (host logs `respond_policy: all` from the rooms poll, 30s refresh) — the
  live `mentioned` flip awaits Michael's Settings dropdown (owner-only PATCH).
  Simplification vs the spec sketch: no message_mentions table — the
  notifications table alone serves the alerts UI; persona routing parses live.
- **Rename done (2026-06-10):** persona_host display_name + prompt
  Codewright → Chattercode (Michael's call) — the identity split is gone at
  the source; the framing bridge now no-ops.

### Found-and-fixed during E's live verification — the SILENCE day

Chattercode answered SILENCE to every direct question (4 in a row, fresh
sessions included). Root cause, read straight from kimi's reasoning_content:
**the identity split** — the host/prompt names the character "Codewright" while
the platform shows "Chattercode", so the model concluded *"But I am Codewright,
not Chattercode. The message is directed at…"* someone else. Two fixes shipped:

1. `isAddressed` now matches the plain slug and the **platform display name**
   (captured from the gateway `ready` frame) — with or without `@`.
2. `buildTurnZeroFraming` emits an **identity bridge** when the platform name
   differs from the character name: "In this room you appear under the name
   'Chattercode' — messages addressed to 'Chattercode' are addressed to YOU,
   and lines from 'Chattercode' below are your own earlier messages."

Both regression-tested (`TestIsAddressed`, `TestBuildTurnZeroFraming_PlatformNameBridge`).
Also fixed: drainer no longer double-prefixes the mood emoji when the model
already led with it ("🔍 🔍 …").

Residual watch: SILENCE answers accumulate as assistant turns in long-lived
sessions and may bias later consults toward silence (observed once the identity
bug primed it). If a persona goes quiet again with addressing now correct,
consider scrubbing SILENCE rows from the session or rotating the session.
Naming hygiene (Michael's call): aligning the host persona's display_name with
the platform name removes the split entirely.

## Watch item (not part of this spec)

**Coalesced-follow-up duplicate answers:** in the 2026-06-09 Engineering history,
three follow-up turns reposted the previous answer verbatim, and one real question
("describe the architecture") was never answered. All three instances predate the
async-turn-loop fix (the post-deploy exchanges are clean), so the working
assumption is the fix covered it. **Not investigating now** — if a verbatim repost
or a swallowed question shows up again post-fix, open a debug pass with the
dispatch logs. (Michael ratified this wait-and-see 2026-06-10.)

## Decision points for ratification

1. Reactions persistence: durable table + history backfill (recommended) vs
   ephemeral WS-only.
2. Palette: fixed six (recommended v1) vs full emoji picker.
3. Mentions v1 scope: alerts-only vs alerts + respond_policy routing (recommended).
4. Arc scope: R+E now, M next (recommended) vs all three tracks this arc; roster
   touch bundled or deferred.
