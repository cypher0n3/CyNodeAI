# E2E: REQ-WORKER-0174 — pod sandbox workload must not have direct egress to arbitrary hosts.
# Traces: REQ-WORKER-0174.

import time
import unittest

from scripts.test_scripts import config, helpers
import scripts.test_scripts.e2e_state as state


class TestSandboxNetworkIsolation(unittest.TestCase):
    """E2E: inference pod path uses isolated sandbox network; external TCP probe fails."""

    tags = ["suite_worker_node", "full_demo", "worker", "no_inference"]
    prereqs = ["gateway", "config", "auth", "node_register", "ollama"]

    def setUp(self):
        ok, detail = helpers.prepare_e2e_cynork_auth()
        self.assertTrue(ok, detail)

    def test_sandbox_cannot_reach_external_tcp(self):
        """Command job in inference pod: cannot TCP-connect to public resolver (8.8.8.8:53)."""
        if not getattr(config, "INFERENCE_PROXY_IMAGE", ""):
            self.skipTest("INFERENCE_PROXY_IMAGE not set")
        cmd = (
            "sh -c 'if nc -z -w 3 8.8.8.8 53 2>/dev/null; then echo REACHABLE; "
            "else echo BLOCKED; fi'"
        )
        ok, out, err = helpers.run_cynork(
            ["task", "create", "--command", cmd, "--use-inference", "-o", "json"],
            state.CONFIG_PATH,
        )
        self.assertTrue(ok, f"task create: {out} {err}")
        data = helpers.parse_json_safe(out)
        task_id = (data or {}).get("task_id")
        self.assertTrue(task_id, f"no task_id: {data}")
        ok_r, _, err_r = helpers.cynork_task_ready(task_id, state.CONFIG_PATH)
        self.assertTrue(ok_r, f"task ready: {err_r}")

        deadline = time.time() + 900
        status, result_data = None, None
        while time.time() < deadline:
            _, out2, _ = helpers.run_cynork(
                ["task", "result", task_id, "-o", "json"],
                state.CONFIG_PATH,
            )
            result_data = helpers.parse_json_safe(out2)
            status = (result_data or {}).get("status")
            if status in ("completed", "failed"):
                break
            time.sleep(3)
        self.assertEqual(status, "completed", f"task did not complete: {result_data}")
        job_result = helpers.get_sba_job_result(result_data)
        if isinstance(job_result, str):
            job_result = helpers.parse_json_safe(job_result)
        stdout = ""
        if isinstance(job_result, dict):
            stdout = str(job_result.get("stdout") or "")
        if not stdout.strip():
            stdout = str((result_data or {}).get("stdout") or "")
        self.assertIn(
            "BLOCKED",
            stdout,
            f"expected blocked external TCP (REQ-WORKER-0174); stdout={stdout!r}",
        )
        self.assertNotIn(
            "REACHABLE",
            stdout,
            f"sandbox must not reach external host directly; stdout={stdout!r}",
        )
