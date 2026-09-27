-- Exact-path reference checks support safe startup reconciliation of stale
-- immutable replay attempts that were written before a worker/crash boundary.
CREATE INDEX IF NOT EXISTS idx_replays_file_path ON replays(file_path);
