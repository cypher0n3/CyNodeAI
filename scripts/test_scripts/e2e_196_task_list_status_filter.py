# E2E: Task list with --status filter; assert status enum and list shape.
# Traces: REQ-ORCHES-0125; cli_management_app_commands_tasks (task list, status enum).
# Requires auth and state.TASK_ID from e2e_050 (to ensure at least one task exists).

import unittest

from scripts.test_scripts import helpers
import scripts.test_scripts.e2e_state as state


class TestTaskListStatusFilter(unittest.TestCase):
    """E2E: task list --status completed (and optionally other statuses); assert JSON shape."""

    tags = ["suite_cynork", "full_demo", "task"]

    def test_task_list_status_completed(self):
        """Task list with --status completed returns JSON array; may be empty."""
        ok, out, err = helpers.run_cynork(
            ["task", "list", "--status", "completed", "-o", "json"],
            state.CONFIG_PATH,
        )
        self.assertTrue(ok, f"task list --status completed failed: {out} {err}")
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
        next_cursor = data.get("next_cursor")
        if next_cursor is not None:
            self.assertIsInstance(next_cursor, str, "next_cursor should be a string when present")

    def test_task_list_status_queued(self):
        """Task list with --status queued returns JSON array."""
        ok, out, err = helpers.run_cynork(
            ["task", "list", "--status", "queued", "-o", "json"],
            state.CONFIG_PATH,
        )
        self.assertTrue(ok, f"task list --status queued failed: {out} {err}")
        data = helpers.parse_json_safe(out)
        self.assertIsNotNone(data, "task list response should be JSON")
        self.assertIn("tasks", data, "task list should have tasks key")
        tasks = data.get("tasks") or []
        self.assertIsInstance(tasks, list, "tasks should be a list")
        for t in tasks:
            self.assertIsInstance(t, dict, "each task should be an object")
            self.assertEqual(
                t.get("status"),
                "queued",
                f"queued filter should only return queued tasks: {t}",
            )
