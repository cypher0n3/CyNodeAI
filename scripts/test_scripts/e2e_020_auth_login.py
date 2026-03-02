# E2E parity: auth login. Run after gateway is up; creates shared config dir.

import unittest

from scripts.test_scripts import config, helpers
import scripts.test_scripts.e2e_state as state


class TestLogin(unittest.TestCase):
    """E2E: auth login; persists token into shared config for later tests."""

    def setUp(self):
        """Create shared config dir for login output."""
        state.init_config()

    def test_login(self):
        """Assert auth login succeeds and config file contains token."""
        ok, out, err = helpers.run_cynork(
            ["auth", "login", "-u", "admin", "-p", config.ADMIN_PASSWORD],
            state.CONFIG_PATH,
        )
        self.assertTrue(ok, f"auth login failed: {out} {err}")
        with open(state.CONFIG_PATH, encoding="utf-8") as f:
            content = f.read()
        self.assertIn("token:", content, "token not in config after login")
