# E2E parity: auth login. Run after gateway is up; creates shared config dir.
# Traces: REQ-IDENTY-0103, 0104; CYNAI.IDENTY.AuthenticationModel.

import unittest

from scripts.test_scripts import config, helpers
import scripts.test_scripts.e2e_state as state


class TestLogin(unittest.TestCase):
    """E2E acceptance: canonical auth login writes token for dependent E2E modules."""

    tags = ["suite_cynork", "full_demo", "auth", "no_inference"]

    def setUp(self):
        """Create shared config dir for login output."""
        state.init_config()

    def test_login(self):
        """Assert canonical login path succeeds and writes token."""
        ok, out, err = helpers.run_cynork(
            ["auth", "login", "-u", "admin", "--password-stdin"],
            state.CONFIG_PATH,
            input_text=f"{config.ADMIN_PASSWORD}\n",
        )
        self.assertTrue(ok, f"auth login failed: {out} {err}")
        with open(state.CONFIG_PATH, encoding="utf-8") as f:
            content = f.read()
        self.assertIn("token:", content, "token not in config after login")
