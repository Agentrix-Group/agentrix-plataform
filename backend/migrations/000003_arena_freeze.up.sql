CREATE TABLE IF NOT EXISTS arena_freezes (
    arena_id INT PRIMARY KEY REFERENCES arenas(id) ON DELETE CASCADE,
    frozen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ladder_json JSONB NOT NULL
);
CREATE TABLE IF NOT EXISTS arena_frozen_matches (
    arena_id INT NOT NULL REFERENCES arena_freezes(arena_id) ON DELETE CASCADE,
    match_id INT NOT NULL REFERENCES matches(id) ON DELETE CASCADE,
    PRIMARY KEY(arena_id,match_id)
);
