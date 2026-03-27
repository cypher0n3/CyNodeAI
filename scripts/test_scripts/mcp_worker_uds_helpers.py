"""Worker node state paths and MCP calls over per-service UDS (managed agent proxy)."""

import base64
import glob
import json
import os
import shutil
import subprocess
import tempfile


def resolve_worker_node_state_dir():
    """Resolve host dir for node-manager ``run/managed_agent_proxy`` (see scripts/dev_stack.sh).

    Tries ``NODE_STATE_DIR``, ``WORKER_API_STATE_DIR``, ``SETUP_DEV_NODE_STATE_DIR``,
    ``CYNODE_STATE_DIR``, then ``<system temp>/cynodeai-node-state`` if that path exists.
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
    default = os.path.join(tempfile.gettempdir(), "cynodeai-node-state")
    if os.path.isdir(default):
        return default
    return ""


def find_managed_agent_proxy_socks(state_dir):
    """Return sorted paths ``.../run/managed_agent_proxy/<service_id>/proxy.sock``."""
    if not state_dir:
        return []
    pattern = os.path.join(state_dir, "run", "managed_agent_proxy", "*", "proxy.sock")
    return sorted(glob.glob(pattern))


def mcp_tool_call_via_worker_uds_internal_proxy(
    proxy_sock_path, tool_name, arguments=None, timeout=60
):
    """PMA-equivalent: managed proxy envelope over per-service UDS (mcp:call).

    POST ``/v1/worker/internal/orchestrator/mcp:call`` with the same JSON envelope as
    ``agents/internal/mcpclient`` ``ManagedServiceProxyRequest`` /
    ``callViaWorkerInternalProxy``.

    Returns ``(curl_ok, envelope_status, mcp_body_dict_or_none, err_detail)``.
    """
    if not shutil.which("curl"):
        return False, None, None, "curl not found in PATH"
    inner = json.dumps({"tool_name": tool_name, "arguments": arguments or {}}).encode(
        "utf-8"
    )
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
        check=False,
    )
    detail = (proc.stderr or "") + (proc.stdout or "")
    if proc.returncode:
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
    """Return ``(http_status, response_body_str)`` like ``helpers.mcp_tool_call``."""
    ok, env_status, data, err = mcp_tool_call_via_worker_uds_internal_proxy(
        proxy_sock_path, tool_name, arguments=arguments, timeout=timeout
    )
    if not ok:
        return 0, err
    status = env_status if env_status is not None else 0
    if data is None:
        return status, ""
    return status, json.dumps(data)
