# E2E: Simultaneous chat requests - multiple one-shot chats in parallel.
# Traces: REQ-ORCHES-0131, REQ-ORCHES-0132; CYNAI.USRGWY.OpenAIChatApi.Reliability;
# gateway handles concurrent completions.
# Skip if E2E_SKIP_INFERENCE_SMOKE. Requires auth (e2e_0030).

import json
import os
import concurrent.futures
import unittest

from scripts.test_scripts import config, helpers
import scripts.test_scripts.e2e_state as state

CHAT_TIMEOUT_SEC = 150
CHAT_URL = "/v1/chat/completions"
CONCURRENT_REQUESTS = 3


def _one_chat_request(token, message, timeout=CHAT_TIMEOUT_SEC):
    """Single POST /v1/chat/completions; return (success, content_or_error)."""
    url = config.USER_API.rstrip("/") + CHAT_URL
    body = json.dumps({
        "model": "cynodeai.pm",
        "messages": [{"role": "user", "content": message}],
    })
    headers = {"Authorization": f"Bearer {token}"}
    ok, resp_body = helpers.run_curl(
        "POST", url, data=body, headers=headers, timeout=timeout
    )
    if not ok:
        data = helpers.parse_json_safe(resp_body)
        if isinstance(data, dict) and isinstance(data.get("error"), dict):
            error = data.get("error") or {}
            if error.get("message"):
                return False, error.get("message")
        return False, resp_body or "non-2xx"
    data = helpers.parse_json_safe(resp_body)
    if not data:
        return False, "invalid json"
    choices = data.get("choices") or []
    if not choices:
        err = (data.get("error") or {}).get("message") or resp_body
        return False, err
    content = ((choices[0] or {}).get("message") or {}).get("content")
    return True, (content or "").strip()


class TestChatSimultaneousMessages(unittest.TestCase):
    """E2E: Run several one-shot chat requests in parallel; assert all complete or fail clearly."""

    tags = ["suite_orchestrator", "full_demo", "inference", "chat"]
    prereqs = ["gateway", "config", "auth", "ollama"]

    def test_chat_simultaneous_three_requests(self):
        """Start three chat requests concurrently; each gets a reply or a clear error."""
        if os.environ.get("E2E_SKIP_INFERENCE_SMOKE", "") or config.E2E_SKIP_INFERENCE_SMOKE:
            self.skipTest("E2E_SKIP_INFERENCE_SMOKE set")
        if not state.CONFIG_PATH or not os.path.isfile(state.CONFIG_PATH):
            self.skipTest("CONFIG_PATH not set (run after auth login prereq)")
        auth_ok, detail = helpers.ensure_valid_auth_session(state.CONFIG_PATH)
        self.assertTrue(auth_ok, f"auth session invalid before concurrent chat test: {detail}")
        token = helpers.read_token_from_config(state.CONFIG_PATH)
        self.assertIsNotNone(token, "no token in config (run login first)")
        messages = [
            "Reply with the number 1 only.",
            "Reply with the number 2 only.",
            "Reply with the number 3 only.",
        ]
        results = []
        with concurrent.futures.ThreadPoolExecutor(max_workers=CONCURRENT_REQUESTS) as ex:
            futures = [
                ex.submit(_one_chat_request, token, msg)
                for msg in messages
            ]
            for f in concurrent.futures.as_completed(futures):
                results.append(f.result())
        successes = sum(1 for ok, _ in results if ok)
        failures = [(ok, val) for ok, val in results if not ok]
        if not successes and failures:
            details = " ".join(str(val).lower() for _, val in failures)
            if (
                "orchestrator_inference_failed" in details
                or "completion failed" in details
                or "model_unavailable" in details
            ):
                self.skipTest("chat inference unavailable in current environment")
        self.assertGreaterEqual(
            successes,
            2,
            f"expected >=2 concurrent successes; failures: {failures}",
        )
        for ok, val in results:
            if ok:
                self.assertIsInstance(val, str, "content should be string")
                self.assertGreater(len(val), 0, "successful reply should be non-empty")
