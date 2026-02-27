#!/bin/bash
# CyNodeAI Development Setup Script
# This script sets up the local development environment for running the E2E demo.
# Requirements: podman or docker, Go 1.25+, PostgreSQL client (psql)

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Detect container runtime (prefer podman)
detect_runtime() {
    if command -v podman &> /dev/null; then
        echo "podman"
    elif command -v docker &> /dev/null; then
        echo "docker"
    else
        log_error "Neither podman nor docker found. Please install one of them."
        exit 1
    fi
}

RUNTIME=$(detect_runtime)
log_info "Using container runtime: $RUNTIME"

# Host address as seen from containers (for DB, worker-api on host). Docker on Linux needs --add-host.
if [ "$RUNTIME" = "podman" ]; then
    CONTAINER_HOST_ALIAS="${CONTAINER_HOST_ALIAS:-host.containers.internal}"
    DOCKER_EXTRA_HOSTS=""
else
    CONTAINER_HOST_ALIAS="${CONTAINER_HOST_ALIAS:-host.docker.internal}"
    # So that host.docker.internal resolves when running containers (e.g. on Linux)
    DOCKER_EXTRA_HOSTS="--add-host=host.docker.internal:host-gateway"
fi

# Configuration (default ports: docs/tech_specs/ports_and_endpoints.md)
POSTGRES_CONTAINER_NAME="cynodeai-postgres-dev"
CONTROL_PLANE_CONTAINER_NAME="${CONTROL_PLANE_CONTAINER_NAME:-cynodeai-control-plane}"
USER_GATEWAY_CONTAINER_NAME="${USER_GATEWAY_CONTAINER_NAME:-cynodeai-user-gateway}"
POSTGRES_PORT="${POSTGRES_PORT:-5432}"
POSTGRES_USER="${POSTGRES_USER:-cynodeai}"
POSTGRES_PASSWORD="${POSTGRES_PASSWORD:-cynodeai-dev-password}"
POSTGRES_DB="${POSTGRES_DB:-cynodeai}"
# Image must include pgvector for 01_extensions.sql (vector extension)
POSTGRES_IMAGE="${POSTGRES_IMAGE:-pgvector/pgvector:pg16}"

# Orchestrator API config
ORCHESTRATOR_PORT="${ORCHESTRATOR_PORT:-12080}"
CONTROL_PLANE_PORT="${CONTROL_PLANE_PORT:-12082}"
JWT_SECRET="${JWT_SECRET:-dev-jwt-secret-change-in-production}"
NODE_PSK="${NODE_PSK:-dev-node-psk-secret}"
ADMIN_PASSWORD="${ADMIN_PASSWORD:-admin123}"

# Node config
NODE_SLUG="${NODE_SLUG:-dev-node-1}"
WORKER_PORT="${WORKER_PORT:-12090}"
WORKER_API_BEARER_TOKEN="${WORKER_API_BEARER_TOKEN:-dev-worker-api-token-change-me}"

# Compose file for orchestrator stack (postgres + control-plane + user-gateway)
COMPOSE_FILE="$(cd "$PROJECT_ROOT" && pwd)/orchestrator/docker-compose.yml"

export DATABASE_URL="postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@localhost:${POSTGRES_PORT}/${POSTGRES_DB}?sslmode=disable"
# URL for use inside orchestrator containers (they reach Postgres via host alias)
DATABASE_URL_FOR_CONTAINERS="postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@${CONTAINER_HOST_ALIAS}:${POSTGRES_PORT}/${POSTGRES_DB}?sslmode=disable"

# Function to start PostgreSQL
start_postgres() {
    log_info "Starting PostgreSQL container..."

    # Check if container already exists
    if $RUNTIME ps -a --format '{{.Names}}' | grep -q "^${POSTGRES_CONTAINER_NAME}$"; then
        if $RUNTIME ps --format '{{.Names}}' | grep -q "^${POSTGRES_CONTAINER_NAME}$"; then
            log_info "PostgreSQL container already running"
            return 0
        else
            log_info "Starting existing PostgreSQL container"
            $RUNTIME start $POSTGRES_CONTAINER_NAME
            sleep 2
            return 0
        fi
    fi

    # Create and start new container (pgvector image required for vector extension)
    $RUNTIME run -d \
        --name "$POSTGRES_CONTAINER_NAME" \
        -e POSTGRES_USER="$POSTGRES_USER" \
        -e POSTGRES_PASSWORD="$POSTGRES_PASSWORD" \
        -e POSTGRES_DB="$POSTGRES_DB" \
        -p "$POSTGRES_PORT:5432" \
        -v cynodeai-postgres-data:/var/lib/postgresql/data \
        "$POSTGRES_IMAGE"

    log_info "Waiting for PostgreSQL to be ready..."
    sleep 3

    # Wait for PostgreSQL to be ready
    for i in {1..30}; do
        if $RUNTIME exec "$POSTGRES_CONTAINER_NAME" pg_isready -U "$POSTGRES_USER" -d "$POSTGRES_DB" > /dev/null 2>&1; then
            log_info "PostgreSQL is ready"
            return 0
        fi
        sleep 1
    done

    log_error "PostgreSQL failed to start within 30 seconds"
    exit 1
}

# Function to stop PostgreSQL
stop_postgres() {
    log_info "Stopping PostgreSQL container..."
    $RUNTIME stop $POSTGRES_CONTAINER_NAME 2>/dev/null || true
}

# Function to clean up PostgreSQL (stop and remove)
clean_postgres() {
    log_info "Cleaning up PostgreSQL container and volume..."
    $RUNTIME stop $POSTGRES_CONTAINER_NAME 2>/dev/null || true
    $RUNTIME rm $POSTGRES_CONTAINER_NAME 2>/dev/null || true
    $RUNTIME volume rm cynodeai-postgres-data 2>/dev/null || true
}

# Function to run migrations (control-plane runs migrations on startup)
run_migrations() {
    log_info "Migrations run when control-plane starts..."
}

# Function to build all binaries via justfile (outputs to module bin/ dirs: orchestrator/bin, worker_node/bin, etc.)
build_binaries() {
    log_info "Building all binaries (just build)..."
    cd "$PROJECT_ROOT"
    if ! command -v just &>/dev/null; then
        log_error "just not found. Install just (https://github.com/casey/just) or run: just build"
        return 1
    fi
    if ! just build; then
        log_error "just build failed"
        return 1
    fi
    log_info "Binaries built: orchestrator/bin, worker_node/bin, cynork/bin, agents/bin"
}

# E2E image cache: only rebuild when build-context hash changes. Cache dir stores one file per image (name.hash).
# Set E2E_FORCE_REBUILD=1 to ignore cache and rebuild all images.
E2E_IMAGE_CACHE_DIR="${E2E_IMAGE_CACHE_DIR:-$PROJECT_ROOT/tmp/e2e-image-cache}"

# Compute a content hash of the build context (dockerfile + paths). Paths are relative to PROJECT_ROOT; use git when available.
compute_build_context_hash() {
    local dockerfile="$1"
    shift
    local paths=("$@")
    cd "$PROJECT_ROOT"
    local input=""
    if [ ! -f "$dockerfile" ]; then
        echo "invalid"
        return 1
    fi
    input=$(cat "$dockerfile" | tr -d '\0' 2>/dev/null || true)
    if git rev-parse --is-inside-work-tree &>/dev/null; then
        local list
        list=$(git ls-files -- "${paths[@]}" 2>/dev/null | sort -u)
        if [ -n "$list" ]; then
            while IFS= read -r f; do
                if [ -f "$f" ]; then
                    # Strip NULs so hashing is consistent (avoids "ignored null byte" in command substitution)
                    input="$input$(cat "$f" | tr -d '\0' 2>/dev/null || true)"
                fi
            done <<< "$list"
        fi
    else
        local f
        for f in "${paths[@]}"; do
            if [ -d "$f" ]; then
                while IFS= read -r ff; do
                    input="$input$(cat "$ff" | tr -d '\0' 2>/dev/null || true)"
                done < <(find "$f" -type f 2>/dev/null)
            elif [ -f "$f" ]; then
                input="$input$(cat "$f" | tr -d '\0' 2>/dev/null || true)"
            fi
        done
    fi
    echo -n "$input" | sha256sum | awk '{print $1}'
}

# Ensure image is built only when build context hash changed. Uses E2E_IMAGE_CACHE_DIR for tracking.
# Usage: ensure_image_build_if_delta <image_key> <tag> <dockerfile_rel> <path1> [path2 ...]
ensure_image_build_if_delta() {
    local key="$1"
    local tag="$2"
    local dockerfile="$3"
    shift 3
    local paths=("$@")
    local cache_file="$E2E_IMAGE_CACHE_DIR/$key.hash"
    mkdir -p "$E2E_IMAGE_CACHE_DIR"
    local current_hash
    current_hash=$(compute_build_context_hash "$PROJECT_ROOT/$dockerfile" "${paths[@]}")
    local cached_hash=""
    if [ "${E2E_FORCE_REBUILD:-0}" = "1" ]; then
        cached_hash=""
    else
        [ -f "$cache_file" ] && cached_hash=$(cat "$cache_file")
    fi
    if [ "$cached_hash" = "$current_hash" ]; then
        if $RUNTIME image inspect "$tag" &>/dev/null; then
            log_info "Image $tag up to date (cache hit)"
            return 0
        fi
    fi
    log_info "Building image $tag (cache miss or image missing)..."
    cd "$PROJECT_ROOT"
    if ! $RUNTIME build -f "$dockerfile" -t "$tag" .; then
        log_error "Failed to build $tag"
        return 1
    fi
    echo -n "$current_hash" > "$cache_file"
    log_info "Built and cached $tag"
    return 0
}

# Ensure orchestrator stack images (control-plane, user-gateway, cynode-pma) are built only when context changed.
ensure_stack_images_build_if_delta() {
    log_info "Ensuring stack images are up to date (rebuild only on delta)..."
    cd "$PROJECT_ROOT"
    ensure_image_build_if_delta "control-plane" "cynodeai-control-plane:dev" \
        "orchestrator/cmd/control-plane/Containerfile" "orchestrator" "go.work" || return 1
    ensure_image_build_if_delta "user-gateway" "cynodeai-user-gateway:dev" \
        "orchestrator/cmd/user-gateway/Containerfile" "orchestrator" "go.work" || return 1
    ensure_image_build_if_delta "cynode-pma" "cynodeai-cynode-pma:dev" \
        "agents/cmd/cynode-pma/Containerfile" "agents" "go.work" || return 1
    log_info "Stack images up to date"
}

# Ensure inference-proxy image is built only when context changed (used by full-demo / node with inference).
ensure_inference_proxy_build_if_delta() {
    ensure_image_build_if_delta "inference-proxy" "cynodeai-inference-proxy:dev" \
        "worker_node/cmd/inference-proxy/Containerfile" "worker_node" "go_shared_libs" "go.work" || return 1
}

# Function to start orchestrator stack (postgres, control-plane, user-gateway) via docker-compose.
# Images are built only when build-context hash changes (see ensure_stack_images_build_if_delta).
start_orchestrator_stack_compose() {
    log_info "Starting orchestrator stack with compose..."
    cd "$PROJECT_ROOT"
    if ! $RUNTIME compose version &>/dev/null; then
        log_error "Compose not available. Install docker compose or podman compose."
        return 1
    fi
    if [ ! -f "$COMPOSE_FILE" ]; then
        log_error "Compose file not found: $COMPOSE_FILE"
        return 1
    fi
    ensure_stack_images_build_if_delta || return 1
    # Tear down any existing stack and remove standalone containers that might hold ports
    $RUNTIME compose -f "$COMPOSE_FILE" down 2>/dev/null || true
    $RUNTIME rm -f "$CONTROL_PLANE_CONTAINER_NAME" "$USER_GATEWAY_CONTAINER_NAME" 2>/dev/null || true
    export POSTGRES_USER POSTGRES_PASSWORD POSTGRES_DB POSTGRES_PORT
    export JWT_SECRET NODE_PSK WORKER_API_BEARER_TOKEN CONTROL_PLANE_PORT ORCHESTRATOR_PORT
    export WORKER_API_TARGET_URL="http://${CONTAINER_HOST_ALIAS}:${WORKER_PORT}"
    export BOOTSTRAP_ADMIN_PASSWORD="$ADMIN_PASSWORD"
    if ! $RUNTIME compose -f "$COMPOSE_FILE" up -d; then
        log_error "Compose up failed"
        return 1
    fi
    log_info "Orchestrator stack started (postgres :5432, control-plane :$CONTROL_PLANE_PORT, user-gateway :$ORCHESTRATOR_PORT)"
}

# Function to stop orchestrator stack (compose down)
stop_orchestrator_stack_compose() {
    log_info "Stopping orchestrator stack..."
    cd "$PROJECT_ROOT"
    if [ -f "$COMPOSE_FILE" ]; then
        $RUNTIME compose -f "$COMPOSE_FILE" down 2>/dev/null || true
    fi
}

# Function to start control-plane (migrations, node API, dispatcher) in a container (standalone; prefer compose)
start_control_plane() {
    log_info "Starting control-plane container on port $CONTROL_PLANE_PORT..."
    cd "$PROJECT_ROOT"
    $RUNTIME rm -f "$CONTROL_PLANE_CONTAINER_NAME" 2>/dev/null || true
    if ! $RUNTIME run -d --name "$CONTROL_PLANE_CONTAINER_NAME" \
        $DOCKER_EXTRA_HOSTS \
        -e DATABASE_URL="$DATABASE_URL_FOR_CONTAINERS" \
        -e CONTROL_PLANE_LISTEN_ADDR=":$CONTROL_PLANE_PORT" \
        -e JWT_SECRET="$JWT_SECRET" \
        -e NODE_REGISTRATION_PSK="$NODE_PSK" \
        -e BOOTSTRAP_ADMIN_PASSWORD="$ADMIN_PASSWORD" \
        -e WORKER_API_TARGET_URL="http://${CONTAINER_HOST_ALIAS}:$WORKER_PORT" \
        -e WORKER_API_BEARER_TOKEN="$WORKER_API_BEARER_TOKEN" \
        -p "$CONTROL_PLANE_PORT:$CONTROL_PLANE_PORT" \
        cynodeai-control-plane:dev; then
        log_error "Failed to start control-plane container"
        exit 1
    fi
    sleep 2
    if $RUNTIME ps --format '{{.Names}}' 2>/dev/null | grep -q "^${CONTROL_PLANE_CONTAINER_NAME}$"; then
        log_info "Control-plane container started"
    else
        log_error "Control-plane container exited; check logs: $RUNTIME logs $CONTROL_PLANE_CONTAINER_NAME"
        exit 1
    fi
}

# Function to start user-gateway (auth, users, tasks) in a container
start_orchestrator() {
    log_info "Starting user-gateway container on port $ORCHESTRATOR_PORT..."
    cd "$PROJECT_ROOT"
    $RUNTIME rm -f "$USER_GATEWAY_CONTAINER_NAME" 2>/dev/null || true
    if ! $RUNTIME run -d --name "$USER_GATEWAY_CONTAINER_NAME" \
        $DOCKER_EXTRA_HOSTS \
        -e DATABASE_URL="$DATABASE_URL_FOR_CONTAINERS" \
        -e USER_GATEWAY_LISTEN_ADDR=":$ORCHESTRATOR_PORT" \
        -e JWT_SECRET="$JWT_SECRET" \
        -e BOOTSTRAP_ADMIN_PASSWORD="$ADMIN_PASSWORD" \
        -p "$ORCHESTRATOR_PORT:$ORCHESTRATOR_PORT" \
        cynodeai-user-gateway:dev; then
        log_error "Failed to start user-gateway container"
        exit 1
    fi
    sleep 2
    if $RUNTIME ps --format '{{.Names}}' 2>/dev/null | grep -q "^${USER_GATEWAY_CONTAINER_NAME}$"; then
        log_info "User-gateway container started"
    else
        log_error "User-gateway container exited; check logs: $RUNTIME logs $USER_GATEWAY_CONTAINER_NAME"
        exit 1
    fi
}

# Wait for control-plane to be listening (GET /readyz returns 200 or 503). Node-manager must not start until control-plane accepts TCP/HTTP.
# readyz returns 200 only after a node is registered and PMA is ready, so we accept 503 (server up, not ready) and then start the node.
wait_for_control_plane_listening() {
    local url="http://127.0.0.1:${CONTROL_PLANE_PORT}/readyz"
    log_info "Waiting for control-plane at $url (up to 90s)..."
    for i in $(seq 1 90); do
        code=$(curl -s -o /dev/null -w "%{http_code}" --connect-timeout 2 "$url" 2>/dev/null || echo "000")
        if [ "$code" = "200" ] || [ "$code" = "503" ]; then
            log_info "Control-plane is listening (readyz $code)"
            return 0
        fi
        [ "$i" -eq 90 ] && { log_error "Control-plane not listening after 90s (last code='$code')"; return 1; }
        sleep 1
    done
    return 1
}

# Function to start node-manager (which fetches config and starts worker-api with token from config)
start_node() {
    cd "$PROJECT_ROOT"

    log_info "Starting node-manager (will fetch config and start worker-api)..."
    export ORCHESTRATOR_URL="http://localhost:$CONTROL_PLANE_PORT"
    export NODE_REGISTRATION_PSK=$NODE_PSK
    export NODE_SLUG=$NODE_SLUG
    export NODE_NAME="${NODE_NAME:-Development Node}"
    # Worker-api is started by node-manager; point to worker_node/bin so exec.LookPath finds it
    export NODE_MANAGER_WORKER_API_BIN="$PROJECT_ROOT/worker_node/bin/worker-api"
    export LISTEN_ADDR=":$WORKER_PORT"
    # Node reports this URL at registration so orchestrator can dispatch; when orchestrator is in container, use host alias
    export NODE_ADVERTISED_WORKER_API_URL="${NODE_ADVERTISED_WORKER_API_URL:-http://${CONTAINER_HOST_ALIAS}:${WORKER_PORT}}"
    export CONTAINER_RUNTIME="${CONTAINER_RUNTIME:-podman}"
    "$PROJECT_ROOT/worker_node/bin/node-manager" &
    NODE_PID=$!
    echo $NODE_PID > /tmp/cynodeai-node-manager.pid

    sleep 2
    if kill -0 $NODE_PID 2>/dev/null; then
        log_info "Node-manager started (worker-api will be started by node-manager after config fetch)"
    else
        log_error "Failed to start node-manager"
        exit 1
    fi
}

# Function to stop all services (orchestrator stack + node processes)
stop_all() {
    log_info "Stopping all services..."

    if [ -f /tmp/cynodeai-node-manager.pid ]; then
        kill "$(cat /tmp/cynodeai-node-manager.pid)" 2>/dev/null || true
        rm -f /tmp/cynodeai-node-manager.pid
    fi
    if [ -f /tmp/cynodeai-worker-api.pid ]; then
        kill "$(cat /tmp/cynodeai-worker-api.pid)" 2>/dev/null || true
        rm -f /tmp/cynodeai-worker-api.pid
    fi
    stop_orchestrator_stack_compose
    # Ensure compose-managed containers are gone (in case compose down missed them)
    $RUNTIME rm -f cynodeai-postgres "$CONTROL_PLANE_CONTAINER_NAME" "$USER_GATEWAY_CONTAINER_NAME" 2>/dev/null || true
}

# Ollama E2E container name (must match worker_node/cmd/node-manager/main.go)
OLLAMA_CONTAINER_NAME="${OLLAMA_CONTAINER_NAME:-cynodeai-ollama}"
OLLAMA_E2E_MODEL="${OLLAMA_E2E_MODEL:-tinyllama}"

# Wait for Ollama container to be running (up to 30s). No-op if container not present.
wait_for_ollama() {
    local _
    for _ in {1..30}; do
        if "$RUNTIME" ps --format '{{.Names}}' 2>/dev/null | grep -q "^${OLLAMA_CONTAINER_NAME}$"; then
            return 0
        fi
        sleep 1
    done
    return 1
}

# Load inference model and run a basic inference (host-side smoke). Skips if Ollama container not found.
# Set E2E_SKIP_INFERENCE_SMOKE=1 to skip pull and inference when registry.ollama.ai is unreachable (e.g. CI).
run_ollama_inference_smoke() {
    if [ -n "${E2E_SKIP_INFERENCE_SMOKE:-}" ]; then
        log_info "Skipping inference smoke (E2E_SKIP_INFERENCE_SMOKE set)"
        return 0
    fi
    if ! "$RUNTIME" ps -a --format '{{.Names}}' 2>/dev/null | grep -q "^${OLLAMA_CONTAINER_NAME}$"; then
        log_warn "Ollama container ${OLLAMA_CONTAINER_NAME} not found; skipping inference smoke (run full-demo to start node)"
        return 0
    fi
    if ! wait_for_ollama; then
        log_error "Ollama container did not become ready in time"
        return 1
    fi
    if "$RUNTIME" exec "$OLLAMA_CONTAINER_NAME" ollama list 2>/dev/null | grep -q "^${OLLAMA_E2E_MODEL}[[:space:]]"; then
        log_info "Model ${OLLAMA_E2E_MODEL} already present; checking for update..."
        update_out=$(timeout 30 "$RUNTIME" exec "$OLLAMA_CONTAINER_NAME" ollama pull "$OLLAMA_E2E_MODEL" 2>&1)
        update_rc=$?
        if [ "$update_rc" -eq 0 ]; then
            log_info "Model up to date or updated"
        else
            log_info "Using existing model (update check failed or timed out: ${update_out:-unknown})"
        fi
    else
        log_info "Pulling inference model: ${OLLAMA_E2E_MODEL}..."
        pull_ok=0
        last_pull_out=""
        for attempt in 1 2 3; do
            [ "$attempt" -gt 1 ] && log_info "Retry ${attempt}/3..."
            last_pull_out=$("$RUNTIME" exec "$OLLAMA_CONTAINER_NAME" ollama pull "$OLLAMA_E2E_MODEL" 2>&1)
            pull_rc=$?
            if [ "$pull_rc" -eq 0 ]; then
                pull_ok=1
                break
            fi
            [ "$attempt" -lt 3 ] && sleep 5
        done
        if [ "$pull_ok" -eq 0 ]; then
            log_error "Failed to pull model ${OLLAMA_E2E_MODEL} after 3 attempts"
            log_error "Last pull output: ${last_pull_out:-<none>}"
            return 1
        fi
    fi
    log_info "Running basic inference..."
    local out
    out=$("$RUNTIME" exec "$OLLAMA_CONTAINER_NAME" ollama run "$OLLAMA_E2E_MODEL" "Say one word: hello" 2>&1) || true
    if [ -z "$(echo "$out" | tr -d '\n\r\t ')" ]; then
        log_error "Inference smoke failed: no output from model"
        return 1
    fi
    log_info "Inference smoke passed"
    return 0
}

# Build cynork-dev for E2E (uses just; binary at cynork/bin/cynork-dev).
ensure_cynork_dev() {
    log_info "Building cynork-dev for E2E..."
    cd "$PROJECT_ROOT"
    if ! command -v just &>/dev/null; then
        log_error "just not found. Install just (https://github.com/casey/just) to build cynork-dev"
        return 1
    fi
    if ! just build-cynork-dev; then
        log_error "just build-cynork-dev failed"
        return 1
    fi
    CYNORK_BIN="$PROJECT_ROOT/cynork/bin/cynork-dev"
    if [ ! -x "$CYNORK_BIN" ]; then
        log_error "cynork-dev binary not found at $CYNORK_BIN"
        return 1
    fi
    log_info "cynork-dev ready: $CYNORK_BIN"
    return 0
}

# Function to run E2E demo test (user API :12080, control-plane :12082, worker :12090).
# Uses cynork-dev for auth, tasks, models list, one-shot chat, refresh, and logout; curl only for control-plane (node registration/capability).
run_e2e_test() {
    log_info "Running E2E demo test..."

    if ! ensure_cynork_dev; then
        return 1
    fi

    USER_API="http://localhost:$ORCHESTRATOR_PORT"
    CONTROL_PLANE_API="http://localhost:$CONTROL_PLANE_PORT"

    E2E_CONFIG_DIR=$(mktemp -d)
    E2E_CONFIG="$E2E_CONFIG_DIR/config.yaml"
    trap 'rm -rf "$E2E_CONFIG_DIR"' RETURN
    export CYNORK_GATEWAY_URL="$USER_API"

    # Wait for user-gateway to be reachable
    for i in {1..30}; do
        if curl -s -o /dev/null -w "%{http_code}" "$USER_API/healthz" | grep -q 200; then
            break
        fi
        [ "$i" -eq 30 ] && { log_error "User API not ready after 30s"; return 1; }
        sleep 1
    done
    sleep 3

    # Inference readiness: load model and run basic inference when Ollama container is present
    if ! run_ollama_inference_smoke; then
        return 1
    fi

    # Test 1: Login as admin (cynork-dev)
    log_info "Test 1: Login as admin (cynork-dev)..."
    if ! "$CYNORK_BIN" --config "$E2E_CONFIG" auth login -u admin -p "$ADMIN_PASSWORD"; then
        log_error "cynork auth login failed"
        return 1
    fi
    ACCESS_TOKEN=$(sed -n 's/^token:[[:space:]]*\(.*\)$/\1/p' "$E2E_CONFIG" | tr -d '"')
    if [ -z "$ACCESS_TOKEN" ]; then
        log_error "Could not read token from E2E config"
        return 1
    fi
    log_info "Login successful (token in config)"

    # Test 2: Get current user info (cynork-dev)
    log_info "Test 2: Get current user info (cynork-dev)..."
    if ! USER_OUT=$("$CYNORK_BIN" --config "$E2E_CONFIG" auth whoami 2>&1); then
        log_error "cynork auth whoami failed: $USER_OUT"
        return 1
    fi
    if ! echo "$USER_OUT" | grep -q 'handle=admin'; then
        log_error "Expected handle=admin, got: $USER_OUT"
        return 1
    fi
    log_info "User info retrieved: admin"

    # Test 3: Create a task (cynork-dev; retry on failure)
    log_info "Test 3: Create a task (cynork-dev)..."
    TASK_ID=""
    for attempt in 1 2 3; do
        [ "$attempt" -gt 1 ] && { log_info "Retry $attempt/3..."; sleep 5; }
        TASK_OUT=$("$CYNORK_BIN" --config "$E2E_CONFIG" task create -p "echo Hello from sandbox" -o json 2>&1) || true
        TASK_ID=$(echo "$TASK_OUT" | jq -r '.task_id // empty')
        if [ -n "$TASK_ID" ]; then
            break
        fi
        log_warn "Create task attempt $attempt failed: $TASK_OUT"
    done
    if [ -z "$TASK_ID" ]; then
        log_error "Create task failed after 3 attempts"
        return 1
    fi
    log_info "Task created with ID: $TASK_ID"

    # Test 4: Get task details (cynork-dev)
    log_info "Test 4: Get task details (cynork-dev)..."
    TASK_DETAILS=$("$CYNORK_BIN" --config "$E2E_CONFIG" task get "$TASK_ID" -o json 2>&1) || true
    TASK_STATUS=$(echo "$TASK_DETAILS" | jq -r '.status // empty')
    if [ -z "$TASK_STATUS" ]; then
        log_error "Get task failed: $TASK_DETAILS"
        return 1
    fi
    log_info "Task status: $TASK_STATUS"

    # Test 5: Get task result (cynork-dev)
    log_info "Test 5: Get task result (cynork-dev)..."
    RESULT_RESPONSE=$("$CYNORK_BIN" --config "$E2E_CONFIG" task result "$TASK_ID" -o json 2>&1) || true
    if ! echo "$RESULT_RESPONSE" | jq -e '.status' &>/dev/null; then
        log_error "Get task result failed: $RESULT_RESPONSE"
        return 1
    fi
    log_info "Task result: $RESULT_RESPONSE"

    # Test 5b: Inference-in-sandbox task (cynork-dev; only when node is inference-ready)
    if [ -n "${INFERENCE_PROXY_IMAGE:-}" ]; then
        log_info "Test 5b: Create task with use_inference (cynork-dev) and verify sandbox sees OLLAMA_BASE_URL..."
        INF_TASK_OUT=$("$CYNORK_BIN" --config "$E2E_CONFIG" task create -p "sh -c 'echo \$OLLAMA_BASE_URL'" --use-inference --input-mode commands -o json 2>&1) || true
        INF_TASK_ID=$(echo "$INF_TASK_OUT" | jq -r '.task_id // empty')
        if [ -z "$INF_TASK_ID" ]; then
            log_error "Create inference task failed: $INF_TASK_OUT"
            return 1
        fi
        log_info "Inference task created: $INF_TASK_ID; polling for result (up to 90s)..."
        INF_STATUS=""
        for _ in $(seq 1 18); do
            sleep 5
            INF_RESULT=$("$CYNORK_BIN" --config "$E2E_CONFIG" task result "$INF_TASK_ID" -o json 2>&1) || true
            INF_STATUS=$(echo "$INF_RESULT" | jq -r '.status // empty')
            if [ "$INF_STATUS" = "completed" ] || [ "$INF_STATUS" = "failed" ]; then
                break
            fi
        done
        if [ "$INF_STATUS" != "completed" ]; then
            log_error "Inference task did not complete: status=$INF_STATUS result=$INF_RESULT"
            return 1
        fi
        # cynork result -o json has .stdout = RunJobResponse JSON string; extract inner .stdout
        INF_STDOUT=$(echo "$INF_RESULT" | jq -r '.stdout // empty' | jq -r '.stdout // empty')
        if [ -z "$INF_STDOUT" ] || ! echo "$INF_STDOUT" | grep -q "http://localhost:11434"; then
            log_error "Inference task stdout missing expected OLLAMA_BASE_URL: $INF_STDOUT"
            return 1
        fi
        log_info "Inference-in-sandbox passed: sandbox saw OLLAMA_BASE_URL=$INF_STDOUT"
    else
        log_info "Skipping inference-in-sandbox test (INFERENCE_PROXY_IMAGE not set)"
    fi

    # Test 5c: Prompt-mode task (cynork-dev; natural-language prompt -> model output)
    log_info "Test 5c: Create task with natural-language prompt (cynork-dev) and verify model output..."
    PROMPT_TASK_OUT=$("$CYNORK_BIN" --config "$E2E_CONFIG" task create -p "What model are you? Reply in one short sentence." -o json 2>&1) || true
    PROMPT_TASK_ID=$(echo "$PROMPT_TASK_OUT" | jq -r '.task_id // empty')
    if [ -z "$PROMPT_TASK_ID" ]; then
        log_error "Create prompt task failed: $PROMPT_TASK_OUT"
        return 1
    fi
    log_info "Prompt task created: $PROMPT_TASK_ID; polling for result (up to 90s)..."
    PROMPT_STATUS=""
    for _ in $(seq 1 18); do
        sleep 5
        PROMPT_RESULT=$("$CYNORK_BIN" --config "$E2E_CONFIG" task result "$PROMPT_TASK_ID" -o json 2>&1) || true
        PROMPT_STATUS=$(echo "$PROMPT_RESULT" | jq -r '.status // empty')
        if [ "$PROMPT_STATUS" = "completed" ] || [ "$PROMPT_STATUS" = "failed" ]; then
            break
        fi
    done
    if [ "$PROMPT_STATUS" != "completed" ]; then
        log_error "Prompt task did not complete: status=$PROMPT_STATUS result=$PROMPT_RESULT"
        return 1
    fi
    PROMPT_STDOUT=$(echo "$PROMPT_RESULT" | jq -r '.stdout // empty' | jq -r '.stdout // empty')
    if [ -z "$PROMPT_STDOUT" ] || [ "$PROMPT_STDOUT" = "(no response)" ]; then
        log_error "Prompt task stdout missing or empty: got '$PROMPT_STDOUT'"
        return 1
    fi
    if [ "$(echo "$PROMPT_STDOUT" | tr -d '\n\r\t ')" = "" ]; then
        log_error "Prompt task stdout is whitespace only: '$PROMPT_STDOUT'"
        return 1
    fi
    log_info "Prompt test passed: model output (first 120 chars)= ${PROMPT_STDOUT:0:120}"

    # Test 5d: Models list and one-shot chat (cynork-dev)
    log_info "Test 5d: Models list (cynork-dev)..."
    MODELS_OUT=$("$CYNORK_BIN" --config "$E2E_CONFIG" models list -o json 2>&1) || true
    if ! echo "$MODELS_OUT" | jq -e '.object == "list" and (.data | length) >= 1' >/dev/null 2>&1; then
        log_error "cynork models list failed or invalid payload: $MODELS_OUT"
        return 1
    fi
    log_info "List-models OK"

    # Run one-shot chat whenever we ran the inference smoke. Skip only when the whole smoke was skipped (E2E_SKIP_INFERENCE_SMOKE).
    if [ -n "${E2E_SKIP_INFERENCE_SMOKE:-}" ]; then
        log_info "Skipping one-shot chat (inference smoke was skipped; model may be unloaded)"
    else
        log_info "Test 5d: One-shot chat (cynork-dev)..."
        CHAT_OUT=$("$CYNORK_BIN" --config "$E2E_CONFIG" chat --message "Reply with exactly: OK" --plain 2>&1) || true
        if [ -z "$CHAT_OUT" ] || [ "$(echo "$CHAT_OUT" | tr -d '\n\r\t ')" = "" ]; then
            log_error "cynork chat --message produced empty output: $CHAT_OUT"
            return 1
        fi
        if echo "$CHAT_OUT" | grep -qi "error:\|EOF"; then
            log_error "cynork chat failed or got EOF: $CHAT_OUT"
            return 1
        fi
        log_info "OpenAI chat OK: completion (first 80 chars)= ${CHAT_OUT:0:80}"
    fi

    # Test 6: Node registration (control-plane)
    log_info "Test 6: Node registration..."
    NODE_RESPONSE=$(curl -s -X POST "$CONTROL_PLANE_API/v1/nodes/register" \
        -H "Content-Type: application/json" \
        -d "{
            \"psk\": \"$NODE_PSK\",
            \"capability\": {
                \"version\": 1,
                \"reported_at\": \"$(date -u +%Y-%m-%dT%H:%M:%SZ)\",
                \"node\": {\"node_slug\": \"test-e2e-node\"},
                \"platform\": {\"os\": \"linux\", \"arch\": \"amd64\"},
                \"compute\": {\"cpu_cores\": 4, \"ram_mb\": 8192}
            }
        }")

    NODE_JWT=$(echo "$NODE_RESPONSE" | jq -r '.auth.node_jwt')
    if [ "$NODE_JWT" == "null" ] || [ -z "$NODE_JWT" ]; then
        log_error "Node registration failed: $NODE_RESPONSE"
        return 1
    fi
    log_info "Node registered successfully"

    # Test 7: Report capability with node JWT (control-plane)
    log_info "Test 7: Report capability..."
    # shellcheck disable=SC2034
    CAP_RESPONSE=$(curl -s -X POST "$CONTROL_PLANE_API/v1/nodes/capability" \
        -H "Authorization: Bearer $NODE_JWT" \
        -H "Content-Type: application/json" \
        -d "{
            \"version\": 1,
            \"reported_at\": \"$(date -u +%Y-%m-%dT%H:%M:%SZ)\",
            \"node\": {\"node_slug\": \"test-e2e-node\"},
            \"platform\": {\"os\": \"linux\", \"arch\": \"amd64\"},
            \"compute\": {\"cpu_cores\": 4, \"ram_mb\": 8192}
        }")

    # Capability report returns 204 No Content on success
    log_info "Capability report submitted"

    # Test 8: Token refresh (cynork-dev)
    log_info "Test 8: Token refresh (cynork-dev)..."
    if ! "$CYNORK_BIN" --config "$E2E_CONFIG" auth refresh 2>&1; then
        log_error "cynork auth refresh failed"
        return 1
    fi
    if ! "$CYNORK_BIN" --config "$E2E_CONFIG" auth whoami 2>&1 | grep -q 'handle=admin'; then
        log_error "After refresh, whoami did not return handle=admin"
        return 1
    fi
    log_info "Token refreshed successfully"

    # Test 9: Logout (cynork-dev)
    log_info "Test 9: Logout (cynork-dev)..."
    if ! "$CYNORK_BIN" --config "$E2E_CONFIG" auth logout 2>&1; then
        log_warn "cynork auth logout returned non-zero"
    else
        log_info "Logout successful"
    fi

    log_info "All E2E tests passed!"
    return 0
}

# Function to show usage
show_usage() {
    echo "CyNodeAI Development Setup Script"
    echo ""
    echo "Usage: $0 <command>"
    echo ""
    echo "Commands:"
    echo "  start-db        Start PostgreSQL container"
    echo "  stop-db         Stop PostgreSQL container"
    echo "  clean-db        Stop and remove PostgreSQL container and volume"
    echo "  migrate         Run database migrations"
    echo "  build           Build orchestrator-api and node binaries"
    echo "  start           Start all services (db, orchestrator-api)"
    echo "  stop            Stop all services"
    echo "  test-e2e        Run E2E demo test (builds cynork-dev at start, uses it for auth/tasks/logout)"
    echo "  full-demo       Full demo: start all services and run E2E test"
    echo "                  Optional: full-demo --stop-on-success  stop containers after all tests pass"
    echo ""
    echo "Environment Variables:"
    echo "  POSTGRES_PORT     PostgreSQL port (default: 5432)"
    echo "  ORCHESTRATOR_PORT Orchestrator API port (default: 12080)"
    echo "  ADMIN_PASSWORD    Admin user password (default: admin123)"
    echo "  NODE_PSK          Node registration PSK (default: dev-node-psk-secret)"
    echo "  E2E_FORCE_REBUILD Set to 1 to rebuild container images even when cache matches (default: 0)"
    echo "  E2E_IMAGE_CACHE_DIR Dir for build-context hashes (default: tmp/e2e-image-cache)"
    echo "  STOP_ON_SUCCESS_ENV Set to 1 to stop containers after full-demo succeeds (same as --stop-on-success)"
}

# Main script
case "${1:-}" in
    start-db)
        start_postgres
        ;;
    stop-db)
        stop_postgres
        ;;
    clean-db)
        clean_postgres
        ;;
    migrate)
        run_migrations
        ;;
    build)
        build_binaries
        ;;
    start)
        build_binaries
        start_orchestrator_stack_compose || exit 1
        log_info "Services started. User API: http://localhost:$ORCHESTRATOR_PORT Control-plane: http://localhost:$CONTROL_PLANE_PORT"
        log_info "Use '$0 test-e2e' to run the E2E demo test. Use '$0 stop' to stop all services"
        ;;
    stop)
        stop_all
        ;;
    test-e2e)
        run_e2e_test
        ;;
    full-demo)
        STOP_ON_SUCCESS=""
        if [ "${2:-}" = "--stop-on-success" ]; then
            STOP_ON_SUCCESS=1
        fi
        if [ -n "${STOP_ON_SUCCESS_ENV:-}" ]; then
            STOP_ON_SUCCESS=1
        fi
        build_binaries || { stop_all; exit 1; }
        start_orchestrator_stack_compose || { stop_all; exit 1; }
        ensure_inference_proxy_build_if_delta || { stop_all; exit 1; }
        export INFERENCE_PROXY_IMAGE="${INFERENCE_PROXY_IMAGE:-cynodeai-inference-proxy:dev}"
        export OLLAMA_UPSTREAM_URL="${OLLAMA_UPSTREAM_URL:-http://host.containers.internal:11434}"
        wait_for_control_plane_listening || { stop_all; exit 1; }
        start_node || { stop_all; exit 1; }
        sleep 3
        run_e2e_test || { stop_all; exit 1; }
        log_info ""
        if [ -n "$STOP_ON_SUCCESS" ]; then
            log_info "Demo completed! Stopping services (--stop-on-success)."
            stop_all
        else
            log_info "Demo completed! Services are still running."
            log_info "Use '$0 stop' to stop all services"
        fi
        ;;
    help|--help|-h)
        show_usage
        ;;
    *)
        show_usage
        exit 1
        ;;
esac
