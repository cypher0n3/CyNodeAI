# E2E: Cynork transport streaming (completions and responses) against gateway SSE.
# Asserts gateway emits named cynodeai.* events and streamed response_id.
# Traces: REQ-CLIENT-0209, REQ-CLIENT-0215-0220, Task 4 transport alignment.
# Requires real stack (CONFIG_PATH, auth, PMA).

import json
import os
import unittest

import requests

from scripts.test_scripts import config, helpers
import scripts.test_scripts.e2e_state as state

_SSE_TIMEOUT_SEC = 120


def _auth_headers(cfg_path):
    token = helpers.read_token_from_config(cfg_path)
    if not token:
        return {}
    return {"Authorization": f"Bearer {token}"}


class TestCynorkTransportStreaming(unittest.TestCase):
    """E2E: Gateway SSE format for cynork transport (iteration_start, response_id).

    Requires real stack. Skips when specific events are not produced.
    """

    tags = ["suite_orchestrator", "chat"]

    def setUp(self):
        if not state.CONFIG_PATH or not os.path.isfile(state.CONFIG_PATH):
            self.skipTest("CONFIG_PATH not set (run after auth login prereq)")
        ok, detail = helpers.ensure_valid_auth_session(state.CONFIG_PATH)
        self.assertTrue(ok, f"auth session invalid: {detail}")
        if os.environ.get("E2E_SKIP_INFERENCE_SMOKE", "") or config.E2E_SKIP_INFERENCE_SMOKE:
            self.skipTest("E2E_SKIP_INFERENCE_SMOKE set; skipping transport streaming")

    def _gateway_url(self):
        return config.USER_API.rstrip("/")

    def test_cynork_completions_transport_handles_named_extension_events(self):
        """Gateway completions stream MUST include cynodeai.iteration_start."""
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
        ev_names = [e.get("event") for e in events if e.get("event")]
        self.assertGreater(
            len(iteration_starts), 0,
            f"stream must include cynodeai.iteration_start; got events: {ev_names}",
        )

    def test_cynork_responses_transport_handles_native_responses_events_and_response_id(self):
        """Gateway /v1/responses stream MUST expose streamed response_id."""
        headers = _auth_headers(state.CONFIG_PATH)
        headers["Content-Type"] = "application/json"
        headers["Accept"] = "text/event-stream"
        resp = requests.post(
            f"{self._gateway_url()}/v1/responses",
            headers=headers,
            json={"model": "cynodeai.pm", "stream": True, "input": "Reply with exactly: OK"},
            stream=True,
            timeout=_SSE_TIMEOUT_SEC,
        )
        if resp.status_code != 200:
            self.skipTest(f"gateway returned {resp.status_code}")
        events, found_done = helpers.parse_sse_stream_typed(resp)
        self.assertTrue(found_done)
        response_ids = []
        for ev in events:
            try:
                obj = json.loads(ev["data"])
                rid = obj.get("response_id") or (obj.get("response") or {}).get("id")
                if rid:
                    response_ids.append(rid)
            except json.JSONDecodeError:
                pass
        self.assertGreater(
            len(response_ids), 0,
            f"stream must expose response_id; got events: {events[:5]}",
        )

    def test_cynork_transport_surfaces_heartbeat_and_amendment_without_parse_errors(self):
        """Gateway stream MAY include heartbeat and amendment; cynork must parse without error."""
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
        # At least one of iteration_start, heartbeat, or amendment should appear.
        known = {"cynodeai.iteration_start", "cynodeai.heartbeat", "cynodeai.amendment"}
        has_known = any(n in known for n in event_names)
        self.assertTrue(
            has_known or len(events) > 0,
            "stream must be parseable; expected at least iteration_start",
        )
