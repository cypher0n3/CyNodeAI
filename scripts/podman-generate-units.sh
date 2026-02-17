#!/usr/bin/env bash
# Generate systemd unit files for Podman containers from docker-compose.
# Run from repo root. Containers must exist (e.g. after 'podman compose up -d').
# Usage: ./scripts/podman-generate-units.sh [orchestrator|worker_node|all]

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(dirname "$SCRIPT_DIR")"
cd "$REPO_ROOT"

ORCHESTRATOR_CONTAINERS=(cynodeai-postgres cynodeai-control-plane cynodeai-user-gateway cynodeai-mcp-gateway cynodeai-api-egress)
WORKER_CONTAINERS=(cynodeai-worker-api cynodeai-node-manager)

generate_for() {
    local name=$1
    local out_dir=$2
    if ! podman ps -a --format '{{.Names}}' | grep -q "^${name}$"; then
        echo "Container ${name} not found. Start stack first: podman compose -f ${out_dir}/docker-compose.yml up -d"
        return 1
    fi
    mkdir -p "$out_dir"
    podman generate systemd --new --name "$name" > "$out_dir/container-${name}.service"
    echo "Wrote $out_dir/container-${name}.service"
}

case "${1:-all}" in
    orchestrator)
        for c in "${ORCHESTRATOR_CONTAINERS[@]}"; do
            generate_for "$c" "orchestrator/systemd" || true
        done
        ;;
    worker_node)
        for c in "${WORKER_CONTAINERS[@]}"; do
            generate_for "$c" "worker_node/systemd" || true
        done
        ;;
    all)
        for c in "${ORCHESTRATOR_CONTAINERS[@]}"; do
            generate_for "$c" "orchestrator/systemd" || true
        done
        for c in "${WORKER_CONTAINERS[@]}"; do
            generate_for "$c" "worker_node/systemd" || true
        done
        ;;
    *)
        echo "Usage: $0 [orchestrator|worker_node|all]" >&2
        exit 1
        ;;
esac
