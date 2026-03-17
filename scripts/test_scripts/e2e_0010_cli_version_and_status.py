# E2E: cynork version (no auth) and status (gateway health). Runs before login.
# Traces: CYNAI.STANDS.CliCynork, REQ-ORCHES-0120 (healthz).

import unittest

from scripts.test_scripts import helpers
import scripts.test_scripts.e2e_state as state


class TestStatusVersion(unittest.TestCase):
    """E2E: cynork version and status (gateway health), no auth required."""

    tags = ["suite_cynork", "full_demo", "no_inference"]

    def setUp(self):
        """Create shared config dir for cynork invocations."""
        state.init_config()

    def test_version(self):
        """Assert cynork version exits successfully and output contains 'cynork'."""
        ok, out, _ = helpers.run_cynork(["version"], state.CONFIG_PATH)
        self.assertTrue(ok, "cynork version failed")
        self.assertIn("cynork", (out or "").lower(), "version output missing cynork")

    def test_status(self):
        """Assert cynork status reports gateway ok."""
        ok, out, err = helpers.run_cynork(["status"], state.CONFIG_PATH)
        self.assertTrue(ok, f"cynork status failed: {out} {err}")
        self.assertIn("ok", (out or "").lower(), "status should report ok")

    def test_status_json(self):
        """Assert cynork status -o json returns gateway: ok."""
        ok, out, _ = helpers.run_cynork(
            ["status", "-o", "json"], state.CONFIG_PATH
        )
        self.assertTrue(ok, "cynork status -o json failed")
        data = helpers.parse_json_safe(out)
        self.assertIsNotNone(data)
        self.assertIn("gateway", data)
        self.assertEqual(data.get("gateway"), "ok")
