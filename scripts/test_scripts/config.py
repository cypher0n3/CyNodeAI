# E2E test configuration from environment (parity with scripts/setup-dev.sh).
# Do not add runtime dependencies beyond stdlib.

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
