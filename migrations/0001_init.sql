-- ai-chattermax platform — initial schema (Postgres 18).
-- Multi-tenant: one deployment hosts many servers (workspaces). See
-- .spec/proposals/platform-design.md §2. Idempotent (IF NOT EXISTS) so the
-- boot-time migration runner is safe to re-run.

-- ---------------------------------------------------------------------------
-- Users — an ibeco.me-authenticated human (or a dev-login user locally).
-- external_subject is the stable identity from the auth provider
-- ("becoming:<id>" / "becoming:<email>" / "dev:<name>").
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS users (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    external_subject text UNIQUE NOT NULL,
    display_name     text NOT NULL,
    email            text,
    avatar_url       text,
    created_at       timestamptz NOT NULL DEFAULT now(),
    last_seen_at     timestamptz NOT NULL DEFAULT now()
);

-- ai-chattermax's own session (established after an ibeco handshake or dev login).
CREATE TABLE IF NOT EXISTS sessions (
    token       text PRIMARY KEY,
    user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at  timestamptz NOT NULL DEFAULT now(),
    expires_at  timestamptz NOT NULL,
    last_active timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS sessions_user_idx ON sessions(user_id);

-- ---------------------------------------------------------------------------
-- Servers (workspaces) + membership.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS servers (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    slug          text UNIQUE NOT NULL,
    name          text NOT NULL,
    owner_user_id uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    join_token    text UNIQUE NOT NULL DEFAULT encode(gen_random_bytes(12), 'hex'),
    settings      jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS server_members (
    server_id uuid NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    user_id   uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role      text NOT NULL DEFAULT 'member'
              CHECK (role IN ('owner','admin','moderator','member')),
    joined_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (server_id, user_id)
);

-- ---------------------------------------------------------------------------
-- Rooms (channels) + private-room membership.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS rooms (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    server_id  uuid NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    slug       text NOT NULL,
    name       text NOT NULL,
    visibility text NOT NULL DEFAULT 'public' CHECK (visibility IN ('public','private')),
    topic      text,
    created_by uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (server_id, slug)
);

CREATE TABLE IF NOT EXISTS room_members (
    room_id   uuid NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    user_id   uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role      text NOT NULL DEFAULT 'member' CHECK (role IN ('moderator','member')),
    joined_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (room_id, user_id)
);

-- ---------------------------------------------------------------------------
-- Personas — social identity owned by a member; mind lives in pg-ai-stewards.
-- host_ref names the host's persona (e.g. 'dm-assistant'); the persona key
-- links the two (split model — platform owns membership, host owns cognition).
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS personas (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    server_id     uuid NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    owner_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    slug          text NOT NULL,
    display_name  text NOT NULL,
    avatar_url    text,
    host_kind     text NOT NULL DEFAULT 'pg-ai-stewards',
    host_ref      text,
    status        text NOT NULL DEFAULT 'active' CHECK (status IN ('active','disabled')),
    created_at    timestamptz NOT NULL DEFAULT now(),
    UNIQUE (server_id, slug)
);

-- Mint record. Only the hash of the key is stored; the raw key is shown once.
CREATE TABLE IF NOT EXISTS persona_keys (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    persona_id   uuid NOT NULL REFERENCES personas(id) ON DELETE CASCADE,
    key_hash     text NOT NULL UNIQUE,
    label        text,
    scopes       jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at   timestamptz NOT NULL DEFAULT now(),
    last_used_at timestamptz,
    revoked_at   timestamptz
);

-- Room-scoped identity for a persona (D&D PC in-character vs OOC). v2 UI; schema now.
CREATE TABLE IF NOT EXISTS sub_personas (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    persona_id      uuid NOT NULL REFERENCES personas(id) ON DELETE CASCADE,
    room_id         uuid NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    display_name    text NOT NULL,
    prompt_override text,
    created_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (persona_id, room_id)
);

-- Which rooms a persona is granted into (optionally as a sub-persona).
CREATE TABLE IF NOT EXISTS persona_room_grants (
    persona_id     uuid NOT NULL REFERENCES personas(id) ON DELETE CASCADE,
    room_id        uuid NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    sub_persona_id uuid REFERENCES sub_personas(id) ON DELETE SET NULL,
    granted_by     uuid REFERENCES users(id) ON DELETE SET NULL,
    granted_at     timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (persona_id, room_id)
);

-- ---------------------------------------------------------------------------
-- Direct messages — 1:1 human↔human or human↔persona.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS dms (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    server_id  uuid REFERENCES servers(id) ON DELETE CASCADE,
    kind       text NOT NULL CHECK (kind IN ('user_user','user_persona')),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS dm_participants (
    dm_id      uuid NOT NULL REFERENCES dms(id) ON DELETE CASCADE,
    user_id    uuid REFERENCES users(id) ON DELETE CASCADE,
    persona_id uuid REFERENCES personas(id) ON DELETE CASCADE,
    CHECK (num_nonnulls(user_id, persona_id) = 1)
);
CREATE INDEX IF NOT EXISTS dm_participants_dm_idx   ON dm_participants(dm_id);
CREATE INDEX IF NOT EXISTS dm_participants_user_idx ON dm_participants(user_id);

-- ---------------------------------------------------------------------------
-- Messages — in a room XOR a dm; sender is a user XOR a persona.
-- FTS via a generated tsvector column (no trigger needed).
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS messages (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    room_id          uuid REFERENCES rooms(id) ON DELETE CASCADE,
    dm_id            uuid REFERENCES dms(id) ON DELETE CASCADE,
    sender_user_id   uuid REFERENCES users(id) ON DELETE SET NULL,
    sender_persona_id uuid REFERENCES personas(id) ON DELETE SET NULL,
    sub_persona_id   uuid REFERENCES sub_personas(id) ON DELETE SET NULL,
    body             text NOT NULL,
    created_at       timestamptz NOT NULL DEFAULT now(),
    edited_at        timestamptz,
    tsv              tsvector GENERATED ALWAYS AS (to_tsvector('english', coalesce(body,''))) STORED,
    CHECK (num_nonnulls(room_id, dm_id) = 1),
    CHECK (num_nonnulls(sender_user_id, sender_persona_id) = 1)
);
CREATE INDEX IF NOT EXISTS messages_room_idx ON messages(room_id, created_at);
CREATE INDEX IF NOT EXISTS messages_dm_idx   ON messages(dm_id, created_at);
CREATE INDEX IF NOT EXISTS messages_tsv_idx  ON messages USING GIN (tsv);
