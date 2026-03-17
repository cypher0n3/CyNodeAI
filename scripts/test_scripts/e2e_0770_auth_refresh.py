# E2E parity: auth refresh and whoami.
# Traces: REQ-IDENTY-0104, 0105 (refresh token, rotation); CYNAI.IDENTY.AuthenticationModel.

import unittest
import json

from scripts.test_scripts import config, helpers
import scripts.test_scripts.e2e_state as state


class TestRefresh(unittest.TestCase):
    """E2E: auth refresh then whoami; expect user=admin."""

    tags = ["suite_cynork", "full_demo", "auth", "no_inference"]
    prereqs = ["gateway", "config", "auth"]

    def test_refresh(self):
        """Assert refresh rotates tokens, invalidates stale refresh token, and preserves session."""
        before_token = helpers.read_config_value(state.CONFIG_PATH, "token")
        before_refresh = helpers.read_config_value(state.CONFIG_PATH, "refresh_token")
        self.assertTrue(before_token, "precondition failed: token missing before refresh")
        self.assertTrue(before_refresh, "precondition failed: refresh_token missing before refresh")

        ok, out, err = helpers.run_cynork(["auth", "refresh"], state.CONFIG_PATH)
        self.assertTrue(ok, f"auth refresh failed: {out} {err}")

        after_token = helpers.read_config_value(state.CONFIG_PATH, "token")
        after_refresh = helpers.read_config_value(state.CONFIG_PATH, "refresh_token")
        self.assertTrue(after_token, "token missing after refresh")
        self.assertTrue(after_refresh, "refresh_token missing after refresh")
        self.assertNotEqual(before_token, after_token, "access token should rotate on refresh")
        self.assertNotEqual(before_refresh, after_refresh, "refresh token should rotate on refresh")

        ok, out, _ = helpers.run_cynork(["auth", "whoami"], state.CONFIG_PATH)
        self.assertTrue(ok, "whoami after refresh failed")
        self.assertIn("user=admin", out)

        stale_payload = json.dumps({"refresh_token": before_refresh})
        code, body = helpers.run_curl_with_status(
            "POST",
            f"{config.USER_API}/v1/auth/refresh",
            data=stale_payload,
            headers={"Content-Type": "application/json"},
        )
        self.assertGreaterEqual(
            code,
            400,
            f"stale refresh token should be rejected; got code={code} body={body!r}",
        )
