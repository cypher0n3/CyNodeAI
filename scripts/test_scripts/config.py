"""E2E test configuration from environment (parity with scripts/setup-dev.sh).

Reads ports, URLs, credentials, and paths from env; no runtime deps beyond stdlib.
"""

import os

# Project root: PROJECT_ROOT env or parent of scripts/ (repo root)
_SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
PROJECT_ROOT = os.environ.get("PROJECT_ROOT") or os.path.dirname(
    os.path.dirname(_SCRIPT_DIR)
)


def _env_int(key: str, default: int) -> int:
    raw = os.environ.get(key)
    if raw is None or not str(raw).strip():
        return default
    try:
        return int(raw)
    except ValueError:
        return default


def _env_bool(key: str, default: bool) -> bool:
    raw = os.environ.get(key)
    if raw is None or not str(raw).strip():
        return default
    return str(raw).strip().lower() not in ("0", "false", "no", "off")


# Orchestrator (docs/tech_specs/ports_and_endpoints.md)
# Default loopback is 127.0.0.1 so urllib/cynork match Docker-published ports
# (avoids ::1 quirks).
_E2E_LOOPBACK = os.environ.get("E2E_LOOPBACK_HOST", "127.0.0.1").strip() or "127.0.0.1"
ORCHESTRATOR_PORT = int(os.environ.get("ORCHESTRATOR_PORT", "12080"))
CONTROL_PLANE_PORT = int(os.environ.get("CONTROL_PLANE_PORT", "12082"))
USER_API = os.environ.get("USER_API") or (
    f"http://{_E2E_LOOPBACK}:{ORCHESTRATOR_PORT}"
)
CONTROL_PLANE_API = os.environ.get("CONTROL_PLANE_API") or (
    f"http://{_E2E_LOOPBACK}:{CONTROL_PLANE_PORT}"
)

# NATS (orchestrator docker-compose; monitoring on host for E2E)
NATS_MONITOR_PORT = _env_int("NATS_MONITOR_PORT", 8222)
NATS_MONITOR_URL = os.environ.get("NATS_MONITOR_URL") or (
    f"http://{_E2E_LOOPBACK}:{NATS_MONITOR_PORT}"
)
NATS_CLIENT_PORT = _env_int("NATS_CLIENT_PORT", 4222)
NATS_CLIENT_URL = os.environ.get("NATS_CLIENT_URL") or (
    f"nats://{_E2E_LOOPBACK}:{NATS_CLIENT_PORT}"
)
NATS_WEBSOCKET_PORT = _env_int("NATS_WEBSOCKET_PORT", 8223)
NATS_WEBSOCKET_URL = os.environ.get("NATS_WEBSOCKET_URL") or (
    f"ws://{_E2E_LOOPBACK}:{NATS_WEBSOCKET_PORT}/nats"
)

# API egress (orchestrator/docker-compose.yml profile optional; port must match compose)
API_EGRESS_PORT = int(os.environ.get("API_EGRESS_PORT", "12084"))
API_EGRESS_API = os.environ.get("API_EGRESS_API") or f"http://{_E2E_LOOPBACK}:{API_EGRESS_PORT}"

# Worker API (for telemetry E2E; dev stack uses same token as dev/justfile defaults)
WORKER_PORT = int(os.environ.get("WORKER_PORT", "12090"))
WORKER_API = os.environ.get("WORKER_API") or f"http://{_E2E_LOOPBACK}:{WORKER_PORT}"
WORKER_API_BEARER_TOKEN = os.environ.get(
    "WORKER_API_BEARER_TOKEN", "dev-worker-api-token-change-me"
)

# Control-plane MCP gateway agent bearer tokens (see orchestrator mcpgateway/allowlist.go).
# When both are set, the gateway enforces the sandbox worker allowlist for the sandbox token.
WORKER_INTERNAL_AGENT_TOKEN = os.environ.get("WORKER_INTERNAL_AGENT_TOKEN", "")
MCP_SANDBOX_AGENT_BEARER_TOKEN = os.environ.get("MCP_SANDBOX_AGENT_BEARER_TOKEN", "")
MCP_PA_AGENT_BEARER_TOKEN = os.environ.get("MCP_PA_AGENT_BEARER_TOKEN", "")

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

# E2E timeout policy (tune per machine; avoid false negatives on slow inference / loaded stacks):
# - E2E_CYNORK_TIMEOUT: default subprocess limit for `run_cynork` when a test omits `timeout=...`.
# - E2E_SSE_REQUEST_TIMEOUT: `requests` read timeout for whole SSE HTTP calls (streaming chat).
# - OLLAMA_SMOKE_CHAT_TIMEOUT: urllib deadline for one Ollama /api/chat during prereq smoke.
# - run_e2e.py --timeout: optional outer cap for `--single` only (default 0 = no cap).
E2E_CYNORK_TIMEOUT = _env_int("E2E_CYNORK_TIMEOUT", 300)
E2E_SSE_REQUEST_TIMEOUT = _env_int("E2E_SSE_REQUEST_TIMEOUT", 600)

# Optional: skip inference smoke and one-shot chat (e.g. CI without Ollama)
E2E_SKIP_INFERENCE_SMOKE = os.environ.get("E2E_SKIP_INFERENCE_SMOKE", "")
# When set, run inference-in-sandbox (5b), prompt (5c), chat (5d)
# Default matches scripts/justfile `just e2e` so bare `python run_e2e.py` runs the same suite.
# Set INFERENCE_PROXY_IMAGE= to empty in the environment to skip inference-in-sandbox tests.
INFERENCE_PROXY_IMAGE = os.environ.get("INFERENCE_PROXY_IMAGE", "cynodeai-inference-proxy:dev")

# Ollama smoke (container name must match worker_node node-manager)
OLLAMA_CONTAINER_NAME = os.environ.get("OLLAMA_CONTAINER_NAME", "cynodeai-ollama")
# Small/smoke model (direct generation via langchaingo, no ReAct agent loop).
# qwen3.5:0.8b is the default smoke model; it is small enough for fast startup
# and produces more coherent output than tinyllama.
OLLAMA_E2E_MODEL = os.environ.get("OLLAMA_E2E_MODEL", "qwen3.5:0.8b")
# HTTP timeout (seconds) for Ollama /api/chat during inference smoke (some models exceed 120s).
# Cold inference (model load) on modest hardware can exceed 5 minutes; keep E2E smoke reliable.
OLLAMA_SMOKE_CHAT_TIMEOUT = _env_int("OLLAMA_SMOKE_CHAT_TIMEOUT", 600)
# Capable model for agent/tool-call tests (spec: qwen3.5:9b → Ollama Hub: qwen3:8b).
# When OLLAMA_CONTAINER_NAME is reachable, E2E prereq pulls this if missing (same as
# OLLAMA_E2E_MODEL). Set OLLAMA_AUTO_PULL_CAPABLE=0 to skip that pull (e.g. CI bandwidth).
# Tests still skip if the model is absent after prereq.
OLLAMA_CAPABLE_MODEL = os.environ.get("OLLAMA_CAPABLE_MODEL", "qwen3:8b")
OLLAMA_AUTO_PULL_CAPABLE = _env_bool("OLLAMA_AUTO_PULL_CAPABLE", True)

# Optional: node state dir (see also WORKER_API_STATE_DIR from scripts/dev_stack.sh).
# E2E helpers also resolve defaults under ${TMPDIR:-/tmp}/cynodeai-node-state when unset.
NODE_STATE_DIR = os.environ.get("NODE_STATE_DIR", "").strip()

# Proxy + PMA isolated tests (minimal services: worker-api proxy + PMA; no orchestrator)
# Ports used only when running test_proxy_pma (avoid clash with main stack)
PROXY_PMA_TEST_PMA_PORT = int(os.environ.get("PROXY_PMA_TEST_PMA_PORT", "18090"))
PROXY_PMA_TEST_WORKER_PORT = int(os.environ.get("PROXY_PMA_TEST_WORKER_PORT", "18091"))
# Proxy + PMA with mock inference (separate ports to run alongside or after no-inference suite)
PROXY_PMA_TEST_MOCK_INFERENCE_PORT = int(
    os.environ.get("PROXY_PMA_TEST_MOCK_INFERENCE_PORT", "18092")
)
PROXY_PMA_TEST_PMA_PORT_WITH_INFERENCE = int(
    os.environ.get("PROXY_PMA_TEST_PMA_PORT_WITH_INFERENCE", "18093")
)
PROXY_PMA_TEST_WORKER_PORT_WITH_INFERENCE = int(
    os.environ.get("PROXY_PMA_TEST_WORKER_PORT_WITH_INFERENCE", "18094")
)
# Real Ollama (container) for proxy+PMA+inference test; avoid clash with main stack
PROXY_PMA_TEST_OLLAMA_CONTAINER_NAME = os.environ.get(
    "PROXY_PMA_TEST_OLLAMA_CONTAINER_NAME", "cynodeai-ollama-proxy-test"
)
PROXY_PMA_TEST_OLLAMA_PORT = int(
    os.environ.get("PROXY_PMA_TEST_OLLAMA_PORT", "18100")
)
PROXY_PMA_TEST_PMA_PORT_REAL_OLLAMA = int(
    os.environ.get("PROXY_PMA_TEST_PMA_PORT_REAL_OLLAMA", "18101")
)
PROXY_PMA_TEST_WORKER_PORT_REAL_OLLAMA = int(
    os.environ.get("PROXY_PMA_TEST_WORKER_PORT_REAL_OLLAMA", "18102")
)
NODE_MANAGER_BIN = os.environ.get("NODE_MANAGER_BIN") or os.path.join(
    PROJECT_ROOT, "worker_node", "bin", "cynodeai-wnm-dev"
)
WORKER_API_BIN = NODE_MANAGER_BIN
PMA_BIN = os.environ.get("PMA_BIN") or os.path.join(
    PROJECT_ROOT, "agents", "bin", "cynode-pma-dev"
)
INFERENCE_PROXY_BIN = os.environ.get("INFERENCE_PROXY_BIN") or os.path.join(
    PROJECT_ROOT, "worker_node", "bin", "inference-proxy-dev"
)
