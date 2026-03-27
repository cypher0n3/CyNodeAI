"""Ollama container and inference smoke helpers for E2E scripts."""

import json
import os
import subprocess
import time
import urllib.error
import urllib.request

from scripts.test_scripts import config


def _container_runtime():
    """Return 'podman' or 'docker' or None."""
    for runtime in ("podman", "docker"):
        try:
            subprocess.run(
                [runtime, "ps"], capture_output=True, timeout=5, check=False
            )
            return runtime
        except (subprocess.TimeoutExpired, FileNotFoundError):
            continue
    return None


def _ollama_container_runtime():
    """Return 'podman' or 'docker' if OLLAMA_CONTAINER_NAME is running there, else None.

    Checks both engines. _container_runtime() prefers Podman when both exist, but Ollama
    may run under Docker only — without this, exec/pull and prereq smoke miss the container.
    """
    for runtime in ("podman", "docker"):
        try:
            r = subprocess.run(
                [runtime, "ps", "--format", "{{.Names}}"],
                capture_output=True,
                text=True,
                timeout=10,
                check=False,
            )
            if r.returncode:
                continue
            names = (r.stdout or "").strip().splitlines()
            if config.OLLAMA_CONTAINER_NAME in names:
                return runtime
        except (subprocess.TimeoutExpired, FileNotFoundError):
            continue
    return None


def container_runtime():
    """Public wrapper for _container_runtime (for E2E that need podman/docker)."""
    return _container_runtime()


def ensure_ollama_container_for_e2e():
    """Start Ollama container via orchestrator compose (profile ollama) if not already running.
    Used when running pma_inference E2E so chat has inference. Return True if container is or
    becomes running; False on failure. Call before the node is started (e.g. in full-demo);
    if the stack was already started without Ollama, the node will not have PMA.
    """
    if _ollama_container_runtime():
        return True
    runtime = _container_runtime()
    if not runtime:
        return False
    try:
        r = subprocess.run(
            [runtime, "ps", "--format", "{{.Names}}"],
            capture_output=True, text=True, timeout=10, check=False
        )
        names = (r.stdout or "").strip().splitlines()
        if config.OLLAMA_CONTAINER_NAME in names:
            return True
    except (subprocess.TimeoutExpired, FileNotFoundError):
        return False
    compose_file = os.path.join(config.PROJECT_ROOT, "orchestrator", "docker-compose.yml")
    if not os.path.isfile(compose_file):
        return False
    env = os.environ.copy()
    r = subprocess.run(
        [runtime, "compose", "-f", compose_file, "--profile", "ollama", "up", "-d", "ollama"],
        cwd=config.PROJECT_ROOT,
        env=env,
        capture_output=True,
        text=True,
        timeout=120,
        check=False,
    )
    if r.returncode:
        return False
    for _ in range(30):
        r = subprocess.run(
            [runtime, "ps", "--format", "{{.Names}}"],
            capture_output=True, text=True, timeout=5, check=False
        )
        if config.OLLAMA_CONTAINER_NAME in (r.stdout or "").strip().splitlines():
            return True
        time.sleep(1)
    return False


def ollama_container_running():
    """Return True if the E2E Ollama container is running."""
    return _ollama_container_runtime() is not None


def get_ollama_container_image():
    """Return the image string of the running Ollama container, or None if not found."""
    runtime = _ollama_container_runtime()
    if not runtime:
        return None
    try:
        r = subprocess.run(
            [
                runtime, "ps", "-a",
                "--filter", f"name={config.OLLAMA_CONTAINER_NAME}",
                "--format", "{{.Image}}",
            ],
            capture_output=True, text=True, timeout=10, check=False,
        )
        if r.returncode or not r.stdout:
            return None
        img = (r.stdout or "").strip().splitlines()
        return img[0] if img else None
    except (subprocess.TimeoutExpired, FileNotFoundError):
        return None


def is_ollama_model_available(model_name: str) -> bool:
    """Return True if *model_name* is already pulled in the Ollama container."""
    runtime = _ollama_container_runtime()
    if not runtime:
        return False
    try:
        r = subprocess.run(
            [runtime, "exec", config.OLLAMA_CONTAINER_NAME, "ollama", "list"],
            capture_output=True, text=True, timeout=10, check=False
        )
        return model_name in (r.stdout or "")
    except (subprocess.TimeoutExpired, FileNotFoundError):
        return False


def _ollama_wait_container_ready(runtime):
    """Return True if Ollama container appears in ps within ~30s."""
    for _ in range(30):
        r = subprocess.run(
            [runtime, "ps", "--format", "{{.Names}}"],
            capture_output=True, text=True, timeout=5, check=False
        )
        if config.OLLAMA_CONTAINER_NAME in (r.stdout or "").strip().splitlines():
            return True
        time.sleep(1)
    return False


def _ollama_ensure_model(runtime, model_name: str) -> bool:
    """Pull model_name in the Ollama container if not already listed. Return True on success."""
    name = (model_name or "").strip()
    if not name:
        return True
    for _attempt in range(3):
        try:
            r = subprocess.run(
                [runtime, "exec", config.OLLAMA_CONTAINER_NAME, "ollama", "list"],
                capture_output=True, text=True, timeout=30, check=False
            )
            if name in (r.stdout or ""):
                return True
            break
        except subprocess.TimeoutExpired:
            time.sleep(5)
            continue
    for attempt in range(3):
        r = subprocess.run(
            [runtime, "exec", config.OLLAMA_CONTAINER_NAME, "ollama", "pull", name],
            capture_output=True, text=True, timeout=600, check=False
        )
        if not r.returncode:
            return True
        if attempt < 2:
            time.sleep(5)
    return False


def _ollama_chat_one_request(ollama_url, payload):
    """Send one chat request; return True if response has non-empty content."""
    try:
        req = urllib.request.Request(
            ollama_url.rstrip("/") + "/api/chat",
            data=payload,
            headers={"Content-Type": "application/json"},
            method="POST",
        )
        timeout_sec = max(30, int(config.OLLAMA_SMOKE_CHAT_TIMEOUT))
        with urllib.request.urlopen(req, timeout=timeout_sec) as resp:
            body = resp.read().decode("utf-8", errors="replace")
            data = json.loads(body)
            content = (data.get("message") or {}).get("content", "").strip()
            return bool(content)
    except (urllib.error.URLError, OSError, json.JSONDecodeError, TimeoutError):
        return False


def run_ollama_inference_smoke():
    """Run inference smoke: ensure Ollama at OLLAMA_BASE_URL responds to a chat request.
    Skip if E2E_SKIP_INFERENCE_SMOKE set. Optional: if a container named OLLAMA_CONTAINER_NAME
    is running, wait for it and pull OLLAMA_E2E_MODEL (and OLLAMA_CAPABLE_MODEL when enabled)
    there; then try chat. Pass/fail is based only on whether the chat request succeeds, not on
    container name or pull. Return True on success.
    """
    if os.environ.get("E2E_SKIP_INFERENCE_SMOKE", "") or config.E2E_SKIP_INFERENCE_SMOKE:
        return True
    ollama_url = os.environ.get("OLLAMA_BASE_URL", "http://localhost:11434")
    payload = json.dumps({
        "model": config.OLLAMA_E2E_MODEL,
        "messages": [{"role": "user", "content": "Say one word: hello"}],
        "stream": False,
    }).encode()
    ollama_rt = _ollama_container_runtime()
    if ollama_rt:
        _ollama_wait_container_ready(ollama_rt)
        _ollama_ensure_model(ollama_rt, config.OLLAMA_E2E_MODEL)
        cap = (config.OLLAMA_CAPABLE_MODEL or "").strip()
        if (
            cap
            and cap != config.OLLAMA_E2E_MODEL
            and config.OLLAMA_AUTO_PULL_CAPABLE
        ):
            _ollama_ensure_model(ollama_rt, cap)
    for attempt in range(5):
        if _ollama_chat_one_request(ollama_url, payload):
            return True
        if not attempt and not _ollama_container_runtime():
            ensure_ollama_container_for_e2e()
            ollama_rt = _ollama_container_runtime()
            if ollama_rt:
                _ollama_wait_container_ready(ollama_rt)
                _ollama_ensure_model(ollama_rt, config.OLLAMA_E2E_MODEL)
        time.sleep(5)
    return False
