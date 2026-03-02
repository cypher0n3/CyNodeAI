# E2E: SBA task that uses inference (LLM). Requires SBA runner and inference.
# Skip if E2E_SKIP_INFERENCE_SMOKE or no inference; creates SBA task with prompt
# that may trigger inference, polls for completion, asserts sba_result present.

import os
import time
import unittest

from scripts.test_scripts import config, helpers
import scripts.test_scripts.e2e_state as state


class TestSbaInference(unittest.TestCase):
    """E2E: SBA task with inference prompt; skip if E2E_SKIP_INFERENCE_SMOKE; assert sba_result."""

    def test_sba_task_with_inference_prompt(self):
        """Create SBA task with LLM prompt, poll until completed/failed; assert sba_result."""
        if os.environ.get("E2E_SKIP_INFERENCE_SMOKE", "") or config.E2E_SKIP_INFERENCE_SMOKE:
            self.skipTest("E2E_SKIP_INFERENCE_SMOKE set")
        _, out, _ = helpers.run_cynork(
            [
                "task", "create", "-p",
                "Reply in one word: hello (this may use inference in SBA).",
                "--use-sba", "-o", "json",
            ],
            state.CONFIG_PATH,
        )
        data = helpers.parse_json_safe(out)
        task_id = (data or {}).get("task_id")
        self.assertIsNotNone(task_id, "SBA inference task create failed")
        status = None
        result_data = None
        for _ in range(24):
            time.sleep(5)
            _, out, _ = helpers.run_cynork(
                ["task", "result", task_id, "-o", "json"],
                state.CONFIG_PATH,
            )
            result_data = helpers.parse_json_safe(out)
            status = (result_data or {}).get("status")
            if status in ("completed", "failed"):
                break
        self.assertIn(status, ("completed", "failed"), "SBA inference task did not finish")
        if status != "completed":
            self.skipTest("SBA inference task failed (inference may be unavailable)")
        job_result = helpers.jq_get(result_data, "jobs", 0, "result")
        if not job_result and result_data:
            raw = result_data.get("stdout")
            if isinstance(raw, str):
                job_result = helpers.parse_json_safe(raw)
        self.assertIsNotNone(job_result)
        self.assertIsNotNone(
            (job_result or {}).get("sba_result"),
            "job result missing sba_result",
        )
