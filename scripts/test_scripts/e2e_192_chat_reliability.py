# E2E: Chat reliability - extended timeout, retries, and clear error handling.
# Traces: REQ-ORCHES-0131, 0132; CYNAI.USRGWY.OpenAIChatApi.Timeouts, Reliability.
# Skip if E2E_SKIP_INFERENCE_SMOKE. Requires auth (e2e_020).

import os
import time
import unittest

from scripts.test_scripts import config, helpers
import scripts.test_scripts.e2e_state as state

# Chat can take 30-120s when cold; use 150s timeout and retries per spec.
CHAT_TIMEOUT_SEC = 150
CHAT_RETRIES = 3


class TestChatReliability(unittest.TestCase):
    """E2E: One-shot chat returns in time or yields clear timeout/error; retries with backoff."""

    tags = ["suite_orchestrator", "full_demo", "inference", "chat"]

    def test_chat_completes_or_clear_error(self):
        """Run one-shot chat with extended timeout and retries; assert reply or structured error."""
        if os.environ.get("E2E_SKIP_INFERENCE_SMOKE", "") or config.E2E_SKIP_INFERENCE_SMOKE:
            self.skipTest("E2E_SKIP_INFERENCE_SMOKE set")
        last_err = None
        for attempt in range(1, CHAT_RETRIES + 1):
            if attempt > 1:
                time.sleep(5)
            _, out, err = helpers.run_cynork(
                ["chat", "--message", "Reply with exactly: OK", "--plain"],
                state.CONFIG_PATH,
                timeout=CHAT_TIMEOUT_SEC,
            )
            out_stripped = (out or "").strip()
            err_lower = (err or "").lower()
            if out_stripped and "error:" not in err_lower and "eof" not in err_lower:
                self.assertTrue(len(out_stripped) > 0, "expected non-empty reply")
                return
            last_err = out or err
        self.fail(
            f"chat did not return a timely reply after {CHAT_RETRIES} attempts. Last: {last_err!r}"
        )
