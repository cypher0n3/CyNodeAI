# E2E: planning_state draft on create; workflow start blocked until POST /ready.
# Traces: REQ-ORCHES-0176, REQ-ORCHES-0177, REQ-ORCHES-0178.

import json
import time
import unittest

from scripts.test_scripts import config, helpers
import scripts.test_scripts.e2e_state as state


def _workflow_start_headers():
    headers = {}
    if getattr(config, "WORKFLOW_RUNNER_BEARER_TOKEN", ""):
        headers["Authorization"] = f"Bearer {config.WORKFLOW_RUNNER_BEARER_TOKEN}"
    return headers or None


class TestTaskPlanningState(unittest.TestCase):
    """E2E: create returns draft; workflow denied until task ready; ready promotes state."""

    tags = ["suite_cynork", "full_demo", "task", "no_inference"]
    prereqs = ["gateway", "config", "auth", "node_register"]

    def setUp(self):
        ok, detail = helpers.prepare_e2e_cynork_auth()
        self.assertTrue(ok, detail)

    def test_create_returns_planning_state_draft(self):
        """POST /v1/tasks via cynork returns planning_state=draft."""
        for attempt in range(1, 4):
            if attempt > 1:
                time.sleep(5)
            ok, out, err = helpers.run_cynork(
                [
                    "task",
                    "create",
                    "-p",
                    "e2e planning_state draft assertion.",
                    "-o",
                    "json",
                ],
                state.CONFIG_PATH,
            )
            if not ok:
                if attempt == 3:
                    self.fail(f"task create failed: {out} {err}")
                continue
            data = helpers.parse_json_safe(out)
            task_id = (data or {}).get("task_id") or ""
            if not task_id:
                if attempt == 3:
                    self.fail(f"no task_id in create response: {out}")
                continue
            self.assertEqual(
                (data or {}).get("planning_state"),
                "draft",
                f"expected planning_state draft: {data}",
            )
            return
        self.fail("unreachable")

    def test_workflow_start_denied_until_ready_then_ok(self):
        """Draft: workflow/start 409; after task ready, start returns 200 or 409 (lease)."""
        if not getattr(config, "WORKFLOW_RUNNER_BEARER_TOKEN", ""):
            self.skipTest(
                "WORKFLOW_RUNNER_BEARER_TOKEN unset; control-plane workflow auth required"
            )
        ok, out, err = helpers.run_cynork(
            [
                "task",
                "create",
                "-p",
                "e2e planning_state workflow gate.",
                "-o",
                "json",
            ],
            state.CONFIG_PATH,
        )
        self.assertTrue(ok, f"task create: {out} {err}")
        data = helpers.parse_json_safe(out)
        task_id = (data or {}).get("task_id")
        self.assertTrue(task_id, f"no task_id: {data}")
        self.assertEqual((data or {}).get("planning_state"), "draft", data)

        body = json.dumps({"task_id": task_id, "holder_id": "e2e-ps-gate-holder"})
        code, resp_body = helpers.run_curl_with_status(
            "POST",
            f"{config.CONTROL_PLANE_API}/v1/workflow/start",
            data=body,
            headers=_workflow_start_headers(),
        )
        self.assertEqual(code, 409, f"expected 409 when draft: {code} {resp_body}")
        err_obj = helpers.parse_json_safe(resp_body) or {}
        self.assertIn(
            "task not ready",
            str(err_obj.get("detail", "")).lower(),
            resp_body,
        )

        ok2, out2, err2 = helpers.run_cynork(
            ["task", "ready", task_id, "-o", "json"],
            state.CONFIG_PATH,
        )
        self.assertTrue(ok2, f"task ready: {out2} {err2}")
        ready_data = helpers.parse_json_safe(out2)
        self.assertEqual(
            (ready_data or {}).get("planning_state"),
            "ready",
            ready_data,
        )

        code3, resp3 = helpers.run_curl_with_status(
            "POST",
            f"{config.CONTROL_PLANE_API}/v1/workflow/start",
            data=body,
            headers=_workflow_start_headers(),
        )
        self.assertIn(
            code3,
            (200, 409),
            f"after ready, workflow start: {code3} {resp3}",
        )
