# E2E parity: auth login. Run after gateway is up; creates shared config dir.

import unittest

from scripts.test_scripts import config, helpers
import scripts.test_scripts.e2e_state as state


class TestLogin(unittest.TestCase):
    def setUp(self):
        state.init_config()

    def test_login(self):
        ok, out, err = helpers.run_cynork(
            ["auth", "login", "-u", "admin", "-p", config.ADMIN_PASSWORD],
            state.config_path,
        )
        self.assertTrue(ok, "auth login failed: %s %s" % (out, err))
        with open(state.config_path, encoding="utf-8") as f:
            content = f.read()
        self.assertIn("token:", content, "token not in config after login")
