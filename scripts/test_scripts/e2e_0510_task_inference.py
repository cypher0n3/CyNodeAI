# E2E parity: inference-in-sandbox task. Requires auth config from e2e_0030.
# Traces: REQ-WORKER-0114 (node inference path); REQ-WORKER-0270 (UDS boundary);
# REQ-ORCHES-0123 (dispatch to worker).

import os
import time
import unittest

from scripts.test_scripts import config, helpers
import scripts.test_scripts.e2e_state as state


class TestInferenceTask(unittest.TestCase):
    """E2E: task create with inference; poll result for UDS proxy URL in stdout."""

    tags = ["suite_worker_node", "full_demo", "inference", "task"]

    def test_inference_task(self):
        """Create inference task; assert sandbox receives UDS inference proxy URL."""
        if not config.INFERENCE_PROXY_IMAGE:
            self.skipTest("INFERENCE_PROXY_IMAGE not set")
        if not state.CONFIG_PATH or not os.path.isfile(state.CONFIG_PATH):
            self.skipTest("CONFIG_PATH not set (run after auth login prereq)")
        token = helpers.read_token_from_config(state.CONFIG_PATH)
        if not token:
            self.skipTest("auth token missing from config (run after auth login prereq)")
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
        state.INF_TASK_ID = task_id
        status = None
        data = None
        for _ in range(18):
            time.sleep(5)
            _, out, _ = helpers.run_cynork(
                ["task", "result", task_id, "-o", "json"], state.CONFIG_PATH
            )
            data = helpers.parse_json_safe(out)
            status = (data or {}).get("status")
            if status in ("completed", "failed"):
                break
        self.assertEqual(
            status, "completed",
            f"inference task did not complete (status={status!r})",
        )
        raw = helpers.jq_get(data, "jobs", 0, "result")
        job_result = helpers.parse_json_safe(raw) if isinstance(raw, str) else raw
        stdout = (job_result or {}).get("stdout") if isinstance(job_result, dict) else None
        self.assertTrue(
            stdout and "http+unix://" in str(stdout),
            f"INFERENCE_PROXY_URL missing or non-UDS in stdout (stdout={stdout!r})",
        )
