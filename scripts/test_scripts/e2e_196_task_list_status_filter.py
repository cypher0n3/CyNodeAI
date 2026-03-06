# E2E: Task list with --status filter; assert status enum and list shape.
# Traces: REQ-ORCHES-0125; cli_management_app_commands_tasks (task list, status enum).
# Requires auth and state.TASK_ID from e2e_050 (to ensure at least one task exists).

import unittest

from scripts.test_scripts import helpers
import scripts.test_scripts.e2e_state as state


class TestTaskListStatusFilter(unittest.TestCase):
    """E2E: task list --status completed (and optionally other statuses); assert JSON shape."""

    tags = ["suite_cynork"]

    def test_task_list_status_completed(self):
        """Task list with --status completed returns JSON array; may be empty."""
        _, out, _ = helpers.run_cynork(
            ["task", "list", "--status", "completed", "-o", "json"],
            state.CONFIG_PATH,
        )
        data = helpers.parse_json_safe(out)
        self.assertIsNotNone(data, "task list response should be JSON")
        self.assertIn("tasks", data, "task list should have tasks key")
        tasks = data.get("tasks") or []
        self.assertIsInstance(tasks, list, "tasks should be a list")
        for t in tasks:
            self.assertIsInstance(t, dict, "each task should be an object")
            if "status" in t:
                self.assertIn(
                    t["status"],
                    ("queued", "running", "completed", "failed", "canceled", "superseded"),
                    f"task status should be valid enum: {t.get('status')}",
                )

    def test_task_list_status_queued(self):
        """Task list with --status queued returns JSON array."""
        _, out, _ = helpers.run_cynork(
            ["task", "list", "--status", "queued", "-o", "json"],
            state.CONFIG_PATH,
        )
        data = helpers.parse_json_safe(out)
        self.assertIsNotNone(data, "task list response should be JSON")
        tasks = data.get("data") or data.get("tasks") or []
        self.assertIsInstance(tasks, list, "tasks should be a list")
