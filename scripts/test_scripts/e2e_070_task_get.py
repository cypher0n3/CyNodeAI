# E2E parity: task get. Requires e2e_050 (state.TASK_ID).
# Traces: REQ-ORCHES-0125; cli_management_app_commands_tasks (task get).

import unittest

from scripts.test_scripts import helpers
import scripts.test_scripts.e2e_state as state


class TestTaskGet(unittest.TestCase):
    """E2E: task get by ID; assert JSON response includes the created task."""

    tags = ["suite_cynork", "full_demo", "task"]

    def test_task_get(self):
        """Assert task get returns the created task ID and a valid status."""
        self.assertIsNotNone(state.TASK_ID)
        ok, out, err = helpers.run_cynork(
            ["task", "get", state.TASK_ID, "-o", "json"],
            state.CONFIG_PATH,
        )
        self.assertTrue(ok, f"task get failed: {out} {err}")
        data = helpers.parse_json_safe(out)
        self.assertIsNotNone(data, f"task get failed: {out} {err}")
        self.assertEqual(
            data.get("task_id") or data.get("id"),
            state.TASK_ID,
            f"task get should return requested task_id: {data}",
        )
        self.assertIn("status", data, f"no status in {data}")
        self.assertIn(
            data.get("status"),
            ("queued", "running", "completed", "failed", "canceled", "superseded"),
            f"task get status must be valid enum per CLI spec: {data.get('status')!r}",
        )
