# E2E: API egress POST /v1/call (allowed -> 501, disallowed -> 403).

import json
import unittest

from scripts.test_scripts import config, helpers


class TestAPIEgressCall(unittest.TestCase):
    """E2E: POST /v1/call on api-egress; assert 501 for allowed, 403 for disallowed."""

    tags = ["suite_orchestrator", "full_demo"]

    def test_allowed_provider_returns_501(self):
        """Allowed provider returns 501 Not Implemented."""
        body = json.dumps({
            "provider": "openai",
            "operation": "chat",
            "task_id": "e2e-t1",
        })
        headers = {}
        if getattr(config, "API_EGRESS_BEARER_TOKEN", None):
            headers["Authorization"] = f"Bearer {config.API_EGRESS_BEARER_TOKEN}"
        code, resp_body = helpers.run_curl_with_status(
            "POST",
            f"{config.API_EGRESS_API}/v1/call",
            data=body,
            headers=headers or None,
        )
        if not code:
            self.fail(
                "api-egress not reachable (start stack: just setup-dev start)"
            )
        self.assertEqual(code, 501, f"allowed provider: {code} {resp_body}")
        data = helpers.parse_json_safe(resp_body)
        self.assertIsNotNone(data)
        self.assertIn("title", data)
        self.assertIn("Not Implemented", (data.get("title") or ""))

    def test_disallowed_provider_returns_403(self):
        """Disallowed provider returns 403 Forbidden."""
        body = json.dumps({
            "provider": "unknown_provider",
            "operation": "op",
            "task_id": "e2e-t2",
        })
        headers = {}
        if getattr(config, "API_EGRESS_BEARER_TOKEN", None):
            headers["Authorization"] = f"Bearer {config.API_EGRESS_BEARER_TOKEN}"
        code, _ = helpers.run_curl_with_status(
            "POST",
            f"{config.API_EGRESS_API}/v1/call",
            data=body,
            headers=headers or None,
        )
        if not code:
            self.fail("api-egress not reachable (start stack with optional profile)")
        self.assertEqual(code, 403, f"disallowed provider expected 403, got {code}")
