# E2E parity: auth whoami. Requires login (e2e_01) first.

import unittest

from scripts.test_scripts import helpers
import scripts.test_scripts.e2e_state as state


class TestWhoami(unittest.TestCase):
    def test_whoami(self):
        ok, out, err = helpers.run_cynork(["auth", "whoami"], state.config_path)
        self.assertTrue(ok, "auth whoami failed: %s %s" % (out, err))
        self.assertIn("handle=admin", out, "expected handle=admin in " + out)
