# E2E: cynork version (no auth) and status (gateway health). Runs before login.

import unittest

from scripts.test_scripts import helpers
import scripts.test_scripts.e2e_state as state


class TestStatusVersion(unittest.TestCase):
    def setUp(self):
        state.init_config()

    def test_version(self):
        ok, out, _ = helpers.run_cynork(["version"], state.CONFIG_PATH)
        self.assertTrue(ok, "cynork version failed")
        self.assertIn("cynork", (out or "").lower(), "version output missing cynork")

    def test_status(self):
        ok, out, err = helpers.run_cynork(["status"], state.CONFIG_PATH)
        self.assertTrue(ok, f"cynork status failed: {out} {err}")
        self.assertIn("ok", (out or "").lower(), "status should report ok")

    def test_status_json(self):
        ok, out, _ = helpers.run_cynork(
            ["status", "-o", "json"], state.CONFIG_PATH
        )
        self.assertTrue(ok, "cynork status -o json failed")
        data = helpers.parse_json_safe(out)
        self.assertIsNotNone(data)
        self.assertIn("gateway", data)
        self.assertEqual(data.get("gateway"), "ok")
