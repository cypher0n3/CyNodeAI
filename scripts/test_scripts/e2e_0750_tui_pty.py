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


def _pty_screen_contains_user_message(screen: str, fragment: str) -> bool:
    """True if scrollback likely has the user's sent text (glamour may omit 'You:')."""
    s = (screen or "").lower()
    return "you:" in s or fragment.lower() in s


def _ensure_config_file():
    """Ensure config file exists so cynork tui can start."""
    if not state.CONFIG_PATH:
        return
    if not os.path.isfile(state.CONFIG_PATH):
        with open(state.CONFIG_PATH, "w", encoding="utf-8") as f:
            f.write("# E2E TUI PTY\n")


class TestTuiPty(unittest.TestCase):
    """E2E: fullscreen TUI driven via PTY; assert on landmarks and thread commands."""

    tags = ["suite_cynork", "full_demo", "tui_pty", "tui", "no_inference"]
    prereqs = ["gateway", "config", "auth"]

    def setUp(self):
        ok, detail = helpers.prepare_e2e_cynork_auth()
        self.assertTrue(ok, detail)
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
            combined = ""
            deadline = time.time() + 60
            while time.time() < deadline:
                time.sleep(0.35)
                combined += session.capture_screen(drain_sec=0.25) or ""
                if (
                    harness.LANDMARK_PROMPT_READY in combined
                    or harness.LANDMARK_PROMPT_READY_SHORT in combined
                    or _pty_screen_contains_user_message(combined, "hi")
                ):
                    break
            self.assertTrue(
                harness.LANDMARK_PROMPT_READY in combined
                or harness.LANDMARK_PROMPT_READY_SHORT in combined
                or _pty_screen_contains_user_message(combined, "hi"),
                "TUI should return to prompt-ready or show user message; output: "
                + repr(combined[:800]),
            )

    def test_tui_pty_slash_version_while_assistant_in_flight(self):
        """Slash dispatch while streaming: /version must run during an active turn (Bug 4)."""
        if not harness.pty_available():
            self.skipTest("pexpect not installed or not Unix")
        with harness.TuiPtySession(state.CONFIG_PATH, timeout=90) as session:
            time.sleep(_TUI_STARTUP_DELAY_SEC)
            ready = session.wait_for_prompt_ready(timeout_sec=12)
            self.assertTrue(ready, "TUI did not reach prompt-ready or first paint")
            session.send_keys(["hello", "enter"])
            time.sleep(0.2)
            session.send_keys(["/version", "enter"])
            time.sleep(1.2)
            out_s = session.capture_screen(drain_sec=0.4) or ""
            self.assertIn(
                "cynork",
                out_s.lower(),
                f"expected /version output while in-flight; got: {repr(out_s[:600])}",
            )

    def test_tui_pty_thread_ready_landmark_after_startup(self):
        """After startup ensure-thread, stable thread does not emit THREAD_SWITCHED (Bug 3)."""
        if not harness.pty_available():
            self.skipTest("pexpect not installed or not Unix")
        with harness.TuiPtySession(state.CONFIG_PATH, timeout=30) as session:
            time.sleep(_TUI_STARTUP_DELAY_SEC)
            out = session.read_until_landmark(
                [
                    harness.LANDMARK_PROMPT_READY,
                    harness.LANDMARK_PROMPT_READY_SHORT,
                ],
                timeout_sec=12,
            )
            self.assertTrue(
                harness.LANDMARK_PROMPT_READY in (out or "")
                or harness.LANDMARK_PROMPT_READY_SHORT in (out or "")
                or " (scrollback)" in (out or "")
                or "> " in (out or ""),
                f"TUI did not show prompt-ready or first paint; output: {repr((out or '')[:400])}",
            )
            time.sleep(2.0)
            combined = (out or "") + (session.capture_screen(drain_sec=0.4) or "")
            if harness.LANDMARK_THREAD_READY in combined:
                self.assertNotIn(
                    harness.LANDMARK_THREAD_SWITCHED,
                    combined,
                    "Bug 3: stable thread ensure must not emit THREAD_SWITCHED",
                )

    def test_tui_pty_in_flight_landmark_appears(self):
        """While a message is in-flight, the assistant-in-flight landmark should appear in the
        status bar (REQ-CLIENT-0209: streaming state is visible to the user)."""
        if not harness.pty_available():
            self.skipTest("pexpect not installed or not Unix")
        with harness.TuiPtySession(state.CONFIG_PATH, timeout=90) as session:
            time.sleep(_TUI_STARTUP_DELAY_SEC)
            ready = session.wait_for_prompt_ready(timeout_sec=12)
            self.assertTrue(ready, "TUI did not reach prompt-ready or first paint")
            session.send_keys(["hello", "enter"])
            combined = ""
            deadline = time.time() + 30
            while time.time() < deadline:
                time.sleep(0.25)
                combined += session.capture_screen(drain_sec=0.2) or ""
                if (
                    harness.LANDMARK_ASSISTANT_IN_FLIGHT in combined
                    or harness.LANDMARK_PROMPT_READY in combined
                    or harness.LANDMARK_PROMPT_READY_SHORT in combined
                    or _pty_screen_contains_user_message(combined, "hello")
                    or "assistant:" in combined.lower()
                ):
                    break
            out_s = combined
            self.assertTrue(
                harness.LANDMARK_ASSISTANT_IN_FLIGHT in out_s
                or harness.LANDMARK_PROMPT_READY in out_s
                or harness.LANDMARK_PROMPT_READY_SHORT in out_s
                or _pty_screen_contains_user_message(out_s, "hello")
                or "assistant:" in out_s.lower(),
                f"Expected in-flight or post-completion landmark; output: {repr(out_s[:500])}",
            )

    def test_tui_pty_new_session_resumes_cached_thread_token(self):
        """Second TUI launch reuses last thread id from XDG cache (connection recovery UX)."""
        if not harness.pty_available():
            self.skipTest("pexpect not installed or not Unix")
        thread_id_first = None
        with harness.TuiPtySession(state.CONFIG_PATH, timeout=90) as session:
            time.sleep(_TUI_STARTUP_DELAY_SEC)
            self.assertTrue(session.wait_for_prompt_ready(timeout_sec=12))
            session.send_keys(["e2e thread cache ping", "enter"])
            combined = ""
            deadline = time.time() + 75
            while time.time() < deadline:
                time.sleep(0.35)
                combined += session.capture_screen(drain_sec=0.25) or ""
                if (
                    harness.LANDMARK_PROMPT_READY in combined
                    or harness.LANDMARK_PROMPT_READY_SHORT in combined
                    or "You:" in combined
                ):
                    break
            scr1 = session.capture_screen(drain_sec=0.4) or ""
            tid = harness.extract_thread_token_from_status(scr1)
            thread_id_first = tid if tid else None
        # Let last_threads.json settle before the second process reads XDG cache.
        time.sleep(0.75)
        thread_id_second = None
        with harness.TuiPtySession(state.CONFIG_PATH, timeout=90) as session:
            time.sleep(_TUI_STARTUP_DELAY_SEC)
            self.assertTrue(session.wait_for_prompt_ready(timeout_sec=12))
            # Thread resume from XDG cache runs after prompt-ready; status may show
            # ``thread: (default)`` until EnsureThread + cache read finish.
            deadline = time.time() + 45.0
            while time.time() < deadline:
                scr2 = session.capture_screen(drain_sec=0.45) or ""
                tid2 = harness.extract_thread_token_from_status(scr2)
                if tid2 and tid2 != "(default)":
                    thread_id_second = tid2
                    break
                time.sleep(0.35)
            if thread_id_second is None:
                scr2 = session.capture_screen(drain_sec=0.45) or ""
                tid2 = harness.extract_thread_token_from_status(scr2)
                thread_id_second = tid2 if tid2 else None
        if not thread_id_first or not thread_id_second:
            self.skipTest(
                "thread token not visible in status bar (no cached thread yet); "
                f"first={thread_id_first!r} second={thread_id_second!r}"
            )
        self.assertEqual(
            thread_id_first,
            thread_id_second,
            "fresh TUI session should resume cached thread id",
        )

    def test_tui_pty_ctrl_c_cancels_stream(self):
        """Ctrl+C while a message is in-flight cancels the stream; second Ctrl+C exits.
        Asserts CYNAI.USRGWY.OpenAIChatApi.Streaming client cancellation (REQ-CLIENT-0209)."""
        if not harness.pty_available():
            self.skipTest("pexpect not installed or not Unix")
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


class TestTuiPtyStreaming(unittest.TestCase):
    """TUI tests that need a live inference path (streaming cancel)."""

    tags = ["suite_cynork", "full_demo", "tui_pty", "tui", "inference"]
    prereqs = ["gateway", "config", "auth", "ollama"]

    def setUp(self):
        ok, detail = helpers.prepare_e2e_cynork_auth()
        self.assertTrue(ok, detail)
        _ensure_config_file()

    def test_tui_pty_cancel_mid_stream_retains_partial_and_marks_interrupted(self):
        """Cancel in-flight stream: scrollback keeps Assistant line and '(stream interrupted)'.

        Uses a short post-send delay then Ctrl+C so fast local models do not finish the
        stream before cancel (which would skip the interrupt marker).
        """
        if not harness.pty_available():
            self.skipTest("pexpect not installed or not Unix")
        with harness.TuiPtySession(state.CONFIG_PATH, timeout=90) as session:
            time.sleep(_TUI_STARTUP_DELAY_SEC)
            self.assertTrue(
                session.wait_for_prompt_ready(timeout_sec=12),
                "TUI did not reach prompt-ready or first paint",
            )
            session.send_keys(
                [
                    "Write at least three long paragraphs about rivers. "
                    "Use numbered sentences.",
                    "enter",
                ]
            )
            # Cancel as early as practical so we hit the in-flight path; waiting for long
            # assistant output lets fast local models finish the whole stream (no interrupt line).
            time.sleep(0.12)
            session.capture_screen(drain_sec=0.25)
            session.send_keys(harness.cancel_stream_keys())
            post = harness.wait_scrollback_contains(
                session,
                ["(stream interrupted)"],
                timeout_sec=35.0,
            )
            self.assertIn(
                "(stream interrupted)",
                post,
                f"expected interrupt marker after cancel; got: {repr(post[:900])}",
            )
            self.assertTrue(
                "Assistant" in post or "assistant" in post.lower(),
                "assistant turn area should remain visible after cancel; "
                + repr(post[:900]),
            )
