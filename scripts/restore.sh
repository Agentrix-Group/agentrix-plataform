#!/bin/bash
# Recovery only into a NEW DB and NEW directories; legacy in-place restore disabled.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
BACKUP_DIR="${BACKUP_DIR:-$ROOT_DIR/var/agentrix/backups}"

if [[ "${1:-}" == "--recover" ]]; then
    shift
    exec python3 "$SCRIPT_DIR/restore_bundle.py" "$@"
fi

if [[ $# -ne 2 || "$1" != "--verify-only" ]]; then
    echo "Uso: bash $0 --verify-only <agentrix_db_YYYYMMDD_HHMMSS.sql.gz>" >&2
    echo "Para recuperación en destinos NUEVOS: bash $0 --recover --help" >&2
    exit 2
fi

TARGET_FILE="$2"
if [[ ! -f "$TARGET_FILE" && -f "$BACKUP_DIR/$TARGET_FILE" ]]; then
    TARGET_FILE="$BACKUP_DIR/$TARGET_FILE"
fi
NAME="$(basename "$TARGET_FILE")"
if [[ ! "$NAME" =~ ^agentrix_db_([0-9]{8}_[0-9]{6})\.sql\.gz$ ]]; then
    echo "Nombre de respaldo no reconocido." >&2
    exit 2
fi
STAMP="${BASH_REMATCH[1]}"
MANIFEST="$(dirname "$TARGET_FILE")/manifest_${STAMP}.json"
python3 "$SCRIPT_DIR/verify_backup.py" "$MANIFEST" --database-file "$TARGET_FILE"
echo "Integridad estructural verificada; no se modificaron BD ni archivos. Los hashes no autentican el origen del SQL."
