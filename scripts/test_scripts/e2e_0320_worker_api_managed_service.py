# E2E: Worker API (part of node-manager binary) healthz and node:info.
# Traces: REQ-WORKER-0140, 0142 (health/ready); worker_api.md, worker_telemetry_api.md.

import unittest

from scripts.test_scripts import config, helpers


class TestWorkerApiManagedService(unittest.TestCase):
    """E2E: Worker API when node-manager is running; healthz and node:info."""

    tags = ["suite_worker_node", "full_demo", "worker", "no_inference"]
    prereqs = []

    def test_worker_api_healthz_when_running(self):
        """Worker API (node-manager) healthz returns 200."""
        code, body = helpers.run_curl_with_status(
            "GET", f"{config.WORKER_API}/healthz", timeout=10
        )
        if not code:
            self.fail("worker API not reachable (start node: just setup-dev start)")
        self.assertEqual(code, 200, f"healthz {code}: {body!r}")

    def test_worker_api_node_info_when_running(self):
        """Worker API (node-manager) node:info returns 200, version and node_slug."""
        if not config.WORKER_API_BEARER_TOKEN:
            self.fail("WORKER_API_BEARER_TOKEN not set")
        headers = {"Authorization": f"Bearer {config.WORKER_API_BEARER_TOKEN}"}
        code, body = helpers.run_curl_with_status(
            "GET",
            f"{config.WORKER_API}/v1/worker/telemetry/node:info",
            headers=headers,
            timeout=10,
        )
        if not code:
            self.fail("worker API not reachable")
        self.assertEqual(code, 200, f"node:info {code}: {body!r}")
        data = helpers.parse_json_safe(body)
        self.assertIsNotNone(data)
        self.assertIn("version", data)
        self.assertIn("node_slug", data)
