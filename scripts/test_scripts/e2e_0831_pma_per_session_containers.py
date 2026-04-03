# E2E: PMA warm pool — greedy provision assigns pma-pool-* slots; idle spares stay running.
# Observes container runtime (cynodeai-managed-pma-pool-*) per orchestrator_bootstrap.md /
# REQ-ORCHES-0190, REQ-ORCHES-0192; worker GET .../telemetry/containers is not fed for managed PMA.
# Traces: REQ-ORCHES-0188, REQ-ORCHES-0190, REQ-ORCHES-0192;
# docs/dev_docs/_plan_005_pma_provisioning.md; docs/tech_specs/orchestrator_bootstrap.md.

import unittest

from scripts.test_scripts import helpers
import scripts.test_scripts.e2e_state as state


class TestPmaPerSessionContainers(unittest.TestCase):
    """Warm pool PMA on login; pool shrinks when sessions end (logout or revoke)."""

    tags = [
        "suite_orchestrator",
        "full_demo",
        "no_inference",
        "pma",
    ]
    prereqs = ["gateway", "config", "auth", "node_register"]

    @classmethod
    def tearDownClass(cls):
        """Revoke sessions; pool shrinks to minimum warm baseline (one pma-pool-*)."""
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
                "cleanup: expected <=1 cynodeai-managed-pma-pool-* after revoke_sessions; "
                f"still {sorted(helpers.runtime_pma_sb_container_name_set())!r}"
            )
        ok_auth, detail = helpers.prepare_e2e_cynork_auth()
        if not ok_auth:
            raise AssertionError(f"cleanup: restore auth after revoke failed: {detail}")

    def setUp(self):
        ok, detail = helpers.prepare_e2e_cynork_auth()
        self.assertTrue(ok, detail)
        if not helpers.container_runtime_ps_available():
            self.skipTest(
                "podman or docker ps not available (exit non-zero or missing binary)"
            )

    def test_two_sequential_logins_expand_pma_warm_pool(self):
        """Two sessions => pool size at least sessions + min free (default 3 containers)."""
        before_sb = helpers.runtime_pma_sb_container_name_set()

        for _ in range(2):
            acc, ref = helpers.fetch_gateway_login_tokens(timeout=60)
            self.assertTrue(acc, "gateway login must return access_token")
            self.assertTrue(
                ref,
                "gateway login must return refresh_token for session binding key",
            )

        ok, _, last_ct = helpers.wait_for_runtime_pma_sb_count_at_least(
            3, timeout_sec=180, poll_sec=4
        )
        self.assertTrue(
            ok,
            (
                "expected >=3 cynodeai-managed-pma-pool-* after 2 concurrent sessions + spare; "
                f"last_count={last_ct} before={sorted(before_sb)!r}"
            ),
        )

        token = helpers.read_token_from_config(state.CONFIG_PATH)
        self.assertTrue(token, "access token required")
        st, _me = helpers.gateway_request("GET", "/v1/users/me", token, timeout=60)
        self.assertEqual(st, 200, "gateway still accepts bearer after extra logins")

    def test_admin_revoke_sessions_shrinks_pma_warm_pool(self):
        """Admin revoke_sessions ends sessions; pool returns to minimum warm size (<=1 slot)."""
        before_sb = helpers.runtime_pma_sb_container_name_set()
        acc, ref = helpers.fetch_gateway_login_tokens(timeout=60)
        self.assertTrue(acc, "login access_token")
        self.assertTrue(ref, "login refresh_token")

        if not helpers.pma_pool_login_unlikely_to_add_new_names(len(before_sb)):
            ok_new, _, _ = helpers.wait_for_at_least_new_pma_sb_container_names(
                before_sb, 1, timeout_sec=180, poll_sec=4
            )
            self.assertTrue(
                ok_new,
                f"expected pool growth vs before login; before={sorted(before_sb)!r}",
            )

        ok_revoke, st, body = helpers.gateway_admin_revoke_sessions_for_me(
            acc, timeout=60
        )
        self.assertTrue(
            ok_revoke,
            f"revoke_sessions expected 204, st={st} body={body[:400]!r}",
        )

        shrunk = helpers.wait_until_runtime_pma_pool_at_most(
            1, timeout_sec=180, poll_sec=4
        )
        self.assertTrue(
            shrunk,
            (
                "expected <=1 warm pool container after revoke_sessions; "
                f"still {sorted(helpers.runtime_pma_sb_container_name_set())!r}"
            ),
        )

    def test_gateway_logout_shrinks_pma_warm_pool(self):
        """Two sessions expand the pool; logging out one session shrinks container count."""
        before_sb = helpers.runtime_pma_sb_container_name_set()
        acc1, ref1 = helpers.fetch_gateway_login_tokens(timeout=60)
        acc2, ref2 = helpers.fetch_gateway_login_tokens(timeout=60)
        self.assertTrue(acc1 and ref1 and acc2 and ref2, "two gateway sessions")

        if not helpers.pma_pool_login_unlikely_to_add_new_names(len(before_sb)):
            ok_new, _, _ = helpers.wait_for_at_least_new_pma_sb_container_names(
                before_sb, 1, timeout_sec=180, poll_sec=4
            )
            self.assertTrue(ok_new, "expected pool growth after two logins")
        mid = helpers.runtime_pma_sb_container_name_set()
        self.assertGreaterEqual(
            len(mid),
            2,
            f"expected >=2 warm pool containers with two sessions; got {sorted(mid)!r}",
        )

        ok_out, st, body = helpers.gateway_logout(acc1, ref1, timeout=60)
        self.assertTrue(
            ok_out,
            f"logout expected 200/204, got {st} {body[:300]!r}",
        )

        shrunk = helpers.wait_until_runtime_pma_sb_count_below(
            len(mid), timeout_sec=180, poll_sec=4
        )
        self.assertTrue(
            shrunk,
            (
                "expected warm pool to shrink after one logout; "
                f"still at {len(helpers.runtime_pma_sb_container_name_set())} from {len(mid)}"
            ),
        )
