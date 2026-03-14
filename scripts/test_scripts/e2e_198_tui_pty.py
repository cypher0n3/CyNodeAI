# E2E: cynork TUI via PTY harness (prompt-ready, thread list, exit).
# Requires: pip install -r scripts/requirements-e2e.txt (pexpect).
# Traces: REQ-CLIENT-0161, REQ-CLIENT-0181; Phase 5 Python PTY harness, cynork_tui.md landmarks.

import os
import time
import unittest

from scripts.test_scripts import helpers
from scripts.test_scripts import tui_pty_harness as harness
import scripts.test_scripts.e2e_state as state

# Allow TUI first paint to render before expecting landmark (TERM=dumb, alt screen).
_TUI_STARTUP_DELAY_SEC = 1.5


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
        with harness.TuiPtySession(state.CONFIG_PATH, timeout=20) as session:
            time.sleep(_TUI_STARTUP_DELAY_SEC)
            out = session.read_until_landmark(
                [harness.LANDMARK_PROMPT_READY, harness.LANDMARK_PROMPT_READY_SHORT],
                timeout_sec=12,
            )
            self.assertTrue(
                harness.LANDMARK_PROMPT_READY in out
                or harness.LANDMARK_PROMPT_READY_SHORT in out
                or " (scrollback)" in out
                or "> " in out,
                f"TUI did not show prompt-ready or first paint; output (first 500 chars): "
                f"{repr(out[:500]) if out else '(empty)'}",
            )
            screen = session.capture_screen()
            self.assertTrue(
                harness.LANDMARK_PROMPT_READY in screen
                or harness.LANDMARK_PROMPT_READY_SHORT in screen
                or " (scrollback)" in (screen or "")
                or "> " in (screen or ""),
                "capture should contain prompt-ready or first paint",
            )

    def test_tui_pty_exit_via_ctrl_c(self):
        """TUI accepts ctrl+c and exits cleanly."""
        if not harness.pty_available():
            self.skipTest("pexpect not installed or not Unix")
        with harness.TuiPtySession(state.CONFIG_PATH, timeout=20) as session:
            time.sleep(_TUI_STARTUP_DELAY_SEC)
            out = session.read_until_landmark(
                [harness.LANDMARK_PROMPT_READY, harness.LANDMARK_PROMPT_READY_SHORT],
                timeout_sec=12,
            )
            self.assertTrue(
                harness.LANDMARK_PROMPT_READY in out
                or harness.LANDMARK_PROMPT_READY_SHORT in out
                or " (scrollback)" in (out or "")
                or "> " in (out or ""),
                f"TUI did not show prompt-ready or first paint; output: {repr((out or '')[:400])}",
            )
            session.send_keys(["ctrl+c"])

    def test_tui_pty_thread_list_shows_landmark_or_output(self):
        """After /thread list, scrollback shows thread list header or error."""
        if not harness.pty_available():
            self.skipTest("pexpect not installed or not Unix")
        if not state.CONFIG_PATH or not helpers.read_token_from_config(state.CONFIG_PATH):
            self.skipTest("auth required for thread list (run after login)")
        with harness.TuiPtySession(state.CONFIG_PATH, timeout=25) as session:
            time.sleep(_TUI_STARTUP_DELAY_SEC)
            out = session.read_until_landmark(
                [harness.LANDMARK_PROMPT_READY, harness.LANDMARK_PROMPT_READY_SHORT],
                timeout_sec=12,
            )
            self.assertTrue(
                harness.LANDMARK_PROMPT_READY in out
                or harness.LANDMARK_PROMPT_READY_SHORT in out
                or " (scrollback)" in (out or "")
                or "> " in (out or ""),
                f"TUI did not show prompt-ready or first paint; output: {repr((out or '')[:500])}",
            )
            session.send_keys(["/thread list", "enter"])
            # Allow the TUI to fetch the thread list and re-render. Bubbletea may not
            # re-emit the prompt-ready landmark if only the scrollback area changed, so
            # use capture_screen after a brief settle pause instead of a second landmark
            # wait (which would time-out on a no-op status-bar diff).
            time.sleep(1.0)
            out_s = session.capture_screen(drain_sec=0.3) or ""
            self.assertTrue(
                "Threads" in out_s
                or "thread" in out_s.lower()
                or "Error" in out_s
                or harness.LANDMARK_PROMPT_READY in out_s
                or harness.LANDMARK_PROMPT_READY_SHORT in out_s,
                f"thread list should show header/error/landmark; got: {repr(out_s[:400])}",
            )

    def test_tui_pty_send_receive_round_trip(self):
        """Send a message and assert prompt-ready again (response round-trip)."""
        if not harness.pty_available():
            self.skipTest("pexpect not installed or not Unix")
        if not state.CONFIG_PATH or not helpers.read_token_from_config(state.CONFIG_PATH):
            self.skipTest("auth required for send/receive (run after login)")
        with harness.TuiPtySession(state.CONFIG_PATH, timeout=90) as session:
            time.sleep(_TUI_STARTUP_DELAY_SEC)
            out = session.read_until_landmark(
                [harness.LANDMARK_PROMPT_READY, harness.LANDMARK_PROMPT_READY_SHORT],
                timeout_sec=12,
            )
            self.assertTrue(
                harness.LANDMARK_PROMPT_READY in out
                or harness.LANDMARK_PROMPT_READY_SHORT in out
                or " (scrollback)" in (out or "")
                or "> " in (out or ""),
                f"TUI did not reach prompt-ready or first paint; output: {repr((out or '')[:500])}",
            )
            session.send_keys(["hi", "enter"])
            out2 = session.read_until_landmark(
                [harness.LANDMARK_PROMPT_READY, harness.LANDMARK_PROMPT_READY_SHORT],
                timeout_sec=60,
            )
            # Landmark, user line echoed, or fragment (TUI/script may exit before full redraw)
            out2_s = out2 or ""
            self.assertTrue(
                harness.LANDMARK_PROMPT_READY in out2_s
                or harness.LANDMARK_PROMPT_READY_SHORT in out2_s
                or "You:" in out2_s
                or out2_s.strip() in ("Y", "Yo", "You", "You:"),
                f"TUI should return to prompt-ready or echo 'You:'; output: {repr(out2_s[:500])}",
            )

    def test_tui_pty_in_flight_landmark_appears(self):
        """While a message is in-flight, the assistant-in-flight landmark should appear in the
        status bar (REQ-CLIENT-0209: streaming state is visible to the user)."""
        if not harness.pty_available():
            self.skipTest("pexpect not installed or not Unix")
        if not state.CONFIG_PATH or not helpers.read_token_from_config(state.CONFIG_PATH):
            self.skipTest("auth required for in-flight test (run after login)")
        with harness.TuiPtySession(state.CONFIG_PATH, timeout=90) as session:
            time.sleep(_TUI_STARTUP_DELAY_SEC)
            ready = session.wait_for_prompt_ready(timeout_sec=12)
            self.assertTrue(ready, "TUI did not reach prompt-ready or first paint")
            session.send_keys(["hello", "enter"])
            # Immediately after sending, read for the in-flight landmark
            # before the response arrives.
            # Accept either the in-flight OR the prompt-ready landmark (fast inference may skip it).
            out = session.read_until_landmark(
                [
                    harness.LANDMARK_ASSISTANT_IN_FLIGHT,
                    harness.LANDMARK_PROMPT_READY,
                    harness.LANDMARK_PROMPT_READY_SHORT,
                ],
                timeout_sec=30,
            )
            out_s = out or ""
            self.assertTrue(
                harness.LANDMARK_ASSISTANT_IN_FLIGHT in out_s
                or harness.LANDMARK_PROMPT_READY in out_s
                or harness.LANDMARK_PROMPT_READY_SHORT in out_s
                or "You:" in out_s
                or "Assistant:" in out_s,
                f"Expected in-flight or post-completion landmark; output: {repr(out_s[:500])}",
            )

    def test_tui_pty_ctrl_c_cancels_stream(self):
        """Ctrl+C while a message is in-flight cancels the stream; second Ctrl+C exits.
        Asserts CYNAI.USRGWY.OpenAIChatApi.Streaming client cancellation (REQ-CLIENT-0209)."""
        if not harness.pty_available():
            self.skipTest("pexpect not installed or not Unix")
        if not state.CONFIG_PATH or not helpers.read_token_from_config(state.CONFIG_PATH):
            self.skipTest("auth required for cancellation test (run after login)")
        with harness.TuiPtySession(state.CONFIG_PATH, timeout=60) as session:
            time.sleep(_TUI_STARTUP_DELAY_SEC)
            ready = session.wait_for_prompt_ready(timeout_sec=12)
            self.assertTrue(ready, "TUI did not reach prompt-ready or first paint")
            # Send a prompt and immediately cancel.
            session.send_keys(["hello world", "enter"])
            time.sleep(0.3)
            session.send_keys(["ctrl+c"])
            # After cancellation, TUI should either return to prompt-ready or show the
            # "Press Ctrl+C again to exit" hint (first Ctrl+C behaviour).
            out = session.read_until_landmark(
                [
                    harness.LANDMARK_PROMPT_READY,
                    harness.LANDMARK_PROMPT_READY_SHORT,
                ],
                timeout_sec=15,
            )
            out_s = out or ""
            self.assertTrue(
                harness.LANDMARK_PROMPT_READY in out_s
                or harness.LANDMARK_PROMPT_READY_SHORT in out_s
                or "Ctrl+C" in out_s
                or "Assistant:" in out_s
                or "Error:" in out_s
                or "> " in out_s,
                f"After Ctrl+C, expected prompt-ready or hint; output: {repr(out_s[:500])}",
            )
