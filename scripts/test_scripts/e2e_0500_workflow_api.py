# E2E: control-plane workflow start/resume/checkpoint/release. Requires task_id prereq.
# Traces: REQ-ORCHES-0144, 0145, 0146; workflow start/resume/lease API.

import json
import unittest

from scripts.test_scripts import config, helpers
import scripts.test_scripts.e2e_state as state


class TestWorkflowAPI(unittest.TestCase):
    """E2E: POST /v1/workflow/start, resume, checkpoint, release on control-plane."""

    tags = ["suite_orchestrator", "full_demo", "no_inference", "control_plane"]
    prereqs = ["gateway", "config", "auth", "task_id"]

    def setUp(self):
        ok, detail = helpers.prepare_e2e_cynork_auth()
        self.assertTrue(ok, detail)

    def test_workflow_start_returns_run_id(self):
        """Start workflow for task; 200 with run_id, or 409 if lease already held."""
        if not getattr(state, "TASK_ID", None):
            self.skipTest("TASK_ID not set (task_id prereq failed or not declared)")
        ok_r, _, err_r = helpers.run_cynork(
            ["task", "ready", state.TASK_ID, "-o", "json"],
            state.CONFIG_PATH,
        )
        self.assertTrue(ok_r, f"task ready failed: {err_r}")
        headers = {}
        if getattr(config, "WORKFLOW_RUNNER_BEARER_TOKEN", ""):
            headers["Authorization"] = f"Bearer {config.WORKFLOW_RUNNER_BEARER_TOKEN}"
        body = json.dumps({"task_id": state.TASK_ID, "holder_id": "e2e-holder"})
        code, resp_body = helpers.run_curl_with_status(
            "POST",
            f"{config.CONTROL_PLANE_API}/v1/workflow/start",
            data=body,
            headers=headers or None,
        )
        self.assertIn(code, (200, 409), f"workflow start: {code} {resp_body}")
        if code == 200:
            data = helpers.parse_json_safe(resp_body)
            self.assertIn("run_id", data or {}, "run_id in response")

    def test_workflow_start_duplicate_returns_409(self):
        """Start workflow twice for same task; second returns 409."""
        if not getattr(state, "TASK_ID", None):
            self.skipTest("TASK_ID not set (task_id prereq failed or not declared)")
        ok_r, _, err_r = helpers.run_cynork(
            ["task", "ready", state.TASK_ID, "-o", "json"],
            state.CONFIG_PATH,
        )
        self.assertTrue(ok_r, f"task ready failed: {err_r}")
        headers = {}
        if getattr(config, "WORKFLOW_RUNNER_BEARER_TOKEN", ""):
            headers["Authorization"] = f"Bearer {config.WORKFLOW_RUNNER_BEARER_TOKEN}"
        body = json.dumps({"task_id": state.TASK_ID, "holder_id": "e2e-holder-2"})
        code1, _ = helpers.run_curl_with_status(
            "POST",
            f"{config.CONTROL_PLANE_API}/v1/workflow/start",
            data=body,
            headers=headers or None,
        )
        code2, _ = helpers.run_curl_with_status(
            "POST",
            f"{config.CONTROL_PLANE_API}/v1/workflow/start",
            data=body,
            headers=headers or None,
        )
        self.assertIn(code1, (200, 409), "first start 200 or 409")
        if code1 == 200:
            self.assertEqual(code2, 409, "second start must be 409 when lease held")

    def test_workflow_start_same_holder_returns_200_already_running(self):
        """Same holder starts again with idempotency_key=lease_id; expect 200 already_running."""
        ok, out, err = helpers.run_cynork(
            ["task", "create", "-p", "e2e same-holder workflow", "-o", "json"],
            state.CONFIG_PATH,
        )
        if not ok:
            self.skipTest(f"task create failed: {out} {err}")
        data = helpers.parse_json_safe(out)
        task_id = (data or {}).get("task_id")
        if not task_id:
            self.skipTest("task create did not return task_id")
        ok_r, _, err_r = helpers.run_cynork(
            ["task", "ready", task_id, "-o", "json"],
            state.CONFIG_PATH,
        )
        self.assertTrue(ok_r, f"task ready failed: {err_r}")
        headers = {}
        if getattr(config, "WORKFLOW_RUNNER_BEARER_TOKEN", ""):
            headers["Authorization"] = f"Bearer {config.WORKFLOW_RUNNER_BEARER_TOKEN}"
        body1 = json.dumps({"task_id": task_id, "holder_id": "e2e-same-holder"})
        code1, resp1 = helpers.run_curl_with_status(
            "POST",
            f"{config.CONTROL_PLANE_API}/v1/workflow/start",
            data=body1,
            headers=headers or None,
        )
        if code1 != 200:
            self.fail(f"first start failed: {code1} {resp1}")
        data1 = helpers.parse_json_safe(resp1)
        lease_id = (data1 or {}).get("lease_id")
        if not lease_id:
            self.fail("first start response missing lease_id")
        body2 = json.dumps({
            "task_id": task_id,
            "holder_id": "e2e-same-holder",
            "idempotency_key": lease_id,
        })
        code2, resp2 = helpers.run_curl_with_status(
            "POST",
            f"{config.CONTROL_PLANE_API}/v1/workflow/start",
            data=body2,
            headers=headers or None,
        )
        self.assertEqual(code2, 200, f"same holder start with idempotency_key: {code2} {resp2}")
        data2 = helpers.parse_json_safe(resp2)
        self.assertIn("status", data2 or {}, "status in response")
        self.assertEqual(
            (data2 or {}).get("status"),
            "already_running",
            f"expected status already_running: {data2}",
        )

    def test_workflow_verification_checkpoint_persists_review_state(self):
        """Checkpoint at verify_step_result stores PMA->PAA review JSON; resume returns it."""
        if not getattr(state, "TASK_ID", None):
            self.skipTest("TASK_ID not set (task_id prereq failed or not declared)")
        ok_r, _, err_r = helpers.run_cynork(
            ["task", "ready", state.TASK_ID, "-o", "json"],
            state.CONFIG_PATH,
        )
        self.assertTrue(ok_r, f"task ready failed: {err_r}")
        headers = {}
        if getattr(config, "WORKFLOW_RUNNER_BEARER_TOKEN", ""):
            headers["Authorization"] = f"Bearer {config.WORKFLOW_RUNNER_BEARER_TOKEN}"
        tid = state.TASK_ID
        body_start = json.dumps({"task_id": tid, "holder_id": "e2e-verify-holder"})
        code, _ = helpers.run_curl_with_status(
            "POST",
            f"{config.CONTROL_PLANE_API}/v1/workflow/start",
            data=body_start,
            headers=headers or None,
        )
        self.assertIn(code, (200, 409), f"workflow start: {code}")
        verify_state = json.dumps(
            {
                "pma_tasked_paa": True,
                "paa_outcome": "accepted",
                "findings": "e2e verification slice",
            }
        )
        body_cp = json.dumps(
            {
                "task_id": tid,
                "last_node_id": "verify_step_result",
                "state": verify_state,
            }
        )
        code_cp, _ = helpers.run_curl_with_status(
            "POST",
            f"{config.CONTROL_PLANE_API}/v1/workflow/checkpoint",
            data=body_cp,
            headers=headers or None,
        )
        self.assertEqual(code_cp, 204, "checkpoint must return 204")
        body_res = json.dumps({"task_id": tid})
        code_r, raw = helpers.run_curl_with_status(
            "POST",
            f"{config.CONTROL_PLANE_API}/v1/workflow/resume",
            data=body_res,
            headers=headers or None,
        )
        self.assertEqual(code_r, 200, f"resume: {raw}")
        data = helpers.parse_json_safe(raw) or {}
        self.assertEqual(
            data.get("last_node_id"),
            "verify_step_result",
            data,
        )
        st = data.get("state")
        self.assertIsNotNone(st, "resume must include state")
        self.assertIn("paa_outcome", str(st))
