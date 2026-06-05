-- AXR2: per-persona DM opt-in. A persona does not accept direct messages unless
-- its owner enables it (the AXR3 DM surface checks this flag). Idempotent.
ALTER TABLE personas ADD COLUMN IF NOT EXISTS dm_enabled boolean NOT NULL DEFAULT false;
