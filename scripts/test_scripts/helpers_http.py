"""HTTP helpers for E2E: MCP tool call and user-gateway requests (stdlib urllib)."""

import json
import time
import urllib.error
import urllib.request

from scripts.test_scripts import config


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


def mcp_pm_agent_bearer_token():
    """Return PM agent bearer for MCP when configured (matches full-demo / compose defaults).

    When any MCP agent token is set on the control-plane, direct ``mcp_tool_call`` requests
    must send ``Authorization``. Use this for catalog tests that need the Project Manager
    allowlist. Returns None when unset (legacy dev with no agent tokens).
    """
    t = (config.WORKER_INTERNAL_AGENT_TOKEN or "").strip()
    return t if t else None


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
