# ==============================================================================
# Agentrix Platform - Master Makefile
# ==============================================================================

.PHONY: all build up down run-local test validate backup restore provision help

all: build

help:
	@echo "Agentrix Platform Commands:"
	@echo "  make build       - Compiles Rust arbiter, Go server, and React SPA"
	@echo "  make run-local   - Runs backend (:8080) and frontend (:3000) locally"
	@echo "  make up          - Starts full stack in Docker Compose"
	@echo "  make down        - Stops Docker Compose stack"
	@echo "  make test        - Runs Go unit tests and Python E2E integration suite"
	@echo "  make validate    - Runs pre-flight environment and sandbox validation"
	@echo "  make backup      - Generates automated PostgreSQL and artifact backup"
	@echo "  make restore     - Restores from backup (use BACKUP=filename.sql.gz)"
	@echo "  make provision   - Provisions tournament and runs benchmark calibration"

build:
	@echo "=== Building Rust Simulation Arbiter ==="
	cd simulation/arbiter && cargo build --release
	@echo "=== Building Go Backend Monolith ==="
	cd backend && go build -o bin/agentrix-server ./cmd/agentrix-server
	@echo "=== Building React SPA ==="
	cd frontend && npm run build

run-local:
	@bash scripts/start_all.sh

up:
	docker compose up -d --build

down:
	docker compose down

test:
	@echo "=== Running Go Unit Tests ==="
	cd backend && go test ./... -v
	@echo "=== Running Platform End-to-End Suite ==="
	python3 scripts/test_platform_e2e.py

validate:
	python3 scripts/validate_arena.py

backup:
	@bash scripts/backup.sh

restore:
	@bash scripts/restore.sh $(BACKUP)

provision:
	python3 scripts/provision_tournament.py --rounds 3
