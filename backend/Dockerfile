# ==============================================================================
# Agentrix Platform - Backend & Simulation Engine Multi-Stage Dockerfile
# Combines Go Monolith Server, Rust Arbiter Simulation Engine,
# Bubblewrap Sandboxing, and Pre-installed AI Agent Runtimes.
# ==============================================================================

# --- Stage 1: Build Rust Arbiter ---
FROM rust:1.80-slim-bookworm AS rust-builder
WORKDIR /build/arbiter
COPY agentrix/arbiter/Cargo.toml agentrix/arbiter/Cargo.lock ./
COPY agentrix/arbiter/src ./src
RUN cargo build --release --locked

# --- Stage 2: Build Go Monolith Server ---
FROM golang:1.24-bookworm AS go-builder
WORKDIR /build/server
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/cmd ./cmd
COPY backend/internal ./internal
COPY backend/migrations ./migrations
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /build/bin/agentrix-server ./cmd/agentrix-server

# --- Stage 3: Runtime Container ---
FROM python:3.11-slim-bookworm AS runtime

# Install system dependencies, Bubblewrap sandbox, and essential runtimes
RUN apt-get update && apt-get install -y --no-install-recommends \
    bubblewrap \
    ca-certificates \
    postgresql-client \
    curl \
    gzip \
    tar \
    gcc \
    g++ \
    make \
    libc6-dev \
    && rm -rf /var/lib/apt/lists/*

# Install standard AI and numeric packages for bot execution
RUN pip install --no-cache-dir --upgrade pip && \
    pip install --no-cache-dir numpy scipy

# Create standard working directories
RUN mkdir -p /var/agentrix/bots /var/agentrix/replays /var/agentrix/backups /opt/agentrix

# Copy compiled binaries from previous build stages
COPY --from=rust-builder /build/arbiter/target/release/agentrix-arbiter /usr/local/bin/agentrix-arbiter
COPY --from=go-builder /build/bin/agentrix-server /usr/local/bin/agentrix-server
COPY backend/migrations /opt/agentrix/migrations

# Verify permissions
RUN chmod +x /usr/local/bin/agentrix-arbiter /usr/local/bin/agentrix-server

WORKDIR /opt/agentrix

ENV PORT=8080 \
    DATABASE_URL="postgres://postgres:postgres@db:5432/agentrix_platform?sslmode=disable" \
    ARBITER_PATH="/usr/local/bin/agentrix-arbiter" \
    BOTS_DIR="/var/agentrix/bots" \
    REPLAYS_DIR="/var/agentrix/replays" \
    AUTO_MATCHMAKER="true" \
    MATCH_INTERVAL_SECONDS=30

EXPOSE 8080

CMD ["/usr/local/bin/agentrix-server"]
