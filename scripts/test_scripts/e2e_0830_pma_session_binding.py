# E2E: PMA session binding path — authenticated chat with model cynodeai.pm after login.
# Complements per-session-binding provisioning (greedy provision on auth, routing by binding).
# Traces: REQ-ORCHES-0188, REQ-ORCHES-0190, REQ-ORCHES-0162;
# docs/dev_docs/_plan_005_pma_provisioning.md Task 7.

import unittest

from scripts.test_scripts import helpers
import scripts.test_scripts.e2e_state as state


class TestPmaSessionBinding(unittest.TestCase):
    """Contract E2E: POST /v1/chat/completions with cynodeai.pm after gateway auth."""

    tags = [
        "suite_orchestrator",
        "full_demo",
        "no_inference",
        "pma",
    ]
    prereqs = ["gateway", "config", "auth", "node_register"]

    def setUp(self):
        ok, detail = helpers.prepare_e2e_cynork_auth()
        self.assertTrue(ok, detail)

    def test_pm_chat_after_auth_returns_acceptable_status(self):
        """PMA chat may succeed or fail with gateway/orchestrator errors."""
        token = helpers.read_token_from_config(state.CONFIG_PATH)
        self.assertTrue(token, "access token required")
        st, me_body = helpers.gateway_request("GET", "/v1/users/me", token, timeout=60)
        self.assertEqual(st, 200, me_body)
        st2, chat_body = helpers.gateway_request(
            "POST",
            "/v1/chat/completions",
            token,
            json_body={
                "model": "cynodeai.pm",
                "messages": [{"role": "user", "content": "ping"}],
            },
            timeout=120,
        )
        snippet = chat_body[:500]
        self.assertIn(
            st2,
            (200, 502, 503, 504),
            f"chat completions: {snippet}",
        )
