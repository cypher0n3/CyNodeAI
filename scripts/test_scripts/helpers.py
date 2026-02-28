# Helpers for E2E: run cynork, curl, wait for gateway. No extra deps (stdlib + subprocess).
# Run from repo root with PYTHONPATH=. so "from scripts.test_scripts import config" resolves.

import json
import os
import subprocess
import tempfile
import time

from scripts.test_scripts import config


def run_cynork(args, config_path, env_extra=None, capture=True):
    """Run cynork-dev with --config; return (ok, stdout, stderr)."""
    cmd = [config.CYNORK_BIN, "--config", config_path] + list(args)
    env = os.environ.copy()
    env["CYNORK_GATEWAY_URL"] = config.USER_API
    if env_extra:
        env.update(env_extra)
    kw = {"env": env, "capture_output": capture, "text": True, "timeout": 120}
    try:
        r = subprocess.run(cmd, check=False, **kw)
        out = r.stdout or ""
        err = r.stderr or ""
        return not r.returncode, out, err
    except (subprocess.TimeoutExpired, FileNotFoundError) as e:
        return False, "", str(e)


def run_curl(method, url, data=None, headers=None):
    """Run curl; return (ok, body). With -w %{http_code}, body is stdout minus last 3 chars."""
    cmd = ["curl", "-s", "-w", "%{http_code}", "-X", method, url]
    if headers:
        for h, v in headers.items():
            cmd.extend(["-H", f"{h}: {v}"])
    if data:
        cmd.extend(["-H", "Content-Type: application/json", "-d", data])
    try:
        r = subprocess.run(
            cmd, capture_output=True, text=True, timeout=30, check=False
        )
        out = (r.stdout or "").strip()
        if len(out) >= 3 and out[-3:].isdigit():
            code = int(out[-3:])
            body = out[:-3]
        else:
            code = 0
            body = out
        return 200 <= code < 300, body
    except (subprocess.TimeoutExpired, FileNotFoundError):
        return False, ""


def wait_for_gateway(max_attempts=30, sleep=1):
    """Wait for user-gateway healthz; return True when 200."""
    for _ in range(max_attempts):
        ok, _ = run_curl("GET", f"{config.USER_API}/healthz")
        if ok:
            return True
        time.sleep(sleep)
    return False


def temp_config_dir():
    """Return a temporary directory path for cynork config (caller cleans up)."""
    return tempfile.mkdtemp(prefix="cynodeai_e2e_config_")


def parse_json_safe(text):
    """Parse JSON; return dict or None."""
    try:
        return json.loads(text) if text else None
    except json.JSONDecodeError:
        return None


def jq_get(obj, *keys, default=None):
    """Get nested key; e.g. jq_get(d, 'jobs', 0, 'result')."""
    for k in keys:
        if obj is None or not isinstance(obj, (dict, list)):
            return default
        if isinstance(obj, list) and isinstance(k, int):
            obj = obj[k] if 0 <= k < len(obj) else None
        else:
            obj = obj.get(k) if isinstance(obj, dict) else None
    return obj


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


def run_ollama_inference_smoke():
    """Run inference smoke: wait for Ollama container, pull model if needed, run one prompt.
    Skip if E2E_SKIP_INFERENCE_SMOKE set or container not present. Return True on success.
    """
    if os.environ.get("E2E_SKIP_INFERENCE_SMOKE", "") or config.E2E_SKIP_INFERENCE_SMOKE:
        return True
    runtime = _container_runtime()
    if not runtime:
        return True
    try:
        r = subprocess.run(
            [runtime, "ps", "-a", "--format", "{{.Names}}"],
            capture_output=True, text=True, timeout=10, check=False
        )
        names = (r.stdout or "").strip().splitlines()
        if config.OLLAMA_CONTAINER_NAME not in names:
            return True
    except (subprocess.TimeoutExpired, FileNotFoundError):
        return True
    for _ in range(30):
        r = subprocess.run(
            [runtime, "ps", "--format", "{{.Names}}"],
            capture_output=True, text=True, timeout=5, check=False
        )
        if config.OLLAMA_CONTAINER_NAME in (r.stdout or "").strip().splitlines():
            break
        time.sleep(1)
    else:
        return False
    r = subprocess.run(
        [runtime, "exec", config.OLLAMA_CONTAINER_NAME, "ollama", "list"],
        capture_output=True, text=True, timeout=10, check=False
    )
    if config.OLLAMA_E2E_MODEL not in (r.stdout or ""):
        for attempt in range(3):
            r = subprocess.run(
                [runtime, "exec", config.OLLAMA_CONTAINER_NAME, "ollama", "pull",
                 config.OLLAMA_E2E_MODEL],
                capture_output=True, text=True, timeout=120, check=False
            )
            if not r.returncode:
                break
            if attempt < 2:
                time.sleep(5)
        else:
            return False
    r = subprocess.run(
        [runtime, "exec", config.OLLAMA_CONTAINER_NAME, "ollama", "run",
         config.OLLAMA_E2E_MODEL, "Say one word: hello"],
        capture_output=True, text=True, timeout=60, check=False
    )
    out = (r.stdout or "").strip()
    return bool(out)
