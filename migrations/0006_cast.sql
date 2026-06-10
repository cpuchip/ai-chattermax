-- DH-2: the cast system. sub_personas becomes a persona's CAST in a room —
-- many named characters (shopkeeps, villains, mobs, PCs) voiced through one
-- registered persona. The 0001 UNIQUE allowed a single alias per room (the
-- original "in-character vs OOC" idea); a cast needs many, unique by name.
ALTER TABLE sub_personas DROP CONSTRAINT IF EXISTS sub_personas_persona_id_room_id_key;
CREATE UNIQUE INDEX IF NOT EXISTS ux_sub_personas_name
  ON sub_personas (persona_id, room_id, lower(display_name));
