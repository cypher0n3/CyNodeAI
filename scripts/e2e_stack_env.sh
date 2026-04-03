#!/usr/bin/env bash
# Export dev E2E env so full_demo / test-e2e runs avoid optional skips (tokens, inference smoke).
# Override any of these in the shell or .env.dev when needed.
# Intentionally no set -e: this file is sourced by just recipes.

export WORKFLOW_RUNNER_BEARER_TOKEN="${WORKFLOW_RUNNER_BEARER_TOKEN:-dev-workflow-runner-bearer}"
export WORKER_INTERNAL_AGENT_TOKEN="${WORKER_INTERNAL_AGENT_TOKEN:-dev-worker-internal-agent-token}"
export MCP_SANDBOX_AGENT_BEARER_TOKEN="${MCP_SANDBOX_AGENT_BEARER_TOKEN:-dev-mcp-sandbox-agent-token}"
export MCP_PA_AGENT_BEARER_TOKEN="${MCP_PA_AGENT_BEARER_TOKEN:-dev-mcp-pa-agent-token}"
# Host-run Python E2E must reach Ollama on the published host port. Dev stack exports
# OLLAMA_BASE_URL with container→host DNS names that often do not resolve from the host
# (e.g. Linux + Podman: host.containers.internal).
_ollama_smoke="${OLLAMA_BASE_URL:-http://127.0.0.1:11434}"
case "$_ollama_smoke" in
  *host.containers.internal*|*host.docker.internal*)
    export OLLAMA_BASE_URL="http://127.0.0.1:11434"
    ;;
  *)
    export OLLAMA_BASE_URL="$_ollama_smoke"
    ;;
esac
# Inference smoke: empty/unset means "run smoke" in E2E (see scripts/test_scripts/config.py).
export E2E_SKIP_INFERENCE_SMOKE="${E2E_SKIP_INFERENCE_SMOKE-}"
