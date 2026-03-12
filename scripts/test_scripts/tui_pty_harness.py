"""PTY harness for cynork TUI E2E using pexpect.

Launch TUI in a PTY with fixed size; inject keys; wait for landmarks.
Requires: pip install -r scripts/requirements-e2e.txt (pexpect).
Landmarks match cynork/internal/chat/landmarks.go for stable assertions.
"""

import os
import sys

from scripts.test_scripts import config

# Landmarks (must match cynork/internal/chat/landmarks.go)
LANDMARK_PROMPT_READY = "[CYNRK_PROMPT_READY]"
LANDMARK_ASSISTANT_IN_FLIGHT = "[CYNRK_ASSISTANT_IN_FLIGHT]"
LANDMARK_RESPONSE_COMPLETE = "[CYNRK_RESPONSE_COMPLETE]"
LANDMARK_THREAD_SWITCHED = "[CYNRK_THREAD_SWITCHED]"
LANDMARK_AUTH_RECOVERY_READY = "[CYNRK_AUTH_RECOVERY_READY]"

# Default TUI size (match model default 80x24)
DEFAULT_COLS = 80
DEFAULT_ROWS = 24

try:
    import pexpect
    _PEXPECT_AVAILABLE = True
except ImportError:
    pexpect = None
    _PEXPECT_AVAILABLE = False

_PTY_AVAILABLE = sys.platform != "win32" and _PEXPECT_AVAILABLE


def pty_available():
    """Return True if PTY harness can run (Unix + pexpect installed)."""
    return _PTY_AVAILABLE


class TuiPtySession:
    """Context manager: launch cynork tui via pexpect; fixed size, key injection, landmark wait."""

    def __init__(
        self,
        config_path,
        *,
        rows=DEFAULT_ROWS,
        cols=DEFAULT_COLS,
        env_extra=None,
        timeout=30,
    ):
        self.config_path = config_path
        self.rows = rows
        self.cols = cols
        self.env_extra = env_extra or {}
        self.timeout = timeout
        self._proc = None

    def __enter__(self):
        if not _PTY_AVAILABLE:
            if not _PEXPECT_AVAILABLE:
                raise RuntimeError(
                    "pexpect not installed; run: pip install -r scripts/requirements-e2e.txt"
                )
            raise RuntimeError("PTY not available on this platform")
        env = os.environ.copy()
        env["CYNORK_GATEWAY_URL"] = config.USER_API
        env["TERM"] = "dumb"
        env.update(self.env_extra)
        argv = [config.CYNORK_BIN, "--config", self.config_path, "tui"]
        self._proc = pexpect.spawn(
            argv[0],
            argv[1:],
            env=env,
            dimensions=(self.rows, self.cols),
            timeout=self.timeout,
            encoding="utf-8",
            codec_errors="replace",
        )
        return self

    def __exit__(self, *_):
        self.close()
        return False

    def close(self):
        """Send ctrl+c and wait for process exit."""
        if self._proc is None:
            return
        try:
            self._proc.sendcontrol("c")
            self._proc.expect(pexpect.EOF, timeout=2)
        except (pexpect.TIMEOUT, pexpect.ExceptionPexpect):
            try:
                self._proc.close(force=True)
            except OSError:
                pass
        self._proc = None

    def send_keys(self, key_sequence):
        """Send key sequence. Use 'enter' for Return, 'ctrl+c' for Control+C, or literal text."""
        if self._proc is None:
            raise RuntimeError("session closed")
        if isinstance(key_sequence, str):
            key_sequence = [key_sequence]
        for part in key_sequence:
            if part == "enter":
                self._proc.sendline("")
            elif part == "ctrl+c":
                self._proc.sendcontrol("c")
            elif part == "ctrl+d":
                self._proc.sendcontrol("d")
            else:
                self._proc.send(part)

    def read_until_landmark(self, landmark, timeout_sec=None):
        """Read until landmark appears or timeout. Return combined before+after (str)."""
        if self._proc is None:
            raise RuntimeError("session closed")
        t = timeout_sec if timeout_sec is not None else self.timeout
        try:
            self._proc.expect([landmark], timeout=t)
            return (self._proc.before or "") + (self._proc.after or "")
        except pexpect.TIMEOUT:
            return self._proc.before or ""
        except pexpect.EOF:
            return self._proc.before or ""

    def wait_for_prompt_ready(self, timeout_sec=None):
        """Wait until LANDMARK_PROMPT_READY appears. Return True if seen."""
        out = self.read_until_landmark(LANDMARK_PROMPT_READY, timeout_sec)
        return LANDMARK_PROMPT_READY in out

    def capture_screen(self, drain_sec=0.15):
        """Drain output for drain_sec and return current buffer (before + any new)."""
        if self._proc is None:
            raise RuntimeError("session closed")
        try:
            self._proc.expect([pexpect.EOF], timeout=drain_sec)
        except pexpect.TIMEOUT:
            pass
        except pexpect.EOF:
            pass
        return (self._proc.before or "") + (self._proc.after or "")
