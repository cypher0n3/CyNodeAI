#!/usr/bin/env bash
# Called from setup-dev stop: tear down per-session PMA bindings and invalidate refresh sessions
# while control-plane is still up, so the next start only runs pma-main (+ no pma-sb until login).
# Requires: same NODE_REGISTRATION_PSK the stack uses for node registration.
set -euo pipefail
root="${1:-}"
if [ -z "$root" ] || [ ! -d "$root" ]; then
  echo "[WARN] setup_dev_reset_session_state: bad repo root; skip" >&2
  exit 0
fi
if [ -f "$root/.env.dev" ]; then
  # shellcheck source=/dev/null
  set -a && . "$root/.env.dev" && set +a
fi
port="${CONTROL_PLANE_PORT:-12082}"
psk="${NODE_REGISTRATION_PSK:-${NODE_PSK:-dev-node-psk-secret}}"
url="http://127.0.0.1:${port}/internal/dev/reset-pma-session-state"
if ! curl -sfS -m 15 -X POST "$url" -H "Authorization: Bearer ${psk}" -o /dev/null; then
  echo "[WARN] setup_dev_reset_session_state: POST ${url} failed (control-plane down?); continuing stop" >&2
  exit 0
fi
echo "[INFO] Dev session + PMA binding state reset (per-session pma-sb cleared in DB)."
