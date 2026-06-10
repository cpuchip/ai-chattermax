-- DH-1/D8: initiative & turn order — a room-scoped table mechanic. One ACTIVE
-- round per room; entries are named combatants (humans, personas, cast
-- members). The server rolls (same fairness story as /roll). Idempotent.
CREATE TABLE IF NOT EXISTS initiative_rounds (
  id                    uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  room_id               uuid NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
  started_by_user_id    uuid REFERENCES users(id)    ON DELETE SET NULL,
  started_by_persona_id uuid REFERENCES personas(id) ON DELETE SET NULL,
  round                 integer NOT NULL DEFAULT 1,
  current_entry_id      uuid,
  active                boolean NOT NULL DEFAULT true,
  created_at            timestamptz NOT NULL DEFAULT now(),
  ended_at              timestamptz
);
CREATE UNIQUE INDEX IF NOT EXISTS ux_initiative_active
  ON initiative_rounds (room_id) WHERE active;

CREATE TABLE IF NOT EXISTS initiative_entries (
  id        uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  round_id  uuid NOT NULL REFERENCES initiative_rounds(id) ON DELETE CASCADE,
  name      text NOT NULL,
  modifier  integer NOT NULL DEFAULT 0,
  roll      integer NOT NULL,
  total     integer NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (round_id, name)
);
