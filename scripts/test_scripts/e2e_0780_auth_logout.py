# E2E: auth logout. Runs after auth/login-dependent tests.
# Traces: REQ-IDENTY-0106; REQ-CLIENT-0150; cli_management_app_commands_core (auth logout).

import os
import unittest

from scripts.test_scripts import helpers
import scripts.test_scripts.e2e_state as state


class TestLogout(unittest.TestCase):
    """E2E: auth logout clears local auth state and blocks later authenticated use."""

    tags = ["suite_cynork", "full_demo", "auth", "no_inference"]
    prereqs = ["gateway", "config", "auth"]

    def test_logout(self):
        """Assert auth logout succeeds, clears stored tokens, and breaks whoami."""
        self.assertTrue(state.CONFIG_PATH, "CONFIG_PATH must be set by earlier auth tests")
        self.assertTrue(os.path.isfile(state.CONFIG_PATH), "config file must exist before logout")
        self.assertTrue(
            helpers.read_config_value(state.CONFIG_PATH, "token"),
            "precondition failed: token missing before logout",
        )

        ok, out, err = helpers.run_cynork(["auth", "logout"], state.CONFIG_PATH)
        self.assertTrue(ok, f"auth logout failed: {out} {err}")
        out_lower = (out or "").lower()
        self.assertTrue(
            "logged out" in out_lower or "logged_out" in out_lower,
            f"unexpected logout output: {out!r}",
        )

        self.assertIsNone(
            helpers.read_config_value(state.CONFIG_PATH, "token"),
            "token should be cleared from config after logout",
        )
        self.assertIsNone(
            helpers.read_config_value(state.CONFIG_PATH, "refresh_token"),
            "refresh_token should be cleared from config after logout",
        )

        ok, out, err = helpers.run_cynork(["auth", "whoami"], state.CONFIG_PATH)
        self.assertFalse(ok, f"whoami should fail after logout: {out} {err}")
        self.assertIn(
            "not logged in",
            ((out or "") + " " + (err or "")).lower(),
            f"whoami after logout should report missing auth: {out!r} {err!r}",
        )

    def tearDown(self):
        """Remove shared E2E config dir after logout completes."""
        state.cleanup_config()
