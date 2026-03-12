# E2E parity: auth whoami. Requires login (e2e_020) first.
# Traces: REQ-IDENTY-0103, 0104; CYNAI.IDENTY.AuthenticationModel (whoami identity).

import unittest

from scripts.test_scripts import helpers
import scripts.test_scripts.e2e_state as state


class TestWhoami(unittest.TestCase):
    """E2E: auth whoami after login; expects user=admin."""

    tags = ["suite_cynork", "full_demo", "auth"]

    def test_whoami(self):
        """Assert whoami succeeds and output contains handle=admin."""
        ok, out, err = helpers.run_cynork(["auth", "whoami"], state.CONFIG_PATH)
        self.assertTrue(ok, f"auth whoami failed: {out} {err}")
        self.assertIn("user=admin", out, "expected user=admin in " + out)
