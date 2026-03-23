# Unit tests for the worker managed-service proxy payload encoding (proxy protocol).
# Traces: worker_api.md managed service proxy; REQ-ORCHES-0162 PMA routing via worker.
# Functional proxy tests (start worker-api + PMA) were removed: standalone worker-api
# proxy path no longer exists; proxy runs only inside node-manager after registration.

import base64
import json
import unittest


def build_proxy_request(method, path, body_bytes, headers=None):
    """Build JSON body for POST /v1/worker/managed-services/{id}/proxy:http."""
    if headers is None:
        headers = {"Content-Type": ["application/json"]}
    return {
        "version": 1,
        "method": method,
        "path": path,
        "headers": headers,
        "body_b64": base64.standard_b64encode(body_bytes).decode("ascii"),
    }


def parse_proxy_response(resp_body_str):
    """Parse proxy response JSON; return (status, body_bytes from body_b64)."""
    data = json.loads(resp_body_str)
    status = data.get("status", 0)
    b64 = data.get("body_b64", "")
    raw = base64.standard_b64decode(b64) if b64 else b""
    return status, raw


def build_chat_completion_body(messages):
    """Build body for POST /internal/chat/completion (PMA handoff)."""
    return json.dumps({"messages": messages}).encode("utf-8")


class TestProxyPayloadEncoding(unittest.TestCase):
    """Unit tests: proxy request/response encoding and decoding."""

    tags = ["suite_proxy_pma", "suite_worker_node", "full_demo", "pma", "no_inference"]
    prereqs = []

    def test_build_proxy_request_shape(self):
        """Proxy request has version, method, path, headers, body_b64."""
        body = build_chat_completion_body([{"role": "user", "content": "Hi"}])
        req = build_proxy_request("POST", "/internal/chat/completion", body)
        self.assertEqual(req["version"], 1)
        self.assertEqual(req["method"], "POST")
        self.assertEqual(req["path"], "/internal/chat/completion")
        self.assertIn("Content-Type", req["headers"])
        self.assertIn("body_b64", req)
        decoded = base64.standard_b64decode(req["body_b64"])
        self.assertEqual(json.loads(decoded), {"messages": [{"role": "user", "content": "Hi"}]})

    def test_parse_proxy_response(self):
        """Parse proxy response status and body_b64."""
        content = '{"content":"hello"}'
        payload = {
            "version": 1,
            "status": 200,
            "body_b64": base64.standard_b64encode(content.encode()).decode("ascii"),
        }
        status, raw = parse_proxy_response(json.dumps(payload))
        self.assertEqual(status, 200)
        self.assertEqual(json.loads(raw.decode()), {"content": "hello"})

    def test_mcp_tool_call_proxy_uses_control_plane_path(self):
        """MCP tools use POST /v1/mcp/tools/call on the control plane.

        Not a separate gateway service.
        """
        body = json.dumps({"tool_name": "help.list", "arguments": {}}).encode()
        req = build_proxy_request("POST", "/v1/mcp/tools/call", body)
        self.assertEqual(req["path"], "/v1/mcp/tools/call")
