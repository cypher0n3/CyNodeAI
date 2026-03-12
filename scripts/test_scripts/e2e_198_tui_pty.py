# E2E: cynork TUI via PTY harness (prompt-ready, thread list, exit).
# Requires: pip install -r scripts/requirements-e2e.txt (pexpect).
# Traces: REQ-CLIENT-0161, REQ-CLIENT-0181; Phase 5 Python PTY harness, cynork_tui.md landmarks.

import os
import unittest

from scripts.test_scripts import helpers
from scripts.test_scripts import tui_pty_harness as harness
import scripts.test_scripts.e2e_state as state


def _ensure_config_file():
    """Ensure config file exists so cynork tui can start."""
    if not state.CONFIG_PATH:
        return
    if not os.path.isfile(state.CONFIG_PATH):
        with open(state.CONFIG_PATH, "w", encoding="utf-8") as f:
            f.write("# E2E TUI PTY\n")


class TestTuiPty(unittest.TestCase):
    """E2E: fullscreen TUI driven via PTY; assert on landmarks and thread commands."""

    tags = ["suite_cynork", "full_demo", "tui_pty"]

    def setUp(self):
        state.init_config()
        _ensure_config_file()

    def test_tui_pty_prompt_ready(self):
        """TUI shows prompt-ready landmark within timeout; then exit."""
        if not harness.pty_available():
            self.skipTest("pexpect not installed or not Unix; install scripts/requirements-e2e.txt")
        with harness.TuiPtySession(state.CONFIG_PATH, timeout=15) as session:
            ready = session.wait_for_prompt_ready(timeout_sec=10)
            self.assertTrue(ready, "TUI did not show prompt-ready landmark")
            screen = session.capture_screen()
            self.assertIn(
                harness.LANDMARK_PROMPT_READY,
                screen,
                "capture should contain prompt-ready landmark",
            )

    def test_tui_pty_exit_via_ctrl_c(self):
        """TUI accepts ctrl+c and exits cleanly."""
        if not harness.pty_available():
            self.skipTest("pexpect not installed or not Unix")
        with harness.TuiPtySession(state.CONFIG_PATH, timeout=10) as session:
            session.wait_for_prompt_ready(timeout_sec=8)
            session.send_keys(["ctrl+c"])

    def test_tui_pty_thread_list_shows_landmark_or_output(self):
        """After /thread list, scrollback shows thread list header or error."""
        if not harness.pty_available():
            self.skipTest("pexpect not installed or not Unix")
        if not state.CONFIG_PATH or not helpers.read_token_from_config(state.CONFIG_PATH):
            self.skipTest("auth required for thread list (run after login)")
        with harness.TuiPtySession(state.CONFIG_PATH, timeout=15) as session:
            session.wait_for_prompt_ready(timeout_sec=8)
            session.send_keys(["/thread list", "enter"])
            out = session.read_until_landmark(
                harness.LANDMARK_PROMPT_READY,
                timeout_sec=10,
            )
            self.assertIn(
                "Threads",
                out,
                "thread list should show header or error containing 'Threads'",
            )
