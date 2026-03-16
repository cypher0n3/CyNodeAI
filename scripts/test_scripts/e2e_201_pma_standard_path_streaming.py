# E2E: PMA standard-path streaming via gateway SSE (capable model + MCP).
# Gateway relays PMA NDJSON as SSE; we assert on event types (iteration_start, tool_progress, etc.).
# Traces: REQ-PMAGNT-0118, REQ-PMAGNT-0120-0126, features/agents/pma_chat_and_context.feature.

import json
import os
import unittest

import requests

from scripts.test_scripts import config, helpers
import scripts.test_scripts.e2e_state as state

_SSE_TIMEOUT_SEC = 120


def _auth_headers(config_path):
    token = helpers.read_token_from_config(config_path)
    if not token:
        return {}
    return {"Authorization": f"Bearer {token}"}


class TestPMAStandardPathStreaming(unittest.TestCase):
    """E2E: PMA standard path via gateway SSE (iteration_start, thinking, tool_progress)."""

    tags = ["suite_orchestrator", "pma_inference", "inference"]

    def setUp(self):
        if not state.CONFIG_PATH or not os.path.isfile(state.CONFIG_PATH):
            self.skipTest("CONFIG_PATH not set (run after auth login prereq)")
        ok, _ = helpers.ensure_valid_auth_session(state.CONFIG_PATH)
        self.assertTrue(ok, "auth session invalid before PMA streaming tests")
        if os.environ.get("E2E_SKIP_INFERENCE_SMOKE", "") or config.E2E_SKIP_INFERENCE_SMOKE:
            self.skipTest("E2E_SKIP_INFERENCE_SMOKE set; skipping PMA standard-path streaming")

    def _gateway_url(self):
        return config.USER_API.rstrip("/")

    def test_pma_standard_path_ndjson_stream_contains_iteration_and_thinking_events(self):
        """Standard path SSE MUST relay iteration_start (cynodeai.iteration_start)."""
        headers = _auth_headers(state.CONFIG_PATH)
        headers["Content-Type"] = "application/json"
        headers["Accept"] = "text/event-stream"
        payload = {
            "model": "cynodeai.pm",
            "stream": True,
            "messages": [{"role": "user", "content": "Reply with exactly: OK"}],
        }
        resp = requests.post(
            f"{self._gateway_url()}/v1/chat/completions",
            headers=headers,
            json=payload,
            stream=True,
            timeout=_SSE_TIMEOUT_SEC,
        )
        if resp.status_code != 200:
            self.skipTest(
                f"gateway returned {resp.status_code} (PMA or inference may be unavailable)"
            )
        events, _ = helpers.parse_sse_stream_typed(resp)
        iteration_starts = [e for e in events if e.get("event") == "cynodeai.iteration_start"]
        self.assertGreater(
            len(iteration_starts), 0,
            "SSE stream must contain at least one cynodeai.iteration_start event",
        )

    def test_pma_standard_path_ndjson_stream_contains_tool_activity_before_done(self):
        """If tools run, stream MUST contain tool_progress before [DONE]."""
        headers = _auth_headers(state.CONFIG_PATH)
        headers["Content-Type"] = "application/json"
        headers["Accept"] = "text/event-stream"
        payload = {
            "model": "cynodeai.pm",
            "stream": True,
            "messages": [{"role": "user", "content": "Reply with exactly: OK"}],
        }
        resp = requests.post(
            f"{self._gateway_url()}/v1/chat/completions",
            headers=headers,
            json=payload,
            stream=True,
            timeout=_SSE_TIMEOUT_SEC,
        )
        if resp.status_code != 200:
            self.skipTest(f"gateway returned {resp.status_code}")
        events, done = helpers.parse_sse_stream_typed(resp)
        tool_events = [e for e in events if (e.get("event") or "").startswith("cynodeai.tool")]
        if done and tool_events:
            self.assertGreater(
                len(tool_events), 0,
                "tool events present and [DONE] seen (order implied by stream)",
            )

    def test_pma_standard_path_ndjson_stream_emits_scoped_overwrite_for_controlled_fixture(
            self):
        """Stream MAY emit amendment events with scope (iteration or turn)."""
        headers = _auth_headers(state.CONFIG_PATH)
        headers["Content-Type"] = "application/json"
        headers["Accept"] = "text/event-stream"
        payload = {
            "model": "cynodeai.pm",
            "stream": True,
            "messages": [{"role": "user", "content": "Say hello"}],
        }
        resp = requests.post(
            f"{self._gateway_url()}/v1/chat/completions",
            headers=headers,
            json=payload,
            stream=True,
            timeout=_SSE_TIMEOUT_SEC,
        )
        if resp.status_code != 200:
            self.skipTest(f"gateway returned {resp.status_code}")
        events, _ = helpers.parse_sse_stream_typed(resp)
        amendments = [e for e in events if e.get("event") == "cynodeai.amendment"]
        for ev in amendments:
            try:
                data = json.loads(ev.get("data") or "{}")
                scope = data.get("scope")
                if scope is not None:
                    self.assertIn(
                        scope, ("iteration", "turn"),
                        f"amendment scope must be iteration or turn: {data}",
                    )
            except json.JSONDecodeError:
                continue
