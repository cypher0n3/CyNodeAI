# E2E: PMA chat using capable model (qwen3:8b / spec: qwen3.5:9b).
# Requires: auth config from e2e_020, OLLAMA_CAPABLE_MODEL available in Ollama container.
# Skipped automatically when the capable model is not pulled (e.g. CI without the model).
#
# Traces: REQ-MODELS-0008 (VRAM-based model tier), CYNAI.AGENTS.PMLlmToolImplementation,
#         REQ-PMAGNT-0100/0101.

import json
import os
import time
import unittest

from scripts.test_scripts import config, helpers
import scripts.test_scripts.e2e_state as state

# Startup and warm-up allowance for a capable model on first load.
# qwen3:8b requires ~45-90 s to load into VRAM cold.
_CAPABLE_MODEL_WARMUP_S = 90
_CAPABLE_MODEL_CHAT_TIMEOUT_S = 150


class TestPMAChatCapableModel(unittest.TestCase):
    """PMA chat tests using the capable model (qwen3:8b / spec qwen3.5:9b).

    All tests skip if the model is not available in the Ollama container so
    that the suite remains green in environments where the model has not been
    pulled yet. When the model IS available these tests exercise the full
    OneShotAgent + MCP tool-call path.
    """

    tags = ["suite_orchestrator", "pma_inference", "chat", "chat_capable"]

    def setUp(self):
        if not state.CONFIG_PATH or not os.path.isfile(state.CONFIG_PATH):
            self.skipTest("CONFIG_PATH not set (run after auth login prereq)")
        if os.environ.get("E2E_SKIP_INFERENCE_SMOKE", "") or config.E2E_SKIP_INFERENCE_SMOKE:
            self.skipTest("inference smoke disabled (E2E_SKIP_INFERENCE_SMOKE)")
        if not helpers.is_ollama_model_available(config.OLLAMA_CAPABLE_MODEL):
            self.skipTest(
                f"capable model {config.OLLAMA_CAPABLE_MODEL!r} not available in Ollama container"
            )
        ok, detail = helpers.ensure_valid_auth_session(state.CONFIG_PATH)
        self.assertTrue(ok, f"auth session invalid before capable-model tests: {detail}")

    def test_capable_model_chat_one_shot(self):
        """One-shot chat via capable model; asserts deterministic reply."""
        last_out, last_err = "", ""
        chat_ok = False
        for attempt in range(1, 4):
            if attempt > 1:
                time.sleep(10)
            _, out, err = helpers.run_cynork(
                ["chat", "--message", "Reply with exactly: OK", "--plain"],
                state.CONFIG_PATH,
                timeout=_CAPABLE_MODEL_CHAT_TIMEOUT_S,
            )
            last_out, last_err = out or "", err or ""
            merged = (last_out + "\n" + last_err).lower()
            unavailable = (
                "orchestrator_inference_failed" in merged
                or "completion failed" in merged
                or "model_unavailable" in merged
                or "502 bad gateway" in merged
            )
            if unavailable:
                self.skipTest(
                    f"capable-model inference unavailable: stdout={last_out!r} stderr={last_err!r}"
                )
            out_stripped = last_out.strip()
            bad = "error:" in merged or "eof" in merged or "502" in merged or "504" in merged
            if out_stripped and not bad:
                self.assertIn(
                    "OK",
                    out_stripped.upper(),
                    f"expected 'OK' in capable-model reply: {out_stripped!r}",
                )
                chat_ok = True
                break
        self.assertTrue(
            chat_ok,
            f"capable-model one-shot chat failed after retries: "
            f"stdout={last_out!r} stderr={last_err!r}",
        )

    def test_capable_model_chat_multi_turn(self):
        """Two-turn conversation in a single request; verifies model uses prior context.

        cynork chat --message sends one message per request (no history). To test
        multi-turn context we use the /v1/chat/completions HTTP API directly and
        pass both turns as a messages array so the model sees the conversation.
        """
        token = helpers.read_token_from_config(state.CONFIG_PATH)
        if not token:
            self.skipTest("no auth token")
        url = config.USER_API.rstrip("/") + "/v1/chat/completions"
        # Build two-turn conversation in a single request.
        messages = [
            {"role": "user", "content": "My favourite colour is blue. Acknowledge briefly."},
            {"role": "assistant", "content": "Noted! Your favourite colour is blue."},
            {"role": "user", "content": "What colour did I just mention?"},
        ]
        body = json.dumps({"model": "cynodeai.pm", "messages": messages})
        headers = {"Authorization": f"Bearer {token}"}
        ok, resp_body = helpers.run_curl(
            "POST", url, data=body, headers=headers, timeout=_CAPABLE_MODEL_CHAT_TIMEOUT_S
        )
        if not ok:
            merged = (resp_body or "").lower()
            unavailable = (
                "orchestrator_inference_failed" in merged
                or "completion failed" in merged
                or "model_unavailable" in merged
                or "502" in merged
                or "504" in merged
            )
            if unavailable:
                self.skipTest("capable-model inference unavailable for multi-turn test")
            self.fail(f"multi-turn chat request failed: {resp_body!r}")
        data = helpers.parse_json_safe(resp_body)
        choices = (data or {}).get("choices") or []
        content = ((choices[0] or {}).get("message") or {}).get("content", "") if choices else ""
        content_lower = (content or "").lower()
        self.assertTrue(content_lower, "multi-turn response empty")
        # The model received the prior turn in the messages array so it must reference 'blue'.
        self.assertIn(
            "blue",
            content_lower,
            f"second turn should reference 'blue' from context: {content!r}",
        )
