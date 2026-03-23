#!/usr/bin/env bash
# Printed by scripts/justfile _start_impl when setup-dev start fails.
# Env (required): SETUP_DEV_ROOT, SETUP_DEV_RT (podman|docker), SETUP_DEV_FAILURE_PHASE
# Optional: SETUP_DEV_LOGS_DIR, SETUP_DEV_NODE_STATE_DIR, SETUP_DEV_COMPOSE_FILE,
#           WORKER_PORT, ORCHESTRATOR_PORT, SETUP_DEV_LAST_READYZ_HTTP_CODE, SETUP_DEV_LOG_TAIL_LINES

set -u

ROOT="${SETUP_DEV_ROOT:?}"
RT="${SETUP_DEV_RT:?}"
PHASE="${SETUP_DEV_FAILURE_PHASE:-unknown}"
LOGS_DIR="${SETUP_DEV_LOGS_DIR:-${TMPDIR:-/tmp}/cynodeai-setup-dev-logs}"
STATE_DIR="${SETUP_DEV_NODE_STATE_DIR:-${TMPDIR:-/tmp}/cynodeai-node-state}"
COMPOSE_FILE="${SETUP_DEV_COMPOSE_FILE:-$ROOT/orchestrator/docker-compose.yml}"
WORKER_PORT="${WORKER_PORT:-12090}"
ORCH_PORT="${ORCHESTRATOR_PORT:-12080}"
TAIL="${SETUP_DEV_LOG_TAIL_LINES:-100}"
LAST_READY="${SETUP_DEV_LAST_READYZ_HTTP_CODE:-}"

wnm_log="${LOGS_DIR}/cynodeai-wnm.log"
telemetry_db="${STATE_DIR}/telemetry/telemetry.db"
pid_file="${TMPDIR:-/tmp}/cynodeai-node-manager.pid"

divider() {
  printf '\n%s\n' "---------- $* ----------"
}

has_container() {
  local n="$1"
  "$RT" ps -a --format '{{.Names}}' 2>/dev/null | grep -Fxq "$n"
}

tail_container() {
  local name="$1"
  if has_container "$name"; then
    divider "$RT logs --tail $TAIL $name"
    "$RT" logs --tail "$TAIL" "$name" 2>&1 || printf '%s\n' "(failed to read logs for $name)"
  else
    printf '[INFO] No container named %s (skipped).\n' "$name"
  fi
}

print_paths() {
  divider "Log and state paths (for manual inspection)"
  printf '%s\n' "Node-manager / worker API log: $wnm_log"
  printf '%s\n' "Telemetry SQLite (node-local): $telemetry_db"
  printf '%s\n' "Node-manager PID file: $pid_file"
  printf '%s\n' "Compose file: $COMPOSE_FILE"
}

compose_ps() {
  divider "Compose stack status"
  if [ -f "$COMPOSE_FILE" ]; then
    # podman-compose does not accept `ps -a` (unrecognized -a); Docker Compose v2 does.
    (cd "$ROOT" && "$RT" compose -f "$COMPOSE_FILE" ps -a) 2>/dev/null \
      || (cd "$ROOT" && "$RT" compose -f "$COMPOSE_FILE" ps) 2>&1 || true
  else
    printf '%s\n' "[WARN] Compose file not found: $COMPOSE_FILE"
  fi
}

compose_all_logs() {
  divider "Compose logs (all services, last $TAIL lines)"
  if [ -f "$COMPOSE_FILE" ]; then
    (cd "$ROOT" && "$RT" compose -f "$COMPOSE_FILE" logs --tail "$TAIL" 2>&1) || true
  fi
}

tail_wnm_log() {
  divider "Tail of node-manager log (last $TAIL lines)"
  if [ -f "$wnm_log" ]; then
    tail -n "$TAIL" "$wnm_log" 2>&1 || true
  else
    printf '%s\n' "[INFO] $wnm_log not present yet."
  fi
}

curl_worker_health() {
  divider "Worker API GET /healthz (http://127.0.0.1:${WORKER_PORT})"
  code=$(curl -sS -m 5 -o /dev/null -w "%{http_code}" "http://127.0.0.1:${WORKER_PORT}/healthz" 2>/dev/null || printf '%s' "000")
  printf '%s\n' "HTTP $code"
  curl -sS -m 5 "http://127.0.0.1:${WORKER_PORT}/healthz" 2>&1 | head -c 4000 || printf '%s\n' "(body read failed)"
  printf '\n'
}

curl_gateway_readyz() {
  divider "User-gateway GET /readyz (http://127.0.0.1:${ORCH_PORT})"
  code=$(curl -sS -m 5 -o /dev/null -w "%{http_code}" "http://127.0.0.1:${ORCH_PORT}/readyz" 2>/dev/null || printf '%s' "000")
  printf '%s\n' "HTTP $code"
  curl -sS -m 5 "http://127.0.0.1:${ORCH_PORT}/readyz" 2>&1 | head -c 8000 || printf '%s\n' "(body read failed)"
  printf '\n'
}

curl_gateway_healthz() {
  divider "User-gateway GET /healthz (http://127.0.0.1:${ORCH_PORT})"
  code=$(curl -sS -m 5 -o /dev/null -w "%{http_code}" "http://127.0.0.1:${ORCH_PORT}/healthz" 2>/dev/null || printf '%s' "000")
  printf '%s\n' "HTTP $code"
  curl -sS -m 5 "http://127.0.0.1:${ORCH_PORT}/healthz" 2>&1 | head -c 4000 || printf '%s\n' "(body read failed)"
  printf '\n'
}

stack_container_logs() {
  # Order: most likely to explain gateway / readiness issues first.
  tail_container "cynodeai-user-gateway"
  tail_container "cynodeai-control-plane"
  tail_container "cynodeai-postgres"
  tail_container "cynodeai-minio"
  tail_container "cynodeai-api-egress"
  tail_container "cynodeai-ollama"
}

infer_hint() {
  divider "Likely failing component (heuristic)"
  case "$PHASE" in
  compose_up)
    printf '%s\n' "- Compose failed to create or start one or more services. Check the compose output above, image pulls, and short-name registry config for Podman."
    ;;
  worker_healthz)
    printf '%s\n' "- Worker API (embedded in node-manager) did not respond on port ${WORKER_PORT} /healthz within 30s."
    printf '%s\n' "  Check node-manager log for startup errors (e.g. CYNODE_SECURE_STORE_MASTER_KEY_B64, or container runtime)."
    ;;
  user_gateway_readyz)
    printf '%s\n' "- User-gateway is up but GET /readyz did not return HTTP 200 within 90s (e2e gate)."
    if [ -n "$LAST_READY" ]; then
      printf '%s\n' "  Last observed HTTP code: $LAST_READY"
    fi
    printf '%s\n' "  Common causes: no dispatchable worker node yet, PMA not ready, or control-plane / DB issues. See /readyz body above and control-plane + node-manager logs."
    ;;
  *)
    printf '%s\n' "- See paths and logs above."
    ;;
  esac
}

printf '\n'
printf '%s\n' "========== CyNodeAI setup-dev: diagnostics =========="
printf '%s\n' "Failure phase: $PHASE"
print_paths
infer_hint

case "$PHASE" in
compose_up)
  compose_ps
  compose_all_logs
  tail_wnm_log
  ;;
worker_healthz)
  curl_worker_health
  tail_wnm_log
  compose_ps
  stack_container_logs
  ;;
user_gateway_readyz)
  curl_gateway_healthz
  curl_gateway_readyz
  tail_wnm_log
  compose_ps
  stack_container_logs
  ;;
*)
  compose_ps
  compose_all_logs
  tail_wnm_log
  stack_container_logs
  ;;
esac

printf '\n%s\n' "========== end diagnostics =========="
exit 0
