# E2E: SBA task that uses inference (LLM). Requires SBA runner and inference.
# Skip if E2E_SKIP_INFERENCE_SMOKE or no inference; creates SBA task with prompt
# that may trigger inference, polls for completion, asserts sba_result present.

import os
import unittest

from scripts.test_scripts import config, helpers
import scripts.test_scripts.e2e_state as state


class TestSbaInference(unittest.TestCase):
    """E2E: SBA task with inference prompt; skip if E2E_SKIP_INFERENCE_SMOKE; assert sba_result."""

    tags = ["suite_agents"]

    def test_sba_task_with_inference_prompt(self):
        """Create SBA task with LLM prompt, poll until completed/failed; assert sba_result."""
        if os.environ.get("E2E_SKIP_INFERENCE_SMOKE", "") or config.E2E_SKIP_INFERENCE_SMOKE:
            self.skipTest("E2E_SKIP_INFERENCE_SMOKE set")
        create_args = [
            "task", "create", "-p",
            "Reply in one word: hello (this may use inference in SBA).",
            "--use-sba", "-o", "json",
        ]
        task_id, status, result_data = helpers.create_and_poll_sba_task(
            create_args, state.CONFIG_PATH
        )
        self.assertIsNotNone(task_id, "SBA inference task create failed")
        if status not in ("completed", "failed"):
            self.fail(
                "SBA inference task did not finish: "
                f"status={status!r} result={result_data}"
            )
        if status != "completed":
            self.fail(
                "SBA inference task failed (per spec inference path must be available): "
                f"status={status!r} result={result_data}"
            )
        job_result = helpers.get_sba_job_result(result_data)
        self.assertIsNotNone(job_result)
        self.assertIsNotNone(
            (job_result or {}).get("sba_result"),
            "job result missing sba_result",
        )
