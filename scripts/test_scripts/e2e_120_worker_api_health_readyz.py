# E2E: Worker API healthz (process alive) and readyz (ready to accept jobs).
# Traces: REQ-WORKER-0140, REQ-WORKER-0142; worker readiness for inference path.

import unittest

from scripts.test_scripts import config, helpers


class TestWorkerApiHealthReadyz(unittest.TestCase):
    """E2E: GET worker API /healthz and /readyz; assert expected status."""

    tags = ["suite_worker_node"]

    def test_worker_healthz_returns_200(self):
        """Worker API healthz returns 200 when process is up."""
        code, body = helpers.run_curl_with_status(
            "GET", f"{config.WORKER_API}/healthz", timeout=10
        )
        if not code:
            self.fail("worker API not reachable (start node: just setup-dev start)")
        self.assertEqual(code, 200, f"healthz should return 200, got {code}: {body!r}")

    def test_worker_readyz_returns_200_or_503(self):
        """Worker API readyz returns 200 (ready) or 503 (not ready); no other status."""
        code, body = helpers.run_curl_with_status(
            "GET", f"{config.WORKER_API}/readyz", timeout=10
        )
        if not code:
            self.fail("worker API not reachable (start node: just setup-dev start)")
        self.assertIn(
            code, (200, 503),
            f"readyz should return 200 or 503, got {code}: {body!r}",
        )
