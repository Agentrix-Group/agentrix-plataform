#!/bin/bash
# ==============================================================================
# Agentrix Platform - Automated Enterprise Backup Script
# Creates transactional database snapshots, replay archives, and bot artifacts
# with SHA-256 cryptographic verification manifests.
# ==============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

# Configuration
BACKUP_DIR="${BACKUP_DIR:-$ROOT_DIR/var/agentrix/backups}"
REPLAYS_DIR="${REPLAYS_DIR:-$ROOT_DIR/var/agentrix/replays}"
BOTS_DIR="${BOTS_DIR:-$ROOT_DIR/var/agentrix/bots}"

DB_HOST="${DB_HOST:-127.0.0.1}"
DB_PORT="${DB_PORT:-5432}"
DB_USER="${DB_USER:-postgres}"
DB_NAME="${DB_NAME:-agentrix_platform}"

C_RESET='\033[0m'
C_RED='\033[0;31m'
C_GREEN='\033[0;32m'
C_YELLOW='\033[1;33m'
C_BLUE='\033[0;34m'
C_BOLD='\033[1m'

log_info()    { echo -e "${C_BLUE}${C_BOLD}[INFO]${C_RESET} $*"; }
log_success() { echo -e "${C_GREEN}${C_BOLD}[SUCCESS]${C_RESET} $*"; }
log_warn()    { echo -e "${C_YELLOW}${C_BOLD}[WARN]${C_RESET} $*"; }
log_error()   { echo -e "${C_RED}${C_BOLD}[ERROR]${C_RESET} $*" >&2; }

mkdir -p "$BACKUP_DIR"

TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_SQL="${BACKUP_DIR}/agentrix_db_${TIMESTAMP}.sql.gz"
BACKUP_REPLAYS="${BACKUP_DIR}/agentrix_replays_${TIMESTAMP}.tar.gz"
BACKUP_BOTS="${BACKUP_DIR}/agentrix_bots_${TIMESTAMP}.tar.gz"
MANIFEST_FILE="${BACKUP_DIR}/manifest_${TIMESTAMP}.json"

log_info "Iniciando respaldo integral de Agentrix Platform (${TIMESTAMP})..."

# 1. Respaldo de Base de Datos PostgreSQL
log_info "Generando volcado de PostgreSQL (${DB_NAME}@${DB_HOST}:${DB_PORT})..."
if pg_dump -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" \
    --clean --if-exists --no-owner --no-privileges | gzip -9 > "$BACKUP_SQL"; then

    if [ ! -s "$BACKUP_SQL" ]; then
        log_error "El archivo de respaldo SQL generado está vacío."
        rm -f "$BACKUP_SQL"
        exit 1
    fi
    SQL_SIZE=$(du -h "$BACKUP_SQL" | cut -f1)
    SQL_SHA=$(sha256sum "$BACKUP_SQL" | awk '{print $1}')
    log_success "Base de datos respaldada: $(basename "$BACKUP_SQL") ($SQL_SIZE | SHA: ${SQL_SHA:0:12}...)"
else
    log_error "Fallo al ejecutar pg_dump."
    rm -f "$BACKUP_SQL"
    exit 1
fi

# 2. Respaldo de Repeticiones (Replays)
REPLAYS_SIZE="0B"
REPLAYS_SHA="none"
if [ -d "$REPLAYS_DIR" ] && [ "$(ls -A "$REPLAYS_DIR" 2>/dev/null)" ]; then
    log_info "Empaquetando repeticiones de partidas..."
    tar -czf "$BACKUP_REPLAYS" -C "$(dirname "$REPLAYS_DIR")" "$(basename "$REPLAYS_DIR")"
    REPLAYS_SIZE=$(du -h "$BACKUP_REPLAYS" | cut -f1)
    REPLAYS_SHA=$(sha256sum "$BACKUP_REPLAYS" | awk '{print $1}')
    log_success "Repeticiones respaldadas: $(basename "$BACKUP_REPLAYS") ($REPLAYS_SIZE)"
else
    log_warn "Directorio de repeticiones vacío o ausente, omitiendo archivo de replays."
fi

# 3. Respaldo de Artefactos de Bots
BOTS_SIZE="0B"
BOTS_SHA="none"
if [ -d "$BOTS_DIR" ] && [ "$(ls -A "$BOTS_DIR" 2>/dev/null)" ]; then
    log_info "Empaquetando artefactos de bots subidos..."
    tar -czf "$BACKUP_BOTS" -C "$(dirname "$BOTS_DIR")" "$(basename "$BOTS_DIR")"
    BOTS_SIZE=$(du -h "$BACKUP_BOTS" | cut -f1)
    BOTS_SHA=$(sha256sum "$BACKUP_BOTS" | awk '{print $1}')
    log_success "Artefactos de bots respaldados: $(basename "$BACKUP_BOTS") ($BOTS_SIZE)"
else
    log_warn "Directorio de bots vacío, omitiendo archivo de bots."
fi

# 4. Generación de Manifiesto Criptográfico
cat <<EOF > "$MANIFEST_FILE"
{
  "timestamp": "${TIMESTAMP}",
  "database": {
    "name": "${DB_NAME}",
    "file": "$(basename "$BACKUP_SQL")",
    "size": "${SQL_SIZE}",
    "sha256": "${SQL_SHA}"
  },
  "replays": {
    "file": "$(basename "$BACKUP_REPLAYS")",
    "size": "${REPLAYS_SIZE}",
    "sha256": "${REPLAYS_SHA}"
  },
  "bots": {
    "file": "$(basename "$BACKUP_BOTS")",
    "size": "${BOTS_SIZE}",
    "sha256": "${BOTS_SHA}"
  },
  "created_at": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
}
EOF

log_success "Manifiesto de verificación guardado en: $(basename "$MANIFEST_FILE")"
log_success "Respaldo completado exitosamente en: $BACKUP_DIR"
