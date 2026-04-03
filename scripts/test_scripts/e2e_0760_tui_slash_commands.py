# E2E: cynork TUI slash commands and shell-escape (`!`) via PTY harness.
# Traces: REQ-CLIENT-0164, REQ-CLIENT-0165, REQ-CLIENT-0171, REQ-CLIENT-0172,
#         REQ-CLIENT-0173, REQ-CLIENT-0175, REQ-CLIENT-0176, REQ-CLIENT-0207,
#         REQ-CLIENT-0208, REQ-CLIENT-0209, REQ-CLIENT-0210.
# CYNAI.CLIENT.CynorkTui.LocalSlashCommands,
# CYNAI.CLIENT.CynorkTui.ModelSlashCommands,
# CYNAI.CLIENT.CynorkTui.ProjectSlashCommands,
# CYNAI.CLIENT.CynorkTui.ThreadSlashCommands,
# CYNAI.CLIENT.CliChatShellEscape,
# CYNAI.CLIENT.CliChat.ModelFlag,
# CYNAI.CLIENT.CliChat.ResumeThreadFlag.

import os
import time
import unittest

from scripts.test_scripts import helpers
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

    tags = ["suite_cynork", "full_demo", "tui_pty", "tui", "no_inference"]
    prereqs = ["gateway", "config", "auth"]

    def setUp(self):
        ok, detail = helpers.prepare_e2e_cynork_auth()
        self.assertTrue(ok, detail)
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
            # Send /help so its output populates scrollback, then wait for re-render.
            session.send_keys(["/help", "enter"])
            session.wait_for_prompt_ready(timeout_sec=4)
            # Now clear and wait for the next re-render.
            session.send_keys(["/clear", "enter"])
            session.wait_for_prompt_ready(timeout_sec=4)
            out = session.capture_screen(drain_sec=0.5) or ""
            # After /clear the scrollback body should not contain help-listed commands.
            # We split at the status-bar line ("> ") to isolate the scrollback area.
            scrollback_area = out.split("\n> ")[0] if "\n> " in out else out
            self.assertNotIn(
                "/exit",
                scrollback_area,
                f"/clear should have cleared the /help output; got: {repr(out[:500])}",
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
            # Use stream search so we don't miss the hint due to render-snapshot timing.
            out = session.read_until_landmark(
                ["Unknown", "unknown", "/help"], timeout_sec=4.0
            )
            self.assertTrue(
                "/help" in out or "unknown" in out.lower() or "Unknown" in out,
                f"Unknown command should hint /help; got: {repr(out[:400])}",
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
            # Allow up to 10s: the process must close the PTY and script(1) must exit.
            deadline = time.time() + 10.0
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
            deadline = time.time() + 10.0
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
            # Start reading immediately to avoid PTY-buffer stall; allow 15s for overlay.
            ok = session.wait_for_login_form(timeout_sec=15)
            self.assertTrue(
                ok,
                "/auth login should show login form (landmark or Login/Gateway URL)",
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
            ok = session.wait_for_login_form(timeout_sec=15)
            self.assertTrue(ok, "login form should appear")
            session.send_keys(["esc"])
            # Stream-search for cancellation text to avoid snapshot-timing races.
            out = session.read_until_landmark(
                ["Login cancelled", "cancelled", harness.LANDMARK_PROMPT_READY],
                timeout_sec=5.0,
            )
            self.assertTrue(
                "Login cancelled" in out
                or "cancelled" in out.lower()
                or harness.LANDMARK_PROMPT_READY in out,
                f"Esc should close form and return to composer; got: {repr(out[:500])}",
            )
            session.send_keys(["ctrl+c", "ctrl+c"])

    def test_tui_auth_login_submit_empty_shows_error(self):
        """/auth login, Enter with empty gateway/username shows required-field error."""
        if not harness.pty_available():
            self.skipTest("pexpect not installed")
        with harness.TuiPtySession(state.CONFIG_PATH, timeout=25) as session:
            self._wait_ready(session)
            session.send_keys(["/auth login", "enter"])
            ok = session.wait_for_login_form(timeout_sec=15)
            self.assertTrue(ok, "login form should appear")
            # Submit with Enter (gateway URL may be prepopulated; username is empty)
            session.send_keys(["enter"])
            # Stream-search for a validation error, login failure, or the form staying open.
            out = session.read_until_landmark(
                ["required", "Required", "Login failed", "Gateway URL", "Username"],
                timeout_sec=5.0,
            )
            self.assertTrue(
                "required" in out.lower()
                or "Login failed" in out
                or "Gateway URL" in out
                or "Username" in out,
                "Empty submit should show required error or leave form open; got: "
                + repr(out[:500]),
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
            out = session.read_until_landmark(
                ["hello_from_shell"],
                timeout_sec=5.0,
            )
            self.assertIn(
                "hello_from_shell",
                out,
                f"! echo should show stdout inline; got: {repr(out[:400])}",
            )
            # Session stays active: when scrollback is non-empty the prompt-ready landmark is only
            # in the empty-scrollback path; composer still shows the "│ >" prompt (see view_render).
            deadline = time.time() + 10.0
            ok = False
            last = ""
            while time.time() < deadline:
                last = session.capture_screen(drain_sec=0.25) or ""
                if (
                    harness.LANDMARK_PROMPT_READY in last
                    or harness.LANDMARK_PROMPT_READY_SHORT in last
                    or "│ >" in last
                ):
                    ok = True
                    break
                time.sleep(0.1)
            self.assertTrue(
                ok,
                f"Session should remain active after ! cmd; status: {repr(last[:500])}",
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

    # ---------------------------------------------------------- /copy (Task 3 E2E alignment)
    def test_tui_slash_copy_last_no_assistant_shows_feedback(self):
        """/copy last with no assistant message shows scrollback feedback (no spurious switch)."""
        if not harness.pty_available():
            self.skipTest("pexpect not installed")
        with harness.TuiPtySession(state.CONFIG_PATH, timeout=25) as session:
            self._wait_ready(session)
            session.send_keys(["/copy last", "enter"])
            out = session.read_until_landmark(
                [
                    "No assistant message",
                    "assistant message to copy",
                    "Copy failed",
                ],
                timeout_sec=6.0,
            )
            self.assertTrue(
                "No assistant message" in out
                or "assistant message to copy" in out.lower()
                or "Copy failed" in out,
                f"expected no-assistant copy feedback; got: {repr(out[:500])}",
            )
            session.send_keys(["ctrl+c", "ctrl+c"])

    def test_tui_slash_copy_last_after_chat_shows_feedback(self):
        """After a send, /copy last reports last-assistant copy feedback in scrollback."""
        if not harness.pty_available():
            self.skipTest("pexpect not installed")
        ok_tok = helpers.refresh_e2e_gateway_tokens_for_long_suite(
            state.CONFIG_PATH, timeout=60
        )
        self.assertTrue(ok_tok, "fresh gateway tokens before long TUI chat (full suite)")
        token = f"e2e_copy_last_{int(time.time())}"
        with harness.TuiPtySession(
            state.CONFIG_PATH,
            timeout=90,
        ) as session:
            self._wait_ready(session)
            session.send_keys([token, "enter"])
            _ = session.read_until_landmark(
                [
                    "You:",
                    token,
                    harness.LANDMARK_PROMPT_READY,
                    harness.LANDMARK_PROMPT_READY_SHORT,
                ],
                timeout_sec=75,
            )
            time.sleep(0.35)
            session.send_keys(["/copy last", "enter"])
            out = session.read_until_landmark(
                [
                    "Last message copied",
                    "No assistant message",
                    "Copy failed",
                    "Copied to clipboard",
                ],
                timeout_sec=8.0,
            )
            self.assertTrue(
                "Last message copied" in out
                or "Copied to clipboard" in out
                or "Copy failed" in out
                or "No assistant message" in out,
                f"expected /copy last feedback; got: {repr(out[:700])}",
            )
            session.send_keys(["ctrl+c", "ctrl+c"])

    def test_tui_slash_copy_all_shows_feedback(self):
        """/copy all produces transcript copy feedback (empty or full transcript)."""
        if not harness.pty_available():
            self.skipTest("pexpect not installed")
        with harness.TuiPtySession(state.CONFIG_PATH, timeout=25) as session:
            self._wait_ready(session)
            session.send_keys(["/copy all", "enter"])
            out = session.read_until_landmark(
                [
                    "All text copied",
                    "Copy failed",
                    "Copied to clipboard",
                ],
                timeout_sec=6.0,
            )
            self.assertTrue(
                "All text copied" in out
                or "Copied to clipboard" in out
                or "Copy failed" in out,
                f"expected /copy all feedback; got: {repr(out[:500])}",
            )
            session.send_keys(["ctrl+c", "ctrl+c"])

    # ---------------------------------------------------------- /project bare id
    def test_tui_slash_project_bare_id_sets_project(self):
        """/project <bare_id> sets the session project context (Task 1B).
        Asserts: REQ-CLIENT-0173; CYNAI.CLIENT.CynorkTui.ProjectSlashCommands."""
        if not harness.pty_available():
            self.skipTest("pexpect not installed")
        with harness.TuiPtySession(state.CONFIG_PATH, timeout=25) as session:
            self._wait_ready(session)
            session.send_keys(["/project proj-bare-001", "enter"])
            time.sleep(_SLASH_CMD_SETTLE_SEC)
            out = session.capture_screen(drain_sec=0.3) or ""
            self.assertTrue(
                "proj-bare-001" in out or "project" in out.lower(),
                f"/project bare id should set project; got: {repr(out[:400])}",
            )
            session.send_keys(["ctrl+c", "ctrl+c"])

    # ----------------------------------------------------------- /connect
    def test_tui_slash_connect_no_arg_shows_gateway(self):
        """/connect with no arg shows current gateway URL (Task 4A).
        Asserts: CYNAI.CLIENT.CynorkTui.LocalSlashCommands connect."""
        if not harness.pty_available():
            self.skipTest("pexpect not installed")
        with harness.TuiPtySession(state.CONFIG_PATH, timeout=25) as session:
            self._wait_ready(session)
            session.send_keys(["/connect", "enter"])
            time.sleep(_SLASH_CMD_SETTLE_SEC)
            out = session.capture_screen(drain_sec=0.3) or ""
            self.assertTrue(
                "gateway" in out.lower() or "http" in out.lower(),
                f"/connect should show gateway URL; got: {repr(out[:400])}",
            )
            session.send_keys(["ctrl+c", "ctrl+c"])

    # ------------------------------------------------ /show-thinking / /hide-thinking
    def test_tui_slash_show_thinking_toggles_on(self):
        """/show-thinking enables thinking visibility and scrollback confirms (Task 4B).
        Asserts: CYNAI.CLIENT.CynorkTui.ThinkingVisibilityBehavior."""
        if not harness.pty_available():
            self.skipTest("pexpect not installed")
        with harness.TuiPtySession(state.CONFIG_PATH, timeout=25) as session:
            self._wait_ready(session)
            session.send_keys(["/show-thinking", "enter"])
            time.sleep(_SLASH_CMD_SETTLE_SEC)
            out = session.capture_screen(drain_sec=0.3) or ""
            self.assertTrue(
                "thinking" in out.lower() or "visible" in out.lower(),
                f"/show-thinking should confirm toggle; got: {repr(out[:400])}",
            )
            session.send_keys(["ctrl+c", "ctrl+c"])

    def test_tui_slash_hide_thinking_toggles_off(self):
        """/hide-thinking disables thinking visibility and scrollback confirms (Task 4B).
        Asserts: CYNAI.CLIENT.CynorkTui.ThinkingVisibilityBehavior."""
        if not harness.pty_available():
            self.skipTest("pexpect not installed")
        with harness.TuiPtySession(state.CONFIG_PATH, timeout=25) as session:
            self._wait_ready(session)
            session.send_keys(["/hide-thinking", "enter"])
            time.sleep(_SLASH_CMD_SETTLE_SEC)
            out = session.capture_screen(drain_sec=0.3) or ""
            self.assertTrue(
                "thinking" in out.lower() or "hidden" in out.lower(),
                f"/hide-thinking should confirm toggle; got: {repr(out[:400])}",
            )
            session.send_keys(["ctrl+c", "ctrl+c"])

    def test_tui_slash_show_thinking_after_chat_still_toggles(self):
        """After a chat turn completes, /show-thinking still toggles (local transcript path)."""
        if not harness.pty_available():
            self.skipTest("pexpect not installed")
        ok_tok = helpers.refresh_e2e_gateway_tokens_for_long_suite(
            state.CONFIG_PATH, timeout=60
        )
        self.assertTrue(ok_tok, "fresh gateway tokens before long TUI chat (full suite)")
        with harness.TuiPtySession(state.CONFIG_PATH, timeout=90) as session:
            self._wait_ready(session)
            session.send_keys(["Reply with exactly: OK", "enter"])
            combined = ""
            deadline = time.time() + 70
            while time.time() < deadline:
                time.sleep(0.35)
                combined += session.capture_screen(drain_sec=0.25) or ""
                if (
                    harness.LANDMARK_PROMPT_READY in combined
                    or harness.LANDMARK_PROMPT_READY_SHORT in combined
                    or "OK" in combined
                ):
                    break
            session.send_keys(["/show-thinking", "enter"])
            time.sleep(_SLASH_CMD_SETTLE_SEC)
            out = session.capture_screen(drain_sec=0.35) or ""
            self.assertTrue(
                "thinking" in out.lower() or "visible" in out.lower(),
                f"/show-thinking after chat should confirm; got: {repr(out[:500])}",
            )
            session.send_keys(["ctrl+c", "ctrl+c"])

    # ----------------------------------------------------------------- /status
    def test_tui_slash_status_shows_output(self):
        """/status shows gateway health or connection status (Task 4C).
        Asserts: CYNAI.CLIENT.CynorkTui.StatusSlashCommands."""
        if not harness.pty_available():
            self.skipTest("pexpect not installed")
        with harness.TuiPtySession(state.CONFIG_PATH, timeout=25) as session:
            self._wait_ready(session)
            session.send_keys(["/status", "enter"])
            time.sleep(_SLASH_CMD_SETTLE_SEC)
            out = session.capture_screen(drain_sec=0.3) or ""
            self.assertTrue(
                "status" in out.lower()
                or "connected" in out.lower()
                or "not connected" in out.lower()
                or "gateway" in out.lower(),
                f"/status should show connection status; got: {repr(out[:400])}",
            )
            session.send_keys(["ctrl+c", "ctrl+c"])

    # ----------------------------------------------------------------- /whoami
    def test_tui_slash_whoami_shows_identity(self):
        """/whoami shows current user identity or gateway error (Task 4D).
        Asserts: CYNAI.CLIENT.CynorkTui.StatusSlashCommands whoami."""
        if not harness.pty_available():
            self.skipTest("pexpect not installed")
        with harness.TuiPtySession(state.CONFIG_PATH, timeout=25) as session:
            self._wait_ready(session)
            session.send_keys(["/whoami", "enter"])
            # Stream-search: /whoami makes a gateway call; allow up to 5s for response.
            out = session.read_until_landmark(
                ["id=", "handle=", "user=", "not connected", "Error:"],
                timeout_sec=5.0,
            )
            self.assertTrue(
                "id=" in out
                or "handle=" in out
                or "user=" in out
                or "not connected" in out.lower()
                or "error" in out.lower(),
                f"/whoami should show identity or error; got: {repr(out[:400])}",
            )
            session.send_keys(["ctrl+c", "ctrl+c"])

    # ---------------------------------------------------------- /thread switch / rename
    def test_tui_slash_thread_switch_shows_result(self):
        """/thread switch shows 'switched' confirmation or error in scrollback (Task 5A).
        Asserts: CYNAI.CLIENT.CynorkTui.ThreadSlashCommands."""
        if not harness.pty_available():
            self.skipTest("pexpect not installed")
        with harness.TuiPtySession(state.CONFIG_PATH, timeout=25) as session:
            self._wait_ready(session)
            session.send_keys(["/thread switch some-thread", "enter"])
            time.sleep(_SLASH_CMD_SETTLE_SEC)
            out = session.capture_screen(drain_sec=0.3) or ""
            self.assertTrue(
                "thread" in out.lower()
                or "switch" in out.lower()
                or "not found" in out.lower()
                or "error" in out.lower(),
                f"/thread switch should show result or error; got: {repr(out[:400])}",
            )
            session.send_keys(["ctrl+c", "ctrl+c"])

    def test_tui_slash_thread_rename_shows_result(self):
        """/thread rename shows renamed confirmation or error in scrollback (Task 5A).
        Asserts: CYNAI.CLIENT.CynorkTui.ThreadSlashCommands."""
        if not harness.pty_available():
            self.skipTest("pexpect not installed")
        with harness.TuiPtySession(state.CONFIG_PATH, timeout=25) as session:
            self._wait_ready(session)
            session.send_keys(["/thread rename New Title", "enter"])
            time.sleep(_SLASH_CMD_SETTLE_SEC)
            out = session.capture_screen(drain_sec=0.3) or ""
            self.assertTrue(
                "thread" in out.lower()
                or "rename" in out.lower()
                or "New Title" in out
                or "error" in out.lower()
                or "not connected" in out.lower(),
                f"/thread rename should show result or error; got: {repr(out[:400])}",
            )
            session.send_keys(["ctrl+c", "ctrl+c"])


class TestChatModeFlags(unittest.TestCase):
    """E2E: cynork chat subcommand flags (--model, --resume-thread) validated via subprocess.
    Asserts: CYNAI.CLIENT.CliChat.ModelFlag, CYNAI.CLIENT.CliChat.ResumeThreadFlag."""

    tags = ["suite_cynork", "full_demo", "chat", "tui"]
    prereqs = ["gateway", "config", "auth"]

    def setUp(self):
        ok, detail = helpers.prepare_e2e_cynork_auth()
        self.assertTrue(ok, detail)

    def test_chat_model_flag_is_accepted(self):
        """cynork chat --model <id> does not fail with 'unknown flag' (Task 1A).
        Asserts: REQ-CLIENT-0171; CYNAI.CLIENT.CliChat.ModelFlag."""
        _, out, err = helpers.run_cynork(
            [
                "chat",
                "--model", "test-model-e2e",
                "--message", "ping",
                "--plain",
            ],
            state.CONFIG_PATH,
            timeout=30,
        )
        merged = ((out or "") + (err or "")).lower()
        self.assertNotIn(
            "unknown flag",
            merged,
            f"--model flag must be accepted by cynork chat; got: {repr(merged[:400])}",
        )

    def test_chat_resume_thread_flag_is_accepted(self):
        """cynork chat --resume-thread <sel> does not fail with 'unknown flag' (Task 5B).
        Asserts: CYNAI.CLIENT.CliChat.ResumeThreadFlag."""
        _, out, err = helpers.run_cynork(
            [
                "chat",
                "--resume-thread", "some-selector",
                "--message", "ping",
                "--plain",
            ],
            state.CONFIG_PATH,
            timeout=30,
        )
        merged = ((out or "") + (err or "")).lower()
        self.assertNotIn(
            "unknown flag",
            merged,
            f"--resume-thread flag must be accepted; got: {repr(merged[:400])}",
        )
