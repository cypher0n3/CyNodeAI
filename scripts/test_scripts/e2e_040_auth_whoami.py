# E2E parity: auth whoami. Requires login (e2e_01) first.

import unittest

from scripts.test_scripts import helpers
import scripts.test_scripts.e2e_state as state


class TestWhoami(unittest.TestCase):
    """E2E: auth whoami after login; expects handle=admin."""

    tags = ["suite_cynork", "full_demo", "auth"]

    def test_whoami(self):
        """Assert whoami succeeds and output contains handle=admin."""
        ok, out, err = helpers.run_cynork(["auth", "whoami"], state.CONFIG_PATH)
        self.assertTrue(ok, f"auth whoami failed: {out} {err}")
        self.assertIn("handle=admin", out, "expected handle=admin in " + out)
