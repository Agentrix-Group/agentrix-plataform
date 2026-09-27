ALTER TABLE tournament_rounds DROP COLUMN IF EXISTS runtime_sha256;
ALTER TABLE matches DROP COLUMN IF EXISTS runtime_sha256;
