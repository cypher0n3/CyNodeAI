# E2E: Task list with --status filter; assert status enum and list shape.
# Traces: REQ-ORCHES-0125; cli_management_app_commands_tasks (task list, status enum).
# Requires auth and state.TASK_ID from e2e_050 (to ensure at least one task exists).

import unittest

from scripts.test_scripts import helpers
import scripts.test_scripts.e2e_state as state


class TestTaskListStatusFilter(unittest.TestCase):
    """E2E: task list status filter; assert canonical response shape and filtering."""

    tags = ["suite_cynork", "full_demo", "task"]

    def test_task_list_status_completed(self):
        """Completed filter returns canonical JSON shape and only completed tasks."""
        ok, out, err = helpers.run_cynork(
            ["task", "list", "--status", "completed", "-o", "json"],
            state.CONFIG_PATH,
        )
        self.assertTrue(ok, f"task list --status completed failed: {out} {err}")
        data = helpers.parse_json_safe(out)
        self.assertIsInstance(data, dict, f"task list should return a JSON object: {out}")
        self.assertIn("tasks", data, "task list should have tasks key")
        self.assertIn("next_cursor", data, "task list should include next_cursor key")
        tasks = data.get("tasks") or []
        self.assertIsInstance(tasks, list, "tasks should be a list")
        for t in tasks:
            self.assertIsInstance(t, dict, "each task should be an object")
            self.assertIn("task_id", t, f"task object missing task_id: {t}")
            self.assertIn("status", t, f"task object missing status: {t}")
            self.assertIsInstance(t.get("task_id"), str, f"task_id should be string: {t}")
            self.assertEqual(
                t.get("status"),
                "completed",
                f"completed filter should only return completed tasks: {t}",
            )
            if "task_name" in t:
                self.assertIsInstance(t["task_name"], str, f"task_name should be string: {t}")
        next_cursor = data.get("next_cursor")
        self.assertIsInstance(next_cursor, str, "next_cursor should be a string")

    def test_task_list_status_queued(self):
        """Queued filter returns canonical JSON shape and only queued tasks."""
        ok, out, err = helpers.run_cynork(
            ["task", "list", "--status", "queued", "-o", "json"],
            state.CONFIG_PATH,
        )
        self.assertTrue(ok, f"task list --status queued failed: {out} {err}")
        data = helpers.parse_json_safe(out)
        self.assertIsInstance(data, dict, f"task list should return a JSON object: {out}")
        self.assertIn("tasks", data, "task list should have tasks key")
        self.assertIn("next_cursor", data, "task list should include next_cursor key")
        tasks = data.get("tasks") or []
        self.assertIsInstance(tasks, list, "tasks should be a list")
        for t in tasks:
            self.assertIsInstance(t, dict, "each task should be an object")
            self.assertIn("task_id", t, f"task object missing task_id: {t}")
            self.assertIn("status", t, f"task object missing status: {t}")
            self.assertIsInstance(t.get("task_id"), str, f"task_id should be string: {t}")
            self.assertEqual(
                t.get("status"),
                "queued",
                f"queued filter should only return queued tasks: {t}",
            )
            if "task_name" in t:
                self.assertIsInstance(t["task_name"], str, f"task_name should be string: {t}")
        next_cursor = data.get("next_cursor")
        self.assertIsInstance(next_cursor, str, "next_cursor should be a string")
