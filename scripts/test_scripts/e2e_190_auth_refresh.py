# E2E parity: auth refresh and whoami.
# Traces: REQ-IDENTY-0104, 0105 (refresh token, rotation); CYNAI.IDENTY.AuthenticationModel.

import unittest

from scripts.test_scripts import helpers
import scripts.test_scripts.e2e_state as state


class TestRefresh(unittest.TestCase):
    """E2E: auth refresh then whoami; expect handle=admin."""

    tags = ["suite_cynork", "full_demo", "auth"]

    def test_refresh(self):
        """Assert auth refresh preserves usable session state and whoami still works."""
        before_token = helpers.read_config_value(state.CONFIG_PATH, "token")
        before_refresh = helpers.read_config_value(state.CONFIG_PATH, "refresh_token")
        self.assertTrue(before_token, "precondition failed: token missing before refresh")
        self.assertTrue(before_refresh, "precondition failed: refresh_token missing before refresh")

        ok, out, err = helpers.run_cynork(["auth", "refresh"], state.CONFIG_PATH)
        self.assertTrue(ok, f"auth refresh failed: {out} {err}")

        after_token = helpers.read_config_value(state.CONFIG_PATH, "token")
        after_refresh = helpers.read_config_value(state.CONFIG_PATH, "refresh_token")
        self.assertTrue(after_token, "token missing after refresh")
        self.assertTrue(after_refresh, "refresh_token missing after refresh")

        ok, out, _ = helpers.run_cynork(["auth", "whoami"], state.CONFIG_PATH)
        self.assertTrue(ok, "whoami after refresh failed")
        self.assertIn("handle=admin", out)
