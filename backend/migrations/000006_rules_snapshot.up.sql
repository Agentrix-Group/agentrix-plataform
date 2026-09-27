ALTER TABLE arenas ALTER COLUMN max_ticks SET DEFAULT 10800;
ALTER TABLE matches ADD COLUMN IF NOT EXISTS rules_json TEXT;
ALTER TABLE matches ADD COLUMN IF NOT EXISTS rules_sha256 TEXT;
ALTER TABLE tournament_rounds ADD COLUMN IF NOT EXISTS rules_json TEXT;
ALTER TABLE tournament_rounds ADD COLUMN IF NOT EXISTS rules_sha256 TEXT;
-- Existing jobs intentionally receive no inferred rules; execution fails closed.
-- Operators must explicitly reschedule legacy jobs after correcting arena rules.
