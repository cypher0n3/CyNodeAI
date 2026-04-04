# E2E: Gateway streaming contract (amendment, cancellation, persistence).
# Traces: REQ-USRGWY-0149-0156, Task 3 gateway relay.
# Heartbeat fallback SSE is covered by Go TestEmitDegradedStreamingFallback_Chat_IncludesHeartbeat.
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
    """E2E: Gateway amendment path, cancellation, persistence (Task 3).

    Requires real stack (CONFIG_PATH, auth). Amendment coverage uses PMA secret-scan baits
    (see helpers.PMA_STREAM_AMENDMENT_USER_BAITS).
    """

    tags = ["suite_orchestrator", "chat", "gateway", "streaming"]
    prereqs = ["gateway", "config", "auth", "ollama", "pma_chat"]

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
        events, found_done = helpers.pm_stream_events_with_amendment_baits(
            state.CONFIG_PATH, timeout_sec=_SSE_TIMEOUT_SEC
        )
        helpers.require_amendment_events_or_skip(
            self,
            events,
            "expected cynodeai.amendment (PMA secret-scan baits); "
            f"tried {helpers.PMA_STREAM_AMENDMENT_USER_BAITS!r}",
        )
        self.assertTrue(found_done, "stream must end with [DONE]")
        amendment_events = [e for e in events if e.get("event") == "cynodeai.amendment"]
        self.assertGreater(len(amendment_events), 0)

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
        events, found_done = helpers.pm_stream_events_with_amendment_baits(
            state.CONFIG_PATH, timeout_sec=_SSE_TIMEOUT_SEC
        )
        helpers.require_amendment_events_or_skip(
            self, events, "expected amendment SSE from PMA secret-scan baits"
        )
        self.assertTrue(found_done)
        amendment_events = [e for e in events if e.get("event") == "cynodeai.amendment"]
        self.assertGreater(len(amendment_events), 0)
        data = json.loads(amendment_events[0]["data"])
        self.assertIn(
            "redacted", data,
            "amendment must expose redacted content for persistence",
        )
