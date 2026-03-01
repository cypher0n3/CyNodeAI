# E2E: task cancel. Create a task, cancel it with -y, assert terminal status.

import time
import unittest

from scripts.test_scripts import helpers
import scripts.test_scripts.e2e_state as state


class TestTaskCancel(unittest.TestCase):
    def test_task_cancel(self):
        _, out, _ = helpers.run_cynork(
            ["task", "create", "-p", "sleep 300", "-o", "json"],
            state.CONFIG_PATH,
        )
        data = helpers.parse_json_safe(out)
        task_id = (data or {}).get("task_id")
        self.assertIsNotNone(task_id, "task create for cancel failed")
        ok, out, err = helpers.run_cynork(
            ["task", "cancel", task_id, "-y", "-o", "json"],
            state.CONFIG_PATH,
        )
        self.assertTrue(ok, f"task cancel failed: {out} {err}")
        for _ in range(6):
            time.sleep(2)
            _, out, _ = helpers.run_cynork(
                ["task", "result", task_id, "-o", "json"],
                state.CONFIG_PATH,
            )
            result = helpers.parse_json_safe(out)
            status = (result or {}).get("status", "")
            if status in ("canceled", "cancelled", "completed", "failed"):
                break
        self.assertIn(
            status, ("canceled", "cancelled"),
            f"expected canceled status, got {status}",
        )
