"""Helpers for E2E: run cynork, curl, wait for gateway.

No extra deps (stdlib + subprocess). Run from repo root with PYTHONPATH=.
"""

import base64
import glob
import json
import os
import shutil
from datetime import datetime, timezone
import subprocess
import tempfile
import time
import urllib.error
import urllib.request

from scripts.test_scripts import config
import scripts.test_scripts.e2e_state as state

# Cynork persists gateway_url + TUI prefs in config.yaml only (no secrets).
# E2E stores access/refresh tokens beside the config file for subprocess env injection.
_E2E_GATEWAY_SESSION_FILE = "e2e_gateway_session.json"


def _e2e_session_path(config_path):
    if not config_path:
        return None
    return os.path.join(os.path.dirname(config_path), _E2E_GATEWAY_SESSION_FILE)


def clear_e2e_gateway_session(config_path):
    """Remove E2E token sidecar (e.g. after auth logout)."""
    path = _e2e_session_path(config_path)
    if path and os.path.isfile(path):
        try:
            os.remove(path)
        except OSError:
            pass


def _read_e2e_session_tokens(config_path):
    """Return (access_token, refresh_token) from sidecar JSON, or (None, None)."""
    path = _e2e_session_path(config_path)
    if not path or not os.path.isfile(path):
        return None, None
    try:
        with open(path, encoding="utf-8") as f:
            data = json.load(f)
        acc = data.get("access_token")
        ref = data.get("refresh_token")
        if isinstance(acc, str) and acc.strip():
            return acc.strip(), ref.strip() if isinstance(ref, str) else None
    except (OSError, json.JSONDecodeError, TypeError, ValueError):
        pass
    return None, None


def write_e2e_gateway_session(config_path, access_token, refresh_token):
    """Write access/refresh tokens to sidecar JSON (0600)."""
    path = _e2e_session_path(config_path)
    if not path:
        return
    d = os.path.dirname(path)
    if d:
        os.makedirs(d, mode=0o700, exist_ok=True)
    payload = {"access_token": access_token, "refresh_token": refresh_token or ""}
    fd, tmp = tempfile.mkstemp(
        prefix=".e2e_session.",
        suffix=".tmp",
        dir=d if d else None,
    )
    try:
        with os.fdopen(fd, "w", encoding="utf-8") as f:
            json.dump(payload, f)
        os.chmod(tmp, 0o600)
        os.replace(tmp, path)
    except OSError:
        try:
            os.remove(tmp)
        except OSError:
            pass


def fetch_gateway_login_tokens(timeout=30):
    """POST /v1/auth/login; return (access_token, refresh_token) or (None, None)."""
    url = config.USER_API.rstrip("/") + "/v1/auth/login"
    body = json.dumps(
        {"handle": "admin", "password": config.ADMIN_PASSWORD}
    ).encode()
    req = urllib.request.Request(
        url,
        data=body,
        method="POST",
        headers={"Content-Type": "application/json"},
    )
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            if resp.status != 200:
                return None, None
            data = json.loads(resp.read().decode())
            acc = data.get("access_token")
            ref = data.get("refresh_token")
            if isinstance(acc, str) and acc.strip():
                return acc.strip(), ref.strip() if isinstance(ref, str) else None
            return None, None
    except (urllib.error.URLError, OSError, json.JSONDecodeError, ValueError, TypeError):
        return None, None


def fetch_gateway_refresh_tokens(refresh_token, timeout=30):
    """POST /v1/auth/refresh; return (access_token, refresh_token) or (None, None)."""
    if not refresh_token or not str(refresh_token).strip():
        return None, None
    url = config.USER_API.rstrip("/") + "/v1/auth/refresh"
    body = json.dumps({"refresh_token": refresh_token}).encode()
    req = urllib.request.Request(
        url,
        data=body,
        method="POST",
        headers={"Content-Type": "application/json"},
    )
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            if resp.status != 200:
                return None, None
            data = json.loads(resp.read().decode())
            acc = data.get("access_token")
            ref = data.get("refresh_token")
            if isinstance(acc, str) and acc.strip():
                return acc.strip(), ref.strip() if isinstance(ref, str) else None
            return None, None
    except (urllib.error.URLError, OSError, json.JSONDecodeError, ValueError, TypeError):
        return None, None


def fetch_gateway_access_token(timeout=30):
    """POST /v1/auth/login; return access_token or None."""
    acc, _ = fetch_gateway_login_tokens(timeout=timeout)
    return acc


def sync_e2e_gateway_session_from_login(config_path, timeout=30):
    """Fetch tokens via POST /v1/auth/login and write E2E sidecar. Return True if written."""
    acc, ref = fetch_gateway_login_tokens(timeout=timeout)
    if acc:
        write_e2e_gateway_session(config_path, acc, ref or "")
        return True
    return False


def prepare_e2e_cynork_auth():
    """Ensure temp config, minimal config.yaml, and a valid gateway login (E2E token sidecar).

    Cynork does not persist bearer tokens in config.yaml (REQ-CLIENT-0103); E2E stores
    access/refresh beside the config file. Call from test ``setUp`` so modules work under
    ``--single`` as well as the full suite (runner ``auth`` prereq is complementary).

    Returns:
        tuple[bool, str]: ``(True, detail)`` on success, ``(False, error_message)`` on failure.
    """
    state.init_config()
    path = state.CONFIG_PATH
    if not path:
        return False, "CONFIG_PATH unset after init_config"
    d = os.path.dirname(path)
    if d:
        try:
            os.makedirs(d, mode=0o700, exist_ok=True)
        except OSError as exc:
            return False, f"create config dir: {exc}"
    if not os.path.isfile(path):
        try:
            with open(path, "w", encoding="utf-8") as f:
                f.write(f"gateway_url: {config.USER_API}\n")
        except OSError as exc:
            return False, f"write config: {exc}"
    return ensure_valid_auth_session(path)


def _e2e_auth_error_text(out, err):
    m = f"{out or ''}\n{err or ''}".lower()
    return "401" in m and (
        "unauthorized" in m or "invalid or expired" in m or "not logged in" in m
    )


def _run_cynork_subprocess(args, config_path, env_extra=None, timeout=None, input_text=None):
    """Run cynork subprocess with sidecar hooks.

    Used by ensure_valid_auth_session (no outer retry loop).
    """
    if timeout is None:
        timeout = int(config.E2E_CYNORK_TIMEOUT)
    cmd = [config.CYNORK_BIN, "--config", config_path] + list(args)
    env = os.environ.copy()
    env["CYNORK_GATEWAY_URL"] = config.USER_API
    args_list = list(args)
    if config_path:
        acc, ref = _read_e2e_session_tokens(config_path)
        if acc:
            env["CYNORK_TOKEN"] = acc
        if ref:
            env["CYNORK_REFRESH_TOKEN"] = ref
    if env_extra:
        env.update(env_extra)
    kw = {
        "env": env,
        "capture_output": True,
        "text": True,
        "timeout": timeout,
        "input": input_text,
    }
    try:
        r = subprocess.run(cmd, check=False, **kw)
        out = r.stdout or ""
        err = r.stderr or ""
        ok = not r.returncode
        if ok and config_path and len(args_list) >= 2 and args_list[0] == "auth":
            if args_list[1] == "login":
                sync_e2e_gateway_session_from_login(config_path, timeout=30)
            elif args_list[1] == "refresh":
                sync_e2e_gateway_session_from_login(config_path, timeout=30)
            elif args_list[1] == "logout":
                clear_e2e_gateway_session(config_path)
        return ok, out, err
    except (subprocess.TimeoutExpired, FileNotFoundError) as e:
        return False, "", str(e)


def run_cynork(args, config_path, env_extra=None, timeout=None, input_text=None):
    """Run cynork-dev with --config; return (ok, stdout, stderr).

    Injects CYNORK_TOKEN / CYNORK_REFRESH_TOKEN from the E2E sidecar when present
    (cynork does not persist tokens in config.yaml). After successful auth login,
    refresh, or logout, updates or clears the sidecar to match the gateway session.
    On 401/invalid token, refreshes the E2E sidecar and retries once (long suites
    outlive JWT lifetime).
    If timeout is None, uses config.E2E_CYNORK_TIMEOUT (env E2E_CYNORK_TIMEOUT).
    """
    args_list = list(args)
    ok, out, err = _run_cynork_subprocess(
        args, config_path, env_extra=env_extra, timeout=timeout, input_text=input_text
    )
    if (
        not ok
        and config_path
        and args_list
        and args_list[0] not in ("auth", "version", "status")
        and _e2e_auth_error_text(out, err)
    ):
        recovered, _ = ensure_valid_auth_session(config_path)
        if recovered:
            ok, out, err = _run_cynork_subprocess(
                args,
                config_path,
                env_extra=env_extra,
                timeout=timeout,
                input_text=input_text,
            )
    return ok, out, err


def run_curl_with_status(method, url, data=None, headers=None, timeout=30):
    """Run curl; return (status_code, body). Caller can assert on code (e.g. 200, 403, 501)."""
    cmd = ["curl", "-s", "-w", "%{http_code}", "-X", method, url]
    if headers:
        for h, v in headers.items():
            cmd.extend(["-H", f"{h}: {v}"])
    if data:
        cmd.extend(["-H", "Content-Type: application/json", "-d", data])
    try:
        r = subprocess.run(
            cmd, capture_output=True, text=True, timeout=timeout, check=False
        )
        out = (r.stdout or "").strip()
        if len(out) >= 3 and out[-3:].isdigit():
            code = int(out[-3:])
            body = out[:-3]
        else:
            code = 0
            body = out
        return code, body
    except (subprocess.TimeoutExpired, FileNotFoundError):
        return 0, ""


def run_curl(method, url, data=None, headers=None, timeout=30):
    """Run curl; return (ok, body). ok is True when status is 2xx."""
    code, body = run_curl_with_status(
        method, url, data=data, headers=headers, timeout=timeout
    )
    return 200 <= code < 300, body


def mcp_tool_call(tool_name, arguments=None, timeout=30, bearer_token=None):
    """POST control-plane ``/v1/mcp/tools/call``.

    When ``bearer_token`` is set, sends ``Authorization: Bearer …`` (PM or sandbox agent token).
    Return (http_status, response_body_str).
    """
    url = config.CONTROL_PLANE_API.rstrip("/") + "/v1/mcp/tools/call"
    payload = {"tool_name": tool_name, "arguments": arguments or {}}
    body = json.dumps(payload).encode("utf-8")
    headers = {"Content-Type": "application/json"}
    if bearer_token:
        headers["Authorization"] = "Bearer " + str(bearer_token).strip()
    req = urllib.request.Request(
        url,
        data=body,
        method="POST",
        headers=headers,
    )
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            raw = resp.read().decode("utf-8", errors="replace")
            return resp.status, raw
    except urllib.error.HTTPError as e:
        try:
            raw = e.read().decode("utf-8", errors="replace") if e.fp else ""
            return e.code, raw
        finally:
            e.close()

    except (urllib.error.URLError, OSError, ValueError):
        return 0, ""


def resolve_worker_node_state_dir():
    """Host directory where node-manager stores ``run/managed_agent_proxy`` (see scripts/dev_stack.sh).

    Tries ``NODE_STATE_DIR``, ``WORKER_API_STATE_DIR``, ``SETUP_DEV_NODE_STATE_DIR``,
    ``CYNODE_STATE_DIR``, then ``${TMPDIR:-/tmp}/cynodeai-node-state`` if that path exists.
    """
    for key in (
        "NODE_STATE_DIR",
        "WORKER_API_STATE_DIR",
        "SETUP_DEV_NODE_STATE_DIR",
        "CYNODE_STATE_DIR",
    ):
        v = os.environ.get(key, "").strip()
        if v and os.path.isdir(v):
            return v
    default = os.path.join(os.environ.get("TMPDIR", "/tmp"), "cynodeai-node-state")
    if os.path.isdir(default):
        return default
    return ""


def find_managed_agent_proxy_socks(state_dir):
    """Return sorted paths ``.../run/managed_agent_proxy/<service_id>/proxy.sock``."""
    if not state_dir:
        return []
    pattern = os.path.join(state_dir, "run", "managed_agent_proxy", "*", "proxy.sock")
    return sorted(glob.glob(pattern))


def mcp_tool_call_via_worker_uds_internal_proxy(proxy_sock_path, tool_name, arguments=None, timeout=60):
    """Live PMA-equivalent path: managed proxy envelope over per-service UDS (mcp:call).

    Same JSON body shape as ``agents/internal/mcpclient`` ``ManagedServiceProxyRequest`` /
    ``callViaWorkerInternalProxy``: POST ``/v1/worker/internal/orchestrator/mcp:call``.

    Returns ``(curl_ok, envelope_status, mcp_body_dict_or_none, err_detail)``:
    ``curl_ok`` when curl exits 0 and response JSON parses; ``envelope_status`` is the
    upstream HTTP status (e.g. 200 for MCP success); ``mcp_body_dict_or_none`` is the decoded
    MCP JSON from ``body_b64`` when present.
    """
    if not shutil.which("curl"):
        return False, None, None, "curl not found in PATH"
    inner = json.dumps(
        {"tool_name": tool_name, "arguments": arguments or {}}
    ).encode("utf-8")
    envelope = {
        "version": 1,
        "method": "POST",
        "path": "/v1/mcp/tools/call",
        "headers": {"Content-Type": ["application/json"]},
        "body_b64": base64.standard_b64encode(inner).decode("ascii"),
    }
    payload = json.dumps(envelope)
    proc = subprocess.run(
        [
            "curl",
            "-sS",
            "--fail-with-body",
            "--unix-socket",
            proxy_sock_path,
            "-X",
            "POST",
            "http://localhost/v1/worker/internal/orchestrator/mcp:call",
            "-H",
            "Content-Type: application/json",
            "-d",
            payload,
        ],
        capture_output=True,
        text=True,
        timeout=timeout,
    )
    detail = (proc.stderr or "") + (proc.stdout or "")
    if proc.returncode != 0:
        return False, None, None, detail
    try:
        outer = json.loads(proc.stdout)
    except json.JSONDecodeError as e:
        return False, None, None, f"invalid JSON: {e}: {detail}"
    env_status = outer.get("status")
    b64 = outer.get("body_b64") or ""
    raw = base64.standard_b64decode(b64) if b64 else b""
    if not raw:
        return True, env_status, None, detail
    try:
        inner_obj = json.loads(raw.decode("utf-8"))
    except json.JSONDecodeError as e:
        return True, env_status, None, f"body_b64 decode: {e}: {raw!r}"
    return True, env_status, inner_obj, detail


def mcp_tool_call_worker_uds(proxy_sock_path, tool_name, arguments=None, timeout=60):
    """Same return shape as :func:`mcp_tool_call`: ``(http_status, response_body_str)``.

    Uses :func:`mcp_tool_call_via_worker_uds_internal_proxy` (live curl to per-service UDS).
    """
    ok, env_status, data, err = mcp_tool_call_via_worker_uds_internal_proxy(
        proxy_sock_path, tool_name, arguments=arguments, timeout=timeout
    )
    if not ok:
        return 0, err
    status = env_status if env_status is not None else 0
    if data is None:
        return status, ""
    return status, json.dumps(data)


def gateway_request(method, path, access_token, json_body=None, timeout=30):
    """Call user gateway ``path`` (e.g. ``/v1/tasks``) with Bearer token. Return (status, body_str)."""
    url = config.USER_API.rstrip("/") + path
    headers = {"Content-Type": "application/json"} if json_body is not None else {}
    if access_token:
        headers["Authorization"] = "Bearer " + access_token.strip()
    data = json.dumps(json_body).encode("utf-8") if json_body is not None else None
    req = urllib.request.Request(url, data=data, method=method, headers=headers)
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            return resp.status, resp.read().decode("utf-8", errors="replace")
    except urllib.error.HTTPError as e:
        try:
            raw = e.read().decode("utf-8", errors="replace") if e.fp else ""
            return e.code, raw
        finally:
            e.close()

    except (urllib.error.URLError, OSError, ValueError):
        return 0, ""


def read_refresh_token_from_config(config_path):
    """Read refresh token from E2E sidecar or legacy YAML. Return None if missing."""
    _, ref = _read_e2e_session_tokens(config_path)
    if ref:
        return ref
    if not config_path or not os.path.isfile(config_path):
        return None
    try:
        with open(config_path, encoding="utf-8") as f:
            for line in f:
                line = line.strip()
                if line.startswith("refresh_token:"):
                    val = line.split(":", 1)[1].strip().strip('"\'')
                    return val or None
    except OSError:
        pass
    return None


def read_token_from_config(config_path):
    """Read access token: E2E sidecar first, then legacy YAML token: line."""
    acc, _ = _read_e2e_session_tokens(config_path)
    if acc:
        return acc
    if not config_path or not os.path.isfile(config_path):
        return None
    try:
        with open(config_path, encoding="utf-8") as f:
            for line in f:
                line = line.strip()
                if line.startswith("token:"):
                    val = line.split(":", 1)[1].strip().strip('"\'')
                    return val or None
    except OSError:
        pass
    return None


def ensure_valid_auth_session(config_path):
    """Ensure cynork config has a currently valid auth session; return (ok, detail)."""
    if not config_path or not os.path.isfile(config_path):
        return False, "config path missing"
    token = read_token_from_config(config_path)
    if not token:
        ok, out, err = _run_cynork_subprocess(
            ["auth", "login", "-u", "admin", "--password-stdin"],
            config_path,
            input_text=f"{config.ADMIN_PASSWORD}\n",
            timeout=30,
        )
        if not ok:
            return False, f"login failed: stdout={out!r} stderr={err!r}"
        return True, "login_ok"
    ok, out, err = _run_cynork_subprocess(["auth", "whoami"], config_path, timeout=30)
    if ok:
        return True, "whoami_ok"
    merged = f"{out}\n{err}".lower()
    if "invalid or expired token" in merged:
        refreshed, rout, rerr = _run_cynork_subprocess(
            ["auth", "refresh"], config_path, timeout=30
        )
        if refreshed:
            recheck, _, _ = _run_cynork_subprocess(
                ["auth", "whoami"], config_path, timeout=30
            )
            if recheck:
                return True, "refresh_ok"
        ok, lout, lerr = _run_cynork_subprocess(
            ["auth", "login", "-u", "admin", "--password-stdin"],
            config_path,
            input_text=f"{config.ADMIN_PASSWORD}\n",
            timeout=30,
        )
        if ok:
            return True, "relogin_ok"
        return (
            False,
            (
                "refresh/login recovery failed: "
                f"refresh_stdout={rout!r} refresh_stderr={rerr!r} "
                f"login_stdout={lout!r} login_stderr={lerr!r}"
            ),
        )
    return False, f"whoami failed: stdout={out!r} stderr={err!r}"


def ensure_e2e_task(config_path, max_attempts=3):
    """Create one prompt task and set state.TASK_ID for tests that need it.

    Idempotent if already set. Return True if state.TASK_ID is set, False on failure.
    Requires valid auth (ensure_valid_auth_session first).
    """
    if getattr(state, "TASK_ID", None):
        return True
    if not config_path or not os.path.isfile(config_path):
        return False
    for attempt in range(1, max_attempts + 1):
        if attempt > 1:
            time.sleep(5)
        _, out, _ = run_cynork(
            [
                "task",
                "create",
                "-p",
                "E2E setup: please reply ok.",
                "-o",
                "json",
            ],
            config_path,
        )
        data = parse_json_safe(out)
        task_id = (data or {}).get("task_id") or ""
        if task_id:
            state.TASK_ID = task_id
            return True
    return False


def ensure_node_registered():
    """Register node with control-plane and set state.NODE_JWT for capability/tests.

    Idempotent if state.NODE_JWT already set. Return True if set, False on failure.
    """
    if getattr(state, "NODE_JWT", None):
        return True
    payload = {
        "psk": config.NODE_PSK,
        "capability": {
            "version": 1,
            "reported_at": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
            "node": {"node_slug": "test-e2e-node"},
            "platform": {"os": "linux", "arch": "amd64"},
            "compute": {"cpu_cores": 4, "ram_mb": 8192},
            "worker_api": {"base_url": config.WORKER_API},
        },
    }
    ok, body = run_curl(
        "POST", config.CONTROL_PLANE_API + "/v1/nodes/register",
        data=json.dumps(payload),
    )
    if not ok:
        return False
    data = parse_json_safe(body)
    jwt = (data or {}).get("auth", {}).get("node_jwt")
    if not jwt:
        return False
    state.NODE_JWT = jwt
    return True


def ensure_e2e_sba_task(config_path):
    """Create one SBA task and set state.SBA_TASK_ID when completed.

    Return True if state.SBA_TASK_ID is set, False on failure or non-completed.
    Requires auth and inference (ollama). For use in tests that need SBA result contract.
    """
    if getattr(state, "SBA_TASK_ID", None):
        return True
    if not config_path or not os.path.isfile(config_path):
        return False
    create_args = [
        "task", "create", "-p", "echo from SBA",
        "--use-sba", "--use-inference", "-o", "json",
    ]
    task_id, status, _ = create_and_poll_sba_task(
        create_args, config_path
    )
    if task_id and status == "completed":
        state.SBA_TASK_ID = task_id
        return True
    return False


def read_config_value(config_path, key):
    """Read a simple YAML-like scalar by key, or E2E session tokens for token/refresh_token."""
    if key == "token":
        return read_token_from_config(config_path)
    if key == "refresh_token":
        return read_refresh_token_from_config(config_path)
    if not config_path or not os.path.isfile(config_path):
        return None
    prefix = f"{key}:"
    try:
        with open(config_path, encoding="utf-8") as f:
            for line in f:
                line = line.strip()
                if line.startswith(prefix):
                    val = line.split(":", 1)[1].strip().strip('"\'')
                    return val or None
    except OSError:
        pass
    return None


def wait_for_gateway(max_attempts=30, sleep=1):
    """Wait for user-gateway healthz; return True when 200."""
    for _ in range(max_attempts):
        ok, _ = run_curl("GET", f"{config.USER_API}/healthz")
        if ok:
            return True
        time.sleep(sleep)
    return False


def wait_for_gateway_readyz(timeout_sec=30):
    """Wait for user-gateway /readyz 200 (ready to accept work). Return True when 200."""
    for _ in range(timeout_sec):
        ok, _ = run_curl("GET", f"{config.USER_API}/readyz", timeout=5)
        if ok:
            return True
        time.sleep(1)
    return False


def wait_for_pma_chat_ready(timeout_sec=120, poll_interval=5):
    """Wait until gateway accepts PMA chat (POST /v1/chat/completions returns 2xx not 503).
    Logs in via cynork to get a token, then polls until PM agent is available or timeout.
    Return True when chat returns 2xx; False on timeout or auth failure.
    """
    with tempfile.TemporaryDirectory(prefix="cynodeai_e2e_wait_chat_") as tmpdir:
        config_path = os.path.join(tmpdir, "config.yaml")
        ok, _, _ = run_cynork(
            ["auth", "login", "-u", "admin", "--password-stdin"],
            config_path,
            input_text=f"{config.ADMIN_PASSWORD}\n",
        )
        if not ok:
            return False
        token = read_token_from_config(config_path)
        if not token:
            return False
    body = '{"model":"cynodeai.pm","messages":[{"role":"user","content":"hi"}]}'
    headers = {"Authorization": f"Bearer {token}"}
    deadline = time.monotonic() + timeout_sec
    while time.monotonic() < deadline:
        code, _ = run_curl_with_status(
            "POST",
            f"{config.USER_API}/v1/chat/completions",
            data=body,
            headers=headers,
            timeout=15,
        )
        if 200 <= code < 300:
            return True
        if code != 503:
            return False
        time.sleep(poll_interval)
    return False


def temp_config_dir():
    """Return a temporary directory path for cynork config (caller cleans up)."""
    return tempfile.mkdtemp(prefix="cynodeai_e2e_config_")


def parse_sse_stream_typed(response):
    """Parse SSE stream preserving event: and data: order.

    Returns (events, found_done). Each event is {"event": type or None, "data": str}.
    event is the value after 'event: ' (e.g. cynodeai.iteration_start); None for unnamed data.
    """
    events = []
    found_done = False
    current_event = None
    for line in response.iter_lines(decode_unicode=True):
        if not line:
            continue
        if line.startswith("event: "):
            current_event = line[len("event: "):].strip()
            continue
        if line.startswith("data: "):
            data = line[len("data: "):]
            if data == "[DONE]":
                found_done = True
                break
            events.append({"event": current_event, "data": data})
            current_event = None
    return events, found_done


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


def get_sba_job_result(result_data):
    """Job result from task result (jobs[0].result or parsed stdout). Return dict or None."""
    job_result = jq_get(result_data, "jobs", 0, "result")
    if not job_result and result_data:
        raw = result_data.get("stdout")
        if isinstance(raw, str):
            job_result = parse_json_safe(raw)
    return job_result


def poll_task_result(task_id, config_path, loops=60):
    """Poll task result until completed/failed or loops exhausted. Return (status, result_data)."""
    result_data = None
    for _ in range(loops):
        time.sleep(5)
        _, out, _ = run_cynork(
            ["task", "result", task_id, "-o", "json"],
            config_path,
        )
        result_data = parse_json_safe(out)
        status = (result_data or {}).get("status")
        if status in ("completed", "failed"):
            return status, result_data
    return None, result_data


def create_and_poll_sba_task(create_args, config_path, max_attempts=3):
    """Create SBA task and poll until terminal status. Return (task_id, status, result_data)."""
    for attempt in range(1, max_attempts + 1):
        _, out, _ = run_cynork(create_args, config_path)
        data = parse_json_safe(out)
        task_id = (data or {}).get("task_id")
        if not task_id:
            return None, None, None
        status, result_data = poll_task_result(task_id, config_path)
        if status not in ("completed", "failed"):
            if attempt < max_attempts:
                continue
            return task_id, status, result_data
        if status == "completed":
            return task_id, status, result_data
        stdout = ((result_data or {}).get("stdout") or "")
        if "jobs:run" in stdout and "EOF" in stdout and attempt < max_attempts:
            time.sleep(3)
            continue
        return task_id, status, result_data
    return None, None, None


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
