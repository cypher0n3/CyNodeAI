# E2E: NATS session credentials and activity paths (gateway login + PMA touch + logout).
# Traces: REQ-ORCHES-0188, REQ-ORCHES-0190, REQ-ORCHES-0192, REQ-ORCHES-0162;
# docs/tech_specs/nats_messaging.md;
# docs/dev_docs/_plan_005a_nats+pma_session_tracking.md Task 10.

import json
import os
import select
import subprocess
import time
import unittest

from scripts.test_scripts import config
from scripts.test_scripts import helpers
from scripts.test_scripts.e2e_json_helpers import fetch_gateway_login_json
import scripts.test_scripts.e2e_state as state


def _norm_url(u):
    if not isinstance(u, str):
        return ""
    return u.strip().rstrip("/")


class TestNatsSessionActivity(unittest.TestCase):
    """Login exposes NATS block; PM chat touches activity; logout completes."""

    tags = ["suite_orchestrator", "full_demo", "no_inference", "nats", "pma"]
    prereqs = ["gateway", "config", "auth", "node_register"]

    def setUp(self):
        ok, detail = helpers.prepare_e2e_cynork_auth()
        self.assertTrue(ok, detail)

    def test_login_returns_nats_config(self):
        """Login JSON includes nats.url, jwt, jwt_expires_at (or jwt_expires_at naming)."""
        data = fetch_gateway_login_json(timeout=60)
        self.assertIsNotNone(data, "login JSON")
        nats = data.get("nats")
        self.assertIsInstance(nats, dict, f"nats block: {data.keys()}")
        self.assertTrue(_norm_url(nats.get("url")), "nats.url")
        self.assertTrue(isinstance(nats.get("jwt"), str) and nats.get("jwt"), "nats.jwt")
        exp = nats.get("jwt_expires_at") or nats.get("jwtExpiresAt")
        self.assertTrue(exp, f"nats jwt expiry field: {sorted(nats.keys())}")

    def test_nats_config_matches_environment_endpoints(self):
        """nats.url and optional websocket_url align with E2E config (not literals in test)."""
        data = fetch_gateway_login_json(timeout=60)
        self.assertIsNotNone(data)
        nats = data.get("nats") or {}
        got_url = _norm_url(nats.get("url"))
        want_url = _norm_url(config.NATS_CLIENT_URL)
        self.assertEqual(
            got_url,
            want_url,
            f"nats.url {got_url!r} must match config NATS_CLIENT_URL {want_url!r}",
        )
        wss = nats.get("websocket_url") or nats.get("websocketUrl")
        if wss:
            self.assertEqual(
                _norm_url(wss),
                _norm_url(config.NATS_WEBSOCKET_URL),
                "nats.websocket_url should match config NATS_WEBSOCKET_URL",
            )

    def test_login_includes_session_binding_fields(self):
        """Login exposes interactive session and binding key (session attached path)."""
        data = fetch_gateway_login_json(timeout=60)
        self.assertIsNotNone(data)
        self.assertTrue(data.get("interactive_session_id"), "interactive_session_id")
        self.assertTrue(data.get("session_binding_key"), "session_binding_key")

    def test_pm_chat_interaction_acceptable(self):
        """POST cynodeai.pm chat updates PMA binding activity path (gateway may error upstream)."""
        token = helpers.read_token_from_config(state.CONFIG_PATH)
        self.assertTrue(token)
        st, chat_body = helpers.gateway_request(
            "POST",
            "/v1/chat/completions",
            token,
            json_body={
                "model": "cynodeai.pm",
                "messages": [{"role": "user", "content": "e2e nats activity ping"}],
            },
            timeout=120,
        )
        self.assertIn(
            st,
            (200, 502, 503, 504),
            f"chat completions: {chat_body[:400]}",
        )

    def test_pm_chat_publishes_session_activity_to_nats(self):
        """Gateway publishes session.activity on PMA chat (REST liveness); subscriber sees envelope.

        Traces: REQ-ORCHES-0188, REQ-ORCHES-0190; nats_messaging.md session activity subject.
        Uses orchestrator/cmd/e2e-nats-subscribe-once (natsutil JWT); same stack as cynork.
        """
        data = fetch_gateway_login_json(timeout=60)
        self.assertIsNotNone(data, "login JSON")
        token = data.get("access_token")
        self.assertTrue(token, "access_token from login")
        session_id = (data.get("interactive_session_id") or "").strip()
        self.assertTrue(session_id, "interactive_session_id")
        nats_block = data.get("nats") or {}
        nats_url = _norm_url(nats_block.get("url"))
        nats_jwt = nats_block.get("jwt")
        exp = nats_block.get("jwt_expires_at") or nats_block.get("jwtExpiresAt")
        self.assertTrue(nats_url, "nats.url")
        self.assertTrue(isinstance(nats_jwt, str) and nats_jwt.strip(), "nats.jwt")
        self.assertTrue(exp, "nats jwt_expires_at")

        subj = f"cynode.session.activity.default.{session_id}"
        orch_dir = os.path.join(config.PROJECT_ROOT, "orchestrator")
        env = os.environ.copy()
        env["NATS_URL"] = nats_url
        env["NATS_JWT"] = nats_jwt
        env["NATS_JWT_EXPIRES_AT"] = exp
        env["NATS_SUBJECT"] = subj
        ca = nats_block.get("ca_bundle_pem") or nats_block.get("caBundlePem")
        if isinstance(ca, str) and ca.strip():
            env["NATS_CA_BUNDLE_PEM"] = ca

        with subprocess.Popen(
            ["go", "run", "./cmd/e2e-nats-subscribe-once"],
            cwd=orch_dir,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            env=env,
            bufsize=0,
        ) as proc:
            self.assertIsNotNone(proc.stderr)
            deadline = time.monotonic() + 30
            ready = False
            try:
                while time.monotonic() < deadline:
                    if proc.poll() is not None:
                        err = proc.stderr.read().decode("utf-8", errors="replace")
                        self.fail(
                            "nats subscriber exited before ready "
                            f"rc={proc.returncode}: {err[:2000]!r}"
                        )
                    readable, _, _ = select.select([proc.stderr], [], [], 0.3)
                    if readable:
                        line = proc.stderr.readline()
                        if b"E2E_NATS_SUB_READY" in line:
                            ready = True
                            break
                self.assertTrue(ready, "subscriber did not print E2E_NATS_SUB_READY")

                st, chat_body = helpers.gateway_request(
                    "POST",
                    "/v1/chat/completions",
                    token,
                    json_body={
                        "model": "cynodeai.pm",
                        "messages": [
                            {
                                "role": "user",
                                "content": "e2e nats session.activity ping",
                            }
                        ],
                    },
                    timeout=120,
                )
                self.assertIn(
                    st,
                    (200, 502, 503, 504),
                    f"chat completions: {chat_body[:400]}",
                )

                out, err_tail = proc.communicate(timeout=120)
                err_snip = err_tail.decode("utf-8", errors="replace")[:2000]
                self.assertEqual(
                    proc.returncode,
                    0,
                    f"subscriber rc={proc.returncode} stderr={err_snip!r}",
                )
                self.assertTrue(out, "empty stdout from subscriber")
                env_msg = json.loads(out.decode("utf-8"))
                self.assertEqual(
                    env_msg.get("event_type"),
                    "session.activity",
                    f"envelope keys={list(env_msg.keys())}",
                )
            finally:
                if proc.poll() is None:
                    proc.kill()
                    proc.communicate(timeout=10)

    def test_logout_succeeds_after_fresh_login(self):
        """Logout ends the session; PMA warm pool shrinks when runtime is visible.

        With podman/docker ps, asserts warm-pool container *count* drops after logout
        (REQ-ORCHES-0190, REQ-ORCHES-0192). Pool slots may keep stable names while idle;
        count-based check avoids flakes when the same pma-pool-* name stays running.
        Admin revoke path: e2e_0831.
        """
        before_sb = frozenset()
        n_before = 0
        n_after_login = None
        if helpers.container_runtime_ps_available():
            before_sb = helpers.runtime_pma_sb_container_name_set()
            n_before = len(before_sb)

        acc, ref = helpers.fetch_gateway_login_tokens(timeout=60)
        self.assertTrue(acc and ref, "fresh login tokens")

        if helpers.container_runtime_ps_available():
            if not helpers.pma_pool_login_unlikely_to_add_new_names(n_before):
                ok_sb, _, _ = helpers.wait_for_at_least_new_pma_sb_container_names(
                    before_sb, 1, timeout_sec=180, poll_sec=4
                )
                self.assertTrue(
                    ok_sb,
                    f"expected new pma-pool container after login; before={sorted(before_sb)!r}",
                )
            n_after_login = len(helpers.runtime_pma_sb_container_name_set())
            if not helpers.pma_pool_login_unlikely_to_add_new_names(n_before):
                self.assertGreater(
                    n_after_login,
                    n_before,
                    "expected strictly more warm pool containers after login",
                )

        ok, st, body = helpers.gateway_logout(acc, ref, timeout=60)
        self.assertTrue(ok, f"logout st={st} body={body[:400]!r}")

        if n_after_login is not None and n_after_login > n_before:
            shrunk = helpers.wait_until_runtime_pma_sb_count_below(
                n_after_login, timeout_sec=240, poll_sec=4
            )
            n_cur = len(helpers.runtime_pma_sb_container_name_set())
            self.assertTrue(
                shrunk,
                (
                    "expected warm pool to shrink after logout; "
                    f"still at {n_cur} from {n_after_login}"
                ),
            )


if __name__ == "__main__":
    unittest.main()
