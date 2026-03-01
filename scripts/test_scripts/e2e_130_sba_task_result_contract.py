# E2E: SBA result contract shape (protocol_version, job_id, status, steps, artifacts).
# Requires state.SBA_TASK_ID from e2e_120_sba_task. Skips if no SBA task was run.

import unittest

from scripts.test_scripts import helpers
import scripts.test_scripts.e2e_state as state


class TestSbaResultContract(unittest.TestCase):
    def test_sba_result_contract_shape(self):
        if not state.SBA_TASK_ID:
            self.skipTest("SBA_TASK_ID not set (run e2e_120_sba_task first)")
        _, out, _ = helpers.run_cynork(
            ["task", "result", state.SBA_TASK_ID, "-o", "json"],
            state.CONFIG_PATH,
        )
        result_data = helpers.parse_json_safe(out)
        self.assertIsNotNone(result_data)
        job_result = helpers.jq_get(result_data, "jobs", 0, "result")
        if not job_result and result_data:
            raw = result_data.get("stdout")
            if isinstance(raw, str):
                job_result = helpers.parse_json_safe(raw)
        if not job_result:
            self.skipTest("no job result for SBA task")
        sba_result = (job_result or {}).get("sba_result")
        self.assertIsNotNone(sba_result, "sba_result missing")
        for key in ("protocol_version", "job_id", "status", "steps", "artifacts"):
            self.assertIn(key, sba_result, f"sba_result missing required key: {key}")
