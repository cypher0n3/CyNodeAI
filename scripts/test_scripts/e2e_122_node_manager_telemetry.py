# E2E: Node manager telemetry ownership (worker_telemetry_api.md).
# Node-manager owns telemetry DB: records node_boot, shutdown log (source_name=node_manager).
# Worker-api serves GET /v1/worker/telemetry/* and may show node_manager shutdown events.

import unittest

from scripts.test_scripts import config, helpers


class TestNodeManagerTelemetry(unittest.TestCase):
    """E2E: Telemetry logs for source_name=node_manager (node-manager lifecycle)."""

    tags = ["suite_worker_node", "full_demo", "worker"]

    def test_logs_node_manager_source_returns_200(self):
        """GET logs?source_kind=service&source_name=node_manager returns 200 and events list."""
        if not config.WORKER_API_BEARER_TOKEN:
            self.fail("WORKER_API_BEARER_TOKEN not set")
        headers = {"Authorization": f"Bearer {config.WORKER_API_BEARER_TOKEN}"}
        code, body = helpers.run_curl_with_status(
            "GET",
            (
                f"{config.WORKER_API}/v1/worker/telemetry/logs"
                "?source_kind=service&source_name=node_manager"
            ),
            headers=headers,
            timeout=10,
        )
        if not code:
            self.fail("worker API not reachable (start node: just setup-dev start)")
        self.assertEqual(code, 200, f"logs node_manager {code}: {body!r}")
        data = helpers.parse_json_safe(body)
        self.assertIsNotNone(data)
        self.assertIn("events", data)
        self.assertIsInstance(data["events"], list)
        self.assertIn("truncated", data)
        # When node-manager has exited, events may include "node manager shutdown" (source_name=node_manager).
