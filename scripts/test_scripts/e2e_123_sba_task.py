# E2E parity: SBA task. Sets state.SBA_TASK_ID only on completed; fail on product failure per spec.
# Traces: REQ-SBAGNT-0001, 0106; CYNAI.SBAGNT.ResultContract, WorkerApiIntegration.

import os
import time
import unittest

from scripts.test_scripts import config, helpers
import scripts.test_scripts.e2e_state as state


def _poll_task_result(task_id, loops=60):
    """Poll task result until completed/failed or loops exhausted. Return (status, result_data)."""
    result_data = None
    for _ in range(loops):
        time.sleep(5)
        _, out, _ = helpers.run_cynork(
            ["task", "result", task_id, "-o", "json"], state.CONFIG_PATH
        )
        result_data = helpers.parse_json_safe(out)
        status = (result_data or {}).get("status")
        if status in ("completed", "failed"):
            return status, result_data
    return None, result_data


def _create_and_poll_sba_task(create_args, max_attempts=3):
    """Create SBA task and poll until terminal status. Return (task_id, status, result_data)."""
    for attempt in range(1, max_attempts + 1):
        _, out, _ = helpers.run_cynork(create_args, state.CONFIG_PATH)
        data = helpers.parse_json_safe(out)
        task_id = (data or {}).get("task_id")
        if not task_id:
            return None, None, None
        status, result_data = _poll_task_result(task_id)
        if status not in ("completed", "failed"):
            if attempt < max_attempts:
                continue
            return task_id, status, result_data
        if status == "completed":
            return task_id, status, result_data
        stdout = ((result_data or {}).get("stdout") or "")
        if "jobs:run" in stdout and "EOF" in stdout and attempt < max_attempts:
            time.sleep(3)
            continue
        return task_id, status, result_data
    return None, None, None


class TestSbaTask(unittest.TestCase):
    """E2E: task create --use-sba --use-inference; poll result; assert job result has sba_result."""

    def test_sba_task(self):
        """Create SBA task, poll until done; set SBA_TASK_ID only on success; assert sba_result."""
        if os.environ.get("E2E_SKIP_INFERENCE_SMOKE", "") or config.E2E_SKIP_INFERENCE_SMOKE:
            self.skipTest("E2E_SKIP_INFERENCE_SMOKE set")
        create_args = [
            "task", "create", "-p", "echo from SBA",
            "--use-sba", "--use-inference", "-o", "json",
        ]
        task_id, status, result_data = _create_and_poll_sba_task(create_args)
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
