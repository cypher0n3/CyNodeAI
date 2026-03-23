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

    def setUp(self):
        ok, detail = helpers.prepare_e2e_cynork_auth()
        self.assertTrue(ok, detail)

    def test_refresh(self):
        """Assert cynork auth refresh succeeds, session updates, and stale refresh is rejected."""
        before_token = helpers.read_config_value(state.CONFIG_PATH, "token")
        before_refresh = helpers.read_config_value(state.CONFIG_PATH, "refresh_token")
        self.assertTrue(before_token, "precondition failed: token missing before refresh")
        self.assertTrue(before_refresh, "precondition failed: refresh_token missing before refresh")

        ok, out, err = helpers.run_cynork(["auth", "refresh"], state.CONFIG_PATH)
        self.assertTrue(ok, f"auth refresh failed: {out} {err}")

        # run_cynork repopulates the E2E sidecar via password login after refresh (cynork does not
        # expose tokens to the parent process).
        after_token = helpers.read_config_value(state.CONFIG_PATH, "token")
        after_refresh = helpers.read_config_value(state.CONFIG_PATH, "refresh_token")
        self.assertTrue(after_token, "token missing after refresh (E2E sidecar)")
        self.assertTrue(after_refresh, "refresh_token missing after refresh (E2E sidecar)")
        self.assertNotEqual(
            before_token,
            after_token,
            "access token should change after refresh flow",
        )
        self.assertNotEqual(
            before_refresh, after_refresh, "refresh token should rotate after refresh flow"
        )

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
