#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

echo "=== Starting Agentrix React Frontend SPA ==="
cd "$ROOT_DIR/frontend"

# Ensure dependencies installed
if [ ! -d "node_modules" ]; then
    npm install
fi

echo "Serving Vite Dev Server on http://localhost:3000..."
exec npm run dev -- --host 0.0.0.0 --port 3000
