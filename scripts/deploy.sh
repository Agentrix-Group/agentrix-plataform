#!/usr/bin/env bash
# ==============================================================================
# Agentrix Platform - Automated Production Deployment Script for Debian / Ubuntu
# ==============================================================================
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

echo "=========================================================="
echo "🚀 Iniciando despliegue de Agentrix Platform"
echo "Directorio base: $ROOT_DIR"
echo "=========================================================="

# 1. Dependencias del sistema
echo "📦 Verificando dependencias del sistema..."
sudo apt-get update -qq
sudo apt-get install -y -qq curl wget jq openssl git

if ! command -v docker &> /dev/null; then
    echo "🐳 Instalando Docker..."
    curl -fsSL https://get.docker.com -o /tmp/get-docker.sh
    sudo sh /tmp/get-docker.sh
    sudo usermod -aG docker "$USER" || true
    rm -f /tmp/get-docker.sh
fi

sudo systemctl enable --now docker

# 2. Configuración de Variables de Entorno (.env)
if [ ! -f "$ROOT_DIR/.env" ]; then
    echo "🔑 Generando archivo .env con credenciales seguras..."
    DB_PASS="$(openssl rand -hex 16)"
    JWT_SEC="$(openssl rand -hex 32)"
    ADMIN_PASS="Admin_$(openssl rand -hex 6)!"

    cat <<EOF > "$ROOT_DIR/.env"
# Generado automáticamente por scripts/deploy.sh
PORT=8080
POSTGRES_PASSWORD=$DB_PASS
DATABASE_URL=postgres://postgres:${DB_PASS}@db:5432/agentrix_platform?sslmode=disable
JWT_SECRET=$JWT_SEC
ARBITER_PATH=/usr/local/bin/agentrix-arbiter
BOTS_DIR=/var/agentrix/bots
REPLAYS_DIR=/var/agentrix/replays
KITS_DIR=/opt/agentrix/kits
AUTO_MATCHMAKER=false
MATCH_INTERVAL_SECONDS=30
BOOTSTRAP_ADMIN_USERNAME=admin
BOOTSTRAP_ADMIN_EMAIL=admin@cocorium.online
BOOTSTRAP_ADMIN_PASSWORD=$ADMIN_PASS
AGENTRIX_DEV_MODE=true
AGENTRIX_RUNTIME_SHA256=0000000000000000000000000000000000000000000000000000000000000000
EOF
    chmod 600 "$ROOT_DIR/.env"
    echo "✅ Archivo .env creado con éxito."
    echo "----------------------------------------------------------"
    echo "👤 Usuario Admin: admin"
    echo "📧 Email Admin:   admin@cocorium.online"
    echo "🔒 Password Admin: $ADMIN_PASS"
    echo "----------------------------------------------------------"
else
    echo "ℹ️ El archivo .env ya existe. Se mantendrán las variables actuales."
fi

# 3. Levantar contenedores Docker Compose
echo "🏗️ Construyendo y levantando contenedores (PostgreSQL, Backend Go, Frontend React)..."
docker compose up -d --build

# 4. Esperar a que el backend esté operativo
echo "⏳ Esperando a que el backend pase las pruebas de salud..."
MAX_RETRIES=30
COUNT=0
HEALTHY=false

while [ $COUNT -lt $MAX_RETRIES ]; do
    if curl -fsS http://127.0.0.1:8080/api/v1/system/status 2>/dev/null | grep -q '"status":"operational"'; then
        HEALTHY=true
        break
    fi
    COUNT=$((COUNT + 1))
    sleep 2
done

if [ "$HEALTHY" = true ]; then
    echo "✅ Backend y Base de Datos operativos al 100%!"
else
    echo "⚠️ El backend tardó más de lo esperado. Revisa los logs con: docker compose logs backend"
fi

# 5. Verificar Frontend en puerto 3000
if curl -fsS http://127.0.0.1:3000 > /dev/null 2>&1; then
    echo "✅ Frontend React listo en http://127.0.0.1:3000"
fi

# 6. Preparar Cloudflare Tunnel
echo "🌐 Verificando Cloudflare Tunnel (cloudflared)..."
if ! command -v cloudflared &> /dev/null; then
    echo "Instalando cloudflared..."
    curl -fsSL https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64.deb -o /tmp/cloudflared.deb
    sudo dpkg -i /tmp/cloudflared.deb || sudo apt-get install -f -y
    rm -f /tmp/cloudflared.deb
fi

echo "=========================================================="
echo "🎉 ¡Despliegue local de Agentrix Platform completado!"
echo "Servicios activos:"
echo "  - Frontend (Web): http://localhost:3000"
echo "  - Backend API:    http://localhost:8080/api/v1"
echo "  - Base de datos:  PostgreSQL en db:5432 (local 127.0.0.1:5433)"
echo "=========================================================="
