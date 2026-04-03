# E2E: NATS monitoring — JetStream and WebSocket listener (dev stack).
# Traces: REQ-ORCHES-0188, REQ-ORCHES-0190;
# docs/tech_specs/nats_messaging.md;
# docs/dev_docs/_plan_005a_nats+pma_session_tracking.md Task 10.

import json
import unittest
import urllib.error
import urllib.request

from scripts.test_scripts import config


class TestNatsConnectivity(unittest.TestCase):
    """Verify NATS monitoring /varz reports JetStream and a WebSocket listener."""

    tags = ["suite_orchestrator", "full_demo", "no_inference", "nats"]
    prereqs = ["gateway"]

    def test_nats_varz_jetstream_and_websocket(self):
        """GET /varz: JetStream enabled; websocket port present (Phase 1 transport)."""
        base = config.NATS_MONITOR_URL.rstrip("/")
        req = urllib.request.Request(base + "/varz")
        try:
            with urllib.request.urlopen(req, timeout=20) as resp:
                self.assertEqual(resp.status, 200, "NATS /varz")
                raw = resp.read().decode()
        except urllib.error.URLError as e:
            self.fail(f"NATS monitoring unreachable at {base}: {e}")

        data = json.loads(raw)
        js = data.get("jetstream")
        self.assertTrue(
            js not in (None, False, ""),
            f"expected jetstream enabled in varz: {list(data.keys())[:20]}",
        )
        ws = data.get("websocket")
        self.assertIsInstance(ws, dict, f"websocket block: {ws!r}")
        port = ws.get("port")
        self.assertTrue(
            port is not None and str(port).isdigit(),
            f"expected websocket port in varz: {ws!r}",
        )


if __name__ == "__main__":
    unittest.main()
