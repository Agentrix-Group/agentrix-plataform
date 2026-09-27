DROP INDEX IF EXISTS idx_round_pending;
DROP INDEX IF EXISTS idx_match_round_index;
ALTER TABLE matches DROP COLUMN IF EXISTS round_match_index;
ALTER TABLE matches DROP COLUMN IF EXISTS round_id;
DROP TABLE IF EXISTS tournament_rounds;
