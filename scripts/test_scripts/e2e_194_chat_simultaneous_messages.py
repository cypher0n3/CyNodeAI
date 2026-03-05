# E2E: Simultaneous chat requests - multiple one-shot chats in parallel.
# Traces: CYNAI.USRGWY.OpenAIChatApi.Reliability; gateway handles concurrent completions.
# Skip if E2E_SKIP_INFERENCE_SMOKE. Requires auth (e2e_020).

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

    def test_chat_simultaneous_three_requests(self):
        """Start three chat requests concurrently; each gets a reply or a clear error."""
        if os.environ.get("E2E_SKIP_INFERENCE_SMOKE", "") or config.E2E_SKIP_INFERENCE_SMOKE:
            self.skipTest("E2E_SKIP_INFERENCE_SMOKE set")
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
        self.assertGreaterEqual(
            successes, 1,
            f"at least one concurrent chat should succeed; failures: {failures}",
        )
        for ok, val in results:
            if ok:
                self.assertIsInstance(val, str, "content should be string")
                self.assertGreater(len(val), 0, "successful reply should be non-empty")
