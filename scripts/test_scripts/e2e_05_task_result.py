# E2E parity: task result. Requires e2e_03 (state.task_id).

import unittest

from scripts.test_scripts import helpers
import scripts.test_scripts.e2e_state as state


class TestTaskResult(unittest.TestCase):
    def test_task_result(self):
        self.assertIsNotNone(state.task_id)
        _, out, err = helpers.run_cynork(
            ["task", "result", state.task_id, "-o", "json"],
            state.config_path,
        )
        data = helpers.parse_json_safe(out)
        self.assertIsNotNone(data, f"task result failed: {out} {err}")
        self.assertIn("status", data)
