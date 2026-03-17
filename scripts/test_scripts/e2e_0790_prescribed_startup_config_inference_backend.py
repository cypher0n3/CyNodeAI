# E2E: prescribed startup - node config includes inference_backend when inference-capable,
# not existing_service. Registers with inference; GET config; asserts inference_backend.
# Traces: REQ-ORCHES-0149 (node config inference backend); REQ-ORCHES-0113, 0114.

import json
import unittest
from datetime import datetime, timezone

from scripts.test_scripts import config, helpers


class TestPrescribedStartupConfigInferenceBackend(unittest.TestCase):
    """E2E: register inference-capable node; GET config must include inference_backend."""

    tags = ["suite_orchestrator", "full_demo", "inference"]

    def test_config_includes_inference_backend_when_node_inference_capable_not_existing(self):
        """Register inference-capable node, GET nodes/config; assert inference_backend.enabled."""
        payload = {
            "psk": config.NODE_PSK,
            "capability": {
                "version": 1,
                "reported_at": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
                "node": {"node_slug": "e2e-prescribed-node"},
                "platform": {"os": "linux", "arch": "amd64"},
                "compute": {"cpu_cores": 4, "ram_mb": 8192},
                "inference": {
                    "supported": True,
                    "existing_service": False,
                    "running": False,
                },
                "worker_api": {"base_url": "http://localhost:12090"},
            },
        }
        ok, body = helpers.run_curl(
            "POST", f"{config.CONTROL_PLANE_API}/v1/nodes/register",
            data=json.dumps(payload),
        )
        self.assertTrue(ok, f"register failed: {body}")
        data = helpers.parse_json_safe(body)
        jwt = (data or {}).get("auth", {}).get("node_jwt")
        self.assertIsNotNone(jwt, "no node_jwt in response")

        ok, config_body = helpers.run_curl(
            "GET", f"{config.CONTROL_PLANE_API}/v1/nodes/config",
            headers={"Authorization": f"Bearer {jwt}"},
        )
        self.assertTrue(ok, f"GET config failed: {config_body}")
        config_data = helpers.parse_json_safe(config_body)
        self.assertIsNotNone(config_data, "config response not JSON")
        backend = (config_data or {}).get("inference_backend")
        self.assertIsNotNone(
            backend, "config should include inference_backend when node is inference-capable"
        )
        self.assertTrue(
            backend.get("enabled", False),
            "inference_backend.enabled should be true",
        )
        self.assertNotIn(
            config.NODE_PSK,
            config_body,
            "config response must not contain PSK",
        )
        self.assertNotIn(
            jwt,
            config_body,
            "config response must not contain node JWT",
        )
