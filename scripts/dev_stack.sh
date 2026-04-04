#!/usr/bin/env bash
# CyNodeAI development stack controller (Postgres, compose, node-manager).
#
# Commands: start-db | stop-db | clean-db | start | stop | restart | clean
# Invoke from repo root: ./scripts/dev_stack.sh . <command> [ollama-in-stack]
# AI agents must NOT pass ollama-in-stack; it bypasses node-manager and invalidates GPU variant E2E.
# Or use the just front-end: just setup-dev start, just scripts/start (justfile
# implements the same flow inline; this script is the canonical shell implementation).
# No Python dependency; requires podman or docker and pre-built dev binaries.
set -euo pipefail

# --- Paths (relative to ROOT) and file locations -------------------------------
ROOT="${1:-.}"
cd "$ROOT"
COMPOSE_FILE="orchestrator/docker-compose.yml"
NODE_MANAGER_BIN="worker_node/bin/cynodeai-wnm-dev"
NODE_MANAGER_WORKER_API_BIN="worker_node/bin/worker-api-dev"
NODE_MANAGER_PID_FILE="${TMPDIR:-/tmp}/cynodeai-node-manager.pid"
NODE_STATE_DIR="${TMPDIR:-/tmp}/cynodeai-node-state"
LOGS_DIR="${CYNODEAI_LOGS_DIR:-${TMPDIR:-/tmp}/cynodeai-setup-dev-logs}"
# Load dev env (e.g. CYNODE_SECURE_STORE_MASTER_KEY_B64) from repo root.
if [ -f "$ROOT/.env.dev" ]; then
  set -a
  # shellcheck disable=SC1091
  source "$ROOT/.env.dev"
  set +a
fi

# --- Postgres and container env (exported for compose and node-manager) -------
export POSTGRES_CONTAINER_NAME="${POSTGRES_CONTAINER_NAME:-cynodeai-postgres-dev}"
export POSTGRES_PORT="${POSTGRES_PORT:-5432}"
export POSTGRES_USER="${POSTGRES_USER:-cynodeai}"
export POSTGRES_PASSWORD="${POSTGRES_PASSWORD:-cynodeai-dev-password}"
export POSTGRES_DB="${POSTGRES_DB:-cynodeai}"
export POSTGRES_IMAGE="${POSTGRES_IMAGE:-pgvector/pgvector:pg16}"
export ORCHESTRATOR_PORT="${ORCHESTRATOR_PORT:-12080}"
export CONTROL_PLANE_PORT="${CONTROL_PLANE_PORT:-12082}"
export WORKER_PORT="${WORKER_PORT:-12090}"
export NODE_PSK="${NODE_PSK:-dev-node-psk-secret}"
export CONTAINER_HOST_ALIAS="${CONTAINER_HOST_ALIAS:-host.containers.internal}"

# --- Runtime detection (podman preferred, then docker) ------------------------
detect_runtime() {
  if command -v podman >/dev/null 2>&1 && podman ps >/dev/null 2>&1; then
    export RUNTIME=podman
    export CONTAINER_HOST_ALIAS="${CONTAINER_HOST_ALIAS:-host.containers.internal}"
    return 0
  fi
  if command -v docker >/dev/null 2>&1 && docker ps >/dev/null 2>&1; then
    export RUNTIME=docker
    export CONTAINER_HOST_ALIAS="${CONTAINER_HOST_ALIAS:-host.docker.internal}"
    return 0
  fi
  echo "[ERROR] podman or docker required" >&2
  return 1
}

# --- Database commands (standalone Postgres container) -------------------------
cmd_start_db() {
  detect_runtime
  echo "[INFO] Starting PostgreSQL container..."
  if $RUNTIME ps --format '{{.Names}}' 2>/dev/null | grep -qxF "$POSTGRES_CONTAINER_NAME"; then
    echo "[INFO] PostgreSQL container already running"
    return 0
  fi
  if $RUNTIME ps -a --format '{{.Names}}' 2>/dev/null | grep -qxF "$POSTGRES_CONTAINER_NAME"; then
    $RUNTIME start "$POSTGRES_CONTAINER_NAME"
  else
    run_args=(
      "$RUNTIME" run -d --name "$POSTGRES_CONTAINER_NAME"
      -e "POSTGRES_USER=$POSTGRES_USER"
      -e "POSTGRES_PASSWORD=$POSTGRES_PASSWORD"
      -e "POSTGRES_DB=$POSTGRES_DB"
      -p "${POSTGRES_PORT}:5432"
      -v cynodeai-postgres-data:/var/lib/postgresql/data
      "$POSTGRES_IMAGE"
    )
    "${run_args[@]}"
  fi
  echo "[INFO] Waiting for PostgreSQL to be ready..."
  for _ in $(seq 1 60); do
    # pg_isready inside container; exit 0 when ready
    if $RUNTIME exec "$POSTGRES_CONTAINER_NAME" pg_isready -U "$POSTGRES_USER" -d "$POSTGRES_DB" 2>/dev/null; then
      echo "[INFO] PostgreSQL is ready"
      return 0
    fi
    sleep 1
  done
  echo "[ERROR] PostgreSQL failed to start within 60s" >&2
  return 1
}

cmd_stop_db() {
  detect_runtime
  echo "[INFO] Stopping PostgreSQL container..."
  $RUNTIME stop "$POSTGRES_CONTAINER_NAME" 2>/dev/null || true
  return 0
}

cmd_clean_db() {
  detect_runtime
  echo "[INFO] Cleaning up PostgreSQL container and volume..."
  $RUNTIME stop "$POSTGRES_CONTAINER_NAME" 2>/dev/null || true
  $RUNTIME rm "$POSTGRES_CONTAINER_NAME" 2>/dev/null || true
  $RUNTIME volume rm cynodeai-postgres-data 2>/dev/null || true
  return 0
}

# --- Compose environment (orchestrator stack: control-plane, user-gateway, etc.) -
compose_env() {
  detect_runtime
  export POSTGRES_USER POSTGRES_PASSWORD POSTGRES_DB POSTGRES_PORT
  export JWT_SECRET="${JWT_SECRET:-dev-jwt-secret-change-in-production}"
  export NODE_REGISTRATION_PSK="$NODE_PSK"
  export WORKER_API_BEARER_TOKEN="${WORKER_API_BEARER_TOKEN:-dev-worker-api-token-change-me}"
  export CONTROL_PLANE_PORT ORCHESTRATOR_PORT
  export PMA_PORT="${PMA_PORT:-8090}"
  export WORKER_API_TARGET_URL="http://${CONTAINER_HOST_ALIAS}:${WORKER_PORT}"
  export BOOTSTRAP_ADMIN_PASSWORD="${ADMIN_PASSWORD:-admin123}"
  export PMA_ENABLED="false"
  export PMA_IMAGE="${PMA_IMAGE:-cynodeai-cynode-pma:dev}"
  export OLLAMA_BASE_URL="${OLLAMA_BASE_URL:-http://${CONTAINER_HOST_ALIAS}:11434}"
}

# --- Full stack start: compose up + node-manager (in-process worker-api) -------
cmd_start() {
  local ollama_in_stack="${1:-}"
  detect_runtime
  if [ ! -f "$COMPOSE_FILE" ]; then
    echo "[ERROR] Compose file not found: $COMPOSE_FILE" >&2
    return 1
  fi
  echo "[INFO] Building cynode-pma image..."
  $RUNTIME build -f agents/cmd/cynode-pma/Containerfile -t cynodeai-cynode-pma:dev . || return 1
  compose_env
  _xdg_cache="${XDG_CACHE_HOME:-$HOME/.cache}"
  export NATS_DEV_JWT_DIR="${NATS_DEV_JWT_DIR:-$_xdg_cache/cynodeai/nats-dev-jwt}"
  mkdir -p "$NATS_DEV_JWT_DIR"
  echo "[INFO] Writing NATS dev JWT bundle to $NATS_DEV_JWT_DIR"
  (cd "$ROOT" && go run ./orchestrator/cmd/gen-nats-dev-jwt -dir "$NATS_DEV_JWT_DIR") || return 1
  _nats_seeds_json="${NATS_DEV_SEEDS_FILE:-$_xdg_cache/cynodeai/nats-dev-seeds.json}"
  if [ -f "$_nats_seeds_json" ]; then
    NATS_ACCOUNT_SEED="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["cynode_account_seed"])' "$_nats_seeds_json")"
    NATS_ACCOUNT_SIGNING_SEED="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["cynode_signing_seed"])' "$_nats_seeds_json")"
    export NATS_ACCOUNT_SEED NATS_ACCOUNT_SIGNING_SEED
  else
    echo "[WARN] NATS dev seeds missing at $_nats_seeds_json; compose services may issue JWTs NATS rejects" >&2
  fi
  echo "[INFO] Orchestrator stack startup..."
  $RUNTIME compose -f "$COMPOSE_FILE" --profile ollama down 2>/dev/null || true
  $RUNTIME rm -f cynodeai-control-plane cynodeai-user-gateway cynodeai-cynode-pma 2>/dev/null || true
  # Only "ollama" is a profile; base stack is postgres + control-plane + user-gateway + mcp-gateway + api-egress.
  profiles=()
  [ -n "$ollama_in_stack" ] && profiles=(--profile ollama)
  $RUNTIME compose -f "$COMPOSE_FILE" "${profiles[@]}" up -d || return 1
  echo "[INFO] Starting node (node-manager -> worker-api)..."
  mkdir -p "$NODE_STATE_DIR" "$LOGS_DIR"
  # Node-manager runs worker-api as subprocess; needs orchestrator URL, PSK, worker port, secure store.
  export ORCHESTRATOR_URL="http://localhost:${CONTROL_PLANE_PORT}"
  export NODE_REGISTRATION_PSK="$NODE_PSK"
  export NODE_SLUG="${NODE_SLUG:-test-e2e-node}"
  export NODE_NAME="${NODE_NAME:-Development Node}"
  export NODE_MANAGER_WORKER_API_BIN="$ROOT/$NODE_MANAGER_WORKER_API_BIN"
  export LISTEN_ADDR=":${WORKER_PORT}"
  export NODE_ADVERTISED_WORKER_API_URL="${NODE_ADVERTISED_WORKER_API_URL:-http://${CONTAINER_HOST_ALIAS}:${WORKER_PORT}}"
  export CONTAINER_RUNTIME="${CONTAINER_RUNTIME:-podman}"
  export WORKER_API_STATE_DIR="$NODE_STATE_DIR"
  export CYNODE_SECURE_STORE_MASTER_KEY_B64="${CYNODE_SECURE_STORE_MASTER_KEY_B64:-}"
  export INFERENCE_PROXY_IMAGE="${INFERENCE_PROXY_IMAGE:-cynodeai-inference-proxy:dev}"
  export OLLAMA_UPSTREAM_URL="${OLLAMA_UPSTREAM_URL:-http://${CONTAINER_HOST_ALIAS}:11434}"
  [ -n "${NODE_MANAGER_WORKER_API_IMAGE:-}" ] && export NODE_MANAGER_WORKER_API_IMAGE

  if [ ! -f "$NODE_MANAGER_BIN" ]; then
    echo "[ERROR] node-manager not found: $NODE_MANAGER_BIN" >&2
    return 1
  fi
  "$ROOT/$NODE_MANAGER_BIN" >>"$LOGS_DIR/cynodeai-wnm.log" 2>&1 &
  echo $! >"$NODE_MANAGER_PID_FILE"
  echo "[INFO] Node-manager started (PID $!); waiting for worker-api..."
  for _ in $(seq 1 30); do
    # Worker-api binds WORKER_PORT; node-manager starts it and registers with control-plane.
    if curl -sf "http://localhost:${WORKER_PORT}/healthz" >/dev/null 2>&1; then
      echo "[INFO] Worker API listening on http://localhost:${WORKER_PORT}"
      break
    fi
    sleep 1
  done
  echo "[INFO] Waiting for orchestrator readyz..."
  for _ in $(seq 1 60); do
    if code=$(curl -sf -o /dev/null -w '%{http_code}' "http://127.0.0.1:${CONTROL_PLANE_PORT}/readyz" 2>/dev/null) && [ "$code" = "200" ]; then
      echo "[INFO] Orchestrator is ready (readyz 200)"
      return 0
    fi
    sleep 1
  done
  echo "[ERROR] Orchestrator not ready after 60s" >&2
  return 1
}

# --- Stop: node-manager (PID file + pgrep fallback), then compose down -------
cmd_stop() {
  detect_runtime
  echo "[INFO] Stopping all services..."
  if [ -f "$NODE_MANAGER_PID_FILE" ]; then
    pid=$(cat "$NODE_MANAGER_PID_FILE")
    kill "$pid" 2>/dev/null || true
    for _ in $(seq 1 15); do kill -0 "$pid" 2>/dev/null || break; sleep 1; done
    kill -9 "$pid" 2>/dev/null || true
    rm -f "$NODE_MANAGER_PID_FILE"
  fi
  # Fallback: kill any stray node-manager or worker-api processes.
  pgrep -f "cynodeai-wnm-dev" 2>/dev/null | xargs -r kill 2>/dev/null || true
  pgrep -f "worker-api-dev" 2>/dev/null | xargs -r kill 2>/dev/null || true
  [ -f "$COMPOSE_FILE" ] && $RUNTIME compose -f "$COMPOSE_FILE" --profile ollama down 2>/dev/null || true
  $RUNTIME rm -f cynodeai-postgres cynodeai-control-plane cynodeai-user-gateway cynodeai-cynode-pma cynodeai-mcp-gateway cynodeai-api-egress 2>/dev/null || true
  echo "[INFO] All services stopped."
  return 0
}

cmd_restart() {
  cmd_stop
  sleep 2
  cmd_start "${1:-}"
}

# --- Clean: stop stack, remove Postgres container/volume, optional OLLAMA volume -
cmd_clean() {
  cmd_stop
  cmd_clean_db
  detect_runtime
  $RUNTIME volume rm orchestrator_ollama-data 2>/dev/null || true
  echo "[INFO] Clean complete."
}

# --- Dispatch ------------------------------------------------------------------
case "${2:-help}" in
  start-db)   cmd_start_db ;;
  stop-db)    cmd_stop_db ;;
  clean-db)   cmd_clean_db ;;
  start)      cmd_start "${3:-}" ;;
  stop)       cmd_stop ;;
  restart)    cmd_restart "${3:-}" ;;
  clean)      cmd_clean ;;
  *)
    echo "Usage: $0 <root_dir> {start-db|stop-db|clean-db|start|stop|restart|clean} [ollama-in-stack]"
    exit 1
    ;;
esac
