# E2E parity: control-plane node register. Sets state.node_jwt.

import json
import unittest
from datetime import datetime, timezone

from scripts.test_scripts import config, helpers
import scripts.test_scripts.e2e_state as state


class TestNodeRegister(unittest.TestCase):
    """E2E: POST /v1/nodes/register with PSK and capability; store node_jwt in state."""

    def test_node_register(self):
        """Register node via control-plane API; assert node_jwt in response; set state.NODE_JWT."""
        payload = {
            "psk": config.NODE_PSK,
            "capability": {
                "version": 1,
                "reported_at": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
                "node": {"node_slug": "test-e2e-node"},
                "platform": {"os": "linux", "arch": "amd64"},
                "compute": {"cpu_cores": 4, "ram_mb": 8192},
            },
        }
        ok, body = helpers.run_curl(
            "POST", config.CONTROL_PLANE_API + "/v1/nodes/register",
            data=json.dumps(payload),
        )
        self.assertTrue(ok, "node register failed")
        data = helpers.parse_json_safe(body)
        jwt = (data or {}).get("auth", {}).get("node_jwt")
        self.assertTrue(jwt, "no node_jwt in response")
        state.NODE_JWT = jwt
