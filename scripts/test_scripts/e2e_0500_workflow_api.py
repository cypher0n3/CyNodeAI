# E2E: control-plane workflow start/resume/checkpoint/release. Uses state.TASK_ID.
# Traces: REQ-ORCHES-0144, 0145, 0146; workflow start/resume/lease API.

import json
import unittest

from scripts.test_scripts import config, helpers
import scripts.test_scripts.e2e_state as state


class TestWorkflowAPI(unittest.TestCase):
    """E2E: POST /v1/workflow/start, resume, checkpoint, release on control-plane."""

    tags = ["suite_orchestrator", "full_demo", "no_inference", "control_plane"]

    def test_workflow_start_returns_run_id(self):
        """Start workflow for task; 200 with run_id, or 409 if lease already held."""
        if not getattr(state, "TASK_ID", None):
            self.skipTest("TASK_ID not set (run after task create)")
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
            self.skipTest("TASK_ID not set")
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
        config_path = getattr(state, "CONFIG_PATH", None)
        if not config_path:
            self.skipTest("CONFIG_PATH not set (run after login)")
        ok, out, err = helpers.run_cynork(
            ["task", "create", "-p", "e2e same-holder workflow", "-o", "json"],
            config_path,
        )
        if not ok:
            self.skipTest(f"task create failed: {out} {err}")
        data = helpers.parse_json_safe(out)
        task_id = (data or {}).get("task_id")
        if not task_id:
            self.skipTest("task create did not return task_id")
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
