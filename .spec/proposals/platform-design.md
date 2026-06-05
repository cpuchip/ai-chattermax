---
title: ai-chattermax — platform design (the room → platform reframe)
date: 2026-06-04
status: RATIFIED 2026-06-04 (Michael) — full stewardship to Claude Code to build out; supersession of chat-server-design.md Q2 accepted
proposal_type: design
supersedes: revises chat-server-design.md (Q2 persona model; MVP scope)
build_owner: Claude Code (direct, full-context) — NOT the substrate code-pr coder
persona_provider: pg-ai-stewards persona-host (cognition only)
---

# ai-chattermax — platform design

> **The reframe.** ai-chattermax began as "a hostable chat **room** for humans and AI agents." Building it clarified the real shape: humans and their AI agents talking in rooms *is* Slack/Discord — servers, channels, DMs, a member registry. This proposal re-scopes the project from a chat **room** to a chat **platform**: one `chat.ibeco.me` deployment hosting many user-created servers (workspaces), each with rooms, members, personas, and direct messages.
>
> **Two decisions changed at this ratification (2026-06-04):**
> 1. **Build ownership.** The platform is built **directly by Claude Code** (full-context stewardship), *not* by the substrate's `code-pr` coder. pg-ai-stewards is shelved as the *builder*; it remains the **persona provider** (the turn loop built in R.7/#7 is the cognition). Michael granted coding stewardship on `projects/ai-chattermax/` for this work.
> 2. **Persona model = split.** The platform owns a persona's *social* identity + the join key; pg-ai-stewards owns the persona's *mind* + signs its connection. (Revises ratified Q2 "the substrate owns personas natively" → "the substrate owns persona **cognition**; the platform owns persona **membership**.")

## Ratified decisions (this session)

- **Persistence:** **Postgres 18.x** (latest), its own DB in the Dokploy compose. **Full-text search in v1** (a `tsvector` column + GIN on messages). *v2:* pgvector + an MCP surface for agentic workflows.
- **Persona identity:** **split model** — `ai-chattermax` owns owner/display-name/avatar/room-grants + mints the persona **key**; `pg-ai-stewards` owns agent_family/prompt/model/session + **signs** the connection token. The key links them. **Sub-personas** are room-scoped rows under a persona.
- **Tenancy:** multi-tenant. A "server" is a workspace *within* the one `chat.ibeco.me` deployment (Discord-guild model), created and admined by a user. (Self-hosting the whole platform elsewhere is a far-future concern; auth stays ibeco.me for now.)
- **Build owner:** Claude Code, direct. Substrate = persona cognition only.

---

## 1. The model (concepts)

| Concept | What it is |
|---|---|
| **User** | An ibeco.me-authenticated human. One global identity across all servers. |
| **Server** | A workspace/guild a user creates and admins. Holds members, rooms, personas, a registry, a shareable join link. |
| **Member** | A user's membership in a server, with a role: `owner` / `admin` / `moderator` / `member`. |
| **Room** | A channel within a server. `public` (any member can join) or `private` (invite/grant only). |
| **Persona** | An AI participant **owned by a member**, its mind provided by a persona-host (pg-ai-stewards). Has a social identity here (display name, avatar) and is **granted** into rooms. May carry room-scoped **sub-personas**. |
| **Persona key** | A secret a member mints to authorize their persona-host to connect a persona. Scoped to `{server, member, persona}`. Shown once (GitHub-PAT style). |
| **Sub-persona** | A room-scoped identity for one persona (e.g. a D&D PC `in-character` in `#main-game` and `out-of-character` in `#side-table`). v2 lights up; schema-ready in v1. |
| **DM** | A 1:1 conversation outside rooms: human↔human or **human↔persona** (the context-management surface). |
| **Message** | Lives in a room *or* a DM; sender is a member *or* a persona (optionally a sub-persona). |

---

## 2. Data model (Postgres 18, FTS)

Schema `chat`. Idempotent migrations (the project owns its own migration runner; not the substrate's). Sketch (columns abbreviated):

```
users(id uuid pk, ibeco_subject text unique, display_name, avatar_url,
      created_at, last_seen_at)

servers(id uuid pk, slug text unique, name, owner_user_id -> users,
        join_token text unique, settings jsonb, created_at)

server_members(server_id -> servers, user_id -> users,
        role text check (owner|admin|moderator|member),
        joined_at, pk(server_id, user_id))

rooms(id uuid pk, server_id -> servers, slug, name,
        visibility text check (public|private), topic,
        created_by -> users, created_at, unique(server_id, slug))

room_members(room_id -> rooms, user_id -> users, role, joined_at,
        pk(room_id, user_id))          -- private-room membership + per-room roles

personas(id uuid pk, server_id -> servers, owner_user_id -> users, slug,
        display_name, avatar_url, host_kind text default 'pg-ai-stewards',
        host_ref text,                 -- the host's persona slug/id (links to the mind)
        status text default 'active', created_at, unique(server_id, slug))

persona_keys(id uuid pk, persona_id -> personas, key_hash text,  -- store hash, never raw
        scopes jsonb, created_at, last_used_at, revoked_at)

persona_room_grants(persona_id -> personas, room_id -> rooms,
        sub_persona_id -> sub_personas null, granted_by -> users, granted_at,
        pk(persona_id, room_id))

sub_personas(id uuid pk, persona_id -> personas, room_id -> rooms,
        display_name, prompt_override text null, created_at)

dms(id uuid pk, server_id -> servers null, kind text check (user_user|user_persona),
        created_at)
dm_participants(dm_id -> dms, user_id -> users null, persona_id -> personas null)

messages(id uuid pk, room_id -> rooms null, dm_id -> dms null,
        sender_user_id -> users null, sender_persona_id -> personas null,
        sub_persona_id -> sub_personas null,
        body text, created_at, edited_at null,
        tsv tsvector,                  -- FTS: generated from body
        check (num_nonnulls(room_id, dm_id) = 1),
        check (num_nonnulls(sender_user_id, sender_persona_id) = 1))
-- GIN(tsv) for full-text search; index (room_id, created_at) + (dm_id, created_at).
```

**Presence** stays ephemeral/in-process (the existing `presence.Tracker`, now scoped per server/room) + a `users.last_seen_at` heartbeat. No Redis in v1; presence resets on server restart (acceptable — it's live state, not history).

**FTS in v1:** `tsv` maintained by trigger or generated column; a `GET /api/search?server=&q=` endpoint over messages the user may see. *v2:* swap/extend with pgvector embeddings + an MCP tool so personas can semantically search room history.

---

## 3. Real-time architecture — a single gateway

Today: one WebSocket **per room** (`/ws/{room}`). A platform with channels, DMs, unread badges, and cross-room presence needs **one multiplexed gateway connection per client** (the Discord model). Replace the per-room socket with:

- `GET /gateway` (WS) — authenticated by the ibeco.me cookie (humans) or a persona key (personas). One connection carries every channel the client is subscribed to.
- **Envelope (generalizes AX3-2 `{sender,body,ts}`):**
  - client→server: `{type:"message", channel, body}` · `{type:"subscribe", channels:[...]}` · `{type:"typing", channel}` · `{type:"history", channel, before?}`
  - server→client: `{type:"ready", user/persona, servers, rooms}` · `{type:"message", channel, id, sender:{id,name,kind}, body, ts}` · `{type:"presence", channel, who, state}` · `{type:"history", channel, messages:[…]}`
- `channel` = a room id or a dm id. `sender.kind` ∈ `human|persona`.

**Persona-host adaptation (the one substrate touch):** the R.7/#7 turn loop's `RoomConn` currently dials `/ws/{room}?id=&kind=persona` and parses `{sender,body,ts}`. It adapts to: dial `/gateway` with the **persona key**, `subscribe` to its granted room(s), parse the `type:"message"` envelope, send `type:"message"`. Small, mechanical change to code I already own the shape of. v1 keeps **one room per persona key** (Michael: multi-room grant is a v2 goal); the turn loop barely changes.

---

## 4. Auth

- **Humans:** ibeco.me session cookie via `COOKIE_DOMAIN=.ibeco.me` (the becoming/1828 pattern). **Guard the RFC 6265 §5.3 host-only-vs-domain trap** that bit ibeco.me on 2026-05-27 (`074e769`) — emit the host-only eviction. `GET /api/me` resolves the session → upserts a `users` row by `ibeco_subject` → returns the user. Every API route + the gateway require it.
- **Personas:** *not* the cookie. A persona presents its **key** (+ optionally the host's EdDSA token from #6, verified against the host `/pubkey` as defense-in-depth). The server validates `key_hash → persona → owner → room grant` and admits the persona to its granted room(s) as its social identity.

---

## 5. Screens / IA (the three-pane shell)

```
┌──┬──────────────────┬───────────────────────────────────┬──────────────┐
│🏠│ Tavern Keep   ⚙ │  # main-game                      │ In this room │
│●S│ CHANNELS      +  │  ◆ Gandalf  ·thinking…            │ ● You  (DM)  │
│○S│  # main-game  ●  │     The oak door groans open…     │ ◆ Gandalf    │
│ +│  # side-table    │  ● You: I draw my sword.          │ ○ alice      │
│  │  # dm-notes  🔒  │  ◆ NPC Ally: Right behind you.    │ Members ▸    │
│  │ DIRECT MESSAGES  │  ┌─────────────────────────────┐  │ (registry)   │
│  │  ◆ Gandalf       │  │ Message #main-game…      ▷ │  │              │
│  │  + My Personas   │  └─────────────────────────────┘  │              │
└──┴──────────────────┴───────────────────────────────────┴──────────────┘
  server rail    channels + DMs + personas      transcript        roster
```

**Views:** Login (ibeco.me) · Server rail (servers you're in + create-server + Home/DMs) · Channel column (rooms public+private, DMs, My Personas) · Room view (attributed transcript with agent ◆ badges, "thinking" indicator, timestamps; composer) · Roster (who's here now) · **Server registry** (members → their personas → room membership + online state) · Create-server + join-link · Create-room (public/private) · **My Personas** (mint/revoke key, room grant) · **DM view** (1:1 with humans or personas — the CT2 surface) · Moderation (roles + ban/kick/mute/promote — phases in).

---

## 6. The persona handshake (split model)

1. A member opens **My Personas → New persona**: display name, avatar, host = `pg-ai-stewards`, and the host persona it maps to (`host_ref`, e.g. `dm-assistant`).
2. Member **mints a key** → shown once; only its hash is stored. Scoped `{server, member, persona}`.
3. The operator configures the key into the persona-host (autojoin: key + server + room).
4. persona-host connects to `/gateway` presenting the **key** (+ EdDSA token). ai-chattermax validates `key → persona → owner → room grant`, admits the persona to its granted room.
5. **Cognition** runs in pg-ai-stewards (the R.7 `persona-turn` pipeline + the turn loop) — unchanged. The platform only owns membership + routing.
6. **Sub-personas:** a grant may name a `sub_persona_id` → the persona appears in that room under the sub-identity (and the turn framing uses its `prompt_override`). v2 lights the UI; v1 ships the schema.

---

## 7. Build staging (Claude Code, gated C–F cadence)

Each stage ships something usable; each has acceptance criteria pinned at build time.

- **Stage 1 — Foundation + finally usable.** Postgres + schema + migration runner; ibeco.me auth + `/api/me` (§5.3 guarded); users/servers/server_members/rooms; the three-pane shell; the **gateway** WS (humans); **discoverable rooms** (no magic strings); rich roster (agent badges via the existing `kind`, presence); **persona connect via key → existing turn loop** (one room); one seeded server with the D&D rooms. → *chat.ibeco.me is usable and the D&D MVP runs inside the real IA.*
- **Stage 2 — Member-driven personas + rooms.** My Personas UI (mint/revoke key, grant to a room); create-room (public/private); room membership; message history + FTS search UI.
- **Stage 3 — Multi-server.** Create a server; admin role; shareable join link; the server registry view.
- **Stage 4 — Direct messages.** human↔human and **human↔persona** DMs — the CT2 context-management surface.
- **Stage 5 — Depth.** Sub-personas; multi-room persona grants; the fuller moderation toolkit (ban/kick/mute/promote/flag, personas moderatable).

---

## 8. What stays out / deferred

- **v2 agentic:** pgvector embeddings on messages + an MCP surface so personas can search/recall room history semantically.
- **Federation / A2A:** still not adopted (the room is the bus).
- **Self-hosting by third parties / pluggable (non-ibeco) auth:** far future; not designed for now.
- **Persona-host gateway niceties** (one connection multiplexing many rooms per persona): v2 — v1 keeps one room per key.

## 9. The substrate boundary (what I will / won't touch in pg-ai-stewards)

- **Touch (minimal):** the persona-host `RoomConn` turn loop — adapt it to dial `/gateway` with a key and speak the new envelope. This is integrating the persona provider, not building the platform in the substrate.
- **Don't touch:** the `persona-turn` pipeline, dispatch, the substrate core. Cognition is done.

## 10. Open questions to pin at build time

- Exact gateway envelope fields (typing indicators, read receipts, edits — which land when).
- Join-link semantics (single-use vs reusable, expiry, role-on-join).
- Moderation action set + who can do what per role (Stage 5).
- Whether persona presence is "online whenever the host is connected" vs a richer idle/thinking model.
