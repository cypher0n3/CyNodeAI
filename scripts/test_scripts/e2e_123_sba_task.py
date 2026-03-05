# E2E parity: SBA task. Sets state.SBA_TASK_ID only on completed; fail on product failure per spec.
# Traces: REQ-SBAGNT-0001, 0106; CYNAI.SBAGNT.ResultContract, WorkerApiIntegration.

import os
import time
import unittest

from scripts.test_scripts import config, helpers
import scripts.test_scripts.e2e_state as state


class TestSbaTask(unittest.TestCase):
    """E2E: task create --use-sba --use-inference; poll result; assert job result has sba_result."""

    def test_sba_task(self):
        """Create SBA task, poll until done; set SBA_TASK_ID only on success; assert sba_result."""
        if os.environ.get("E2E_SKIP_INFERENCE_SMOKE", "") or config.E2E_SKIP_INFERENCE_SMOKE:
            self.skipTest("E2E_SKIP_INFERENCE_SMOKE set")
        status = None
        result_data = None
        task_id = None
        for attempt in range(1, 4):
            _, out, _ = helpers.run_cynork(
                [
                    "task",
                    "create",
                    "-p",
                    "echo from SBA",
                    "--use-sba",
                    "--use-inference",
                    "-o",
                    "json",
                ],
                state.CONFIG_PATH,
            )
            data = helpers.parse_json_safe(out)
            task_id = (data or {}).get("task_id")
            self.assertIsNotNone(task_id, "SBA task create failed")
            status = None
            result_data = None
            for _ in range(60):
                time.sleep(5)
                _, out, _ = helpers.run_cynork(
                    ["task", "result", task_id, "-o", "json"], state.CONFIG_PATH
                )
                result_data = helpers.parse_json_safe(out)
                status = (result_data or {}).get("status")
                if status in ("completed", "failed"):
                    break
            if status not in ("completed", "failed"):
                if attempt < 3:
                    continue
                self.fail(
                    "SBA task did not reach terminal status in time: "
                    f"status={status!r} result={result_data}"
                )
            if status == "completed":
                break
            stdout = ((result_data or {}).get("stdout") or "")
            if "jobs:run" in stdout and "EOF" in stdout and attempt < 3:
                time.sleep(3)
                continue
            self.fail(
                "SBA task failed (per REQ-SBAGNT-0109 inference must be reachable): "
                f"status={status!r} result={result_data}"
            )
        state.SBA_TASK_ID = task_id
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
