# E2E parity: inference-in-sandbox task. Skip if INFERENCE_PROXY_IMAGE unset.

import time
import unittest

from scripts.test_scripts import config, helpers
import scripts.test_scripts.e2e_state as state


class TestInferenceTask(unittest.TestCase):
    """E2E: task create with --use-inference; poll result for OLLAMA_BASE_URL in stdout."""

    tags = ["suite_worker_node"]

    def test_inference_task(self):
        """Create inference task, poll until completed; assert stdout contains Ollama URL."""
        if not config.INFERENCE_PROXY_IMAGE:
            self.skipTest("INFERENCE_PROXY_IMAGE not set")
        _, out, _ = helpers.run_cynork(
            ["task", "create", "-p", "sh -c 'echo $OLLAMA_BASE_URL'",
             "--use-inference", "--input-mode", "commands", "-o", "json"],
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
        self.assertEqual(status, "completed", "inference task did not complete")
        inner = helpers.jq_get(data, "stdout")
        if isinstance(inner, str):
            inner = helpers.parse_json_safe(inner)
        stdout = (inner or {}).get("stdout") if isinstance(inner, dict) else None
        self.assertTrue(
            stdout and "http://localhost:11434" in str(stdout),
            "OLLAMA_BASE_URL missing in stdout",
        )
