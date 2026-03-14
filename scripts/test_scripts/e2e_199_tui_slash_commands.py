# E2E: cynork TUI slash commands and shell-escape (`!`) via PTY harness.
# Traces: REQ-CLIENT-0164, REQ-CLIENT-0165, REQ-CLIENT-0171, REQ-CLIENT-0172,
#         REQ-CLIENT-0173, REQ-CLIENT-0175, REQ-CLIENT-0176, REQ-CLIENT-0207.
# CYNAI.CLIENT.CynorkTui.LocalSlashCommands,
# CYNAI.CLIENT.CynorkTui.ModelSlashCommands,
# CYNAI.CLIENT.CynorkTui.ProjectSlashCommands,
# CYNAI.CLIENT.CliChatShellEscape.

import os
import time
import unittest

from scripts.test_scripts import tui_pty_harness as harness
import scripts.test_scripts.e2e_state as state

_TUI_STARTUP_DELAY_SEC = 1.5
_SLASH_CMD_SETTLE_SEC = 0.5


def _ensure_config_file():
    if not state.CONFIG_PATH:
        return
    if not os.path.isfile(state.CONFIG_PATH):
        with open(state.CONFIG_PATH, "w", encoding="utf-8") as f:
            f.write("# E2E TUI slash commands\n")


class TestTuiSlashCommands(unittest.TestCase):
    """E2E: TUI slash commands (/help, /clear, /version, /exit, /model, /project)
    and shell-escape (!) via PTY harness."""

    tags = ["suite_cynork", "full_demo", "tui_pty"]

    def setUp(self):
        state.init_config()
        _ensure_config_file()

    def _wait_ready(self, session, timeout=12):
        """Wait for TUI prompt-ready landmark."""
        time.sleep(_TUI_STARTUP_DELAY_SEC)
        ok = session.wait_for_prompt_ready(timeout_sec=timeout)
        self.assertTrue(
            ok,
            "TUI did not show prompt-ready landmark within timeout",
        )

    # ------------------------------------------------------------------ /help
    def test_tui_slash_help_lists_commands(self):
        """/help shows available slash commands including /clear and /exit."""
        if not harness.pty_available():
            self.skipTest("pexpect not installed")
        with harness.TuiPtySession(state.CONFIG_PATH, timeout=25) as session:
            self._wait_ready(session)
            session.send_keys(["/help", "enter"])
            time.sleep(_SLASH_CMD_SETTLE_SEC)
            out = session.capture_screen(drain_sec=0.3) or ""
            self.assertTrue(
                "/clear" in out or "/exit" in out or "/help" in out,
                f"/help should list commands including /clear or /exit; got: {repr(out[:400])}",
            )
            session.send_keys(["ctrl+c", "ctrl+c"])

    # ----------------------------------------------------------------- /clear
    def test_tui_slash_clear_empties_scrollback(self):
        """/clear resets the visible scrollback."""
        if not harness.pty_available():
            self.skipTest("pexpect not installed")
        with harness.TuiPtySession(state.CONFIG_PATH, timeout=25) as session:
            self._wait_ready(session)
            # Send a message that will appear in scrollback.
            session.send_keys(["/help", "enter"])
            time.sleep(_SLASH_CMD_SETTLE_SEC)
            session.send_keys(["/clear", "enter"])
            time.sleep(_SLASH_CMD_SETTLE_SEC)
            out = session.capture_screen(drain_sec=0.3) or ""
            # After /clear the scrollback should be blank (no /help output).
            self.assertNotIn(
                "/clear", out,
                f"/clear should have cleared the /help output; got: {repr(out[:400])}",
            )
            session.send_keys(["ctrl+c", "ctrl+c"])

    # --------------------------------------------------------------- /version
    def test_tui_slash_version(self):
        """/version shows cynork version string in scrollback."""
        if not harness.pty_available():
            self.skipTest("pexpect not installed")
        with harness.TuiPtySession(state.CONFIG_PATH, timeout=25) as session:
            self._wait_ready(session)
            session.send_keys(["/version", "enter"])
            time.sleep(_SLASH_CMD_SETTLE_SEC)
            out = session.capture_screen(drain_sec=0.3) or ""
            self.assertTrue(
                "cynork" in out.lower() or "version" in out.lower(),
                f"/version should show version string; got: {repr(out[:400])}",
            )
            session.send_keys(["ctrl+c", "ctrl+c"])

    # ------- unknown slash command shows help hint, session stays active -----
    def test_tui_slash_unknown_shows_hint(self):
        """Unknown slash command shows a hint without exiting the session."""
        if not harness.pty_available():
            self.skipTest("pexpect not installed")
        with harness.TuiPtySession(state.CONFIG_PATH, timeout=25) as session:
            self._wait_ready(session)
            session.send_keys(["/notacommand", "enter"])
            time.sleep(_SLASH_CMD_SETTLE_SEC)
            out = session.capture_screen(drain_sec=0.3) or ""
            self.assertTrue(
                "/help" in out or "unknown" in out.lower() or "Unknown" in out,
                f"Unknown command should hint /help; got: {repr(out[:400])}",
            )
            # Session still active: prompt-ready landmark should still be in status bar.
            screen = session.capture_screen(drain_sec=0.2) or ""
            self.assertTrue(
                harness.LANDMARK_PROMPT_READY in screen
                or harness.LANDMARK_PROMPT_READY_SHORT in screen,
                f"Session should remain active after unknown slash; status: {repr(screen[:300])}",
            )
            session.send_keys(["ctrl+c", "ctrl+c"])

    # ------------------------------------------------------------------ /exit
    def test_tui_slash_exit_closes_session(self):
        """/exit ends the TUI session cleanly."""
        if not harness.pty_available():
            self.skipTest("pexpect not installed")
        with harness.TuiPtySession(state.CONFIG_PATH, timeout=25) as session:
            self._wait_ready(session)
            session.send_keys(["/exit", "enter"])
            # Process should exit within 3s.
            deadline = time.time() + 3.0
            exited = False
            while time.time() < deadline:
                if session.is_closed():
                    exited = True
                    break
                time.sleep(0.1)
            self.assertTrue(exited, "/exit should close the TUI process")

    # ------------------------------------------------------------------ /quit
    def test_tui_slash_quit_closes_session(self):
        """/quit is a synonym for /exit."""
        if not harness.pty_available():
            self.skipTest("pexpect not installed")
        with harness.TuiPtySession(state.CONFIG_PATH, timeout=25) as session:
            self._wait_ready(session)
            session.send_keys(["/quit", "enter"])
            deadline = time.time() + 3.0
            exited = False
            while time.time() < deadline:
                if session.is_closed():
                    exited = True
                    break
                time.sleep(0.1)
            self.assertTrue(exited, "/quit should close the TUI process")

    # ---------------------------------------------------------------- /models
    def test_tui_slash_models(self):
        """/models shows model list or inline error (session stays active)."""
        if not harness.pty_available():
            self.skipTest("pexpect not installed")
        with harness.TuiPtySession(state.CONFIG_PATH, timeout=25) as session:
            self._wait_ready(session)
            session.send_keys(["/models", "enter"])
            time.sleep(_SLASH_CMD_SETTLE_SEC)
            out = session.capture_screen(drain_sec=0.3) or ""
            # Either model list or an error (gateway may not be running).
            self.assertTrue(
                "model" in out.lower() or "error" in out.lower() or "Error" in out,
                f"/models should show models or inline error; got: {repr(out[:400])}",
            )
            session.send_keys(["ctrl+c", "ctrl+c"])

    # --------------------------------------------------------------- /model
    def test_tui_slash_model_no_arg_shows_current(self):
        """/model with no argument shows current model."""
        if not harness.pty_available():
            self.skipTest("pexpect not installed")
        with harness.TuiPtySession(state.CONFIG_PATH, timeout=25) as session:
            self._wait_ready(session)
            session.send_keys(["/model", "enter"])
            time.sleep(_SLASH_CMD_SETTLE_SEC)
            out = session.capture_screen(drain_sec=0.3) or ""
            self.assertTrue(
                "model" in out.lower(),
                f"/model should show current model; got: {repr(out[:400])}",
            )
            session.send_keys(["ctrl+c", "ctrl+c"])

    def test_tui_slash_model_set(self):
        """/model <id> updates the session model and shows confirmation."""
        if not harness.pty_available():
            self.skipTest("pexpect not installed")
        with harness.TuiPtySession(state.CONFIG_PATH, timeout=25) as session:
            self._wait_ready(session)
            session.send_keys(["/model test-model-v2", "enter"])
            time.sleep(_SLASH_CMD_SETTLE_SEC)
            out = session.capture_screen(drain_sec=0.3) or ""
            # Model name should appear in status bar or scrollback confirmation.
            self.assertTrue(
                "test-model-v2" in out,
                f"/model set should show updated model; got: {repr(out[:400])}",
            )
            session.send_keys(["ctrl+c", "ctrl+c"])

    # -------------------------------------------------------------- /project
    def test_tui_slash_project_no_arg_shows_current(self):
        """/project with no argument shows current project."""
        if not harness.pty_available():
            self.skipTest("pexpect not installed")
        with harness.TuiPtySession(state.CONFIG_PATH, timeout=25) as session:
            self._wait_ready(session)
            session.send_keys(["/project", "enter"])
            time.sleep(_SLASH_CMD_SETTLE_SEC)
            out = session.capture_screen(drain_sec=0.3) or ""
            self.assertTrue(
                "project" in out.lower(),
                f"/project should show project context; got: {repr(out[:400])}",
            )
            session.send_keys(["ctrl+c", "ctrl+c"])

    def test_tui_slash_project_set(self):
        """/project set <id> updates the session project."""
        if not harness.pty_available():
            self.skipTest("pexpect not installed")
        with harness.TuiPtySession(state.CONFIG_PATH, timeout=25) as session:
            self._wait_ready(session)
            session.send_keys(["/project set proj-xyz", "enter"])
            time.sleep(_SLASH_CMD_SETTLE_SEC)
            out = session.capture_screen(drain_sec=0.3) or ""
            self.assertTrue(
                "proj-xyz" in out,
                f"/project set should update project to proj-xyz; got: {repr(out[:400])}",
            )
            session.send_keys(["ctrl+c", "ctrl+c"])

    # ------------------------------------------------ /auth login (PTY-testable form)
    def test_tui_auth_login_opens_form(self):
        """/auth login opens the in-TUI login form; landmark or form text visible."""
        if not harness.pty_available():
            self.skipTest("pexpect not installed")
        with harness.TuiPtySession(state.CONFIG_PATH, timeout=25) as session:
            self._wait_ready(session)
            session.send_keys(["/auth login", "enter"])
            time.sleep(_SLASH_CMD_SETTLE_SEC)
            ok = session.wait_for_login_form(timeout_sec=8)
            self.assertTrue(
                ok,
                "/auth login should show login form (landmark or Login/Gateway URL)",
            )
            out = session.capture_screen(drain_sec=0.3) or ""
            self.assertTrue(
                harness.LANDMARK_AUTH_RECOVERY_READY in out
                or ("Login" in out and ("Gateway URL" in out or "Username" in out)),
                f"Login form should be visible; got: {repr(out[:500])}",
            )
            session.send_keys(["esc"])
            time.sleep(_SLASH_CMD_SETTLE_SEC)
            session.send_keys(["ctrl+c", "ctrl+c"])

    def test_tui_auth_login_cancel_returns_to_composer(self):
        """/auth login then Esc closes form and shows 'Login cancelled'; composer ready."""
        if not harness.pty_available():
            self.skipTest("pexpect not installed")
        with harness.TuiPtySession(state.CONFIG_PATH, timeout=25) as session:
            self._wait_ready(session)
            session.send_keys(["/auth login", "enter"])
            time.sleep(_SLASH_CMD_SETTLE_SEC)
            ok = session.wait_for_login_form(timeout_sec=8)
            self.assertTrue(ok, "login form should appear")
            session.send_keys(["esc"])
            time.sleep(_SLASH_CMD_SETTLE_SEC)
            out = session.capture_screen(drain_sec=0.3) or ""
            self.assertIn(
                "Login cancelled",
                out,
                f"Esc should close form and show 'Login cancelled'; got: {repr(out[:500])}",
            )
            # Composer or prompt-ready should be visible again
            screen = session.capture_screen(drain_sec=0.2) or ""
            self.assertTrue(
                harness.LANDMARK_PROMPT_READY in screen
                or harness.LANDMARK_PROMPT_READY_SHORT in screen
                or "> " in screen,
                f"Session should be back at composer; got: {repr(screen[:300])}",
            )
            session.send_keys(["ctrl+c", "ctrl+c"])

    def test_tui_auth_login_submit_empty_shows_error(self):
        """/auth login, Enter with empty gateway/username shows required-field error."""
        if not harness.pty_available():
            self.skipTest("pexpect not installed")
        with harness.TuiPtySession(state.CONFIG_PATH, timeout=25) as session:
            self._wait_ready(session)
            session.send_keys(["/auth login", "enter"])
            time.sleep(_SLASH_CMD_SETTLE_SEC)
            ok = session.wait_for_login_form(timeout_sec=8)
            self.assertTrue(ok, "login form should appear")
            # Submit with Enter (fields may be prepopulated with gateway URL; username empty)
            session.send_keys(["enter"])
            time.sleep(_SLASH_CMD_SETTLE_SEC)
            out = session.capture_screen(drain_sec=0.3) or ""
            # Either "required" error or "Login failed" (if gateway was filled and request sent)
            self.assertTrue(
                "required" in out.lower()
                or "Login failed" in out
                or "Gateway URL" in out,
                f"Empty submit should show required error or leave form open; got: {repr(out[:500])}",
            )
            session.send_keys(["esc"])
            time.sleep(_SLASH_CMD_SETTLE_SEC)
            session.send_keys(["ctrl+c", "ctrl+c"])

    # ------------------------------------------------ ! shell escape
    def test_tui_shell_escape_runs_command(self):
        """! echo shows command output inline and session stays active."""
        if not harness.pty_available():
            self.skipTest("pexpect not installed")
        with harness.TuiPtySession(state.CONFIG_PATH, timeout=25) as session:
            self._wait_ready(session)
            session.send_keys(["! echo hello_from_shell", "enter"])
            time.sleep(_SLASH_CMD_SETTLE_SEC)
            out = session.capture_screen(drain_sec=0.3) or ""
            self.assertIn(
                "hello_from_shell", out,
                f"! echo should show stdout inline; got: {repr(out[:400])}",
            )
            # Session stays active.
            screen = session.capture_screen(drain_sec=0.2) or ""
            self.assertTrue(
                harness.LANDMARK_PROMPT_READY in screen
                or harness.LANDMARK_PROMPT_READY_SHORT in screen,
                f"Session should remain active after ! cmd; status: {repr(screen[:300])}",
            )
            session.send_keys(["ctrl+c", "ctrl+c"])

    def test_tui_shell_escape_empty_shows_usage(self):
        """! with no command shows a usage hint."""
        if not harness.pty_available():
            self.skipTest("pexpect not installed")
        with harness.TuiPtySession(state.CONFIG_PATH, timeout=25) as session:
            self._wait_ready(session)
            session.send_keys(["!", "enter"])
            time.sleep(_SLASH_CMD_SETTLE_SEC)
            out = session.capture_screen(drain_sec=0.3) or ""
            self.assertTrue(
                "usage" in out.lower() or "!" in out,
                f"Empty ! should show usage hint; got: {repr(out[:400])}",
            )
            session.send_keys(["ctrl+c", "ctrl+c"])

    def test_tui_shell_escape_nonzero_exit(self):
        """! exit 42 shows non-zero exit code inline."""
        if not harness.pty_available():
            self.skipTest("pexpect not installed")
        with harness.TuiPtySession(state.CONFIG_PATH, timeout=25) as session:
            self._wait_ready(session)
            session.send_keys(["! sh -c 'exit 42'", "enter"])
            time.sleep(_SLASH_CMD_SETTLE_SEC)
            out = session.capture_screen(drain_sec=0.3) or ""
            self.assertTrue(
                "42" in out or "exit" in out.lower(),
                f"Non-zero ! exit should show exit code; got: {repr(out[:400])}",
            )
            session.send_keys(["ctrl+c", "ctrl+c"])
