# E2E parity: SBA task. Sets state.SBA_TASK_ID only on completed; fail on product failure per spec.
# Traces: REQ-SBAGNT-0001, 0106; CYNAI.SBAGNT.ResultContract, WorkerApiIntegration.

import os
import unittest

from scripts.test_scripts import config, helpers
import scripts.test_scripts.e2e_state as state


class TestSbaTask(unittest.TestCase):
    """E2E: task create --use-sba --use-inference; poll result; assert job result has sba_result."""

    tags = ["suite_agents", "full_demo", "inference", "sba_inference", "sba"]
    prereqs = ["gateway", "config", "auth", "ollama"]

    def setUp(self):
        if not state.CONFIG_PATH or not os.path.isfile(state.CONFIG_PATH):
            self.skipTest("CONFIG_PATH not set (run after auth login prereq)")
        ok, detail = helpers.ensure_valid_auth_session(state.CONFIG_PATH)
        if not ok:
            self.skipTest(f"auth session not valid: {detail}")

    def test_sba_task(self):
        """Create SBA task, poll until done; set SBA_TASK_ID only on success; assert sba_result."""
        if os.environ.get("E2E_SKIP_INFERENCE_SMOKE", "") or config.E2E_SKIP_INFERENCE_SMOKE:
            self.skipTest("E2E_SKIP_INFERENCE_SMOKE set")
        create_args = [
            "task", "create", "-p", "echo from SBA",
            "--use-sba", "--use-inference", "-o", "json",
        ]
        task_id, status, result_data = helpers.create_and_poll_sba_task(
            create_args, state.CONFIG_PATH
        )
        self.assertIsNotNone(task_id, "SBA task create failed")
        if status not in ("completed", "failed"):
            self.fail(
                "SBA task did not reach terminal status in time: "
                f"status={status!r} result={result_data}"
            )
        if status != "completed":
            self.fail(
                "SBA task failed (per REQ-SBAGNT-0109 inference must be reachable): "
                f"status={status!r} result={result_data}"
            )
        state.SBA_TASK_ID = task_id
        job_result = helpers.get_sba_job_result(result_data)
        self.assertIsNotNone(job_result, "no job result")
        self.assertIsNotNone(
            (job_result or {}).get("sba_result"),
            "job result missing sba_result",
        )
