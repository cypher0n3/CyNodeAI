# E2E parity: SBA task. Sets state.sba_task_id.

import time
import unittest

from scripts.test_scripts import helpers
import scripts.test_scripts.e2e_state as state


class TestSbaTask(unittest.TestCase):
    """E2E: task create --use-sba; poll result; assert job result has sba_result."""

    def test_sba_task(self):
        """Create SBA task, poll until completed; set state.SBA_TASK_ID; assert sba_result."""
        _, out, _ = helpers.run_cynork(
            ["task", "create", "-p", "echo from SBA", "--use-sba", "-o", "json"],
            state.CONFIG_PATH,
        )
        data = helpers.parse_json_safe(out)
        task_id = (data or {}).get("task_id")
        self.assertIsNotNone(task_id, "SBA task create failed")
        state.SBA_TASK_ID = task_id
        status = None
        result_data = None
        for _ in range(18):
            time.sleep(5)
            _, out, _ = helpers.run_cynork(
                ["task", "result", task_id, "-o", "json"], state.CONFIG_PATH
            )
            result_data = helpers.parse_json_safe(out)
            status = (result_data or {}).get("status")
            if status in ("completed", "failed"):
                break
        self.assertEqual(status, "completed", "SBA task did not complete")
        # Cynork result: .jobs[0].result or .stdout (when .jobs absent)
        job_result = helpers.jq_get(result_data, "jobs", 0, "result")
        if not job_result and result_data:
            raw = result_data.get("stdout")
            if isinstance(raw, str):
                job_result = helpers.parse_json_safe(raw)
        self.assertIsNotNone(job_result, "no job result")
        self.assertIsNotNone(
            (job_result or {}).get("sba_result"),
            "job result missing sba_result",
        )
