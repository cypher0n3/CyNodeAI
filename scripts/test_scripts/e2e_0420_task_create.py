# E2E parity: task create. Requires login; sets state.task_id.
# Traces: REQ-ORCHES-0122, 0126; CYNAI.CLIENT.CliCommandSurface (task subset).

import re
import os
import time
import unittest

from scripts.test_scripts import helpers
from scripts.test_scripts import config
import scripts.test_scripts.e2e_state as state


class TestTaskCreate(unittest.TestCase):
    """E2E: prompt-mode task create and named/attachment variants; sets state.TASK_ID."""

    tags = ["suite_cynork", "full_demo", "task", "no_inference"]

    def test_task_create(self):
        """Create prompt task, retry up to 3 times; store task_id in state."""
        for attempt in range(1, 4):
            if attempt > 1:
                time.sleep(5)
            _, out, err = helpers.run_cynork(
                [
                    "task",
                    "create",
                    "-p",
                    "Please reply with exactly: hello from task create.",
                    "-o",
                    "json",
                ],
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
        """Create task with canonical --name, then assert task_name in get response."""
        for attempt in range(1, 4):
            if attempt > 1:
                time.sleep(5)
            ok, out, err = helpers.run_cynork(
                [
                    "task",
                    "create",
                    "-p",
                    "Please acknowledge this named task request.",
                    "--name",
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
                f"task name should be e2e-task-name or e2e-task-name-N: got {name!r}; "
                f"full response: {get_data}",
            )
            state.TASK_NAME = name
            return
        self.fail("unreachable")

    def test_task_create_with_attachments(self):
        """Create task with canonical --attach paths; assert attachments in create/get."""
        if state.CONFIG_PATH is None:
            self.skipTest("CONFIG_PATH not set (run full suite or auth first)")
        tmp_dir = os.path.join(config.PROJECT_ROOT, "tmp")
        os.makedirs(tmp_dir, exist_ok=True)
        with open(os.path.join(tmp_dir, "att1.txt"), "w", encoding="utf-8") as f:
            f.write("att1")
        with open(os.path.join(tmp_dir, "att2.txt"), "w", encoding="utf-8") as f:
            f.write("att2")
        for attempt in range(1, 4):
            if attempt > 1:
                time.sleep(5)
            ok, out, err = helpers.run_cynork(
                [
                    "task",
                    "create",
                    "-p",
                    "Read the attached files and acknowledge receipt.",
                    "--attach",
                    "tmp/att1.txt",
                    "--attach",
                    "tmp/att2.txt",
                    "-o",
                    "json",
                ],
                state.CONFIG_PATH,
            )
            if not ok:
                if attempt == 3:
                    self.fail(f"task create with attachments failed: {out} {err}")
                continue
            data = helpers.parse_json_safe(out)
            task_id = (data or {}).get("task_id") or ""
            if not task_id:
                if attempt == 3:
                    self.fail(f"no task_id in create response: {out}")
                continue
            create_atts = (data or {}).get("attachments") or []
            self.assertIsInstance(create_atts, list, "create attachments should be a list")
            self.assertIn("tmp/att1.txt", create_atts, f"create missing att1: {data}")
            self.assertIn("tmp/att2.txt", create_atts, f"create missing att2: {data}")
            ok2, out2, _ = helpers.run_cynork(
                ["task", "get", task_id, "-o", "json"], state.CONFIG_PATH
            )
            self.assertTrue(ok2, f"task get failed: {out2}")
            get_data = helpers.parse_json_safe(out2)
            self.assertIsNotNone(get_data, f"task get not JSON: {out2}")
            get_atts = get_data.get("attachments") or []
            self.assertIsInstance(get_atts, list, "get attachments should be a list")
            self.assertIn("tmp/att1.txt", get_atts, f"attachments: {get_data}")
            self.assertIn("tmp/att2.txt", get_atts, f"attachments: {get_data}")
            return
        self.fail("unreachable")
