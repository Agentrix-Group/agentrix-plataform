CREATE TABLE IF NOT EXISTS tournament_rounds (
 id SERIAL PRIMARY KEY,
 arena_id INT NOT NULL REFERENCES arenas(id),
 idempotency_key TEXT NOT NULL,
 format_version TEXT NOT NULL,
 roster_json JSONB NOT NULL,
 seed BIGINT NOT NULL,
 total_matches INT NOT NULL,
 next_match INT NOT NULL DEFAULT 0,
 status TEXT NOT NULL DEFAULT 'scheduled',
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 UNIQUE(arena_id,idempotency_key)
);
ALTER TABLE matches ADD COLUMN IF NOT EXISTS round_id INT REFERENCES tournament_rounds(id);
ALTER TABLE matches ADD COLUMN IF NOT EXISTS round_match_index INT;
CREATE UNIQUE INDEX IF NOT EXISTS idx_match_round_index ON matches(round_id,round_match_index) WHERE round_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_round_pending ON tournament_rounds(arena_id,id) WHERE status='scheduled';
