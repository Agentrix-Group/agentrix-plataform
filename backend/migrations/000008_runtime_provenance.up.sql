ALTER TABLE matches ADD COLUMN IF NOT EXISTS runtime_sha256 TEXT;
ALTER TABLE tournament_rounds ADD COLUMN IF NOT EXISTS runtime_sha256 TEXT;
-- Legacy work has no trustworthy runtime identity and is rejected at execution.
