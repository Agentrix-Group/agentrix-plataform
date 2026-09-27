ALTER TABLE tournament_rounds DROP COLUMN IF EXISTS rules_sha256;
ALTER TABLE tournament_rounds DROP COLUMN IF EXISTS rules_json;
ALTER TABLE matches DROP COLUMN IF EXISTS rules_sha256;
ALTER TABLE matches DROP COLUMN IF EXISTS rules_json;
