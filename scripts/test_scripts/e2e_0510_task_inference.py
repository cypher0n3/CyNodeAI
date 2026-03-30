# E2E parity: inference-in-sandbox task (auth via setUp login prereq).
# Traces: REQ-WORKER-0114 (node inference path); REQ-WORKER-0270 (UDS boundary);
# REQ-ORCHES-0123 (dispatch to worker).

import time
import unittest

from scripts.test_scripts import config, helpers
import scripts.test_scripts.e2e_state as state


class TestInferenceTask(unittest.TestCase):
    """E2E: task create with inference; poll result for UDS proxy URL in stdout."""

    tags = ["suite_worker_node", "full_demo", "inference", "task"]
    prereqs = ["gateway", "config", "auth", "ollama"]

    def setUp(self):
        ok, detail = helpers.prepare_e2e_cynork_auth()
        self.assertTrue(ok, detail)

    def test_inference_task(self):
        """Create inference task; assert sandbox receives UDS inference proxy URL."""
        if not config.INFERENCE_PROXY_IMAGE:
            self.skipTest("INFERENCE_PROXY_IMAGE not set")
        _, out, _ = helpers.run_cynork(
            [
                "task",
                "create",
                "--command",
                "sh -c 'echo $INFERENCE_PROXY_URL'",
                "--use-inference",
                "-o",
                "json",
            ],
            state.CONFIG_PATH,
        )
        data = helpers.parse_json_safe(out)
        task_id = (data or {}).get("task_id")
        self.assertIsNotNone(task_id, "inference task create failed")
        ok_r, _, err_r = helpers.cynork_task_ready(task_id, state.CONFIG_PATH)
        self.assertTrue(ok_r, f"task ready failed: {err_r}")
        state.INF_TASK_ID = task_id
        # Full E2E runs many tasks first; inference sandbox jobs can sit queued for several minutes.
        # Poll frequently (3s) for up to 15m — coarser 5s×60 polls were still timing out under load.
        deadline = time.time() + 900
        status, data = None, None
        while time.time() < deadline:
            _, out, _ = helpers.run_cynork(
                ["task", "result", task_id, "-o", "json"], state.CONFIG_PATH
            )
            data = helpers.parse_json_safe(out)
            status = (data or {}).get("status")
            if status in ("completed", "failed"):
                break
            time.sleep(3)
        self.assertIsNotNone(
            status,
            "inference task result polling timed out (15m) without terminal status",
        )
        self.assertEqual(
            status, "completed",
            f"inference task did not complete (status={status!r})",
        )
        # cynork -o json flattens job RunJobResponse into top-level stdout
        # (see cynork printTaskResultJSON); raw API shape uses jobs[0].result —
        # get_sba_job_result handles both.
        job_result = helpers.get_sba_job_result(data)
        if isinstance(job_result, str):
            job_result = helpers.parse_json_safe(job_result)
        stdout = (job_result or {}).get("stdout") if isinstance(job_result, dict) else None
        self.assertTrue(
            stdout and "http+unix://" in str(stdout),
            f"INFERENCE_PROXY_URL missing or non-UDS in stdout (stdout={stdout!r})",
        )
