# E2E: Gateway healthz vs readyz (process alive vs ready to accept work).
# Traces: REQ-ORCHES-0120; CYNAI.ORCHES.Rule.HealthEndpoints.

import unittest

from scripts.test_scripts import config, helpers


class TestGatewayHealthReadyz(unittest.TestCase):
    """E2E: GET /healthz returns 200; GET /readyz returns 200 or 503 with reason."""

    tags = ["suite_orchestrator", "full_demo", "no_inference", "gateway"]
    prereqs = ["gateway"]

    def test_healthz_ok(self):
        """Assert user-gateway healthz returns 200 (process alive)."""
        code, _ = helpers.run_curl_with_status(
            "GET", config.USER_API + "/healthz", timeout=10
        )
        self.assertEqual(code, 200, "healthz should return 200 when gateway is up")

    def test_readyz_200_or_503(self):
        """Assert readyz returns 200 (ready) or 503 (not ready); no other status."""
        code, body = helpers.run_curl_with_status(
            "GET", config.USER_API + "/readyz", timeout=10
        )
        self.assertIn(
            code, (200, 503),
            f"readyz should return 200 or 503, got {code}: {body!r}",
        )
