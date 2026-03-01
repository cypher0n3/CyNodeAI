# E2E parity: task create (echo). Requires login; sets state.task_id.

import time
import unittest

from scripts.test_scripts import helpers
import scripts.test_scripts.e2e_state as state


class TestTaskCreate(unittest.TestCase):
    def test_task_create(self):
        for attempt in range(1, 4):
            if attempt > 1:
                time.sleep(5)
            _, out, err = helpers.run_cynork(
                ["task", "create", "-p", "echo Hello from sandbox", "-o", "json"],
                state.CONFIG_PATH,
            )
            data = helpers.parse_json_safe(out)
            task_id = (data or {}).get("task_id") or ""
            if task_id:
                state.TASK_ID = task_id
                return
            if attempt == 3:
                self.fail(f"task create failed after 3 attempts: {out} {err}")
        self.fail("unreachable")
