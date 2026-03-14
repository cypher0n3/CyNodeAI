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

# Timeout for SSE requests (seconds); generous for local inference.
_SSE_TIMEOUT_SEC = 120
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
            data = line[len("data: "):]
            if data == "[DONE]":
                found_done = True
                break
            events.append(data)
    return events, found_done


class TestSSEStreaming(unittest.TestCase):
    """E2E: SSE streaming via /v1/chat/completions and /v1/responses with stream=true."""

    tags = ["suite_orchestrator", "full_demo", "inference", "pma_inference"]

    def setUp(self):
        if not state.CONFIG_PATH or not os.path.isfile(state.CONFIG_PATH):
            self.skipTest("CONFIG_PATH not set (run after auth login prereq)")
        ok, detail = helpers.ensure_valid_auth_session(state.CONFIG_PATH)
        self.assertTrue(ok, f"auth session invalid before streaming tests: {detail}")
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

        # Validate chunk structure.
        full_content = ""
        for ev in events:
            try:
                chunk = json.loads(ev)
            except json.JSONDecodeError:
                self.fail(f"Non-JSON SSE event data: {ev!r}")
            self.assertEqual(chunk.get("object"), "chat.completion.chunk",
                             f"chunk.object wrong: {chunk!r}")
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

        full_content = ""
        for ev in events:
            try:
                chunk = json.loads(ev)
            except json.JSONDecodeError:
                self.fail(f"Non-JSON SSE event in /v1/responses: {ev!r}")
            choices = chunk.get("choices", [])
            if choices:
                full_content += choices[0].get("delta", {}).get("content", "")

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

        self.assertEqual(resp.status_code, 200,
                         f"Non-stream got {resp.status_code}: {resp.text[:200]}")
        ct = resp.headers.get("Content-Type", "")
        self.assertIn("application/json", ct, f"Expected JSON Content-Type, got {ct!r}")
        data = resp.json()
        choices = data.get("choices", [])
        self.assertGreater(len(choices), 0, "Non-stream response has no choices")
        content = choices[0].get("message", {}).get("content", "")
        self.assertTrue(content.strip(), "Non-stream response content is empty")
        self.assertNotIn("<think>", content, "Non-stream content must not contain <think> blocks")
