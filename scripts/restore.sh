#!/bin/bash
# ==============================================================================
# Agentrix Platform - Automated Enterprise Restore Script
# Restores PostgreSQL snapshot, replays, and bots with integrity checks.
# ==============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

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

list_backups() {
    log_info "Respaldos disponibles en $BACKUP_DIR:"
    if ls -1t "$BACKUP_DIR"/agentrix_db_*.sql.gz 2>/dev/null; then
        echo ""
        log_info "Uso: bash $0 <ruta_al_archivo_db.sql.gz>"
    else
        log_warn "No se encontraron respaldos en $BACKUP_DIR."
    fi
    exit 1
}

if [ $# -eq 0 ]; then
    list_backups
fi

TARGET_FILE="$1"
if [ ! -f "$TARGET_FILE" ]; then
    # Intenta resolver dentro de BACKUP_DIR
    if [ -f "$BACKUP_DIR/$TARGET_FILE" ]; then
        TARGET_FILE="$BACKUP_DIR/$TARGET_FILE"
    else
        log_error "Archivo de respaldo no encontrado: $TARGET_FILE"
        exit 1
    fi
fi

log_info "Iniciando restauración de Agentrix Platform desde: $TARGET_FILE"

# 1. Cierre seguro de conexiones activas a la base de datos
log_info "Cerrando conexiones activas a $DB_NAME..."
psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d postgres -c "
    SELECT pg_terminate_backend(pid)
    FROM pg_stat_activity
    WHERE datname = '$DB_NAME' AND pid <> pg_backend_pid();
" >/dev/null 2>&1 || true

# 2. Restauración de Base de Datos
log_info "Restaurando base de datos PostgreSQL ($DB_NAME)..."
if gzip -dc "$TARGET_FILE" | psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -v ON_ERROR_STOP=1 >/dev/null 2>&1; then
    log_success "Base de datos restaurada correctamente."
else
    log_error "Error al aplicar el archivo SQL en PostgreSQL."
    exit 1
fi

# 3. Restaurar repeticiones si existe archivo complementario
BASE_NAME=$(basename "$TARGET_FILE" | sed -E 's/(agentrix_db_|\.sql\.gz)//g')
REPLAYS_TAR="${BACKUP_DIR}/agentrix_replays_${BASE_NAME}.tar.gz"
if [ -f "$REPLAYS_TAR" ]; then
    log_info "Restaurando directorio de repeticiones desde $(basename "$REPLAYS_TAR")..."
    mkdir -p "$ROOT_DIR/var/agentrix"
    tar -xzf "$REPLAYS_TAR" -C "$ROOT_DIR/var/agentrix"
    log_success "Repeticiones restauradas."
fi

# 4. Restaurar bots si existe archivo complementario
BOTS_TAR="${BACKUP_DIR}/agentrix_bots_${BASE_NAME}.tar.gz"
if [ -f "$BOTS_TAR" ]; then
    log_info "Restaurando artefactos de bots desde $(basename "$BOTS_TAR")..."
    mkdir -p "$ROOT_DIR/var/agentrix"
    tar -xzf "$BOTS_TAR" -C "$ROOT_DIR/var/agentrix"
    log_success "Artefactos de bots restaurados."
fi

# 5. Comprobación de integridad post-restauración
log_info "Verificando integridad post-restauración..."
USERS_COUNT=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -tAc "SELECT count(*) FROM users;")
ARENAS_COUNT=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -tAc "SELECT count(*) FROM arenas;")
LADDER_COUNT=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -tAc "SELECT count(*) FROM ladder_entries;")

log_success "Verificación completada: $USERS_COUNT usuarios, $ARENAS_COUNT arenas, $LADDER_COUNT bots en el ranking."
log_success "Restauración integral de Agentrix finalizada con éxito."
