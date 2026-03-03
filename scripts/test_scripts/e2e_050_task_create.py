# E2E parity: task create (echo). Requires login; sets state.task_id.

import re
import time
import unittest

from scripts.test_scripts import helpers
import scripts.test_scripts.e2e_state as state


class TestTaskCreate(unittest.TestCase):
    """E2E: task create (echo) and create with task name; sets state.TASK_ID."""

    def test_task_create(self):
        """Create echo task, retry up to 3 times; store task_id in state."""
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

    def test_task_create_named(self):
        """Create task with --task-name, then task get; assert task_name in response."""
        for attempt in range(1, 4):
            if attempt > 1:
                time.sleep(5)
            ok, out, err = helpers.run_cynork(
                [
                    "task",
                    "create",
                    "-p",
                    "echo named",
                    "--task-name",
                    "e2e-task-name",
                    "-o",
                    "json",
                ],
                state.CONFIG_PATH,
            )
            if not ok:
                if attempt == 3:
                    self.fail(f"task create with task name failed: {out} {err}")
                continue
            data = helpers.parse_json_safe(out)
            task_id = (data or {}).get("task_id") or ""
            if not task_id:
                if attempt == 3:
                    self.fail(f"no task_id in create response: {out}")
                continue
            ok2, out2, _ = helpers.run_cynork(
                ["task", "get", task_id, "-o", "json"], state.CONFIG_PATH
            )
            self.assertTrue(ok2, f"task get failed: {out2}")
            get_data = helpers.parse_json_safe(out2)
            self.assertIsNotNone(get_data, f"task get response not JSON: {out2}")
            # API may return task_name and/or summary (both from normalized name).
            # Backend ensures uniqueness per user by appending -2, -3, ... when needed.
            name = get_data.get("task_name") or get_data.get("summary")
            self.assertIsNotNone(
                name,
                f"task_name or summary missing in get response: {get_data}",
            )
            self.assertTrue(
                re.match(r"^e2e-task-name(-\d+)?$", name),
                f"task name should be e2e-task-name or e2e-task-name-N: got {name!r}; full response: {get_data}",
            )
            return
        self.fail("unreachable")
