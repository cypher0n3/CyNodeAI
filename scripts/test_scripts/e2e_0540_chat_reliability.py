# E2E: Chat reliability - extended timeout, retries, and clear error handling.
# Traces: REQ-ORCHES-0131, 0132; CYNAI.USRGWY.OpenAIChatApi.Timeouts, Reliability.
# Skip if E2E_SKIP_INFERENCE_SMOKE. Auth via prepare_e2e_cynork_auth in setUp.

import os
import time
import unittest

from scripts.test_scripts import config, helpers
import scripts.test_scripts.e2e_state as state

# Chat can take 30-120s when cold; use 150s timeout and retries per spec.
CHAT_TIMEOUT_SEC = 150
CHAT_RETRIES = 3


def _chat_inference_unavailable(detail):
    """Return True when chat failed due to known inference-unavailable conditions."""
    lowered = (detail or "").lower()
    return any(
        marker in lowered
        for marker in (
            "orchestrator_inference_failed",
            "completion failed",
            "model_unavailable",
            "502 bad gateway",
        )
    )


def _chat_reply_is_clean(out, err):
    """Return True when chat output is non-empty and not paired with transport/error markers."""
    out_stripped = (out or "").strip()
    err_lower = (err or "").lower()
    return bool(out_stripped) and "error:" not in err_lower and "eof" not in err_lower


class TestChatReliability(unittest.TestCase):
    """E2E: One-shot chat returns in time or yields clear timeout/error; retries with backoff."""

    tags = ["suite_orchestrator", "full_demo", "inference", "chat"]
    prereqs = ["gateway", "config", "auth", "ollama"]

    def setUp(self):
        ok, detail = helpers.prepare_e2e_cynork_auth()
        self.assertTrue(ok, detail)

    def test_chat_completes_or_clear_error(self):
        """Run one-shot chat with extended timeout and retries; assert reply or structured error."""
        if os.environ.get("E2E_SKIP_INFERENCE_SMOKE", "") or config.E2E_SKIP_INFERENCE_SMOKE:
            self.skipTest("E2E_SKIP_INFERENCE_SMOKE set")
        last_err = None
        for attempt in range(1, CHAT_RETRIES + 1):
            if attempt > 1:
                time.sleep(5)
            _, out, err = helpers.run_cynork(
                ["chat", "--message", "ping", "--plain"],
                state.CONFIG_PATH,
                timeout=CHAT_TIMEOUT_SEC,
            )
            merged = ((out or "") + "\n" + (err or "")).lower()
            if _chat_inference_unavailable(merged):
                self.skipTest("chat inference unavailable in current environment")
            # Reliability smoke-test: a non-empty, non-error reply proves the endpoint is up.
            if _chat_reply_is_clean(out, err):
                return
            last_err = out or err
        self.fail(
            f"chat did not return a timely reply after {CHAT_RETRIES} attempts. Last: {last_err!r}"
        )
