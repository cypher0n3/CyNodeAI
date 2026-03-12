# E2E: Node manager telemetry ownership (worker_telemetry_api.md).
# Node-manager owns telemetry DB: records node_boot, shutdown log (source_name=node_manager).
# Traces: REQ-WORKER-0200, 0230; worker_telemetry_api.md (source_kind=service, source_name).

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
        self.assertGreater(len(data["events"]), 0, f"expected lifecycle events: {data!r}")
        messages = " ".join((evt.get("message") or "").lower() for evt in data["events"])
        self.assertTrue(
            any(word in messages for word in ("start", "boot", "shutdown")),
            f"expected startup/shutdown lifecycle signal in node_manager logs: {data!r}",
        )
        self.assertIn("truncated", data)
        self.assertIn("limited_by", data["truncated"])
        self.assertIn(data["truncated"]["limited_by"], {"none", "count", "bytes"})
        self.assertEqual(data["truncated"].get("max_bytes"), 1048576)
