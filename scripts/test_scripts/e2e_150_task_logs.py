# E2E: task logs by ID. Requires state.TASK_ID from e2e_050.
# Traces: REQ-ORCHES-0124; cli_management_app_commands_tasks (task logs).

import unittest

from scripts.test_scripts import helpers
import scripts.test_scripts.e2e_state as state


class TestTaskLogs(unittest.TestCase):
    """E2E: task logs by ID; assert the response identifies the task and log streams."""

    tags = ["suite_cynork", "full_demo", "task"]

    def test_task_logs(self):
        """Assert task logs returns JSON with task_id, stdout, and stderr fields."""
        self.assertIsNotNone(
            state.TASK_ID,
            "state.TASK_ID must be set by e2e_050 (task create); run tests in order",
        )
        ok, out, err = helpers.run_cynork(
            ["task", "logs", state.TASK_ID, "-o", "json"],
            state.CONFIG_PATH,
        )
        self.assertTrue(ok, f"task logs failed: {out} {err}")
        data = helpers.parse_json_safe(out)
        self.assertIsInstance(data, dict, f"task logs should return a JSON object: {out}")
        self.assertEqual(
            data.get("task_id"),
            state.TASK_ID,
            f"task logs should identify the requested task: {data}",
        )
        self.assertIn("stdout", data, f"task logs missing stdout: {data}")
        self.assertIn("stderr", data, f"task logs missing stderr: {data}")
        self.assertIsInstance(data.get("stdout"), str, f"stdout should be a string: {data}")
        self.assertIsInstance(data.get("stderr"), str, f"stderr should be a string: {data}")
