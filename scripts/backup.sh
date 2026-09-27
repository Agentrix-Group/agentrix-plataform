#!/bin/bash
# Offline backup only: the caller must stop the API and worker first.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
exec python3 "$SCRIPT_DIR/backup_bundle.py" \
    --backup-dir "${BACKUP_DIR:-$ROOT_DIR/var/agentrix/backups}" \
    --bots-dir "${BOTS_DIR:-$ROOT_DIR/var/agentrix/bots}" \
    --replays-dir "${REPLAYS_DIR:-$ROOT_DIR/var/agentrix/replays}" "$@"
