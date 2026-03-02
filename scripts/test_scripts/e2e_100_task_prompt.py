# E2E parity: prompt task (LLM). Sets state.prompt_task_id.

import time
import unittest

from scripts.test_scripts import helpers
import scripts.test_scripts.e2e_state as state


class TestPromptTask(unittest.TestCase):
    """E2E: LLM prompt task create; poll result; assert completed with non-empty stdout."""

    def test_prompt_task(self):
        """Create prompt task, poll until completed; set state.PROMPT_TASK_ID; assert stdout."""
        task_id = None
        for attempt in range(1, 4):
            if attempt > 1:
                time.sleep(5)
            _, out, _ = helpers.run_cynork(
                ["task", "create", "-p",
                 "What model are you? Reply in one short sentence.", "-o", "json"],
                state.CONFIG_PATH,
            )
            data = helpers.parse_json_safe(out)
            task_id = (data or {}).get("task_id")
            if task_id:
                break
        self.assertIsNotNone(task_id, "prompt task create failed")
        state.PROMPT_TASK_ID = task_id
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
        self.assertEqual(status, "completed", "prompt task did not complete")
        inner = helpers.jq_get(result_data, "stdout")
        if isinstance(inner, str):
            inner = helpers.parse_json_safe(inner)
        stdout = (inner or {}).get("stdout") if isinstance(inner, dict) else str(inner)
        self.assertTrue(
            stdout and str(stdout).strip() and str(stdout) != "(no response)",
            "prompt stdout empty or invalid",
        )
