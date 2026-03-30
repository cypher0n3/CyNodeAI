# E2E: TUI queue model (Enter queues while streaming; slash still dispatches).
# Traces: REQ-CLIENT-0196, cynork_tui.md Queued Drafts; Bug 4 plan _plan_002_bugs.md Task 2.
# Tags align with plan: suite_cynork, full_demo, tui_pty, no_inference.

import time
import unittest

from scripts.test_scripts import helpers
from scripts.test_scripts import tui_pty_harness as harness
import scripts.test_scripts.e2e_state as state

_TUI_STARTUP_DELAY_SEC = 1.5


def _pty_screen_contains_user_message(screen: str, fragment: str) -> bool:
    s = (screen or "").lower()
    return "you:" in s or fragment.lower() in s


class TestTUIQueueModel(unittest.TestCase):
    """E2E smoke: queue + slash during stream (no_inference stack)."""

    tags = ["suite_cynork", "full_demo", "tui_pty", "tui", "no_inference"]
    prereqs = ["gateway", "config", "auth"]

    def setUp(self):
        ok, detail = helpers.prepare_e2e_cynork_auth()
        self.assertTrue(ok, detail)

    def test_tui_enter_queues_during_stream_slash_still_works(self):
        """While a turn is in flight, plain Enter queues; /version still runs (Bug 4)."""
        if not harness.pty_available():
            self.skipTest("pexpect not installed or not Unix")
        with harness.TuiPtySession(state.CONFIG_PATH, timeout=90) as session:
            time.sleep(_TUI_STARTUP_DELAY_SEC)
            self.assertTrue(session.wait_for_prompt_ready(timeout_sec=12), "prompt-ready")
            session.send_keys(["hello", "enter"])
            time.sleep(0.25)
            session.send_keys(["queued-line-bug4", "enter"])
            time.sleep(0.2)
            session.send_keys(["/version", "enter"])
            time.sleep(1.2)
            out_s = session.capture_screen(drain_sec=0.5) or ""
            self.assertIn(
                "cynork",
                out_s.lower(),
                f"expected /version while in-flight; got: {repr(out_s[:700])}",
            )
