# E2E parity: auth refresh and whoami.

import unittest

from scripts.test_scripts import helpers
import scripts.test_scripts.e2e_state as state


class TestRefresh(unittest.TestCase):
    def test_refresh(self):
        ok, out, _ = helpers.run_cynork(["auth", "refresh"], state.CONFIG_PATH)
        self.assertTrue(ok, "auth refresh failed")
        ok, out, _ = helpers.run_cynork(["auth", "whoami"], state.CONFIG_PATH)
        self.assertTrue(ok, "whoami after refresh failed")
        self.assertIn("handle=admin", out)
