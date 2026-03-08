# E2E: Worker API telemetry node:info and node:stats. Requires WORKER_API and bearer.
# Traces: REQ-WORKER-0200, 0230, 0231, 0232, 0234; worker_telemetry_api.md.

import unittest

from scripts.test_scripts import config, helpers


class TestWorkerTelemetry(unittest.TestCase):
    """E2E: GET /v1/worker/telemetry/node:info and node:stats; assert 200 and JSON shape."""

    tags = ["suite_worker_node", "full_demo", "worker"]

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

    def test_containers_returns_list(self):
        """GET containers with bearer returns 200 and containers array."""
        if not config.WORKER_API_BEARER_TOKEN:
            self.fail("WORKER_API_BEARER_TOKEN not set")
        headers = {"Authorization": f"Bearer {config.WORKER_API_BEARER_TOKEN}"}
        code, body = helpers.run_curl_with_status(
            "GET",
            f"{config.WORKER_API}/v1/worker/telemetry/containers",
            headers=headers,
        )
        if not code:
            self.fail("worker API not reachable")
        self.assertEqual(code, 200, f"containers {code} {body}")
        data = helpers.parse_json_safe(body)
        self.assertIsNotNone(data)
        self.assertIn("containers", data)
        self.assertIsInstance(data["containers"], list)

    def test_logs_returns_entries(self):
        """GET logs with bearer returns 200 and events list."""
        if not config.WORKER_API_BEARER_TOKEN:
            self.fail("WORKER_API_BEARER_TOKEN not set")
        headers = {"Authorization": f"Bearer {config.WORKER_API_BEARER_TOKEN}"}
        code, body = helpers.run_curl_with_status(
            "GET",
            (
                f"{config.WORKER_API}/v1/worker/telemetry/logs"
                "?source_kind=service&source_name=node-manager"
            ),
            headers=headers,
        )
        if not code:
            self.fail("worker API not reachable")
        self.assertEqual(code, 200, f"logs {code} {body}")
        data = helpers.parse_json_safe(body)
        self.assertIsNotNone(data)
        self.assertIn("events", data)
        self.assertIsInstance(data["events"], list)

    def test_containers_response_has_version(self):
        """GET containers response has version 1 and containers array per spec."""
        if not config.WORKER_API_BEARER_TOKEN:
            self.fail("WORKER_API_BEARER_TOKEN not set")
        headers = {"Authorization": f"Bearer {config.WORKER_API_BEARER_TOKEN}"}
        code, body = helpers.run_curl_with_status(
            "GET",
            f"{config.WORKER_API}/v1/worker/telemetry/containers",
            headers=headers,
        )
        if not code:
            self.fail("worker API not reachable")
        self.assertEqual(code, 200, f"containers {code} {body}")
        data = helpers.parse_json_safe(body)
        self.assertIsNotNone(data)
        self.assertEqual(data.get("version"), 1, "response must have version 1")
        self.assertIn("containers", data)
        self.assertIsInstance(data["containers"], list)

    def test_logs_response_has_truncated_metadata(self):
        """GET logs response has events and truncated.limited_by, truncated.max_bytes per spec."""
        if not config.WORKER_API_BEARER_TOKEN:
            self.fail("WORKER_API_BEARER_TOKEN not set")
        headers = {"Authorization": f"Bearer {config.WORKER_API_BEARER_TOKEN}"}
        code, body = helpers.run_curl_with_status(
            "GET",
            f"{config.WORKER_API}/v1/worker/telemetry/logs"
            "?source_kind=service&source_name=worker_api",
            headers=headers,
        )
        if not code:
            self.fail("worker API not reachable")
        self.assertEqual(code, 200, f"logs {code} {body}")
        data = helpers.parse_json_safe(body)
        self.assertIsNotNone(data)
        self.assertIn("events", data)
        self.assertIsInstance(data["events"], list)
        self.assertIn("truncated", data)
        self.assertIn("limited_by", data["truncated"])
        self.assertIn("max_bytes", data["truncated"])
        self.assertEqual(data["truncated"]["max_bytes"], 1048576)

    def test_get_container_not_found_returns_404(self):
        """GET containers/{id} for non-existent id returns 404."""
        if not config.WORKER_API_BEARER_TOKEN:
            self.fail("WORKER_API_BEARER_TOKEN not set")
        headers = {"Authorization": f"Bearer {config.WORKER_API_BEARER_TOKEN}"}
        code, _ = helpers.run_curl_with_status(
            "GET",
            f"{config.WORKER_API}/v1/worker/telemetry/containers/nonexistent-container-id-404",
            headers=headers,
        )
        if not code:
            self.fail("worker API not reachable")
        self.assertEqual(code, 404, "GET container by unknown id must return 404")

    def test_telemetry_responses_do_not_contain_bearer_token(self):
        """Telemetry response bodies must not contain the bearer token (no credential leak)."""
        if not config.WORKER_API_BEARER_TOKEN:
            self.fail("WORKER_API_BEARER_TOKEN not set")
        token = config.WORKER_API_BEARER_TOKEN
        headers = {"Authorization": f"Bearer {token}"}
        for path in ("/v1/worker/telemetry/node:info", "/v1/worker/telemetry/node:stats"):
            code, body = helpers.run_curl_with_status(
                "GET", f"{config.WORKER_API}{path}", headers=headers
            )
            if not code:
                self.fail("worker API not reachable")
            self.assertEqual(code, 200, f"{path} {code} {body}")
            self.assertNotIn(
                token,
                body,
                f"{path} response must not contain bearer token",
            )
