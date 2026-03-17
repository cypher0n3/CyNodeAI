# E2E: task cancel. Create a task, cancel it with -y, assert terminal status.
# Traces: REQ-ORCHES-0125; cli_management_app_commands_tasks (task cancel).

import time
import unittest

from scripts.test_scripts import helpers
import scripts.test_scripts.e2e_state as state


class TestTaskCancel(unittest.TestCase):
    """E2E: create long-running command task, cancel with -y; assert canceled status."""

    tags = ["suite_cynork", "full_demo", "task", "no_inference"]
    prereqs = ["gateway", "config", "auth", "task_id"]

    def _assert_clear_name_resolution_error(self, out, err):
        detail = f"{out}\n{err}".lower()
        self.assertTrue(
            ("not found" in detail)
            or ("invalid task id" in detail)
            or ("must be a uuid" in detail)
            or ("bad request" in detail),
            f"name-based cancel failed without clear error detail: out={out!r} err={err!r}",
        )

    def test_task_cancel(self):
        """Create command-mode sleep task, cancel it, poll result until canceled."""
        ok, out, err = helpers.run_cynork(
            ["task", "create", "--command", "sleep 300", "-o", "json"],
            state.CONFIG_PATH,
        )
        self.assertTrue(ok, f"task create for cancel failed: {out} {err}")
        data = helpers.parse_json_safe(out)
        task_id = (data or {}).get("task_id")
        self.assertIsNotNone(task_id, "task create for cancel failed")
        ok, out, err = helpers.run_cynork(
            ["task", "cancel", task_id, "-y", "-o", "json"],
            state.CONFIG_PATH,
        )
        self.assertTrue(ok, f"task cancel failed: {out} {err}")
        cancel_data = helpers.parse_json_safe(out)
        self.assertIsNotNone(cancel_data, f"task cancel response not JSON: {out}")
        self.assertEqual(
            cancel_data.get("task_id"),
            task_id,
            f"task cancel should return requested task_id: {cancel_data}",
        )
        self.assertIs(
            cancel_data.get("canceled"),
            True,
            f"task cancel response should confirm cancellation: {cancel_data}",
        )
        for _ in range(6):
            time.sleep(2)
            _, out, _ = helpers.run_cynork(
                ["task", "result", task_id, "-o", "json"],
                state.CONFIG_PATH,
            )
            result = helpers.parse_json_safe(out)
            status = (result or {}).get("status", "")
            if status in ("canceled", "completed", "failed"):
                break
        self.assertEqual(
            (result or {}).get("task_id"),
            task_id,
            f"task result after cancel should reference the canceled task: {result}",
        )
        self.assertIn(
            status, ("canceled",),
            f"expected canceled status, got {status}",
        )

    def test_task_cancel_by_name(self):
        """Cancel a named task by name; must succeed and confirm cancellation."""
        task_name = "e2e-cancel-by-name"
        ok, out, err = helpers.run_cynork(
            [
                "task",
                "create",
                "--command",
                "sleep 300",
                "--name",
                task_name,
                "-o",
                "json",
            ],
            state.CONFIG_PATH,
        )
        self.assertTrue(ok, f"named task create for cancel failed: {out} {err}")
        ok, out, err = helpers.run_cynork(
            ["task", "cancel", task_name, "-y", "-o", "json"],
            state.CONFIG_PATH,
        )
        self.assertTrue(ok, f"task cancel by name failed: {out} {err}")
        data = helpers.parse_json_safe(out)
        self.assertIsInstance(data, dict, f"task cancel by name should return JSON object: {out}")
        self.assertIs(
            data.get("canceled"),
            True,
            f"task cancel by name should confirm cancel: {data}",
        )
