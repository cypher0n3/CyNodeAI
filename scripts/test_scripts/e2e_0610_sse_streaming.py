# E2E: SSE streaming for /v1/chat/completions and /v1/responses with stream=true.
# Verifies CYNAI.USRGWY.OpenAIChatApi.Streaming: events arrive, [DONE] sentinel present,
# no <think> blocks in visible content, content is non-empty.
# Traces: REQ-USRGWY-0149, REQ-USRGWY-0150, CYNAI.USRGWY.OpenAIChatApi.Streaming.

import json
import os
import time
import unittest

import requests

from scripts.test_scripts import config, helpers
import scripts.test_scripts.e2e_state as state

# Whole-request HTTP read timeout for SSE (first byte + stream); tunable via
# E2E_SSE_REQUEST_TIMEOUT.
_SSE_TIMEOUT_SEC = int(config.E2E_SSE_REQUEST_TIMEOUT)
# Prompt that yields a short deterministic response.
_SHORT_PROMPT = "Reply with exactly: OK"


def _auth_headers(config_path):
    """Return Authorization header dict for the session token."""
    token = helpers.read_token_from_config(config_path)
    if not token:
        return {}
    return {"Authorization": f"Bearer {token}"}


def _parse_sse_stream(response):
    """Parse an SSE response stream; return list of (data_str) for non-[DONE] data lines."""
    events = []
    found_done = False
    for line in response.iter_lines(decode_unicode=True):
        if not line:
            continue
        if line.startswith("data: "):
            data = line[len("data: "):].strip("\r")
            if data == "[DONE]":
                found_done = True
                break
            events.append(data)
    return events, found_done


_INFERENCE_ERROR_CODES = frozenset([
    "orchestrator_inference_failed",
    "model_unavailable",
    "completion failed",
])


def _skip_if_inference_error(test_case, events, endpoint):
    """Skip test_case if any SSE event signals that inference is unavailable."""
    for ev in events:
        try:
            obj = json.loads(ev)
        except json.JSONDecodeError:
            continue
        err = obj.get("error", {})
        if isinstance(err, dict):
            code = (err.get("code") or "").lower()
            msg = (err.get("message") or "").lower()
            if any(k in code or k in msg for k in _INFERENCE_ERROR_CODES):
                test_case.skipTest(
                    f"{endpoint} inference unavailable in current environment: {ev!r}"
                )


class TestSSEStreaming(unittest.TestCase):
    """E2E: SSE streaming via /v1/chat/completions and /v1/responses with stream=true."""

    tags = ["suite_orchestrator", "full_demo", "inference", "pma_inference", "streaming"]
    prereqs = ["gateway", "config", "auth", "ollama"]

    def setUp(self):
        ok, detail = helpers.prepare_e2e_cynork_auth()
        self.assertTrue(ok, detail)
        if os.environ.get("E2E_SKIP_INFERENCE_SMOKE", "") or config.E2E_SKIP_INFERENCE_SMOKE:
            self.skipTest("E2E_SKIP_INFERENCE_SMOKE set; skipping streaming tests")

    def _gateway_url(self):
        """Return the user gateway base URL."""
        return config.USER_API.rstrip("/")

    def test_chat_completions_stream_returns_sse(self):
        """POST /v1/chat/completions with stream=true returns SSE with chat.completion.chunk events
        and a terminal [DONE] event per CYNAI.USRGWY.OpenAIChatApi.Streaming."""
        headers = _auth_headers(state.CONFIG_PATH)
        headers["Content-Type"] = "application/json"
        headers["Accept"] = "text/event-stream"
        payload = {
            "model": "cynodeai.pm",
            "stream": True,
            "messages": [{"role": "user", "content": _SHORT_PROMPT}],
        }
        last_exc = None
        for attempt in range(1, 4):
            if attempt > 1:
                time.sleep(5)
            try:
                resp = requests.post(
                    f"{self._gateway_url()}/v1/chat/completions",
                    headers=headers,
                    json=payload,
                    stream=True,
                    timeout=_SSE_TIMEOUT_SEC,
                )
                break
            except requests.RequestException as e:
                last_exc = e
        else:
            self.fail(f"SSE request failed after retries: {last_exc}")

        self.assertEqual(
            resp.status_code, 200,
            f"Expected 200 for stream=true, got {resp.status_code}: {resp.text[:200]}",
        )
        ct = resp.headers.get("Content-Type", "")
        self.assertIn("text/event-stream", ct, f"Content-Type should be SSE, got {ct!r}")

        events, found_done = _parse_sse_stream(resp)
        self.assertTrue(found_done, "SSE stream did not end with [DONE]")
        self.assertGreater(len(events), 0, "No SSE event data lines before [DONE]")

        # Check for inference unavailability in the first event before validating structure.
        _skip_if_inference_error(self, events, "/v1/chat/completions")

        # PMA may emit extension payloads on the same data: lines (e.g. iteration markers)
        # before chat.completion.chunk; only validate OpenAI-shaped chunks.
        chunk_payloads = []
        for ev in events:
            try:
                chunk = json.loads(ev)
            except json.JSONDecodeError:
                self.fail(f"Non-JSON SSE event data: {ev!r}")
            if chunk.get("object") != "chat.completion.chunk":
                continue
            chunk_payloads.append(chunk)

        self.assertGreater(
            len(chunk_payloads),
            0,
            "Expected at least one chat.completion.chunk in the SSE stream; "
            f"got only non-chunk payloads: {events!r}",
        )

        full_content = ""
        for chunk in chunk_payloads:
            choices = chunk.get("choices", [])
            self.assertGreater(len(choices), 0, f"chunk has no choices: {chunk!r}")
            delta = choices[0].get("delta", {})
            full_content += delta.get("content", "")

        # Content must be non-empty and must not contain raw <think> blocks.
        self.assertTrue(
            full_content.strip(),
            f"Accumulated stream content is empty; events={events!r}",
        )
        self.assertNotIn(
            "<think>", full_content,
            f"Visible stream content MUST NOT contain <think> blocks: {full_content!r}",
        )
        self.assertNotIn(
            "</think>", full_content,
            f"Visible stream content MUST NOT contain </think> blocks: {full_content!r}",
        )

    def test_responses_stream_returns_sse(self):
        """POST /v1/responses with stream=true returns SSE events and a [DONE] sentinel
        per CYNAI.USRGWY.OpenAIChatApi.Streaming."""
        headers = _auth_headers(state.CONFIG_PATH)
        headers["Content-Type"] = "application/json"
        headers["Accept"] = "text/event-stream"
        payload = {
            "model": "cynodeai.pm",
            "stream": True,
            "input": _SHORT_PROMPT,
        }
        last_exc = None
        for attempt in range(1, 4):
            if attempt > 1:
                time.sleep(5)
            try:
                resp = requests.post(
                    f"{self._gateway_url()}/v1/responses",
                    headers=headers,
                    json=payload,
                    stream=True,
                    timeout=_SSE_TIMEOUT_SEC,
                )
                break
            except requests.RequestException as e:
                last_exc = e
        else:
            self.fail(f"SSE /v1/responses request failed after retries: {last_exc}")

        self.assertEqual(
            resp.status_code, 200,
            f"Expected 200 for stream=true, got {resp.status_code}: {resp.text[:200]}",
        )
        ct = resp.headers.get("Content-Type", "")
        self.assertIn("text/event-stream", ct, f"Content-Type should be SSE, got {ct!r}")

        events, found_done = _parse_sse_stream(resp)
        self.assertTrue(found_done, "SSE /v1/responses stream did not end with [DONE]")
        self.assertGreater(len(events), 0, "No SSE events before [DONE] in /v1/responses")

        # Check for inference unavailability in the first event before validating structure.
        _skip_if_inference_error(self, events, "/v1/responses")

        full_content = ""
        for ev in events:
            try:
                chunk = json.loads(ev)
            except json.JSONDecodeError:
                self.fail(f"Non-JSON SSE event in /v1/responses: {ev!r}")
            choices = chunk.get("choices", [])
            if choices:
                full_content += choices[0].get("delta", {}).get("content", "")
                continue
            # Native responses stream uses top-level string deltas (e.g. {"delta":"OK"}), not chat.completion.chunk.
            d = chunk.get("delta")
            if isinstance(d, str):
                full_content += d
            elif isinstance(d, dict):
                full_content += d.get("content", "") or ""

        self.assertTrue(
            full_content.strip(),
            f"Accumulated /v1/responses stream content is empty; events={events!r}",
        )
        self.assertNotIn(
            "<think>", full_content,
            "Visible /v1/responses content MUST NOT contain <think> blocks",
        )

    def test_chat_completions_non_stream_still_works(self):
        """Regression: POST /v1/chat/completions without stream=true still returns JSON."""
        headers = _auth_headers(state.CONFIG_PATH)
        headers["Content-Type"] = "application/json"
        payload = {
            "model": "cynodeai.pm",
            "messages": [{"role": "user", "content": _SHORT_PROMPT}],
        }
        last_exc = None
        for attempt in range(1, 4):
            if attempt > 1:
                time.sleep(5)
            try:
                resp = requests.post(
                    f"{self._gateway_url()}/v1/chat/completions",
                    headers=headers,
                    json=payload,
                    timeout=_SSE_TIMEOUT_SEC,
                )
                break
            except requests.RequestException as e:
                last_exc = e
        else:
            self.fail(f"Non-stream request failed after retries: {last_exc}")

        # 502 / 503 means inference backend is not available; skip gracefully.
        if resp.status_code in (502, 503):
            self.skipTest(
                f"/v1/chat/completions (non-stream) inference unavailable "
                f"(HTTP {resp.status_code}): {resp.text[:200]}"
            )
        self.assertEqual(resp.status_code, 200,
                         f"Non-stream got {resp.status_code}: {resp.text[:200]}")
        ct = resp.headers.get("Content-Type", "")
        self.assertIn("application/json", ct, f"Expected JSON Content-Type, got {ct!r}")
        data = resp.json()
        # Check for inference unavailability in the response body before asserting content.
        if "error" in data:
            err = data["error"] if isinstance(data["error"], dict) else {}
            code = (err.get("code") or "").lower()
            msg = (err.get("message") or "").lower()
            if any(k in code or k in msg for k in _INFERENCE_ERROR_CODES):
                self.skipTest(
                    f"/v1/chat/completions (non-stream) inference unavailable: {data!r}"
                )
        choices = data.get("choices", [])
        self.assertGreater(len(choices), 0, "Non-stream response has no choices")
        content = choices[0].get("message", {}).get("content", "")
        self.assertTrue(content.strip(), "Non-stream response content is empty")
        self.assertNotIn("<think>", content, "Non-stream content must not contain <think> blocks")

    def test_chat_completions_stream_exposes_named_cynodeai_extension_events(self):
        """Chat completions stream MUST expose named cynodeai.* SSE events per
        CYNAI.USRGWY.OpenAIChatApi.StreamingPerEndpointSSEFormat (e.g. thinking_delta, tool_call).
        """
        headers = _auth_headers(state.CONFIG_PATH)
        headers["Content-Type"] = "application/json"
        headers["Accept"] = "text/event-stream"
        payload = {
            "model": "cynodeai.pm",
            "stream": True,
            "messages": [{"role": "user", "content": _SHORT_PROMPT}],
        }
        resp = requests.post(
            f"{self._gateway_url()}/v1/chat/completions",
            headers=headers,
            json=payload,
            stream=True,
            timeout=_SSE_TIMEOUT_SEC,
        )
        self.assertEqual(resp.status_code, 200, f"Expected 200, got {resp.status_code}")
        typed_events, found_done = helpers.parse_sse_stream_typed(resp)
        _skip_if_inference_error(
            self, [e["data"] for e in typed_events if e["data"]], "/v1/chat/completions"
        )
        self.assertTrue(found_done, "Stream must end with [DONE]")
        cynodeai_events = [
            e for e in typed_events
            if e.get("event") and e["event"].startswith("cynodeai.")
        ]
        ev_list = [e.get("event") for e in typed_events]
        self.assertGreater(
            len(cynodeai_events), 0,
            "Stream must expose at least one named cynodeai.* extension event "
            f"(e.g. cynodeai.heartbeat or cynodeai.thinking_delta); got events: {ev_list}",
        )

    def test_chat_completions_stream_relays_thinking_tool_and_iteration_events(self):
        """Chat completions stream MUST relay iteration_start (done). When PMA sends thinking/tool
        events, gateway MUST relay cynodeai.thinking_delta / cynodeai.tool_* (Task 3 Red: fails
        until relay implemented)."""
        headers = _auth_headers(state.CONFIG_PATH)
        headers["Content-Type"] = "application/json"
        headers["Accept"] = "text/event-stream"
        payload = {
            "model": "cynodeai.pm",
            "stream": True,
            "messages": [{"role": "user", "content": _SHORT_PROMPT}],
        }
        resp = requests.post(
            f"{self._gateway_url()}/v1/chat/completions",
            headers=headers,
            json=payload,
            stream=True,
            timeout=_SSE_TIMEOUT_SEC,
        )
        self.assertEqual(resp.status_code, 200, f"Expected 200, got {resp.status_code}")
        typed_events, found_done = helpers.parse_sse_stream_typed(resp)
        _skip_if_inference_error(
            self, [e["data"] for e in typed_events if e["data"]], "/v1/chat/completions"
        )
        self.assertTrue(found_done, "Stream must end with [DONE]")
        event_names = [e.get("event") for e in typed_events if e.get("event")]
        iteration_starts = [e for e in event_names if e == "cynodeai.iteration_start"]
        self.assertGreater(
            len(iteration_starts), 0,
            f"Stream must relay at least one cynodeai.iteration_start; got events: {event_names}",
        )

    def test_responses_stream_uses_native_responses_events_and_exposes_streamed_response_id(self):
        """Responses stream MUST use native responses event shape and expose streamed response_id
        per CYNAI.USRGWY.OpenAIChatApi.StreamingPerEndpointSSEFormat."""
        headers = _auth_headers(state.CONFIG_PATH)
        headers["Content-Type"] = "application/json"
        headers["Accept"] = "text/event-stream"
        payload = {
            "model": "cynodeai.pm",
            "stream": True,
            "input": _SHORT_PROMPT,
        }
        resp = requests.post(
            f"{self._gateway_url()}/v1/responses",
            headers=headers,
            json=payload,
            stream=True,
            timeout=_SSE_TIMEOUT_SEC,
        )
        self.assertEqual(resp.status_code, 200, f"Expected 200, got {resp.status_code}")
        typed_events, found_done = helpers.parse_sse_stream_typed(resp)
        _skip_if_inference_error(
            self, [e["data"] for e in typed_events if e["data"]], "/v1/responses"
        )
        self.assertTrue(found_done, "Stream must terminate with completion/done")
        response_ids = []
        for ev in typed_events:
            try:
                obj = json.loads(ev["data"])
            except json.JSONDecodeError:
                continue
            rid = obj.get("response_id") or (obj.get("response") or {}).get("id")
            if rid:
                response_ids.append(rid)
        ev_preview = typed_events[:5]
        self.assertGreater(
            len(response_ids), 0,
            f"Stream must expose response_id; got: {ev_preview}",
        )
