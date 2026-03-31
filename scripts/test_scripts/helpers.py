"""Helpers for E2E: run cynork, curl, wait for gateway.

No extra deps (stdlib + subprocess). Run from repo root with PYTHONPATH=.
"""

import json
import os
from datetime import datetime, timezone
import subprocess
import tempfile
import time
import urllib.error
import urllib.request

from scripts.test_scripts import config
from scripts.test_scripts.e2e_config_file import ensure_minimal_gateway_config_yaml
from scripts.test_scripts.e2e_json import parse_json_loose
from scripts.test_scripts import e2e_json_helpers
import scripts.test_scripts.e2e_state as state

parse_json_safe = e2e_json_helpers.parse_json_safe
jq_get = e2e_json_helpers.jq_get
get_sba_job_result = e2e_json_helpers.get_sba_job_result

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
    ok_yaml, yaml_detail = ensure_minimal_gateway_config_yaml(path)
    if not ok_yaml:
        return False, yaml_detail
    return ensure_valid_auth_session(path)


def _e2e_auth_error_text(out, err):
    m = f"{out or ''}\n{err or ''}".lower()
    return "401" in m and (
        "unauthorized" in m or "invalid or expired" in m or "not logged in" in m
    )


def _run_cynork_subprocess(args, config_path, env_extra=None, timeout=None, input_text=None):
    """Run cynork subprocess with sidecar hooks.

    When config_path is set, sets XDG_CACHE_HOME beside that config so session.json
    does not collide with the developer default cache. Used by ensure_valid_auth_session
    (no outer retry loop).
    """
    if timeout is None:
        timeout = int(config.E2E_CYNORK_TIMEOUT)
    cmd = [config.CYNORK_BIN, "--config", config_path] + list(args)
    env = os.environ.copy()
    env["CYNORK_GATEWAY_URL"] = config.USER_API
    args_list = list(args)
    if config_path:
        cfg_dir = os.path.dirname(os.path.abspath(config_path))
        if cfg_dir:
            cache_home = os.path.join(cfg_dir, ".e2e-xdg-cache")
            try:
                os.makedirs(cache_home, mode=0o700, exist_ok=True)
            except OSError:
                pass
            else:
                env["XDG_CACHE_HOME"] = cache_home
        env.setdefault("CYNORK_DISABLE_OS_CREDSTORE", "1")
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


def run_curl_with_status_file(method, url, file_path, headers=None, timeout=120):
    """POST body from ``file_path`` via curl ``--data-binary @file``.

    For bodies too large for ``-d`` (e.g. API max-body checks). Returns ``(status_code, body)``.
    """
    cmd = ["curl", "-s", "-S", "-w", "%{http_code}", "-X", method, url]
    if headers:
        for h, v in headers.items():
            cmd.extend(["-H", f"{h}: {v}"])
    cmd.extend(["--data-binary", "@" + file_path])
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


def gateway_request(method, path, access_token, json_body=None, timeout=30):
    """Call user gateway path with Bearer token; return ``(status, body_str)``.

    ``path`` is e.g. ``/v1/tasks``.
    """
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


def gateway_post_task_no_inference(token, prompt, timeout=60, retries=4):
    """POST ``/v1/tasks`` with ``use_inference: false``; retry on transport failure (st==0).

    Long local E2E runs (MCP matrices, UDS proxy) can leave the next gateway call timing out;
    a short backoff reduces flakes.
    """
    last_st, last_body = 0, ""
    for _ in range(retries):
        st, body = gateway_request(
            "POST",
            "/v1/tasks",
            token,
            {"prompt": prompt, "use_inference": False},
            timeout=timeout,
        )
        if st > 0:
            return st, body
        last_st, last_body = st, body
        time.sleep(2)
    return last_st, last_body


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
    ok_yaml, yaml_detail = ensure_minimal_gateway_config_yaml(config_path)
    if not ok_yaml:
        return False, yaml_detail
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
    Refreshes the shared auth session before calling ``cynork`` so ``CYNORK_TOKEN``
    matches the E2E sidecar after long suites.

    Prefer ``cynork task create`` (binary); if that yields no task id after retries,
    fall back to user-gateway ``POST /v1/tasks`` when an access token is available.
    """
    if getattr(state, "TASK_ID", None):
        return True
    if not config_path:
        return False
    # Do not require config.yaml here: ``ensure_valid_auth_session`` creates a minimal
    # gateway_url file when only the E2E token sidecar exists (auth prereq path).
    auth_ok, _ = ensure_valid_auth_session(config_path)
    if not auth_ok:
        return False
    if _ensure_e2e_task_via_cynork(config_path, max_attempts):
        return True
    token = read_token_from_config(config_path)
    if not token:
        return False
    return _ensure_e2e_task_via_gateway(token, max_attempts)


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


def _task_id_from_create_task_payload(data):
    """Return task id string from create-task JSON (user API / cynork), or ""."""
    if not isinstance(data, dict):
        return ""
    tid = data.get("task_id") or data.get("id") or ""
    if tid is None:
        return ""
    return str(tid).strip()


def _ensure_e2e_task_via_cynork(config_path, max_attempts):
    """Run ``cynork task create``; set ``state.TASK_ID`` on success. Return bool."""
    for attempt in range(1, max_attempts + 1):
        if attempt > 1:
            time.sleep(5)
        _ok, out, err = run_cynork(
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
        data = parse_json_loose(out) or parse_json_loose(err)
        task_id = _task_id_from_create_task_payload(data)
        if task_id:
            state.TASK_ID = task_id
            return True
    return False


def _ensure_e2e_task_via_gateway(token, max_attempts):
    """POST ``/v1/tasks``; set ``state.TASK_ID`` on success. Return bool."""
    for attempt in range(1, max_attempts + 1):
        if attempt > 1:
            time.sleep(5)
        st, body = gateway_post_task_no_inference(
            token, "E2E setup: please reply ok.", timeout=60, retries=4
        )
        if st not in (200, 201):
            continue
        data = parse_json_loose(body) or {}
        tid = _task_id_from_create_task_payload(data)
        if tid:
            state.TASK_ID = tid
            return True
    return False


def cynork_task_ready(task_id, config_path):
    """Run ``cynork task ready`` so draft tasks dispatch jobs. Returns (ok, out, err)."""
    return run_cynork(
        ["task", "ready", task_id, "-o", "json"],
        config_path,
    )


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
        ok_ready, _, _ = cynork_task_ready(task_id, config_path)
        if not ok_ready:
            if attempt < max_attempts:
                time.sleep(3)
                continue
            return task_id, None, None
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
