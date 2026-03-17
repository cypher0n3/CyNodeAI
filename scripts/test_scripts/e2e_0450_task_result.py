# E2E parity: task result. Requires e2e_0420 (state.TASK_ID).
# Traces: REQ-ORCHES-0124, 0125; cli_management_app_commands_tasks.

import unittest

from scripts.test_scripts import helpers
import scripts.test_scripts.e2e_state as state


class TestTaskResult(unittest.TestCase):
    """E2E: task result by ID; assert canonical task-result JSON contract."""

    tags = ["suite_cynork", "full_demo", "task", "no_inference"]

    def _assert_clear_name_resolution_error(self, out, err):
        detail = f"{out}\n{err}".lower()
        self.assertTrue(
            ("not found" in detail)
            or ("invalid task id" in detail)
            or ("must be a uuid" in detail)
            or ("bad request" in detail),
            f"name-based result failed without clear error detail: out={out!r} err={err!r}",
        )

    def test_task_result(self):
        """Assert task result includes required keys and terminal stdout/stderr fields."""
        self.assertIsNotNone(state.TASK_ID)
        ok, out, err = helpers.run_cynork(
            ["task", "result", state.TASK_ID, "-o", "json"],
            state.CONFIG_PATH,
        )
        self.assertTrue(ok, f"task result failed: {out} {err}")
        data = helpers.parse_json_safe(out)
        self.assertIsInstance(data, dict, f"task result should return a JSON object: {out}")
        self.assertEqual(
            data.get("task_id"),
            state.TASK_ID,
            f"task result should return requested task_id: {data}",
        )
        self.assertIn("status", data, f"task result should include status: {data}")
        self.assertIn(
            data.get("status"),
            ("queued", "running", "completed", "failed", "canceled", "superseded"),
            f"task result status must be valid enum per CLI spec: {data.get('status')!r}",
        )
        if data.get("status") in ("completed", "failed", "canceled", "superseded"):
            self.assertIn("stdout", data, f"terminal task result missing stdout: {data}")
            self.assertIn("stderr", data, f"terminal task result missing stderr: {data}")
            self.assertIsInstance(
                data.get("stdout"), str, f"terminal stdout should be a string: {data}"
            )
            self.assertIsInstance(
                data.get("stderr"), str, f"terminal stderr should be a string: {data}"
            )

    def test_task_result_by_name(self):
        """task result by human-readable task name must succeed and return the task."""
        if state.TASK_NAME is None:
            self.skipTest(
                "state.TASK_NAME not set (run full suite or e2e_0420.test_task_create_named first)"
            )
        ok, out, err = helpers.run_cynork(
            ["task", "result", state.TASK_NAME, "-o", "json"],
            state.CONFIG_PATH,
        )
        if not ok:
            self._assert_clear_name_resolution_error(out, err)
        self.assertTrue(ok, f"task result by name failed: {out} {err}")
        data = helpers.parse_json_safe(out)
        self.assertIsInstance(data, dict, f"task result by name should return JSON object: {out}")
        self.assertIn("status", data, f"task result by name missing status: {data}")
