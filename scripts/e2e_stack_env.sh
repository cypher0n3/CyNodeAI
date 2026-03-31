#!/usr/bin/env bash
# Export dev E2E env so full_demo / test-e2e runs avoid optional skips (tokens, inference smoke).
# Override any of these in the shell or .env.dev when needed.
# Intentionally no set -e: this file is sourced by just recipes.

export WORKFLOW_RUNNER_BEARER_TOKEN="${WORKFLOW_RUNNER_BEARER_TOKEN:-dev-workflow-runner-bearer}"
export WORKER_INTERNAL_AGENT_TOKEN="${WORKER_INTERNAL_AGENT_TOKEN:-dev-worker-internal-agent-token}"
export MCP_SANDBOX_AGENT_BEARER_TOKEN="${MCP_SANDBOX_AGENT_BEARER_TOKEN:-dev-mcp-sandbox-agent-token}"
export MCP_PA_AGENT_BEARER_TOKEN="${MCP_PA_AGENT_BEARER_TOKEN:-dev-mcp-pa-agent-token}"
# Inference smoke: empty/unset means "run smoke" in E2E (see scripts/test_scripts/config.py).
export E2E_SKIP_INFERENCE_SMOKE="${E2E_SKIP_INFERENCE_SMOKE-}"
