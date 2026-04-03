# E2E: control-plane POST /internal/dev/reset-pma-session-state (parity with
# scripts/setup_dev_reset_session_state.sh on setup-dev stop).
# Traces: orchestrator/internal/handlers/dev_reset_pma.go; REQ-ORCHES-0190.

import http
import unittest

from scripts.test_scripts import config, helpers


class TestControlPlaneDevResetPMASessionState(unittest.TestCase):
    """Dev-only internal route: Bearer NODE_REGISTRATION_PSK (same as node register)."""

    tags = ["suite_orchestrator", "full_demo", "no_inference", "control_plane"]
    prereqs = []

    def _reset_url(self):
        return config.CONTROL_PLANE_API.rstrip("/") + "/internal/dev/reset-pma-session-state"

    def test_dev_reset_rejects_wrong_psk(self):
        """Wrong PSK yields 401 when the route is enabled (non-empty registration PSK)."""
        url = self._reset_url()
        code, _ = helpers.run_curl_with_status(
            "POST",
            url,
            headers={"Authorization": "Bearer not-the-real-node-psk"},
            timeout=30,
        )
        if code == http.HTTPStatus.NOT_FOUND:
            self.skipTest(
                "dev reset disabled (empty NODE_REGISTRATION_PSK on control-plane)"
            )
        self.assertEqual(
            code,
            http.HTTPStatus.UNAUTHORIZED,
            "wrong bearer must be rejected",
        )

    def test_dev_reset_pma_session_state_returns_204(self):
        """Valid PSK returns 204 No Content (teardown bindings + invalidate refresh sessions)."""
        psk = (config.NODE_PSK or "").strip()
        if not psk:
            self.skipTest("NODE_PSK empty; cannot authenticate dev reset")
        url = self._reset_url()
        code, body = helpers.run_curl_with_status(
            "POST",
            url,
            headers={"Authorization": "Bearer " + psk},
            timeout=30,
        )
        if code == http.HTTPStatus.NOT_FOUND:
            self.skipTest(
                "dev reset disabled (empty NODE_REGISTRATION_PSK on control-plane)"
            )
        self.assertEqual(
            code,
            http.HTTPStatus.NO_CONTENT,
            f"expected 204 No Content, got {code} body={body!r}",
        )
