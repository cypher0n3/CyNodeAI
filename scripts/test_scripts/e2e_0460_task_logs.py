# E2E: task logs by ID. Requires task_id prereq (state.TASK_ID). Atomic: works with --single.
# Traces: REQ-ORCHES-0124; cli_management_app_commands_tasks (task logs).

import unittest

from scripts.test_scripts import helpers
import scripts.test_scripts.e2e_state as state


class TestTaskLogs(unittest.TestCase):
    """E2E: task logs by ID; assert the response identifies the task and log streams."""

    tags = ["suite_cynork", "full_demo", "task", "no_inference"]
    prereqs = ["gateway", "config", "auth", "task_id"]

    def _assert_clear_name_resolution_error(self, out, err):
        detail = f"{out}\n{err}".lower()
        self.assertTrue(
            ("not found" in detail)
            or ("invalid task id" in detail)
            or ("must be a uuid" in detail)
            or ("bad request" in detail),
            f"name-based logs failed without clear error detail: out={out!r} err={err!r}",
        )

    def test_task_logs(self):
        """Assert task logs returns JSON with task_id, stdout, and stderr fields."""
        self.assertIsNotNone(
            state.TASK_ID,
            "state.TASK_ID not set (task_id prereq failed or not declared)",
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

    def test_task_logs_by_name(self):
        """task logs by human-readable task name must succeed and return log fields."""
        self.assertIsNotNone(
            state.TASK_NAME,
            "state.TASK_NAME not set (run test_task_create_named or suite)",
        )
        ok, out, err = helpers.run_cynork(
            ["task", "logs", state.TASK_NAME, "-o", "json"],
            state.CONFIG_PATH,
        )
        self.assertTrue(ok, f"task logs by name failed: {out} {err}")
        data = helpers.parse_json_safe(out)
        self.assertIsInstance(data, dict, f"task logs by name should return JSON object: {out}")
        self.assertIn("stdout", data, f"task logs by name missing stdout: {data}")
        self.assertIn("stderr", data, f"task logs by name missing stderr: {data}")
