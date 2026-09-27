#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

echo "=== Starting Agentrix Go Backend Monolith ==="
: "${JWT_SECRET:?Set a private JWT_SECRET before starting Agentrix}"
: "${DATABASE_URL:?Set an explicit DATABASE_URL before starting Agentrix}"
: "${AGENTRIX_RUNTIME_ROOT:?Set the dedicated sandbox runtime root before starting Agentrix}"
: "${AGENTRIX_RUNTIME_SHA256:?Set the pinned SHA-256 digest of the dedicated sandbox runtime}"
if [ ! -d "$AGENTRIX_RUNTIME_ROOT" ]; then
    echo "Sandbox runtime directory is unavailable." >&2
    exit 1
fi
export ARBITER_PATH="${ARBITER_PATH:-$ROOT_DIR/simulation/arbiter/target/release/agentrix-arbiter}"
if [ ! -x "$ARBITER_PATH" ]; then
    echo "Arbiter is unavailable; run make build or configure ARBITER_PATH explicitly." >&2
    exit 1
fi
cd "$ROOT_DIR/backend"
go build -o bin/agentrix-server ./cmd/agentrix-server

export PORT="${PORT:-8080}"
export AUTO_MATCHMAKER="${AUTO_MATCHMAKER:-true}"

echo "Listening on http://0.0.0.0:$PORT"
exec ./bin/agentrix-server
