-- REM-3: @mentions + notifications + persona respond_policy + human mood.
-- Idempotent.

-- How a persona decides to take a turn:
--   all       — turn on every human message (the model may answer SILENCE)
--   mentioned — turn only when the message names/addresses the persona
--   judgment  — like all, but the framing explicitly licenses chiming in
ALTER TABLE personas ADD COLUMN IF NOT EXISTS respond_policy text NOT NULL DEFAULT 'all';
DO $$ BEGIN
  ALTER TABLE personas ADD CONSTRAINT personas_respond_policy_chk
    CHECK (respond_policy IN ('all', 'mentioned', 'judgment'));
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- Human mood: an emoji status shown next to the name in the roster.
ALTER TABLE users ADD COLUMN IF NOT EXISTS mood text NOT NULL DEFAULT '';

-- Mention notifications (humans). One row per mentioned user per message.
CREATE TABLE IF NOT EXISTS notifications (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id    uuid NOT NULL REFERENCES users(id)    ON DELETE CASCADE,
  kind       text NOT NULL DEFAULT 'mention',
  message_id uuid REFERENCES messages(id) ON DELETE CASCADE,
  room_id    uuid REFERENCES rooms(id)    ON DELETE CASCADE,
  created_at timestamptz NOT NULL DEFAULT now(),
  read_at    timestamptz
);
CREATE INDEX IF NOT EXISTS ix_notifications_user
  ON notifications (user_id, created_at DESC);
