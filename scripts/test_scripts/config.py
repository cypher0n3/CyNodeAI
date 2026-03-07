"""E2E test configuration from environment (parity with scripts/setup-dev.sh).

Reads ports, URLs, credentials, and paths from env; no runtime deps beyond stdlib.
"""

import os

# Project root: PROJECT_ROOT env or parent of scripts/ (repo root)
_SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
PROJECT_ROOT = os.environ.get("PROJECT_ROOT") or os.path.dirname(
    os.path.dirname(_SCRIPT_DIR)
)

# Orchestrator (docs/tech_specs/ports_and_endpoints.md)
ORCHESTRATOR_PORT = int(os.environ.get("ORCHESTRATOR_PORT", "12080"))
CONTROL_PLANE_PORT = int(os.environ.get("CONTROL_PLANE_PORT", "12082"))
USER_API = f"http://localhost:{ORCHESTRATOR_PORT}"
CONTROL_PLANE_API = f"http://localhost:{CONTROL_PLANE_PORT}"

# API egress (orchestrator/docker-compose.yml profile optional; port must match compose)
API_EGRESS_PORT = int(os.environ.get("API_EGRESS_PORT", "12084"))
API_EGRESS_API = os.environ.get("API_EGRESS_API") or f"http://localhost:{API_EGRESS_PORT}"

# Worker API (for telemetry E2E; dev stack uses same token as setup_dev_config)
WORKER_PORT = int(os.environ.get("WORKER_PORT", "12090"))
WORKER_API = os.environ.get("WORKER_API") or f"http://localhost:{WORKER_PORT}"
WORKER_API_BEARER_TOKEN = os.environ.get(
    "WORKER_API_BEARER_TOKEN", "dev-worker-api-token-change-me"
)

# Optional bearer tokens (when set, E2E sends them)
WORKFLOW_RUNNER_BEARER_TOKEN = os.environ.get("WORKFLOW_RUNNER_BEARER_TOKEN", "")
API_EGRESS_BEARER_TOKEN = os.environ.get("API_EGRESS_BEARER_TOKEN", "")

# Auth and node
ADMIN_PASSWORD = os.environ.get("ADMIN_PASSWORD", "admin123")
NODE_PSK = os.environ.get("NODE_PSK", "dev-node-psk-secret")

# Cynork CLI (build with: just build-cynork-dev)
CYNORK_BIN = os.environ.get("CYNORK_BIN") or os.path.join(
    PROJECT_ROOT, "cynork", "bin", "cynork-dev"
)

# Optional: skip inference smoke and one-shot chat (e.g. CI without Ollama)
E2E_SKIP_INFERENCE_SMOKE = os.environ.get("E2E_SKIP_INFERENCE_SMOKE", "")
# When set, run inference-in-sandbox (5b), prompt (5c), chat (5d)
INFERENCE_PROXY_IMAGE = os.environ.get("INFERENCE_PROXY_IMAGE", "")

# Ollama smoke (container name must match worker_node node-manager)
OLLAMA_CONTAINER_NAME = os.environ.get("OLLAMA_CONTAINER_NAME", "cynodeai-ollama")
OLLAMA_E2E_MODEL = os.environ.get("OLLAMA_E2E_MODEL", "tinyllama")

# Optional: node state dir for E2E that assert on secure store (e.g. full-demo); skip tests if unset
NODE_STATE_DIR = os.environ.get("NODE_STATE_DIR", "").strip()

# Proxy + PMA isolated tests (minimal services: worker-api proxy + PMA; no orchestrator)
# Ports used only when running test_proxy_pma (avoid clash with main stack)
PROXY_PMA_TEST_PMA_PORT = int(os.environ.get("PROXY_PMA_TEST_PMA_PORT", "18090"))
PROXY_PMA_TEST_WORKER_PORT = int(os.environ.get("PROXY_PMA_TEST_WORKER_PORT", "18091"))
WORKER_API_BIN = os.environ.get("WORKER_API_BIN") or os.path.join(
    PROJECT_ROOT, "worker_node", "bin", "worker-api-dev"
)
PMA_BIN = os.environ.get("PMA_BIN") or os.path.join(
    PROJECT_ROOT, "agents", "bin", "cynode-pma-dev"
)
