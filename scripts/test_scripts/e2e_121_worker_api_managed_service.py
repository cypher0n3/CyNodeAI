# E2E: Worker API when run as a managed service (container started by node-manager).
# When NODE_MANAGER_WORKER_API_IMAGE is set, node-manager starts worker-api as container.
# Traces: REQ-WORKER-0160, 0161; worker_api.md (managed service, observed state).

import os
import subprocess
import unittest

from scripts.test_scripts import config, helpers


class TestWorkerApiManagedService(unittest.TestCase):
    """E2E: Worker API as managed service; healthz and node:info when worker-api is up."""

    tags = ["suite_worker_node", "full_demo", "worker"]

    def test_worker_api_healthz_when_running(self):
        """Worker API (binary or container) healthz returns 200."""
        code, body = helpers.run_curl_with_status(
            "GET", f"{config.WORKER_API}/healthz", timeout=10
        )
        if not code:
            self.fail("worker API not reachable (start node: just setup-dev start)")
        self.assertEqual(code, 200, f"healthz {code}: {body!r}")

    def test_worker_api_node_info_when_running(self):
        """Worker API (binary or container) node:info returns 200, version and node_slug."""
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

    def test_worker_api_container_exists_when_image_configured(self):
        """When NODE_MANAGER_WORKER_API_IMAGE is set, cynodeai-worker-api container exists."""
        image_env = os.environ.get("NODE_MANAGER_WORKER_API_IMAGE", "").strip()
        if not image_env:
            self.skipTest("NODE_MANAGER_WORKER_API_IMAGE not set (use worker-api as container)")
        runtime = os.environ.get("CONTAINER_RUNTIME", "podman")
        try:
            r = subprocess.run(
                [runtime, "ps", "-a", "--format", "{{.Names}}"],
                capture_output=True,
                text=True,
                timeout=10,
                check=False,
            )
        except FileNotFoundError:
            self.skipTest(f"{runtime} not found")
        if r.returncode:
            self.skipTest(f"{runtime} ps failed: {r.stderr!r}")
        if "cynodeai-worker-api" not in (r.stdout or ""):
            self.fail(
                "cynodeai-worker-api container not found (node-manager should start it when "
                "NODE_MANAGER_WORKER_API_IMAGE is set)"
            )
