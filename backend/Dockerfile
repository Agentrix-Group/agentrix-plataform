# ==============================================================================
# Agentrix Platform - Backend & Simulation Engine Multi-Stage Dockerfile
# Combines Go Monolith Server, Rust Arbiter Simulation Engine,
# Bubblewrap Sandboxing, and Pre-installed AI Agent Runtimes.
# ==============================================================================

# --- Stage 1: Build Rust Arbiter ---
FROM rust:1.98.1-slim-bookworm AS rust-builder
WORKDIR /build/arbiter
COPY simulation/arbiter/Cargo.toml simulation/arbiter/Cargo.lock ./
COPY simulation/arbiter/.cargo ./.cargo
COPY simulation/arbiter/vendor ./vendor
COPY simulation/arbiter/src ./src
RUN cargo build --release --locked --offline

# --- Stage 2: Build Go Monolith Server ---
FROM golang:1.27.1-bookworm AS go-builder
WORKDIR /build/server
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/cmd ./cmd
COPY backend/internal ./internal
COPY backend/migrations ./migrations
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /build/bin/agentrix-server ./cmd/agentrix-server

# --- Stage 3: Runtime Container ---
# The example ONNX bot requires NumPy >=2.5, which requires Python >=3.12.
# Match the actually verified Python 3.14 SDK instead of shipping a 3.11 image
# that cannot install the pinned model runtime.
FROM python:3.14.7-slim-bookworm AS runtime

# Install system dependencies, Bubblewrap sandbox, and essential runtimes
RUN apt-get update && apt-get install -y --no-install-recommends \
    bubblewrap \
    ca-certificates \
    postgresql-client \
    libgomp1 \
    curl \
    gzip \
    tar \
    && rm -rf /var/lib/apt/lists/*

# Install the same pinned ONNX runtime stack used to validate the SDK example.
COPY sdk/requirements-py314.lock /opt/agentrix/requirements-py314.lock
RUN python -m pip install --no-cache-dir --disable-pip-version-check \
    --requirement /opt/agentrix/requirements-py314.lock

# Create standard working directories
RUN mkdir -p /var/agentrix/bots /var/agentrix/replays /var/agentrix/backups /opt/agentrix /opt/agentrix/kits

# Copy compiled binaries from previous build stages
COPY --from=rust-builder /build/arbiter/target/release/agentrix-arbiter /usr/local/bin/agentrix-arbiter
COPY --from=go-builder /build/bin/agentrix-server /usr/local/bin/agentrix-server
COPY backend/migrations /opt/agentrix/migrations

# Verify permissions
RUN chmod +x /usr/local/bin/agentrix-arbiter /usr/local/bin/agentrix-server

# Build the official participant starter kit inside the image
COPY scripts/build_starter_kit.py /opt/agentrix/scripts/build_starter_kit.py
COPY templates /opt/agentrix/templates
COPY sdk /opt/agentrix/sdk
COPY notebooks /opt/agentrix/notebooks
RUN python /opt/agentrix/scripts/build_starter_kit.py \
    --arbiter-bin /usr/local/bin/agentrix-arbiter \
    --repo-root /opt/agentrix \
    --output-dir /opt/agentrix/kits

WORKDIR /opt/agentrix

ENV PORT=8080 \
    ARBITER_PATH="/usr/local/bin/agentrix-arbiter" \
    BOTS_DIR="/var/agentrix/bots" \
    REPLAYS_DIR="/var/agentrix/replays" \
    KITS_DIR="/opt/agentrix/kits" \
    AUTO_MATCHMAKER="true" \
    MATCH_INTERVAL_SECONDS=30

EXPOSE 8080

CMD ["/usr/local/bin/agentrix-server"]
