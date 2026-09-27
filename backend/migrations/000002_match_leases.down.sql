DROP INDEX IF EXISTS idx_matches_expired;
DROP INDEX IF EXISTS idx_matches_queue;
ALTER TABLE matches DROP COLUMN IF EXISTS attempt;
ALTER TABLE matches DROP COLUMN IF EXISTS heartbeat_at;
ALTER TABLE matches DROP COLUMN IF EXISTS lease_expires_at;
ALTER TABLE matches DROP COLUMN IF EXISTS lease_owner;
