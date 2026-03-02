# E2E: task logs. Requires state.TASK_ID (e2e_03).

import unittest

from scripts.test_scripts import helpers
import scripts.test_scripts.e2e_state as state


class TestTaskLogs(unittest.TestCase):
    """E2E: task logs by ID; assert JSON response is dict or list."""

    def test_task_logs(self):
        """Assert task logs state.TASK_ID returns success and JSON (dict or list)."""
        self.assertIsNotNone(state.TASK_ID)
        ok, out, err = helpers.run_cynork(
            ["task", "logs", state.TASK_ID, "-o", "json"],
            state.CONFIG_PATH,
        )
        self.assertTrue(ok, f"task logs failed: {out} {err}")
        data = helpers.parse_json_safe(out)
        self.assertIsNotNone(data)
        self.assertIsInstance(data, (dict, list), "logs should be dict or list")
