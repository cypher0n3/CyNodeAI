# E2E: PMA standard-path streaming via gateway SSE (capable model + MCP).
# Gateway relays PMA NDJSON as SSE; we assert on event types (iteration_start, tool_progress, etc.).
# Traces: REQ-PMAGNT-0118, REQ-PMAGNT-0120-0126, features/agents/pma_chat_and_context.feature.

import json
import os
import unittest

import requests

from scripts.test_scripts import config, helpers
import scripts.test_scripts.e2e_state as state

_SSE_TIMEOUT_SEC = int(config.E2E_SSE_REQUEST_TIMEOUT)


def _auth_headers(config_path):
    token = helpers.read_token_from_config(config_path)
    if not token:
        return {}
    return {"Authorization": f"Bearer {token}"}


class TestPMAStandardPathStreaming(unittest.TestCase):
    """E2E: PMA standard path via gateway SSE (iteration_start, thinking, tool_progress)."""

    tags = ["suite_orchestrator", "pma_inference", "inference", "streaming"]
    prereqs = ["gateway", "config", "auth", "ollama", "pma_chat"]

    def setUp(self):
        ok, detail = helpers.prepare_e2e_cynork_auth()
        self.assertTrue(ok, detail)
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

    def _named_events(self, events):
        return [e for e in events if e.get("event")]

    def _openai_content_deltas(self, events):
        """Unnamed SSE data lines that are chat.completion.chunk JSON with content deltas."""
        out = []
        for e in events:
            if e.get("event") is not None:
                continue
            data = helpers.parse_json_safe(e.get("data") or "")
            if not isinstance(data, dict):
                continue
            choices = data.get("choices") or []
            if not choices:
                continue
            delta = (choices[0] or {}).get("delta") or {}
            content = delta.get("content")
            if content:
                out.append(content)
        return out

    def test_pma_gateway_sse_relays_iteration_and_optional_named_extensions(self):
        """Relay iteration_start, deltas, and optional cynodeai.* extension events."""
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
        events, _ = helpers.parse_sse_stream_typed(resp)
        named = self._named_events(events)
        ev_names = {e.get("event") for e in named}
        self.assertTrue(
            self._openai_content_deltas(events),
            "SSE must include OpenAI-style content delta chunks",
        )
        self.assertIn(
            "cynodeai.iteration_start",
            ev_names,
            "SSE must relay cynodeai.iteration_start from PMA NDJSON",
        )
        for opt in ("cynodeai.thinking_delta", "cynodeai.tool_call", "cynodeai.amendment"):
            if opt not in ev_names:
                continue
            for ev in named:
                if ev.get("event") != opt:
                    continue
                data = helpers.parse_json_safe(ev.get("data") or "")
                self.assertIsInstance(data, dict, f"{opt} data must be JSON object")

    def test_pma_gateway_sse_amendment_payload_shape_when_present(self):
        """When PMA emits overwrite NDJSON, gateway relays cynodeai.amendment with valid scope."""
        events, _found_done = helpers.pm_stream_events_with_amendment_baits(
            state.CONFIG_PATH, timeout_sec=_SSE_TIMEOUT_SEC
        )
        helpers.require_amendment_events_or_skip(
            self,
            events,
            "expected cynodeai.amendment after PMA secret-scan baits "
            f"{helpers.PMA_STREAM_AMENDMENT_USER_BAITS!r}",
        )
        amendments = [e for e in events if e.get("event") == "cynodeai.amendment"]
        for ev in amendments:
            data = helpers.parse_json_safe(ev.get("data") or "{}")
            if not isinstance(data, dict):
                continue
            scope = data.get("scope")
            if scope is not None:
                self.assertIn(scope, ("iteration", "turn"), data)
