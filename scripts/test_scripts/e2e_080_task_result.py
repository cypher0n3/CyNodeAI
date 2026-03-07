# E2E parity: task result. Requires e2e_050 (state.TASK_ID).
# Traces: REQ-ORCHES-0124, 0125; cli_management_app_commands_tasks.

import unittest

from scripts.test_scripts import helpers
import scripts.test_scripts.e2e_state as state


class TestTaskResult(unittest.TestCase):
    """E2E: task result by ID; assert JSON includes status."""

    tags = ["suite_cynork", "full_demo", "task"]

    def test_task_result(self):
        """Assert task result state.TASK_ID returns JSON with status."""
        self.assertIsNotNone(state.TASK_ID)
        _, out, err = helpers.run_cynork(
            ["task", "result", state.TASK_ID, "-o", "json"],
            state.CONFIG_PATH,
        )
        data = helpers.parse_json_safe(out)
        self.assertIsNotNone(data, f"task result failed: {out} {err}")
        self.assertIn("status", data)
        self.assertIn(
            data.get("status"),
            ("queued", "running", "completed", "failed", "canceled", "superseded"),
            f"task result status must be valid enum per CLI spec: {data.get('status')!r}",
        )
