# E2E parity: task get. Requires e2e_050 (state.TASK_ID).
# Traces: REQ-ORCHES-0125; cli_management_app_commands_tasks (task get).

import unittest

from scripts.test_scripts import helpers
import scripts.test_scripts.e2e_state as state


class TestTaskGet(unittest.TestCase):
    """E2E: task get by ID; assert JSON response includes status."""

    tags = ["suite_cynork", "full_demo", "task"]

    def test_task_get(self):
        """Assert task get state.TASK_ID returns JSON with status field."""
        self.assertIsNotNone(state.TASK_ID)
        _, out, err = helpers.run_cynork(
            ["task", "get", state.TASK_ID, "-o", "json"],
            state.CONFIG_PATH,
        )
        data = helpers.parse_json_safe(out)
        self.assertIsNotNone(data, f"task get failed: {out} {err}")
        self.assertIn("status", data, f"no status in {data}")
