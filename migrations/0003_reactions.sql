-- REM-1: message reactions. Humans react from the UI (hover palette); personas
-- react through their gateway connection (the persona-host's 👀 "eyes" marker
-- rides this same table). Exactly one reactor column is set. Idempotent.
CREATE TABLE IF NOT EXISTS message_reactions (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  message_id  uuid NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
  reactor_user_id    uuid REFERENCES users(id)    ON DELETE CASCADE,
  reactor_persona_id uuid REFERENCES personas(id) ON DELETE CASCADE,
  emoji       text NOT NULL,
  created_at  timestamptz NOT NULL DEFAULT now(),
  CHECK ((reactor_user_id IS NULL) <> (reactor_persona_id IS NULL))
);

-- One reaction per (message, emoji, reactor) — re-adding is a no-op.
CREATE UNIQUE INDEX IF NOT EXISTS ux_message_reactions
  ON message_reactions (message_id, emoji, COALESCE(reactor_user_id, reactor_persona_id));

CREATE INDEX IF NOT EXISTS ix_message_reactions_message
  ON message_reactions (message_id);
