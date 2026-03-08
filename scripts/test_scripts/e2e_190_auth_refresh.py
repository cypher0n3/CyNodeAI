# E2E parity: auth refresh and whoami.
# Traces: REQ-IDENTY-0104, 0105 (refresh token, rotation); CYNAI.IDENTY.AuthenticationModel.

import unittest

from scripts.test_scripts import helpers
import scripts.test_scripts.e2e_state as state


class TestRefresh(unittest.TestCase):
    """E2E: auth refresh then whoami; expect handle=admin."""

    tags = ["suite_cynork", "full_demo", "auth"]

    def test_refresh(self):
        """Assert auth refresh and whoami succeed; whoami output contains handle=admin."""
        ok, out, _ = helpers.run_cynork(["auth", "refresh"], state.CONFIG_PATH)
        self.assertTrue(ok, "auth refresh failed")
        ok, out, _ = helpers.run_cynork(["auth", "whoami"], state.CONFIG_PATH)
        self.assertTrue(ok, "whoami after refresh failed")
        self.assertIn("handle=admin", out)
