# E2E: Per-session PMA instances — greedy provision on login creates distinct pma-sb-* workers.
# Observes container runtime (cynodeai-managed-pma-sb-*) per orchestrator_bootstrap.md /
# REQ-ORCHES-0190; worker GET .../telemetry/containers is not fed for managed PMA by node-manager.
# Traces: docs/dev_docs/_plan_005_pma_provisioning.md; docs/tech_specs/orchestrator_bootstrap.md.

import unittest

from scripts.test_scripts import helpers
import scripts.test_scripts.e2e_state as state


class TestPmaPerSessionContainers(unittest.TestCase):
    """Two sequential gateway logins yield two additional pma-sb managed containers."""

    tags = [
        "suite_orchestrator",
        "full_demo",
        "pma",
        "pma_inference",
    ]
    prereqs = ["gateway", "config", "auth", "node_register"]

    @classmethod
    def tearDownClass(cls):
        """Revoke admin sessions so orchestrator tears down PMA bindings; worker removes pma-sb-*."""
        if not helpers.container_runtime_ps_available():
            return
        state.init_config()
        token = helpers.read_token_from_config(state.CONFIG_PATH)
        if not token:
            acc, _ = helpers.fetch_gateway_login_tokens(timeout=60)
            token = acc
        if not token:
            return
        ok_revoke, st, body = helpers.gateway_admin_revoke_sessions_for_me(token, timeout=60)
        if not ok_revoke:
            raise AssertionError(
                f"cleanup: admin revoke_sessions failed st={st} body={body[:500]!r}"
            )
        if not helpers.wait_until_runtime_pma_sb_empty(timeout_sec=180, poll_sec=4):
            raise AssertionError(
                "cleanup: expected no cynodeai-managed-pma-sb-* after revoke_sessions; "
                f"still {sorted(helpers.runtime_pma_sb_container_name_set())!r}"
            )
        ok_auth, detail = helpers.prepare_e2e_cynork_auth()
        if not ok_auth:
            raise AssertionError(f"cleanup: restore auth after revoke failed: {detail}")

    def setUp(self):
        ok, detail = helpers.prepare_e2e_cynork_auth()
        self.assertTrue(ok, detail)
        if not helpers.container_runtime_ps_available():
            self.skipTest("podman or docker ps not available (exit non-zero or missing binary)")

    def test_two_sequential_logins_add_two_pma_sb_containers(self):
        """Each POST /v1/auth/login creates a refresh session; greedy PMA adds pma-sb-* on the worker."""
        before_sb = helpers.runtime_pma_sb_container_name_set()

        for _ in range(2):
            acc, ref = helpers.fetch_gateway_login_tokens(timeout=60)
            self.assertTrue(acc, "gateway login must return access_token")
            self.assertTrue(ref, "gateway login must return refresh_token for session binding key")

        ok, new_sb, _all_sb = helpers.wait_for_at_least_new_pma_sb_container_names(
            before_sb, 2, timeout_sec=180, poll_sec=4
        )
        self.assertTrue(
            ok,
            (
                "expected 2 new cynodeai-managed-pma-sb-* containers after 2 logins; "
                f"new={sorted(new_sb)!r} before_count={len(before_sb)}"
            ),
        )

        token = helpers.read_token_from_config(state.CONFIG_PATH)
        self.assertTrue(token, "access token required")
        st, _me = helpers.gateway_request("GET", "/v1/users/me", token, timeout=60)
        self.assertEqual(st, 200, "gateway still accepts bearer after extra logins")

    def test_gateway_logout_removes_session_pma_container(self):
        """Logout with that login's refresh_token drops binding; worker removes pma-sb container."""
        before_sb = helpers.runtime_pma_sb_container_name_set()
        acc, ref = helpers.fetch_gateway_login_tokens(timeout=60)
        self.assertTrue(acc, "login access_token")
        self.assertTrue(ref, "login refresh_token for logout + binding")

        ok_new, new_sb, _ = helpers.wait_for_at_least_new_pma_sb_container_names(
            before_sb, 1, timeout_sec=180, poll_sec=4
        )
        self.assertTrue(
            ok_new,
            f"expected new pma-sb container after login; new={sorted(new_sb)!r}",
        )
        # new_sb may include >1 name if other sessions raced; logout removes this session's binding only.
        new_ones = set(new_sb)

        ok_out, st, body = helpers.gateway_logout(acc, ref, timeout=60)
        self.assertTrue(
            ok_out,
            f"logout expected 200/204, got {st} {body[:300]!r}",
        )

        gone = helpers.wait_until_some_pma_sb_names_removed(
            new_ones, timeout_sec=180, poll_sec=4
        )
        self.assertTrue(
            gone,
            (
                "expected at least one new pma-sb container removed after logout "
                f"(orchestrator teardown + worker reconcile); still have all of {sorted(new_ones)!r}"
            ),
        )
