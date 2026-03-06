# E2E: SBA result contract shape (protocol_version, job_id, status, steps, artifacts).
# Requires state.SBA_TASK_ID from e2e_123_sba_task. Skips if no SBA task was run.
# Traces: REQ-SBAGNT-0103; CYNAI.SBAGNT.ResultContract; CYNAI.WORKER.NodeMediatedSbaResultSync.

import unittest

from scripts.test_scripts import helpers
import scripts.test_scripts.e2e_state as state


class TestSbaResultContract(unittest.TestCase):
    """E2E: SBA task result must include protocol_version, job_id, status, steps, artifacts."""

    tags = ["suite_agents"]

    def test_sba_result_contract_shape(self):
        """Assert sba_result in job result contains all required contract keys."""
        if not state.SBA_TASK_ID:
            self.skipTest("SBA_TASK_ID not set (run e2e_123_sba_task first)")
        _, out, _ = helpers.run_cynork(
            ["task", "result", state.SBA_TASK_ID, "-o", "json"],
            state.CONFIG_PATH,
        )
        result_data = helpers.parse_json_safe(out)
        self.assertIsNotNone(result_data)
        self.assertEqual(
            result_data.get("status"), "completed",
            "e2e_130 validates contract only for completed SBA tasks (SBA_TASK_ID from e2e_123)"
        )
        job_result = helpers.get_sba_job_result(result_data)
        if not job_result:
            self.skipTest("no job result for SBA task")
        sba_result = (job_result or {}).get("sba_result")
        self.assertIsNotNone(sba_result, "sba_result missing")
        for key in ("protocol_version", "job_id", "status", "steps", "artifacts"):
            self.assertIn(key, sba_result, f"sba_result missing required key: {key}")
