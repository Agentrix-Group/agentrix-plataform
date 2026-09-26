#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

echo "=================================================="
echo "      AGENTRIX MULTI-AGENT PLATFORM - START       "
echo "=================================================="

# Function to kill all children on exit
cleanup() {
    echo ""
    echo "Shutting down Agentrix services..."
    kill 0
}
trap cleanup EXIT INT TERM

# 1. Start Go Backend Monolith
"$SCRIPT_DIR/start_backend.sh" &
BACKEND_PID=$!
echo "Backend running (PID: $BACKEND_PID)"

# Wait for backend to be ready
until curl -s http://127.0.0.1:8080/api/v1/system/status >/dev/null 2>&1; do
    sleep 1
done
echo "Backend REST API ready at http://localhost:8080"

# 2. Start React Frontend SPA
"$SCRIPT_DIR/start_frontend.sh" &
FRONTEND_PID=$!
echo "Frontend running (PID: $FRONTEND_PID)"

echo "=================================================="
echo "  Frontend SPA: http://localhost:3000             "
echo "  Backend API:  http://localhost:8080/api/v1      "
echo "  Admin Login:  admin / admin123                  "
echo "=================================================="
echo "Press Ctrl+C to stop all services."

wait
