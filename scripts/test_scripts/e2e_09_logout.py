# E2E parity: auth logout. Cleans up shared config dir.

import unittest

from scripts.test_scripts import helpers
import scripts.test_scripts.e2e_state as state


class TestLogout(unittest.TestCase):
    def test_logout(self):
        helpers.run_cynork(["auth", "logout"], state.config_path)
        # Logout can return non-zero; we only warn in bash

    def tearDown(self):
        state.cleanup_config()
