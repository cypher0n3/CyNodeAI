# E2E: User-gateway access control and request-body limits (plans 1–3 regression hooks).
# Traces: features/e2e/access_control_gateway.feature; REQ-MCPGAT-0001; httplimits.DefaultMaxAPIRequestBodyBytes.

import os
import tempfile
import unittest

from scripts.test_scripts import config, helpers
import scripts.test_scripts.e2e_state as state

# Parity with go_shared_libs/httplimits.DefaultMaxAPIRequestBodyBytes
_DEFAULT_MAX_API_BODY_BYTES = 10 * 1024 * 1024


class TestGatewayUnauthenticatedAccess(unittest.TestCase):
    """E2E: protected routes reject unauthenticated callers."""

    tags = ["suite_orchestrator", "full_demo", "no_inference", "gateway"]
    prereqs = ["gateway"]

    def test_get_tasks_without_bearer_returns_401(self):
        """GET /v1/tasks without Authorization must be 401 (gateway parity with Gherkin)."""
        st, body = helpers.gateway_request(
            "GET", "/v1/tasks", access_token=None, json_body=None, timeout=30
        )
        self.assertEqual(st, 401, body[:500] if body else "")


class TestGatewayBodyLimitsAndMCPAuth(unittest.TestCase):
    """E2E: oversized POST /v1/tasks rejected; MCP requires bearer when agent tokens are configured."""

    tags = ["suite_orchestrator", "full_demo", "no_inference", "gateway", "control_plane"]
    prereqs = ["gateway", "config", "auth"]

    def setUp(self):
        ok, detail = helpers.prepare_e2e_cynork_auth()
        self.assertTrue(ok, detail)

    def test_post_tasks_oversized_body_rejected(self):
        """POST /v1/tasks with JSON > DefaultMaxAPIRequestBodyBytes must not succeed (400 from decode)."""
        token = helpers.read_token_from_config(state.CONFIG_PATH)
        self.assertTrue(token, "access token required")
        prefix = b'{"prompt":"'
        suffix = b'","use_inference":false}'
        pad_len = _DEFAULT_MAX_API_BODY_BYTES + 1 - len(prefix) - len(suffix)
        self.assertGreater(pad_len, 0, "padding math for oversized body")
        fd, path = tempfile.mkstemp(prefix="e2e_oversize_", suffix=".json")
        try:
            with os.fdopen(fd, "wb") as f:
                f.write(prefix)
                f.write(b"x" * pad_len)
                f.write(suffix)
            url = config.USER_API.rstrip("/") + "/v1/tasks"
            headers = {
                "Authorization": "Bearer " + token.strip(),
                "Content-Type": "application/json",
            }
            st, body = helpers.run_curl_with_status_file(
                "POST", url, path, headers=headers, timeout=300
            )
            self.assertGreater(st, 0, "gateway unreachable (curl status 0)")
            self.assertNotIn(
                st,
                range(200, 300),
                f"oversized task create must not succeed: {st} {body[:200]!r}",
            )
            self.assertEqual(
                st,
                400,
                "task handler maps oversize JSON decode errors to 400; "
                f"got {st}: {body[:300]!r}",
            )
        finally:
            try:
                os.remove(path)
            except OSError:
                pass

    def test_mcp_tool_call_without_bearer_returns_401_when_enforced(self):
        """POST /v1/mcp/tools/call without Authorization when agent tokens are set → 401."""
        if not (
            config.WORKER_INTERNAL_AGENT_TOKEN.strip()
            or config.MCP_SANDBOX_AGENT_BEARER_TOKEN.strip()
            or config.MCP_PA_AGENT_BEARER_TOKEN.strip()
        ):
            self.skipTest(
                "no MCP agent bearer tokens in env; control-plane does not enforce missing bearer"
            )
        st, raw = helpers.mcp_tool_call("help.list", {}, bearer_token=None, timeout=30)
        self.assertEqual(st, 401, raw[:500] if raw else "")
