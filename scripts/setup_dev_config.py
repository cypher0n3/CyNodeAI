# Config for setup_dev (parity with scripts/setup-dev.sh). From os.environ with defaults.

import os
import subprocess
import sys
import tempfile

SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
PROJECT_ROOT = os.path.dirname(SCRIPT_DIR)


class _RuntimeState:
    """Container for runtime-detected values (avoids global statement)."""
    RUNTIME = None
    CONTAINER_HOST_ALIAS = None


# Postgres (standalone container for start-db)
POSTGRES_CONTAINER_NAME = "cynodeai-postgres-dev"
POSTGRES_PORT = os.environ.get("POSTGRES_PORT", "5432")
POSTGRES_USER = os.environ.get("POSTGRES_USER", "cynodeai")
POSTGRES_PASSWORD = os.environ.get("POSTGRES_PASSWORD", "cynodeai-dev-password")
POSTGRES_DB = os.environ.get("POSTGRES_DB", "cynodeai")
POSTGRES_IMAGE = os.environ.get("POSTGRES_IMAGE", "pgvector/pgvector:pg16")

# Orchestrator
CONTROL_PLANE_CONTAINER_NAME = os.environ.get(
    "CONTROL_PLANE_CONTAINER_NAME", "cynodeai-control-plane"
)
USER_GATEWAY_CONTAINER_NAME = os.environ.get(
    "USER_GATEWAY_CONTAINER_NAME", "cynodeai-user-gateway"
)
ORCHESTRATOR_PORT = os.environ.get("ORCHESTRATOR_PORT", "12080")
CONTROL_PLANE_PORT = os.environ.get("CONTROL_PLANE_PORT", "12082")
OLLAMA_CONTAINER_NAME = os.environ.get("OLLAMA_CONTAINER_NAME", "cynodeai-ollama")
PMA_CONTAINER_NAME = os.environ.get("PMA_CONTAINER_NAME", "cynodeai-cynode-pma")
PMA_PORT = os.environ.get("PMA_PORT", "8090")
JWT_SECRET = os.environ.get("JWT_SECRET", "dev-jwt-secret-change-in-production")
NODE_PSK = os.environ.get("NODE_PSK", "dev-node-psk-secret")
ADMIN_PASSWORD = os.environ.get("ADMIN_PASSWORD", "admin123")
WORKER_PORT = os.environ.get("WORKER_PORT", "12090")
WORKER_API_BEARER_TOKEN = os.environ.get(
    "WORKER_API_BEARER_TOKEN", "dev-worker-api-token-change-me"
)
NODE_SLUG = os.environ.get("NODE_SLUG", "dev-node-1")
NODE_NAME = os.environ.get("NODE_NAME", "Development Node")

COMPOSE_FILE = os.path.join(PROJECT_ROOT, "orchestrator", "docker-compose.yml")
NODE_MANAGER_PID_FILE = os.path.join(
    tempfile.gettempdir(), "cynodeai-node-manager.pid"
)
# Node state dir for full-demo: node writes secrets here; E2E reads NODE_STATE_DIR to assert on it.
# Use a path under TMPDIR so the managed-agent proxy socket stays under UNIX_PATH_MAX (108).
NODE_STATE_DIR = os.path.join(tempfile.gettempdir(), "cynodeai-node-state")
# Persistent dir for setup_dev run logs (container + node-manager); overwritten each run.
LOGS_DIR = os.environ.get(
    "CYNODEAI_LOGS_DIR",
    os.path.join(tempfile.gettempdir(), "cynodeai-setup-dev-logs"),
)
# Dev builds (faster; use just build-dev)
NODE_MANAGER_BIN = os.path.join(PROJECT_ROOT, "worker_node", "bin", "node-manager-dev")
NODE_MANAGER_WORKER_API_BIN = os.path.join(
    PROJECT_ROOT, "worker_node", "bin", "worker-api-dev"
)
# When set, node-manager starts worker-api as a container (worker-managed service) instead of the binary.
# Build the image with: just build-worker-api-image
NODE_MANAGER_WORKER_API_IMAGE = os.environ.get("NODE_MANAGER_WORKER_API_IMAGE", "")


def ensure_runtime():
    """Set RUNTIME and CONTAINER_HOST_ALIAS. Return True if ok."""
    if _RuntimeState.RUNTIME:
        return True
    for r in ("podman", "docker"):
        try:
            subprocess.run(
                [r, "ps"],
                capture_output=True,
                timeout=5,
                check=False,
                shell=False,
            )
            _RuntimeState.RUNTIME = r
            _RuntimeState.CONTAINER_HOST_ALIAS = (
                os.environ.get("CONTAINER_HOST_ALIAS", "host.containers.internal")
                if r == "podman"
                else os.environ.get("CONTAINER_HOST_ALIAS", "host.docker.internal")
            )
            _sync_runtime_aliases()
            return True
        except (subprocess.TimeoutExpired, FileNotFoundError):
            continue
    return False


# Public aliases for impl (call ensure_runtime() before using)
RUNTIME = None
CONTAINER_HOST_ALIAS = None


def _sync_runtime_aliases():
    """Update RUNTIME and CONTAINER_HOST_ALIAS from _RuntimeState (for impl)."""
    mod = sys.modules[__name__]
    mod.RUNTIME = _RuntimeState.RUNTIME
    mod.CONTAINER_HOST_ALIAS = _RuntimeState.CONTAINER_HOST_ALIAS


def compose_env():
    """Env dict for compose up (exported to subprocess)."""
    ensure_runtime()
    _sync_runtime_aliases()
    # PMA via worker capability (orchestrator <-> worker proxy); no local PMA in control-plane.
    # PMA_IMAGE so control-plane sends dev image to node (node runs managed PMA container).
    return {
        "POSTGRES_USER": POSTGRES_USER,
        "POSTGRES_PASSWORD": POSTGRES_PASSWORD,
        "POSTGRES_DB": POSTGRES_DB,
        "POSTGRES_PORT": POSTGRES_PORT,
        "JWT_SECRET": JWT_SECRET,
        "NODE_REGISTRATION_PSK": NODE_PSK,
        "WORKER_API_BEARER_TOKEN": WORKER_API_BEARER_TOKEN,
        "CONTROL_PLANE_PORT": CONTROL_PLANE_PORT,
        "ORCHESTRATOR_PORT": ORCHESTRATOR_PORT,
        "PMA_PORT": PMA_PORT,
        "WORKER_API_TARGET_URL": f"http://{CONTAINER_HOST_ALIAS}:{WORKER_PORT}",
        "BOOTSTRAP_ADMIN_PASSWORD": ADMIN_PASSWORD,
        "PMA_ENABLED": "false",
        "PMA_IMAGE": os.environ.get("PMA_IMAGE", "cynodeai-cynode-pma:dev"),
    }
