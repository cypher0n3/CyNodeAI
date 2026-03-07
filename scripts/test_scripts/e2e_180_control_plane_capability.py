# E2E parity: control-plane capability report. Requires state.node_jwt (e2e_06).

import json
import unittest
from datetime import datetime, timezone

from scripts.test_scripts import config, helpers
import scripts.test_scripts.e2e_state as state


class TestCapability(unittest.TestCase):
    """E2E: POST /v1/nodes/capability with Bearer node_jwt; assert success."""

    tags = ["suite_orchestrator", "full_demo"]

    def test_capability(self):
        """Report capability with state.NODE_JWT; assert 2xx response."""
        self.assertIsNotNone(state.NODE_JWT)
        payload = {
            "version": 1,
            "reported_at": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
            "node": {"node_slug": "test-e2e-node"},
            "platform": {"os": "linux", "arch": "amd64"},
            "compute": {"cpu_cores": 4, "ram_mb": 8192},
        }
        ok, body = helpers.run_curl(
            "POST", f"{config.CONTROL_PLANE_API}/v1/nodes/capability",
            data=json.dumps(payload),
            headers={"Authorization": f"Bearer {state.NODE_JWT}"},
        )
        self.assertTrue(ok, f"capability report failed: {body}")
