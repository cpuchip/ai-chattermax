-- DH-2 follow-up: cast names are ROOM-unique, not persona-unique. At a table,
-- one name is one character — the 2026-06-10 Holodeck-3 session ended up with
-- two "Grimble the shopkeep"s (the DM's and Starlet's) answering the same
-- customer. Dedupe keeps the OLDEST claim (first speaker owns the name), then
-- the unique index makes it structural. Idempotent.
DELETE FROM sub_personas sp
USING sub_personas keep
WHERE keep.room_id = sp.room_id
  AND lower(keep.display_name) = lower(sp.display_name)
  AND keep.created_at < sp.created_at;

DROP INDEX IF EXISTS ux_sub_personas_name;
CREATE UNIQUE INDEX IF NOT EXISTS ux_sub_personas_room_name
  ON sub_personas (room_id, lower(display_name));
