# E2E: PMA chat with project context (OpenAI-Project header).
# Verifies gateway accepts chat with project-id and returns completion (PMA handoff).

import os
import unittest

from scripts.test_scripts import config, helpers
import scripts.test_scripts.e2e_state as state


class TestPmaChatContext(unittest.TestCase):
    """E2E: one-shot chat with --project-id (OpenAI-Project header); PMA handoff path."""

    def test_chat_with_project_context(self):
        """Send chat with --project-id; assert success when inference is available."""
        if os.environ.get("E2E_SKIP_INFERENCE_SMOKE", "") or config.E2E_SKIP_INFERENCE_SMOKE:
            self.skipTest("inference smoke skipped")
        ok, out, _ = helpers.run_cynork(
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
        out_stripped = (out or "").strip()
        bad = "error:" in (out or "").lower() or "eof" in (out or "").lower()
        self.assertTrue(ok or (out_stripped and not bad), "chat with project-id failed")
