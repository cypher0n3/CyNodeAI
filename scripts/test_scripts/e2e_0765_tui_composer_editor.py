# E2E: cynork TUI composer — footnote, multiline (Alt+Enter / Ctrl+J),
# input history (Ctrl+up/down).
# Traces: REQ-CLIENT-0164 (TUI); cynork TUI spec delta (composer wrap, login overlay).
# CYNAI.CLIENT.CynorkTui.ComposerInput

import os
import tempfile
import time
import unittest

from scripts.test_scripts import helpers
from scripts.test_scripts import tui_pty_harness as harness
import scripts.test_scripts.e2e_state as state

_TUI_STARTUP_DELAY_SEC = 1.5
_SETTLE_SEC = 0.45


def _ensure_config_file():
    if not state.CONFIG_PATH:
        return
    if not os.path.isfile(state.CONFIG_PATH):
        with open(state.CONFIG_PATH, "w", encoding="utf-8") as f:
            f.write("# E2E TUI composer editor\n")


class TestTuiComposerEditor(unittest.TestCase):
    """E2E: composer footnote text, multiline input, prior-message history via PTY."""

    tags = ["suite_cynork", "full_demo", "tui_pty", "tui", "no_inference"]
    prereqs = ["gateway", "config", "auth"]

    def setUp(self):
        ok, detail = helpers.prepare_e2e_cynork_auth()
        self.assertTrue(ok, detail)
        _ensure_config_file()

    def _wait_ready(self, session, timeout=12):
        time.sleep(_TUI_STARTUP_DELAY_SEC)
        ok = session.wait_for_prompt_ready(timeout_sec=timeout)
        self.assertTrue(
            ok,
            "TUI did not show prompt-ready landmark within timeout",
        )

    def test_tui_composer_footnote_shows_prior_messages_and_alt_enter(self):
        """Status area includes copy/scroll footnote fragments (prior messages, Alt+Enter)."""
        if not harness.pty_available():
            self.skipTest("pexpect not installed")
        with harness.TuiPtySession(state.CONFIG_PATH, timeout=25) as session:
            self._wait_ready(session)
            out = session.capture_screen(drain_sec=0.4) or ""
            self.assertTrue(
                "prior messages" in out
                and "Alt+Enter" in out
                and "Ctrl+J" in out,
                f"Expected footnote substrings; got: {repr(out[:900])}",
            )
            session.send_keys(["ctrl+c", "ctrl+c"])

    def test_tui_composer_multiline_ctrl_j_visible_before_send(self):
        """Ctrl+J inserts a second line in the composer; both lines appear in the PTY snapshot."""
        if not harness.pty_available():
            self.skipTest("pexpect not installed")
        line_a = "e2e_ml_top_line"
        line_b = "e2e_ml_bottom_line"
        with harness.TuiPtySession(state.CONFIG_PATH, timeout=25) as session:
            self._wait_ready(session)
            session.send_keys([line_a, "ctrl+j", line_b])
            time.sleep(_SETTLE_SEC)
            out = session.capture_screen(drain_sec=0.35) or ""
            self.assertIn(
                line_a,
                out,
                f"First line should remain visible in composer; got: {repr(out[:700])}",
            )
            self.assertIn(
                line_b,
                out,
                f"Second line after ctrl+j should be visible; got: {repr(out[:700])}",
            )
            session.send_keys(["ctrl+c", "ctrl+c"])

    def test_tui_composer_multiline_alt_enter_visible_before_send(self):
        """Alt+Enter inserts a newline in the composer (same intent as Ctrl+J)."""
        if not harness.pty_available():
            self.skipTest("pexpect not installed")
        u1, u2 = "e2e_alt_ln1", "e2e_alt_ln2"
        with harness.TuiPtySession(state.CONFIG_PATH, timeout=25) as session:
            self._wait_ready(session)
            session.send_keys([u1, "alt+enter", u2])
            time.sleep(_SETTLE_SEC)
            out = session.capture_screen(drain_sec=0.35) or ""
            self.assertIn(u1, out, repr(out[:700]))
            self.assertIn(u2, out, repr(out[:700]))
            session.send_keys(["ctrl+c", "ctrl+c"])

    def test_tui_auth_login_shows_sign_in_or_landmark(self):
        """/auth login shows recovery landmark or Sign in + gateway fields."""
        if not harness.pty_available():
            self.skipTest("pexpect not installed")
        with harness.TuiPtySession(state.CONFIG_PATH, timeout=25) as session:
            self._wait_ready(session)
            session.send_keys(["/auth login", "enter"])
            ok = session.wait_for_login_form(timeout_sec=15)
            self.assertTrue(ok, "login overlay should appear")
            snap = session.capture_screen(drain_sec=0.35) or ""
            self.assertTrue(
                harness.LANDMARK_AUTH_RECOVERY_READY in snap
                or ("Sign in" in snap and "Gateway URL" in snap)
                or ("Gateway URL" in snap and "Username" in snap),
                f"Expected landmark or Sign in + fields; got: {repr(snap[:600])}",
            )
            session.send_keys(["esc"])
            time.sleep(_SETTLE_SEC)
            session.send_keys(["ctrl+c", "ctrl+c"])

    def test_tui_empty_env_tokens_shows_login_overlay(self):
        """With bearer tokens cleared in env, TUI shows the in-session login overlay on startup."""
        if not harness.pty_available():
            self.skipTest("pexpect not installed")
        # Empty CYNORK_TOKEN does not block ApplySessionStore; use a fresh XDG cache so no
        # session.json is loaded (same pattern as helpers._run_cynork_subprocess isolation).
        with tempfile.TemporaryDirectory() as isolated_cache:
            with harness.TuiPtySession(
                state.CONFIG_PATH,
                timeout=25,
                env_extra={
                    # str() avoids bandit B105 on literal ""; values are intentional env clears.
                    "CYNORK_TOKEN": str(),
                    "CYNORK_REFRESH_TOKEN": str(),
                    "XDG_CACHE_HOME": isolated_cache,
                },
            ) as session:
                time.sleep(_TUI_STARTUP_DELAY_SEC)
                ok = session.wait_for_login_form(timeout_sec=15)
                self.assertTrue(
                    ok,
                    (
                        "login overlay should appear when env tokens are empty "
                        "and session cache is isolated"
                    ),
                )
                session.send_keys(["esc"])
                time.sleep(_SETTLE_SEC)
                session.send_keys(["ctrl+c", "ctrl+c"])

    def test_tui_composer_ctrl_up_recalls_last_sent_message(self):
        """After sending a chat line, Ctrl+Up restores it into the composer."""
        if not harness.pty_available():
            self.skipTest("pexpect not installed")
        bearer = helpers.read_token_from_config(state.CONFIG_PATH)
        self.assertTrue(bearer, "no access token after E2E login prereq")
        token = f"e2e_hist_token_{int(time.time())}"
        with harness.TuiPtySession(
            state.CONFIG_PATH,
            timeout=90,
            env_extra={"CYNORK_TOKEN": bearer},
        ) as session:
            self._wait_ready(session)
            session.send_keys([token, "enter"])
            # History is pushed on send; the prompt-ready landmark may not re-emit until
            # streaming finishes (same PTY race as e2e_0750). Wait for echo or the token.
            out = session.read_until_landmark(
                ["You:", token, harness.LANDMARK_PROMPT_READY, harness.LANDMARK_PROMPT_READY_SHORT],
                timeout_sec=75,
            )
            self.assertTrue(
                "You:" in out
                or token in out
                or harness.LANDMARK_PROMPT_READY in out
                or harness.LANDMARK_PROMPT_READY_SHORT in out,
                f"Expected user echo or prompt landmark after send; got: {repr(out[:500])}",
            )
            time.sleep(0.35)
            session.send_keys(["ctrl+up"])
            time.sleep(_SETTLE_SEC)
            snap = session.capture_screen(drain_sec=0.4) or ""
            self.assertIn(
                token,
                snap,
                f"Ctrl+Up should recall last sent line; got: {repr(snap[:700])}",
            )
            session.send_keys(["ctrl+c", "ctrl+c"])

    def test_tui_composer_ctrl_down_navigates_forward_in_history(self):
        """After Ctrl+Up recalls older line, Ctrl+Down moves forward to newer history entry."""
        if not harness.pty_available():
            self.skipTest("pexpect not installed")
        bearer = helpers.read_token_from_config(state.CONFIG_PATH)
        self.assertTrue(bearer, "no access token after E2E login prereq")
        first = f"e2e_hist_a_{int(time.time())}"
        second = f"e2e_hist_b_{int(time.time())}"
        with harness.TuiPtySession(
            state.CONFIG_PATH,
            timeout=120,
            env_extra={"CYNORK_TOKEN": bearer},
        ) as session:
            self._wait_ready(session)
            session.send_keys([first, "enter"])
            _ = session.read_until_landmark(
                [
                    "You:",
                    first,
                    harness.LANDMARK_PROMPT_READY,
                    harness.LANDMARK_PROMPT_READY_SHORT,
                ],
                timeout_sec=75,
            )
            time.sleep(0.35)
            session.send_keys([second, "enter"])
            _ = session.read_until_landmark(
                [
                    "You:",
                    second,
                    harness.LANDMARK_PROMPT_READY,
                    harness.LANDMARK_PROMPT_READY_SHORT,
                ],
                timeout_sec=75,
            )
            time.sleep(0.35)
            # Empty scrollback so "first" is not matched from prior "You:" lines when
            # asserting composer text.
            session.send_keys(["/clear", "enter"])
            time.sleep(_SETTLE_SEC)
            session.send_keys(["ctrl+up"])
            time.sleep(_SETTLE_SEC)
            session.send_keys(["ctrl+up"])
            time.sleep(_SETTLE_SEC)
            snap_old = session.capture_screen(drain_sec=0.4) or ""
            self.assertIn(
                first,
                snap_old,
                f"Ctrl+Up twice should show older sent line; got: {repr(snap_old[:700])}",
            )
            session.send_keys(["ctrl+down"])
            time.sleep(_SETTLE_SEC)
            snap_newer = session.capture_screen(drain_sec=0.4) or ""
            self.assertIn(
                second,
                snap_newer,
                "Ctrl+Down should recall newer history; got: "
                + repr(snap_newer[:700]),
            )
            session.send_keys(["ctrl+c", "ctrl+c"])

    def test_tui_narrow_terminal_footnote_still_present(self):
        """With narrow cols, composer footnote fragments still render (wrap regression smoke)."""
        if not harness.pty_available():
            self.skipTest("pexpect not installed")
        with harness.TuiPtySession(
            state.CONFIG_PATH, timeout=25, cols=42, rows=24
        ) as session:
            self._wait_ready(session)
            out = session.capture_screen(drain_sec=0.45) or ""
            self.assertIn(
                "prior messages",
                out,
                f"Narrow PTY should still show footnote hint; got: {repr(out[:900])}",
            )
            session.send_keys(["ctrl+c", "ctrl+c"])
