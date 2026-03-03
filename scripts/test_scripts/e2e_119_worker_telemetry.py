# E2E: Worker API telemetry node:info and node:stats. Requires WORKER_API and bearer.

import unittest

from scripts.test_scripts import config, helpers


class TestWorkerTelemetry(unittest.TestCase):
    """E2E: GET /v1/worker/telemetry/node:info and node:stats; assert 200 and JSON shape."""

    def test_node_info_returns_version_and_slug(self):
        """GET node:info with bearer returns 200, version and node_slug."""
        if not config.WORKER_API_BEARER_TOKEN:
            self.fail("WORKER_API_BEARER_TOKEN not set (E2E config defaults it; check config.py)")
        headers = {"Authorization": f"Bearer {config.WORKER_API_BEARER_TOKEN}"}
        code, body = helpers.run_curl_with_status(
            "GET",
            f"{config.WORKER_API}/v1/worker/telemetry/node:info",
            headers=headers,
        )
        if not code:
            self.fail("worker API not reachable (start node: just setup-dev start)")
        self.assertEqual(code, 200, f"node:info {code} {body}")
        data = helpers.parse_json_safe(body)
        self.assertIsNotNone(data)
        self.assertIn("version", data)
        self.assertIn("node_slug", data)

    def test_node_stats_returns_captured_at(self):
        """GET node:stats with bearer returns 200 and captured_at."""
        if not config.WORKER_API_BEARER_TOKEN:
            self.fail("WORKER_API_BEARER_TOKEN not set (E2E config defaults it; check config.py)")
        headers = {"Authorization": f"Bearer {config.WORKER_API_BEARER_TOKEN}"}
        code, body = helpers.run_curl_with_status(
            "GET",
            f"{config.WORKER_API}/v1/worker/telemetry/node:stats",
            headers=headers,
        )
        if not code:
            self.fail("worker API not reachable (start node: just setup-dev start)")
        self.assertEqual(code, 200, f"node:stats {code} {body}")
        data = helpers.parse_json_safe(body)
        self.assertIsNotNone(data)
        self.assertIn("version", data)
        self.assertIn("captured_at", data)
