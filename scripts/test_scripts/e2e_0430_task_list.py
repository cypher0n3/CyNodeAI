# E2E: task list. Requires task_id prereq (sets state.TASK_ID). Atomic: works with --single.
# Traces: REQ-ORCHES-0125; cli_management_app_commands_tasks (task list).

import unittest

from scripts.test_scripts import helpers
import scripts.test_scripts.e2e_state as state


class TestTaskList(unittest.TestCase):
    """E2E: task list -o json; assert created task appears in list."""

    tags = ["suite_cynork", "full_demo", "task", "no_inference"]
    prereqs = ["gateway", "config", "auth", "task_id"]

    def test_task_list(self):
        """Assert task list returns JSON with tasks list containing state.TASK_ID."""
        self.assertIsNotNone(
            state.TASK_ID,
            "state.TASK_ID not set (task_id prereq failed or not declared)",
        )
        ok, out, err = helpers.run_cynork(
            ["task", "list", "-o", "json", "-l", "10"],
            state.CONFIG_PATH,
        )
        self.assertTrue(ok, f"task list failed: {out} {err}")
        data = helpers.parse_json_safe(out)
        self.assertIsNotNone(data, f"task list response not JSON: {out}")
        tasks = data.get("tasks") if isinstance(data, dict) else None
        self.assertIsInstance(tasks, list, "tasks should be a list")
        self.assertGreaterEqual(
            len(tasks), 1,
            "at least one task (task_id prereq creates one)",
        )
        ids = [t.get("task_id") or t.get("id") for t in tasks if isinstance(t, dict)]
        self.assertIn(
            state.TASK_ID,
            ids,
            f"created task {state.TASK_ID!r} should be in list (got ids: {ids})",
        )
