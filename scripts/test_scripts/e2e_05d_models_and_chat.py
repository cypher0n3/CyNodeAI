# E2E parity: models list and one-shot chat. Skip chat if E2E_SKIP_INFERENCE_SMOKE.

import os
import time
import unittest

from scripts.test_scripts import config, helpers
import scripts.test_scripts.e2e_state as state


class TestModelsAndChat(unittest.TestCase):
    def test_models_and_chat(self):
        ok, out, _ = helpers.run_cynork(
            ["models", "list", "-o", "json"], state.CONFIG_PATH
        )
        self.assertTrue(ok, "models list failed")
        data = helpers.parse_json_safe(out)
        self.assertIsNotNone(data)
        self.assertEqual(data.get("object"), "list")
        self.assertGreaterEqual(len(data.get("data") or []), 1)
        if os.environ.get("E2E_SKIP_INFERENCE_SMOKE", "") or config.E2E_SKIP_INFERENCE_SMOKE:
            return
        chat_ok = False
        for attempt in range(1, 4):
            if attempt > 1:
                time.sleep(5)
            _, out, _ = helpers.run_cynork(
                ["chat", "--message", "Reply with exactly: OK", "--plain"],
                state.CONFIG_PATH,
            )
            out_stripped = (out or "").strip()
            bad = "error:" in out.lower() or "eof" in out.lower() or "502" in out
            if out_stripped and not bad:
                chat_ok = True
                break
        self.assertTrue(chat_ok, "one-shot chat failed after retries")
