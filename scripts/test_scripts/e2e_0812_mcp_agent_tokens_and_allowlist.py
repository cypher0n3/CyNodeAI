# E2E: MCP agent bearer tokens (pre-agent token in node config; PM vs sandbox allowlists).
# Traces: REQ-MCPGAT-0114, REQ-MCPGAT-0116; docs/tech_specs/mcp/mcp_gateway_enforcement.md;
# orchestrator/internal/mcpgateway/allowlist.go.

import json
import unittest
import uuid
from datetime import datetime, timezone

from scripts.test_scripts import config, helpers
import scripts.test_scripts.e2e_state as state


class TestMCPAgentTokensAndAllowlist(unittest.TestCase):
    """Live control-plane: node config delivers agent_token; gateway enforces sandbox allowlist."""

    tags = ["suite_orchestrator", "full_demo", "no_inference", "control_plane"]
    prereqs = ["gateway", "config", "auth"]

    def setUp(self):
        ok, detail = helpers.prepare_e2e_cynork_auth()
        self.assertTrue(ok, detail)

    def test_pre_agent_token_in_node_config_matches_orchestrator(self):
        """GET /v1/nodes/config includes managed_services.orchestrator.agent_token for PMA."""
        slug = "e2e-mcp-tok-" + uuid.uuid4().hex[:8]
        payload = {
            "psk": config.NODE_PSK,
            "capability": {
                "version": 1,
                "reported_at": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
                "node": {"node_slug": slug},
                "platform": {"os": "linux", "arch": "amd64"},
                "compute": {"cpu_cores": 4, "ram_mb": 8192},
                "inference": {
                    "supported": True,
                    "existing_service": False,
                    "running": False,
                },
                "worker_api": {"base_url": "http://localhost:12090"},
            },
        }
        ok, body = helpers.run_curl(
            "POST",
            f"{config.CONTROL_PLANE_API}/v1/nodes/register",
            data=json.dumps(payload),
        )
        self.assertTrue(ok, f"register failed: {body}")
        data = helpers.parse_json_safe(body) or {}
        jwt = (data.get("auth") or {}).get("node_jwt")
        self.assertTrue(jwt, "no node_jwt")

        ok, config_body = helpers.run_curl(
            "GET",
            f"{config.CONTROL_PLANE_API}/v1/nodes/config",
            headers={"Authorization": f"Bearer {jwt}"},
        )
        self.assertTrue(ok, f"GET config failed: {config_body}")
        cfg = helpers.parse_json_safe(config_body) or {}
        managed = cfg.get("managed_services") or {}
        services = managed.get("services") or []
        if not services:
            self.skipTest(
                "managed_services not present (PMA host selection / inference); "
                "not an MCP token failure"
            )
        orch = (services[0].get("orchestrator") or {}) if services else {}
        agent_tok = orch.get("agent_token")
        self.assertTrue(
            agent_tok,
            "expected orchestrator.agent_token on managed service for worker proxy",
        )
        expected = (
            config.WORKER_INTERNAL_AGENT_TOKEN.strip()
            or config.WORKER_API_BEARER_TOKEN
        )
        self.assertEqual(
            agent_tok,
            expected,
            "node config agent_token must match orchestrator-issued token for workers",
        )

    def test_mcp_bearer_pm_and_sandbox_allowlist(self):
        """PM bearer may call task tools; sandbox bearer is denied; invalid bearer 401."""
        pm = config.WORKER_INTERNAL_AGENT_TOKEN.strip()
        sand = config.MCP_SANDBOX_AGENT_BEARER_TOKEN.strip()
        if not pm or not sand:
            self.skipTest(
                "set WORKER_INTERNAL_AGENT_TOKEN and MCP_SANDBOX_AGENT_BEARER_TOKEN "
                "for gateway allowlist E2E"
            )

        token = helpers.read_token_from_config(state.CONFIG_PATH)
        self.assertTrue(token, "access token required")
        st, t_body = helpers.gateway_request(
            "POST",
            "/v1/tasks",
            token,
            {"prompt": "e2e mcp allowlist", "use_inference": False},
            timeout=60,
        )
        self.assertGreater(st, 0, "user gateway unreachable")
        self.assertIn(st, (200, 201), t_body)
        task_data = helpers.parse_json_safe(t_body) or {}
        task_id = task_data.get("task_id") or task_data.get("id")
        self.assertTrue(task_id, t_body)

        code, raw = helpers.mcp_tool_call(
            "help.list", {}, bearer_token=pm, timeout=60
        )
        self.assertEqual(code, 200, raw)

        code, raw = helpers.mcp_tool_call(
            "task.get", {"task_id": task_id}, bearer_token=pm, timeout=60
        )
        self.assertEqual(code, 200, raw)

        code, raw = helpers.mcp_tool_call(
            "task.get", {"task_id": task_id}, bearer_token=sand, timeout=60
        )
        self.assertEqual(code, 403, raw)

        code, raw = helpers.mcp_tool_call(
            "help.list", {}, bearer_token=sand, timeout=60
        )
        self.assertEqual(code, 200, raw)

        code, raw = helpers.mcp_tool_call(
            "help.list", {}, bearer_token=pm + "-invalid", timeout=60
        )
        self.assertEqual(code, 401, raw)
