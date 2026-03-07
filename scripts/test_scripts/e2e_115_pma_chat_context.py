# E2E: PMA chat with project context (OpenAI-Project header).
# Verifies gateway accepts chat with project-id and returns completion (PMA handoff).

import os
import time
import unittest

from scripts.test_scripts import config, helpers
import scripts.test_scripts.e2e_state as state


class TestPmaChatContext(unittest.TestCase):
    """E2E: one-shot chat with --project-id (OpenAI-Project header); PMA handoff path."""

    tags = ["suite_e2e", "suite_orchestrator"]

    def test_chat_with_project_context(self):
        """Send chat with --project-id; assert success when inference is available."""
        if os.environ.get("E2E_SKIP_INFERENCE_SMOKE", "") or config.E2E_SKIP_INFERENCE_SMOKE:
            self.skipTest("inference smoke skipped")
        chat_ok = False
        last_out = ""
        last_err = ""
        for attempt in range(1, 4):
            if attempt > 1:
                time.sleep(5)
            ok, out, err = helpers.run_cynork(
                [
                    "chat",
                    "--message",
                    "Reply with OK",
                    "--project-id",
                    "default",
                    "--plain",
                ],
                state.CONFIG_PATH,
            )
            last_out, last_err = out or "", err or ""
            merged = (last_out + "\n" + last_err).lower()
            out_stripped = last_out.strip()
            bad = "error:" in merged or "eof" in merged or "502" in merged
            unavailable = (
                "model_unavailable" in merged
                or "completion failed" in merged
                or "pm agent is not available" in merged
            )
            if unavailable:
                self.skipTest("project chat unavailable in current environment")
            # Success: exit 0, or non-empty response without error/502
            if ok or (out_stripped and not bad):
                chat_ok = True
                break
        self.assertTrue(
            chat_ok,
            f"chat with project-id failed (ok=False or empty/bad response): "
            f"stdout={last_out!r} stderr={last_err!r}",
        )
