# E2E: SBA task with --use-inference --use-sba and prompt "Reply back with the current time."
# Validates the fix for empty stdout / placeholder "sba-run" only: task must complete with
# a user-facing reply (non-empty job stdout or SBA result that indicates inference was used).
# Traces: REQ-SBAGNT-0103, 0109; user-facing SBA reply with inference.

import os
import time
import unittest

from scripts.test_scripts import config, helpers
import scripts.test_scripts.e2e_state as state


def _job_result_parsed(result_data):
    """Return parsed first job result dict, or None."""
    # Newer task-result shape may expose the worker job payload under top-level stdout.
    # Reuse shared helper to normalize both jobs[0].result and stdout-based payloads.
    job = helpers.get_sba_job_result(result_data or {})
    if isinstance(job, dict):
        return job
    raw = helpers.jq_get(result_data, "jobs", 0, "result")
    if isinstance(raw, str):
        return helpers.parse_json_safe(raw)
    if isinstance(raw, dict):
        return raw
    return None


def _has_user_facing_reply(job):
    """True if job result shows a real reply (fix for empty stdout / sba-run placeholder)."""
    if not job:
        return False
    stdout = (job.get("stdout") or "").strip()
    if stdout:
        return True
    sba = job.get("sba_result") or {}
    steps = sba.get("steps") or []
    if len(steps) > 1:
        return True
    if len(steps) == 1:
        out = (steps[0].get("output") or "").strip().replace("\n", "")
        if out and out != "sba-run":
            return True
    if sba.get("final_reply") or sba.get("reply"):
        return True
    if sba.get("final_answer"):
        return True
    return False


class TestSbaInferenceReply(unittest.TestCase):
    """E2E: SBA + inference prompt produces user-facing reply (not empty / sba-run only)."""

    tags = ["suite_agents", "full_demo", "inference", "sba_inference"]

    def test_sba_inference_reply_current_time(self):
        """Create SBA inference task and assert user-facing reply."""
        if os.environ.get("E2E_SKIP_INFERENCE_SMOKE", "") or config.E2E_SKIP_INFERENCE_SMOKE:
            self.skipTest("E2E_SKIP_INFERENCE_SMOKE set")
        if not state.CONFIG_PATH or not os.path.isfile(state.CONFIG_PATH):
            self.skipTest("CONFIG_PATH not set (run after auth login prereq)")
        auth_ok, detail = helpers.ensure_valid_auth_session(state.CONFIG_PATH)
        self.assertTrue(auth_ok, f"auth session invalid before SBA inference reply test: {detail}")
        _, out, _ = helpers.run_cynork(
            [
                "task", "create", "-p", "Reply back with the current time.",
                "--use-inference", "--use-sba", "-o", "json",
            ],
            state.CONFIG_PATH,
        )
        data = helpers.parse_json_safe(out)
        task_id = (data or {}).get("task_id")
        self.assertIsNotNone(task_id, "SBA inference reply task create failed")
        status = None
        result_data = None
        for _ in range(60):
            time.sleep(5)
            _, out, _ = helpers.run_cynork(
                ["task", "result", task_id, "-o", "json"],
                state.CONFIG_PATH,
            )
            result_data = helpers.parse_json_safe(out)
            status = (result_data or {}).get("status")
            if status in ("completed", "failed"):
                break
        self.assertIn(status, ("completed", "failed"), "task did not reach terminal status")
        if status != "completed":
            self.fail(
                "SBA inference reply task failed (per spec user-facing reply requires inference): "
                f"status={status!r} result={result_data}"
            )
        job = _job_result_parsed(result_data)
        self.assertTrue(
            _has_user_facing_reply(job),
            (
                "SBA inference produced no user-facing reply "
                "(empty stdout and only sba-run placeholder)"
            ),
        )
