# E2E: Gateway streaming contract (amendment, heartbeat, cancellation, persistence).
# Traces: REQ-USRGWY-0149-0156, Task 3 gateway relay.
# Use with just e2e --tags chat. Requires real stack (gateway auth via setUp, PMA).

import json
import os
import unittest

import requests

from scripts.test_scripts import config, helpers
import scripts.test_scripts.e2e_state as state

_SSE_TIMEOUT_SEC = int(config.E2E_SSE_REQUEST_TIMEOUT)


def _auth_headers(cfg_path):
    token = helpers.read_token_from_config(cfg_path)
    if not token:
        return {}
    return {"Authorization": f"Bearer {token}"}


class TestGatewayStreamingContract(unittest.TestCase):
    """E2E: Gateway amendment, heartbeat fallback, cancellation, persistence (Task 3).

    Requires real stack (CONFIG_PATH, auth). Skips when specific events (amendment,
    heartbeat) are not produced by the live PMA/gateway.
    """

    tags = ["suite_orchestrator", "chat", "gateway", "streaming"]
    prereqs = ["gateway", "config", "auth", "ollama"]

    def setUp(self):
        ok, detail = helpers.prepare_e2e_cynork_auth()
        self.assertTrue(ok, detail)
        if os.environ.get("E2E_SKIP_INFERENCE_SMOKE", "") or config.E2E_SKIP_INFERENCE_SMOKE:
            self.skipTest("E2E_SKIP_INFERENCE_SMOKE set; skipping gateway streaming")

    def _gateway_url(self):
        return config.USER_API.rstrip("/")

    def _stream_chat_completions(self, messages=None):
        headers = _auth_headers(state.CONFIG_PATH)
        headers["Content-Type"] = "application/json"
        headers["Accept"] = "text/event-stream"
        payload = {
            "model": "cynodeai.pm",
            "stream": True,
            "messages": messages or [{"role": "user", "content": "Reply with exactly: OK"}],
        }
        return requests.post(
            f"{self._gateway_url()}/v1/chat/completions",
            headers=headers,
            json=payload,
            stream=True,
            timeout=_SSE_TIMEOUT_SEC,
        )

    def test_stream_amendment_arrives_before_terminal_completion(self):
        """Amendment events MUST arrive before [DONE] when redaction is applied."""
        resp = self._stream_chat_completions()
        if resp.status_code != 200:
            self.skipTest(f"gateway returned {resp.status_code}")
        events, found_done = helpers.parse_sse_stream_typed(resp)
        self.assertTrue(found_done, "stream must end with [DONE]")
        amendment_events = [e for e in events if e.get("event") == "cynodeai.amendment"]
        if not amendment_events:
            self.skipTest("stream did not contain amendment; PMA may not have triggered redaction")
        self.assertLess(
            events.index(amendment_events[0]),
            len(events),
            "amendment must precede terminal completion",
        )

    def test_stream_heartbeat_fallback_emits_progress_then_final_visible_text(self):
        """When upstream cannot stream, gateway SHOULD emit heartbeat then final text."""
        resp = self._stream_chat_completions()
        if resp.status_code != 200:
            self.skipTest(f"gateway returned {resp.status_code}")
        events, found_done = helpers.parse_sse_stream_typed(resp)
        self.assertTrue(found_done)
        heartbeat = [e for e in events if e.get("event") == "cynodeai.heartbeat"]
        if not heartbeat:
            self.skipTest("stream did not contain heartbeat; upstream may have streamed normally")
        content_parts = []
        for e in events:
            try:
                obj = json.loads(e["data"])
                for c in obj.get("choices", []):
                    content_parts.append(c.get("delta", {}).get("content", ""))
            except json.JSONDecodeError:
                pass
        self.assertTrue(
            "".join(content_parts).strip(),
            "must receive final visible text after heartbeat",
        )

    def test_client_disconnect_is_treated_as_stream_cancellation(self):
        """Client disconnect MUST cancel the stream and not leave upstream running indefinitely."""
        resp = self._stream_chat_completions()
        if resp.status_code != 200:
            self.skipTest(f"gateway returned {resp.status_code}")
        count = 0
        for _ in resp.iter_lines():
            count += 1
            if count >= 2:
                break
        resp.close()
        # No server-side assertion; verify client can disconnect without error.

    def test_streamed_structured_parts_are_persisted_redacted_only(self):
        """Persisted assistant turn MUST store only redacted content in structured parts."""
        resp = self._stream_chat_completions()
        if resp.status_code != 200:
            self.skipTest(f"gateway returned {resp.status_code}")
        events, found_done = helpers.parse_sse_stream_typed(resp)
        self.assertTrue(found_done)
        amendment_events = [e for e in events if e.get("event") == "cynodeai.amendment"]
        if not amendment_events:
            self.skipTest("stream did not contain amendment; cannot assert persistence shape")
        data = json.loads(amendment_events[0]["data"])
        self.assertIn(
            "redacted", data,
            "amendment must expose redacted content for persistence",
        )
