# ==============================================================================
# Agentrix Platform - Master Makefile
# ==============================================================================

.PHONY: all build up down run-local test test-e2e validate migrate backup restore provision help

all: build

help:
	@echo "Agentrix Platform Commands:"
	@echo "  make build       - Compiles Rust arbiter, Go server, and React SPA"
	@echo "  make run-local   - Runs backend (:8080) and frontend (:3000) locally"
	@echo "  make migrate     - Applies all 8 database migrations to PostgreSQL"
	@echo "  make up          - Starts full stack in Docker Compose"
	@echo "  make down        - Stops Docker Compose stack"
	@echo "  make test        - Runs Go, Frontend, and Python unit test suites"
	@echo "  make test-e2e    - Runs full sandboxed E2E acceptance suite (requires probe runtime)"
	@echo "  make validate    - Runs pre-flight environment and sandbox validation"
	@echo "  make backup      - Generates automated PostgreSQL and artifact backup"
	@echo "  make restore     - Verifies backup integrity (use BACKUP=filename.sql.gz [RECOVER=1])"
	@echo "  make provision   - Provisions tournament and runs benchmark calibration"

build:
	@echo "=== Building Rust Simulation Arbiter ==="
	cd simulation/arbiter && cargo build --release --locked
	@echo "=== Building Go Backend Monolith ==="
	cd backend && go build -o bin/agentrix-server ./cmd/agentrix-server
	@echo "=== Building React SPA ==="
	cd frontend && npm run build

run-local:
	@bash scripts/start_all.sh

migrate:
	@echo "=== Running Database Migrations ==="
	cd backend && go run ./cmd/agentrix-server --migrate-only

up:
	docker compose up -d --build

down:
	docker compose down

test:
	@echo "=== Running Go Unit Tests ==="
	cd backend && go test ./...
	@echo "=== Running Frontend Unit Tests ==="
	cd frontend && npm test
	@echo "=== Running Operational Script Tests ==="
	python3 -m unittest discover -s scripts -p "test_*.py"

test-e2e:
	@echo "=== Running Platform End-to-End Suite ==="
	python3 scripts/test_platform_e2e.py

validate:
	python3 scripts/validate_arena.py

backup:
	@bash scripts/backup.sh

restore:
ifdef RECOVER
	@bash scripts/restore.sh --recover $(BACKUP)
else
	@bash scripts/restore.sh --verify-only $(BACKUP)
endif

provision:
	python3 scripts/provision_tournament.py --rounds 3
