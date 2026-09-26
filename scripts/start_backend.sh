#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

echo "=== Starting Agentrix Go Backend Monolith ==="
cd "$ROOT_DIR/backend"
go build -o bin/agentrix-server ./cmd/agentrix-server

export PORT="${PORT:-8080}"
export DATABASE_URL="${DATABASE_URL:-postgres://postgres@127.0.0.1:5432/agentrix_platform?sslmode=disable}"
if [ -f "$ROOT_DIR/simulation/arbiter/target/release/agentrix-arbiter" ]; then
    DEFAULT_ARBITER="$ROOT_DIR/simulation/arbiter/target/release/agentrix-arbiter"
else
    DEFAULT_ARBITER="$ROOT_DIR/agentrix/arbiter/target/release/agentrix-arbiter"
fi
export ARBITER_PATH="${ARBITER_PATH:-$DEFAULT_ARBITER}"
export AUTO_MATCHMAKER="${AUTO_MATCHMAKER:-true}"

echo "Listening on http://0.0.0.0:$PORT"
exec ./bin/agentrix-server
