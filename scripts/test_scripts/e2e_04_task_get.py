# E2E parity: task get. Requires e2e_03 (state.task_id).

import unittest

from scripts.test_scripts import helpers
import scripts.test_scripts.e2e_state as state


class TestTaskGet(unittest.TestCase):
    def test_task_get(self):
        self.assertIsNotNone(state.task_id)
        _, out, err = helpers.run_cynork(
            ["task", "get", state.task_id, "-o", "json"],
            state.config_path,
        )
        data = helpers.parse_json_safe(out)
        self.assertIsNotNone(data, f"task get failed: {out} {err}")
        self.assertIn("status", data, f"no status in {data}")
