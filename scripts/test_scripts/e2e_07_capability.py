# E2E parity: control-plane capability report. Requires state.node_jwt (e2e_06).

import json
import unittest
from datetime import datetime, timezone

from scripts.test_scripts import config, helpers
import scripts.test_scripts.e2e_state as state


class TestCapability(unittest.TestCase):
    def test_capability(self):
        self.assertIsNotNone(state.node_jwt)
        payload = {
            "version": 1,
            "reported_at": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
            "node": {"node_slug": "test-e2e-node"},
            "platform": {"os": "linux", "arch": "amd64"},
            "compute": {"cpu_cores": 4, "ram_mb": 8192},
        }
        ok, body = helpers.run_curl(
            "POST", "%s/v1/nodes/capability" % config.CONTROL_PLANE_API,
            data=json.dumps(payload),
            headers={"Authorization": "Bearer %s" % state.node_jwt},
        )
        self.assertTrue(ok, "capability report failed: %s" % body)
