# E2E parity: auth logout. Cleans up shared config dir.

import unittest

from scripts.test_scripts import helpers
import scripts.test_scripts.e2e_state as state


class TestLogout(unittest.TestCase):
    """E2E: auth logout; tearDown removes shared config dir."""

    tags = ["suite_cynork"]

    def test_logout(self):
        """Run auth logout (exit code not asserted; CLI may return non-zero)."""
        helpers.run_cynork(["auth", "logout"], state.CONFIG_PATH)
        # Logout can return non-zero; we only warn in bash

    def tearDown(self):
        """Remove shared E2E config dir after test."""
        state.cleanup_config()
