ALTER TABLE matches ADD COLUMN IF NOT EXISTS lease_owner TEXT;
ALTER TABLE matches ADD COLUMN IF NOT EXISTS lease_expires_at TIMESTAMPTZ;
ALTER TABLE matches ADD COLUMN IF NOT EXISTS heartbeat_at TIMESTAMPTZ;
ALTER TABLE matches ADD COLUMN IF NOT EXISTS attempt INT NOT NULL DEFAULT 0;
-- After an upgrade, wait out the old sandbox lifetime before recovering work
-- that predates lease ownership. Never immediately replay an unknown live job.
UPDATE matches SET lease_owner='legacy-unowned', lease_expires_at=now()+interval '15 minutes'
WHERE status='running' AND lease_expires_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_matches_queue ON matches(created_at, id) WHERE status='scheduled';
CREATE INDEX IF NOT EXISTS idx_matches_expired ON matches(lease_expires_at) WHERE status='running';
