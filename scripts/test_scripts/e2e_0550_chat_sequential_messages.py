# E2E: Sequential multi-message chat via gateway POST /v1/chat/completions.
# Traces: REQ-USRGWY-0130; CYNAI.USRGWY.OpenAIChatApi; chat_threads_and_messages.
# Skip if E2E_SKIP_INFERENCE_SMOKE. Requires auth (e2e_0030).

import json
import os
import unittest

from scripts.test_scripts import config, helpers
import scripts.test_scripts.e2e_state as state

CHAT_TIMEOUT_SEC = 150
CHAT_URL = "/v1/chat/completions"


def _chat_request(messages, token, timeout=CHAT_TIMEOUT_SEC):
    """POST chat completions; return (ok, body)."""
    url = config.USER_API.rstrip("/") + CHAT_URL
    body = json.dumps({"model": "cynodeai.pm", "messages": messages})
    headers = {"Authorization": f"Bearer {token}"}
    return helpers.run_curl(
        "POST", url, data=body, headers=headers, timeout=timeout
    )


def _content_from_response(body):
    """Extract choices[0].message.content from OpenAI-format response."""
    data = helpers.parse_json_safe(body)
    if not data:
        return None
    choices = data.get("choices") or []
    if not choices:
        return None
    msg = (choices[0] or {}).get("message") or {}
    return msg.get("content")


class TestChatSequentialMessages(unittest.TestCase):
    """E2E: Send two turns (user, then user again with context); assert both replies."""

    tags = ["suite_orchestrator", "full_demo", "inference", "chat"]

    def test_chat_sequential_two_turns(self):
        """Two turns: first 'Say one word: first', then 'What word?'; assert both replies."""
        if os.environ.get("E2E_SKIP_INFERENCE_SMOKE", "") or config.E2E_SKIP_INFERENCE_SMOKE:
            self.skipTest("E2E_SKIP_INFERENCE_SMOKE set")
        if not state.CONFIG_PATH or not os.path.isfile(state.CONFIG_PATH):
            self.skipTest("CONFIG_PATH not set (run after auth login prereq)")
        auth_ok, detail = helpers.ensure_valid_auth_session(state.CONFIG_PATH)
        self.assertTrue(auth_ok, f"auth session invalid before sequential chat test: {detail}")
        token = helpers.read_token_from_config(state.CONFIG_PATH)
        self.assertIsNotNone(token, "no token in config (run login first)")
        messages = [{"role": "user", "content": "Say one word: first"}]
        ok, body = _chat_request(messages, token)
        if not ok:
            data = helpers.parse_json_safe(body)
            error = (data or {}).get("error") if isinstance(data, dict) else None
            if isinstance(error, dict):
                self.assertIn("message", error, f"chat error missing message: {body!r}")
                self.assertIn("type", error, f"chat error missing type: {body!r}")
                detail = f"{error.get('code', '')} {error.get('message', '')}".lower()
                if (
                    "orchestrator_inference_failed" in detail
                    or "completion failed" in detail
                    or "model_unavailable" in detail
                ):
                    self.skipTest("chat inference unavailable in current environment")
        self.assertTrue(ok, f"first chat request failed: {body}")
        first_content = _content_from_response(body)
        self.assertIsNotNone(first_content, f"no content in first response: {body}")
        first_content = (first_content or "").strip()
        self.assertGreater(len(first_content), 0, "first reply empty")
        # Extract meaningful words from the first response (skip stopwords).
        _stopwords = {"the", "a", "an", "is", "i", "it", "to", "of", "and", "in", "my"}
        first_words = {
            w.strip(".,!?\"':").lower()
            for w in first_content.split()
            if len(w.strip(".,!?\"':")) > 1
        } - _stopwords
        self.assertTrue(first_words, f"first reply has no meaningful words: {first_content!r}")
        messages.append({"role": "assistant", "content": first_content})
        messages.append({
            "role": "user",
            "content": "What word did you just say? Reply with that word only.",
        })
        ok2, body2 = _chat_request(messages, token)
        self.assertTrue(ok2, f"second chat request failed: {body2}")
        second_content = _content_from_response(body2)
        self.assertIsNotNone(second_content, f"no content in second response: {body2}")
        second_content = (second_content or "").strip()
        self.assertGreater(len(second_content), 0, "second reply empty")
        # Context retention: at least one word from the first reply must appear in the second.
        # This is a loose check because small models (qwen3.5:0.8b) may be verbose.
        second_words = {
            w.strip(".,!?\"':").lower()
            for w in second_content.split()
        }
        overlap = first_words & second_words
        msg = (
            f"sequential context not retained: first_words={first_words!r} "
            f"second={second_content!r}"
        )
        self.assertTrue(overlap, msg)
