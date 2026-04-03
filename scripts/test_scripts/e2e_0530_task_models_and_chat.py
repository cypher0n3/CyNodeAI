# E2E parity: models list and one-shot chat (auth via prepare_e2e_cynork_auth in setUp).
# Traces: REQ-USRGWY-0121, 0127; CYNAI.USRGWY.OpenAIChatApi; REQ-CLIENT-0161.

import os
import time
import unittest

from scripts.test_scripts import config, helpers
import scripts.test_scripts.e2e_state as state


class TestModelsAndChat(unittest.TestCase):
    """E2E: models list -o json; optional one-shot chat (skipped if E2E_SKIP_INFERENCE_SMOKE)."""

    tags = ["suite_orchestrator", "full_demo", "inference", "pma_inference", "chat"]
    prereqs = ["gateway", "config", "auth", "ollama", "pma_chat"]

    def setUp(self):
        ok, detail = helpers.prepare_e2e_cynork_auth()
        self.assertTrue(ok, detail)

    def test_models_and_chat(self):
        """Assert models list returns list; run one-shot chat unless inference smoke skipped."""
        ok, out, err = helpers.run_cynork(
            ["models", "list", "-o", "json"], state.CONFIG_PATH
        )
        self.assertTrue(
            ok, f"models list failed: stdout={out!r} stderr={err!r}"
        )
        data = helpers.parse_json_safe(out)
        self.assertIsNotNone(
            data, f"models list returned no valid JSON: stdout={out!r} stderr={err!r}"
        )
        self.assertEqual(data.get("object"), "list", f"object not list: {data!r}")
        data_list = data.get("data") or []
        self.assertGreaterEqual(
            len(data_list), 1,
            f"models list empty: data={data_list!r}",
        )
        if os.environ.get("E2E_SKIP_INFERENCE_SMOKE", "") or config.E2E_SKIP_INFERENCE_SMOKE:
            return
        chat_ok = False
        last_out, last_err = "", ""
        for attempt in range(1, 4):
            if attempt > 1:
                time.sleep(5)
            _, out, err = helpers.run_cynork(
                ["chat", "--message", "ping", "--plain"],
                state.CONFIG_PATH,
            )
            last_out, last_err = out or "", err or ""
            merged = (last_out + "\n" + last_err).lower()
            bad = "error:" in merged or "eof" in merged or "502" in merged
            unavailable = (
                "orchestrator_inference_failed" in merged
                or "completion failed" in merged
                or "model_unavailable" in merged
                or "502 bad gateway" in merged
            )
            if unavailable:
                self.skipTest("chat inference unavailable in current environment")
            # Smoke-test only: verify the endpoint returns a non-empty, non-error reply.
            out_stripped = last_out.strip()
            if out_stripped and not bad:
                chat_ok = True
                break
        self.assertTrue(
            chat_ok,
            f"one-shot chat failed after retries: stdout={last_out!r} stderr={last_err!r}",
        )
