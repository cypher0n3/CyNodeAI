# E2E parity: SBA task. Sets state.sba_task_id.

import time
import unittest

from scripts.test_scripts import helpers
import scripts.test_scripts.e2e_state as state


class TestSbaTask(unittest.TestCase):
    def test_sba_task(self):
        _, out, err = helpers.run_cynork(
            ["task", "create", "-p", "echo from SBA", "--use-sba", "-o", "json"],
            state.config_path,
        )
        data = helpers.parse_json_safe(out)
        task_id = (data or {}).get("task_id")
        self.assertIsNotNone(task_id, "SBA task create failed")
        state.sba_task_id = task_id
        status = None
        result_data = None
        for _ in range(18):
            time.sleep(5)
            _, out, _ = helpers.run_cynork(
                ["task", "result", task_id, "-o", "json"], state.config_path
            )
            result_data = helpers.parse_json_safe(out)
            status = (result_data or {}).get("status")
            if status in ("completed", "failed"):
                break
        self.assertEqual(status, "completed", "SBA task did not complete")
        job_result = helpers.jq_get(result_data, "jobs", 0, "result")
        self.assertIsNotNone(job_result, "no job result")
        self.assertIsNotNone(
            (job_result or {}).get("sba_result"),
            "job result missing sba_result",
        )
