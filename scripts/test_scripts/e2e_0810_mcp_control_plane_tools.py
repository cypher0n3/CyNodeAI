# E2E: POST /v1/mcp/tools/call on control-plane — every routed MCP tool name (or 404 path).
# Same matrix over the live worker UDS path (POST …/mcp:call) as cynode-pma uses.
# Traces: docs/tech_specs/mcp_tooling.md; REQ-MCPGAT (gateway routing); node_tools.md.

import os
import unittest
import uuid

from scripts.test_scripts import helpers
import scripts.test_scripts.e2e_state as state


class TestMCPControlPlaneToolRoutes(unittest.TestCase):
    """Exercise orchestrator MCP gateway handlers against a live control-plane + DB."""

    tags = ["suite_orchestrator", "full_demo", "no_inference", "control_plane"]
    prereqs = ["gateway", "config", "auth"]

    def setUp(self):
        ok, detail = helpers.prepare_e2e_cynork_auth()
        self.assertTrue(ok, detail)

    def _mcp_tool_route_suite(self, mcp):
        """Run every catalog MCP tool call (and legacy 404 paths).

        Pass a single ``mcp(tool, args) -> (code, raw, data)``; use direct HTTP to the control
        plane in one test and worker UDS ``mcp:call`` in another. Do not mix both in one suite
        (mutating tools would double-apply and diverge).
        """
        token = helpers.read_token_from_config(state.CONFIG_PATH)
        self.assertTrue(token, "access token required (auth whoami sidecar)")

        st, me_body = helpers.gateway_request(
            "GET", "/v1/users/me", token, timeout=60
        )
        self.assertEqual(st, 200, me_body)
        me = helpers.parse_json_safe(me_body) or {}
        user_id = me.get("id")
        self.assertTrue(user_id, f"users/me: {me_body}")

        # --- help.list (no store) ---
        with self.subTest(tool="help.list"):
            code, raw, data = mcp("help.list", {})
            self.assertEqual(code, 200, raw)
            self.assertIsInstance(data, dict)
            self.assertIn("topics", data)

        with self.subTest(tool="legacy_db_prefix_rejected"):
            code, raw, _ = mcp("db.preference.get", {})
            self.assertEqual(code, 404, raw)

        # --- node.list / node.get ---
        with self.subTest(tool="node.list"):
            code, raw, data = mcp("node.list", {})
            self.assertEqual(code, 200, raw)
            self.assertIsInstance(data, dict)
            self.assertIn("nodes", data)
        node_slug = None
        nodes = data.get("nodes") if isinstance(data, dict) else None
        if isinstance(nodes, list) and nodes:
            first = nodes[0]
            if isinstance(first, dict):
                node_slug = first.get("node_slug")
        with self.subTest(tool="node.get"):
            if node_slug:
                code, raw, _ = mcp("node.get", {"node_slug": node_slug})
                self.assertEqual(code, 200, raw)
            else:
                code, raw, _ = mcp(
                    "node.get", {"node_slug": "no-such-node-" + uuid.uuid4().hex}
                )
                self.assertEqual(code, 404, raw)

        # --- Create task (user API) so task-scoped tools have a real task_id ---
        st, t_body = helpers.gateway_request(
            "POST",
            "/v1/tasks",
            token,
            {"prompt": "e2e mcp tool routing (no inference)", "use_inference": False},
            timeout=60,
        )
        self.assertGreater(st, 0, "user gateway unreachable (check USER_API / stack)")
        self.assertIn(st, (200, 201), t_body)
        task_data = helpers.parse_json_safe(t_body) or {}
        task_id = task_data.get("task_id") or task_data.get("id")
        self.assertTrue(task_id, f"task create: {t_body}")

        with self.subTest(tool="task.get"):
            code, raw, data = mcp("task.get", {"task_id": task_id})
            self.assertEqual(code, 200, raw)
            self.assertEqual(data.get("id"), task_id)

        with self.subTest(tool="task.list"):
            code, raw, data = mcp("task.list", {"user_id": user_id})
            self.assertEqual(code, 200, raw)
            self.assertIsInstance(data, dict)
            self.assertIn("tasks", data)

        with self.subTest(tool="help.get"):
            code, raw, data = mcp("help.get", {"topic": "tools"})
            self.assertEqual(code, 200, raw)
            self.assertIn("content", data)

        with self.subTest(tool="help.get_no_task_id"):
            code, raw, data = mcp("help.get", {"topic": "tools"})
            self.assertEqual(code, 200, raw)
            self.assertIn("content", data)
            self.assertNotIn("task_id", data)

        with self.subTest(tool="preference.effective"):
            code, raw, data = mcp("preference.effective", {"task_id": task_id})
            self.assertEqual(code, 200, raw)
            self.assertIsInstance(data, dict)
            self.assertIn("effective", data)

        with self.subTest(tool="preference.list"):
            code, raw, data = mcp("preference.list", {"scope_type": "system"})
            self.assertEqual(code, 200, raw)
            self.assertIn("entries", data)

        pref_key = "e2e.mcp." + uuid.uuid4().hex
        with self.subTest(tool="preference.create"):
            code, raw, _ = mcp(
                "preference.create",
                {
                    "scope_type": "system",
                    "key": pref_key,
                    "value": '"e2e"',
                    "value_type": "string",
                },
            )
            self.assertEqual(code, 200, raw)

        with self.subTest(tool="preference.get"):
            code, raw, data = mcp(
                "preference.get",
                {"scope_type": "system", "key": pref_key},
            )
            self.assertEqual(code, 200, raw)
            self.assertEqual(data.get("key"), pref_key)

        with self.subTest(tool="preference.update"):
            code, raw, _ = mcp(
                "preference.update",
                {
                    "scope_type": "system",
                    "key": pref_key,
                    "value": '"e2e2"',
                    "value_type": "string",
                },
            )
            self.assertEqual(code, 200, raw)

        with self.subTest(tool="preference.delete"):
            code, raw, _ = mcp(
                "preference.delete",
                {"scope_type": "system", "key": pref_key},
            )
            self.assertEqual(code, 200, raw)

        ss_key = "e2e.mcp.system_setting." + uuid.uuid4().hex
        with self.subTest(tool="system_setting.list"):
            code, raw, data = mcp("system_setting.list", {})
            self.assertEqual(code, 200, raw)
            self.assertIn("entries", data)
        with self.subTest(tool="system_setting.create"):
            code, raw, _ = mcp(
                "system_setting.create",
                {
                    "key": ss_key,
                    "value": '"e2e"',
                    "value_type": "string",
                },
            )
            self.assertEqual(code, 200, raw)
        with self.subTest(tool="system_setting.get"):
            code, raw, data = mcp("system_setting.get", {"key": ss_key})
            self.assertEqual(code, 200, raw)
            self.assertEqual(data.get("key"), ss_key)
        with self.subTest(tool="system_setting.update"):
            code, raw, _ = mcp(
                "system_setting.update",
                {
                    "key": ss_key,
                    "value": '"e2e2"',
                    "value_type": "string",
                },
            )
            self.assertEqual(code, 200, raw)
        with self.subTest(tool="system_setting.delete"):
            code, raw, _ = mcp("system_setting.delete", {"key": ss_key})
            self.assertEqual(code, 200, raw)

        with self.subTest(tool="task.result"):
            code, raw, data = mcp("task.result", {"task_id": task_id})
            self.assertEqual(code, 200, raw)
            self.assertIsInstance(data, dict)
            self.assertIn("jobs", data)

        with self.subTest(tool="task.logs"):
            code, raw, data = mcp("task.logs", {"task_id": task_id})
            self.assertEqual(code, 200, raw)
            self.assertIsInstance(data, dict)
            self.assertEqual(data.get("task_id"), task_id)

        with self.subTest(tool="job.get"):
            bogus_job = str(uuid.uuid4())
            code, raw, _ = mcp("job.get", {"job_id": bogus_job})
            self.assertEqual(code, 404, raw)

        with self.subTest(tool="artifact.get"):
            code, raw, _ = mcp(
                "artifact.get",
                {"task_id": task_id, "path": "no/such/artifact.txt"},
            )
            self.assertEqual(code, 404, raw)

        proj_id = None
        with self.subTest(tool="project.list"):
            code, raw, data = mcp("project.list", {"user_id": user_id})
            self.assertEqual(code, 200, raw)
            projects = data.get("projects") if isinstance(data, dict) else None
            self.assertIsInstance(projects, list)
            self.assertGreaterEqual(len(projects), 1, data)
            proj_id = projects[0].get("id") if projects else None
            self.assertTrue(proj_id)

        with self.subTest(tool="project.get"):
            code, raw, data = mcp(
                "project.get",
                {"user_id": user_id, "project_id": proj_id},
            )
            self.assertEqual(code, 200, raw)
            self.assertEqual(data.get("id"), proj_id)

        skill_body = "# E2E MCP skill\n\nPlain content for routing test.\n"
        skill_id = None
        with self.subTest(tool="skills.create"):
            code, raw, data = mcp(
                "skills.create",
                {
                    "user_id": user_id,
                    "name": "e2e-mcp-skill",
                    "content": skill_body,
                    "scope": "user",
                },
            )
            self.assertEqual(code, 200, raw)
            skill_id = data.get("id")
            self.assertTrue(skill_id)

        with self.subTest(tool="skills.list"):
            code, raw, data = mcp("skills.list", {"user_id": user_id})
            self.assertEqual(code, 200, raw)
            self.assertIn("skills", data)

        with self.subTest(tool="skills.get"):
            code, raw, data = mcp(
                "skills.get",
                {"user_id": user_id, "skill_id": skill_id},
            )
            self.assertEqual(code, 200, raw)
            self.assertEqual(data.get("id"), skill_id)

        with self.subTest(tool="skills.update"):
            code, raw, _ = mcp(
                "skills.update",
                {
                    "user_id": user_id,
                    "skill_id": skill_id,
                    "content": skill_body + "\nupdated line.\n",
                },
            )
            self.assertEqual(code, 200, raw)

        with self.subTest(tool="skills.delete"):
            code, raw, _ = mcp(
                "skills.delete",
                {"user_id": user_id, "skill_id": skill_id},
            )
            self.assertEqual(code, 200, raw)

        # Second task for cancel (leave first task for reads above).
        st, t2_body = helpers.gateway_request(
            "POST",
            "/v1/tasks",
            token,
            {"prompt": "e2e mcp cancel target", "use_inference": False},
            timeout=60,
        )
        self.assertGreater(st, 0, "user gateway unreachable (check USER_API / stack)")
        self.assertIn(st, (200, 201), t2_body)
        t2 = helpers.parse_json_safe(t2_body) or {}
        task_id_cancel = t2.get("task_id") or t2.get("id")
        self.assertTrue(task_id_cancel)

        with self.subTest(tool="task.cancel"):
            code, raw, data = mcp("task.cancel", {"task_id": task_id_cancel})
            self.assertEqual(code, 200, raw)
            self.assertTrue(data.get("canceled"), raw)

    def test_mcp_tool_routes_round_trip(self):
        """Direct control-plane POST /v1/mcp/tools/call — full catalog matrix."""

        def mcp_direct(tool, args=None):
            code, raw = helpers.mcp_tool_call(
                tool, arguments=args, timeout=60
            )
            return code, raw, helpers.parse_json_safe(raw)

        self._mcp_tool_route_suite(mcp_direct)

    def test_mcp_via_worker_uds_internal_proxy_live_path(self):
        """Full catalog matrix via UDS only: worker ``mcp:call`` → control-plane (PMA path)."""
        if os.environ.get("E2E_SKIP_WORKER_MCP_UDS", "").strip().lower() in (
            "1",
            "true",
            "yes",
            "on",
        ):
            self.skipTest("E2E_SKIP_WORKER_MCP_UDS is set")

        state_dir = helpers.resolve_worker_node_state_dir()
        socks = helpers.find_managed_agent_proxy_socks(state_dir)
        if not socks:
            msg = (
                "no per-service proxy.sock under run/managed_agent_proxy "
                f"(resolved state_dir={state_dir!r}; set NODE_STATE_DIR or WORKER_API_STATE_DIR). "
                "Requires a managed service with orchestrator MCP proxy (e.g. PMA). "
                "Set E2E_REQUIRE_WORKER_MCP_UDS=1 to fail instead of skip."
            )
            if os.environ.get("E2E_REQUIRE_WORKER_MCP_UDS", "").strip().lower() in (
                "1",
                "true",
                "yes",
                "on",
            ):
                self.fail(msg)
            self.skipTest(msg)

        for sock in socks:

            def mcp_uds(tool, args=None, _sock=sock):
                code, raw = helpers.mcp_tool_call_worker_uds(
                    _sock, tool, arguments=args, timeout=120
                )
                return code, raw, helpers.parse_json_safe(raw)

            with self.subTest(proxy_sock=sock):
                self._mcp_tool_route_suite(mcp_uds)
