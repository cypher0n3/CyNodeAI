# E2E parity: task get. Requires e2e_0420 (state.TASK_ID).
# Traces: REQ-ORCHES-0125; cli_management_app_commands_tasks (task get).

import unittest

from scripts.test_scripts import helpers
import scripts.test_scripts.e2e_state as state


class TestTaskGet(unittest.TestCase):
    """E2E: task get by ID; assert JSON response includes the created task."""

    tags = ["suite_cynork", "full_demo", "task", "no_inference"]
    prereqs = ["gateway", "config", "auth", "task_id"]

    def _assert_clear_name_resolution_error(self, out, err):
        """If name lookup is not implemented yet, require a clear failure signal."""
        detail = f"{out}\n{err}".lower()
        self.assertTrue(
            ("not found" in detail)
            or ("invalid task id" in detail)
            or ("must be a uuid" in detail)
            or ("bad request" in detail),
            f"name-based lookup failed without clear error detail: out={out!r} err={err!r}",
        )

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

    def test_task_get_by_name(self):
        """task get by human-readable task name must succeed and return the task."""
        if state.TASK_NAME is None:
            self.skipTest(
                "state.TASK_NAME not set (run full suite or e2e_0420.test_task_create_named first)"
            )
        ok, out, err = helpers.run_cynork(
            ["task", "get", state.TASK_NAME, "-o", "json"],
            state.CONFIG_PATH,
        )
        if not ok:
            self._assert_clear_name_resolution_error(out, err)
        self.assertTrue(ok, f"task get by name failed: {out} {err}")
        data = helpers.parse_json_safe(out)
        self.assertIsInstance(data, dict, f"task get by name should return JSON object: {out}")
        self.assertIn("status", data, f"task get by name missing status: {data}")
