-- ==============================================================================
-- AGENTRIX GREENFIELD SCHEMA: MIGRATION 000001
-- Clean, modern relational schema for multi-agent game contests
-- Strictly decoupled from ICPC concepts (zero testcases, zero balloons, zero round freezes)
-- ==============================================================================

CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(64) UNIQUE NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    role VARCHAR(32) NOT NULL DEFAULT 'player', -- 'admin', 'player'
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS api_keys (
    id SERIAL PRIMARY KEY,
    user_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    key_hash VARCHAR(64) UNIQUE NOT NULL,
    key_prefix VARCHAR(16) NOT NULL,
    name VARCHAR(128) NOT NULL,
    last_used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS teams (
    id SERIAL PRIMARY KEY,
    name VARCHAR(128) UNIQUE NOT NULL,
    affiliation VARCHAR(128) NOT NULL DEFAULT '',
    owner_id INT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS arenas (
    id SERIAL PRIMARY KEY,
    slug VARCHAR(64) UNIQUE NOT NULL,
    name VARCHAR(128) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    game_type VARCHAR(64) NOT NULL DEFAULT 'battle_royale_5p',
    max_players INT NOT NULL DEFAULT 5,
    max_ticks INT NOT NULL DEFAULT 1000,
    config_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS agent_versions (
    id SERIAL PRIMARY KEY,
    team_id INT NOT NULL REFERENCES teams(id) ON DELETE RESTRICT,
    arena_id INT NOT NULL REFERENCES arenas(id) ON DELETE RESTRICT,
    name VARCHAR(128) NOT NULL,
    version INT NOT NULL DEFAULT 1,
    runtime VARCHAR(64) NOT NULL DEFAULT 'python-standard',
    entrypoint VARCHAR(255) NOT NULL DEFAULT 'python3 agent.py',
    artifact_path TEXT NOT NULL,
    sha256 VARCHAR(64) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'active', -- 'pending_validation', 'active', 'disqualified', 'retired'
    failure_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_agent_team_arena_version UNIQUE(team_id, arena_id, version)
);

CREATE TABLE IF NOT EXISTS ladder_entries (
    id SERIAL PRIMARY KEY,
    arena_id INT NOT NULL REFERENCES arenas(id) ON DELETE CASCADE,
    agent_version_id INT NOT NULL REFERENCES agent_versions(id) ON DELETE CASCADE,
    rating_mu DOUBLE PRECISION NOT NULL DEFAULT 1500.0,
    rating_sigma DOUBLE PRECISION NOT NULL DEFAULT 350.0,
    display_rating INT NOT NULL DEFAULT 1500,
    matches_played INT NOT NULL DEFAULT 0,
    wins INT NOT NULL DEFAULT 0,
    kills INT NOT NULL DEFAULT 0,
    survival_ticks_total BIGINT NOT NULL DEFAULT 0,
    last_match_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_ladder_arena_agent UNIQUE(arena_id, agent_version_id)
);

CREATE TABLE IF NOT EXISTS matches (
    id SERIAL PRIMARY KEY,
    arena_id INT NOT NULL REFERENCES arenas(id) ON DELETE RESTRICT,
    seed BIGINT NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'scheduled', -- 'scheduled', 'running', 'finished', 'failed'
    ticks_played INT NOT NULL DEFAULT 0,
    winner_agent_id INT REFERENCES agent_versions(id) ON DELETE SET NULL,
    error_message TEXT,
    execution_log TEXT,
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS match_participants (
    id SERIAL PRIMARY KEY,
    match_id INT NOT NULL REFERENCES matches(id) ON DELETE CASCADE,
    agent_version_id INT NOT NULL REFERENCES agent_versions(id) ON DELETE RESTRICT,
    seat INT NOT NULL,
    rank_place INT NOT NULL,
    kills INT NOT NULL DEFAULT 0,
    survival_ticks INT NOT NULL DEFAULT 0,
    score DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    disqualified BOOLEAN NOT NULL DEFAULT false,
    disqualification_reason TEXT,
    old_rating DOUBLE PRECISION NOT NULL DEFAULT 1500.0,
    new_rating DOUBLE PRECISION NOT NULL DEFAULT 1500.0,
    rating_delta DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_match_seat UNIQUE(match_id, seat),
    CONSTRAINT uq_match_agent UNIQUE(match_id, agent_version_id)
);

CREATE TABLE IF NOT EXISTS replays (
    id SERIAL PRIMARY KEY,
    match_id INT UNIQUE NOT NULL REFERENCES matches(id) ON DELETE CASCADE,
    file_path TEXT NOT NULL,
    sha256 VARCHAR(64) NOT NULL,
    size_bytes BIGINT NOT NULL DEFAULT 0,
    tick_count INT NOT NULL DEFAULT 0,
    summary_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS audit_logs (
    id SERIAL PRIMARY KEY,
    user_id INT REFERENCES users(id) ON DELETE SET NULL,
    action VARCHAR(64) NOT NULL,
    target_type VARCHAR(64) NOT NULL,
    target_id VARCHAR(64) NOT NULL,
    ip_address VARCHAR(45) NOT NULL DEFAULT '',
    details_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS system_settings (
    key VARCHAR(64) PRIMARY KEY,
    value_json JSONB NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indices for performance
CREATE INDEX IF NOT EXISTS idx_ladder_display_rating ON ladder_entries(arena_id, display_rating DESC);
CREATE INDEX IF NOT EXISTS idx_matches_arena_status ON matches(arena_id, status);
CREATE INDEX IF NOT EXISTS idx_match_participants_agent ON match_participants(agent_version_id);
CREATE INDEX IF NOT EXISTS idx_agent_versions_arena ON agent_versions(arena_id, status);
CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON audit_logs(created_at DESC);
