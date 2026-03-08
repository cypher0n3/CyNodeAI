# E2E parity: auth logout. Cleans up shared config dir.
# Traces: REQ-IDENTY-0106 (revocation); REQ-CLIENT-0150 (session credentials cleared).

import unittest

from scripts.test_scripts import helpers
import scripts.test_scripts.e2e_state as state


class TestLogout(unittest.TestCase):
    """E2E: auth logout; tearDown removes shared config dir."""

    tags = ["suite_cynork", "full_demo", "auth"]

    def test_logout(self):
        """Run auth logout (exit code not asserted; CLI may return non-zero)."""
        helpers.run_cynork(["auth", "logout"], state.CONFIG_PATH)
        # Logout can return non-zero; we only warn in bash

    def tearDown(self):
        """Remove shared E2E config dir after test."""
        state.cleanup_config()
