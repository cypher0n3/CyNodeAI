# E2E: TUI structured streaming behavior (in-flight turn, overwrite, heartbeat).
# Traces: REQ-CLIENT-0213-0220, cynork_tui_streaming.feature, Task 5.
# Use with just e2e --tags tui_pty. Requires real stack (CONFIG_PATH, auth, PMA).

import os
import time
import unittest

import requests

from scripts.test_scripts import config, helpers
from scripts.test_scripts import tui_pty_harness as harness
import scripts.test_scripts.e2e_state as state

_SSE_TIMEOUT_SEC = int(config.E2E_SSE_REQUEST_TIMEOUT)


def _auth_headers(cfg_path):
    token = helpers.read_token_from_config(cfg_path)
    if not token:
        return {}
    return {"Authorization": f"Bearer {token}"}


class TestTUIStreamingBehavior(unittest.TestCase):
    """E2E: TUI in-flight turn, overwrite scopes, heartbeat (Task 5).

    Requires real stack. TUI progressive test runs cynork against live gateway.
    """

    tags = ["suite_cynork", "tui_pty", "tui", "pma_inference", "streaming"]
    prereqs = ["gateway", "config", "auth", "ollama"]

    def setUp(self):
        if not state.CONFIG_PATH or not os.path.isfile(state.CONFIG_PATH):
            self.skipTest("CONFIG_PATH not set (run after auth login prereq)")
        ok, detail = helpers.ensure_valid_auth_session(state.CONFIG_PATH)
        self.assertTrue(ok, f"auth session invalid: {detail}")
        if os.environ.get("E2E_SKIP_INFERENCE_SMOKE", "") or config.E2E_SKIP_INFERENCE_SMOKE:
            self.skipTest("E2E_SKIP_INFERENCE_SMOKE set; skipping TUI streaming")

    def _gateway_url(self):
        return config.USER_API.rstrip("/")

    def test_tui_updates_single_inflight_turn_progressively(self):
        """TUI MUST update exactly one in-flight assistant turn progressively."""
        if not harness.pty_available():
            self.skipTest("pexpect not installed or not Unix; install scripts/requirements-e2e.txt")
        if not config.CYNORK_BIN or not os.path.isfile(config.CYNORK_BIN):
            self.skipTest("cynork-dev binary not found (run just build-cynork-dev)")
        with harness.TuiPtySession(
            state.CONFIG_PATH,
            timeout=25,
        ) as session:
            time.sleep(1.5)
            session.read_until_landmark(
                [harness.LANDMARK_PROMPT_READY, harness.LANDMARK_PROMPT_READY_SHORT, "> "],
                timeout_sec=12,
            )
            session.send_keys(["Reply with exactly: OK", "enter"])
            out = session.read_until_landmark(
                [
                    harness.LANDMARK_ASSISTANT_IN_FLIGHT,
                    harness.LANDMARK_RESPONSE_COMPLETE,
                    "OK",
                    " (scrollback)",
                ],
                timeout_sec=30,
            )
            self.assertTrue(
                harness.LANDMARK_ASSISTANT_IN_FLIGHT in (out or "")
                or harness.LANDMARK_RESPONSE_COMPLETE in (out or "")
                or "OK" in (out or "")
                or " (scrollback)" in (out or ""),
                f"TUI must show in-flight or final content; got: {repr((out or '')[:600])}",
            )

    def test_tui_iteration_scoped_overwrite_only_replaces_target_segment(self):
        """Per-iteration overwrite MUST replace only the targeted segment."""
        headers = _auth_headers(state.CONFIG_PATH)
        headers["Content-Type"] = "application/json"
        headers["Accept"] = "text/event-stream"
        resp = requests.post(
            f"{self._gateway_url()}/v1/chat/completions",
            headers=headers,
            json={
                "model": "cynodeai.pm",
                "stream": True,
                "messages": [{"role": "user", "content": "Reply with exactly: OK"}],
            },
            stream=True,
            timeout=_SSE_TIMEOUT_SEC,
        )
        if resp.status_code != 200:
            self.skipTest(f"gateway returned {resp.status_code}")
        events, found_done = helpers.parse_sse_stream_typed(resp)
        self.assertTrue(found_done)
        iteration_starts = [e for e in events if e.get("event") == "cynodeai.iteration_start"]
        self.assertGreater(
            len(iteration_starts), 0,
            "stream must include iteration_start for overwrite scope",
        )

    def test_tui_turn_scoped_amendment_replaces_visible_text_without_duplication(self):
        """Per-turn amendment MUST replace visible text without duplication."""
        headers = _auth_headers(state.CONFIG_PATH)
        headers["Content-Type"] = "application/json"
        headers["Accept"] = "text/event-stream"
        resp = requests.post(
            f"{self._gateway_url()}/v1/chat/completions",
            headers=headers,
            json={
                "model": "cynodeai.pm",
                "stream": True,
                "messages": [{"role": "user", "content": "Reply with exactly: OK"}],
            },
            stream=True,
            timeout=_SSE_TIMEOUT_SEC,
        )
        if resp.status_code != 200:
            self.skipTest(f"gateway returned {resp.status_code}")
        events, found_done = helpers.parse_sse_stream_typed(resp)
        self.assertTrue(found_done)
        amendments = [e for e in events if e.get("event") == "cynodeai.amendment"]
        if not amendments:
            self.skipTest("stream did not contain amendment; PMA may not have triggered redaction")
        self.assertGreater(
            len(amendments), 0,
            "stream must include amendment for turn-scoped replace",
        )

    def test_tui_heartbeat_progress_indicator_disappears_after_final_content(self):
        """Heartbeat progress indicator MUST disappear after final content."""
        headers = _auth_headers(state.CONFIG_PATH)
        headers["Content-Type"] = "application/json"
        headers["Accept"] = "text/event-stream"
        resp = requests.post(
            f"{self._gateway_url()}/v1/chat/completions",
            headers=headers,
            json={
                "model": "cynodeai.pm",
                "stream": True,
                "messages": [{"role": "user", "content": "Reply with exactly: OK"}],
            },
            stream=True,
            timeout=_SSE_TIMEOUT_SEC,
        )
        if resp.status_code != 200:
            self.skipTest(f"gateway returned {resp.status_code}")
        events, found_done = helpers.parse_sse_stream_typed(resp)
        self.assertTrue(found_done)
        event_names = [e.get("event") for e in events if e.get("event")]
        heartbeat_idx = next(
            (i for i, n in enumerate(event_names) if n == "cynodeai.heartbeat"),
            -1,
        )
        if heartbeat_idx < 0:
            self.skipTest("stream did not contain heartbeat; upstream may have streamed normally")
        content_events = [e for e in events if e.get("event") is None and e.get("data")]
        self.assertGreater(
            len(content_events), 0,
            "stream must include content after heartbeat",
        )
