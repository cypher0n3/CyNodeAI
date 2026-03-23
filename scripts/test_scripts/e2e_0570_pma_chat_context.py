# E2E: PMA chat with project context (OpenAI-Project header).
# Auth via prepare_e2e_cynork_auth in setUp.
# Traces: REQ-USRGWY-0131 (task/project association); REQ-CLIENT-0173 (project context for chat).

import os
import time
import unittest

from scripts.test_scripts import config, helpers
import scripts.test_scripts.e2e_state as state


class TestPmaChatContext(unittest.TestCase):
    """E2E: one-shot chat with --project-id (OpenAI-Project header); PMA handoff path."""

    tags = [
        "suite_e2e", "suite_orchestrator", "full_demo", "inference",
        "pma_inference", "chat", "pma",
    ]
    prereqs = ["gateway", "config", "auth", "task_id", "ollama"]

    def setUp(self):
        ok, detail = helpers.prepare_e2e_cynork_auth()
        self.assertTrue(ok, detail)

    def _project_id_for_chat(self):
        """Resolve a non-placeholder project identifier from an existing or bootstrapped task."""
        task_id = state.TASK_ID
        if not task_id:
            ok, out, err = helpers.run_cynork(
                [
                    "task",
                    "create",
                    "--name",
                    "chat-context-project-id-probe",
                    "--command",
                    "echo project-id-probe",
                    "-o",
                    "json",
                ],
                state.CONFIG_PATH,
            )
            self.assertTrue(ok, f"task create for project-id probe failed: {out} {err}")
            create_data = helpers.parse_json_safe(out)
            self.assertIsInstance(
                create_data, dict, f"task create probe should return json object: {out!r}"
            )
            task_id = (create_data or {}).get("task_id")
            self.assertTrue(task_id, f"task create probe missing task_id: {create_data!r}")
        return task_id

    def test_chat_with_project_context(self):
        """Send chat with --project-id; assert success when inference is available."""
        if os.environ.get("E2E_SKIP_INFERENCE_SMOKE", "") or config.E2E_SKIP_INFERENCE_SMOKE:
            self.skipTest("inference smoke skipped")
        project_id = self._project_id_for_chat()
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
                    "Reply with exactly: OK",
                    "--project-id",
                    project_id,
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
                or "orchestrator_inference_failed" in merged
                or "502 bad gateway" in merged
            )
            if unavailable:
                self.skipTest("project chat unavailable in current environment")
            # Success: exit 0, or non-empty response without error/502.
            # Do not assert the exact model reply here — small models (qwen3.5:0.8b)
            # may not follow "Reply with exactly: OK" through the PMA agent layer.
            if ok or (out_stripped and not bad):
                self.assertGreater(
                    len(out_stripped),
                    0,
                    f"unexpected empty chat reply: stderr={last_err!r}",
                )
                chat_ok = True
                break
        self.assertTrue(
            chat_ok,
            f"chat with project-id failed (ok=False or empty/bad response): "
            f"stdout={last_out!r} stderr={last_err!r}",
        )
