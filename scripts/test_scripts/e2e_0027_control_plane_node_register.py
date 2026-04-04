# E2E parity: control-plane node register (early in suite). Sets state.NODE_JWT.
# Traces: REQ-ORCHES-0113, REQ-ORCHES-0114, REQ-ORCHES-0193; CYNAI.WORKER.Payload.CapabilityReportV1.
#
# Ordering: ``e2e_0027_*`` runs right after gateway smoke so ``node_register`` prereq and
# helpers see a JWT before worker/control-plane tests. Re-POSTing the same node_slug without
# unregister exercises ``handleExistingNodeRegistration`` (HTTP 200); ``test_d`` covers unregister
# then fresh register (201) for an ephemeral slug.

import json
import unittest
import uuid
from datetime import datetime, timezone

from scripts.test_scripts import config, helpers
import scripts.test_scripts.e2e_state as state


def _registration_payload(node_slug: str) -> dict:
    return {
        "psk": config.NODE_PSK,
        "capability": {
            "version": 1,
            "reported_at": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
            "node": {
                "node_slug": node_slug,
                "labels": ["orchestrator_host"] if node_slug == "test-e2e-node" else [],
            },
            "platform": {"os": "linux", "arch": "amd64"},
            "compute": {"cpu_cores": 4, "ram_mb": 8192},
            "worker_api": {"base_url": config.WORKER_API},
        },
    }


class TestNodeRegister(unittest.TestCase):
    """E2E: POST /v1/nodes/register — new node (201) vs existing slug (200)."""

    tags = ["suite_orchestrator", "full_demo", "no_inference", "control_plane"]
    prereqs = ["gateway"]

    def test_a_fresh_slug_first_register_201_then_existing_200(self):
        """Never-seen slug → 201 Created; same slug again → 200 (re-register path)."""
        slug = f"e2e-fresh-{uuid.uuid4().hex}"
        url = config.CONTROL_PLANE_API + "/v1/nodes/register"
        body = json.dumps(_registration_payload(slug))

        code1, resp1 = helpers.run_curl_with_status("POST", url, data=body, timeout=60)
        self.assertEqual(code1, 201, f"first register expected 201: {resp1[:500]!r}")
        jwt1 = (helpers.parse_json_safe(resp1) or {}).get("auth", {}).get("node_jwt")
        self.assertTrue(jwt1, "first response must include node_jwt")

        code2, resp2 = helpers.run_curl_with_status("POST", url, data=body, timeout=60)
        self.assertEqual(code2, 200, f"re-register same slug expected 200: {resp2[:500]!r}")
        jwt2 = (helpers.parse_json_safe(resp2) or {}).get("auth", {}).get("node_jwt")
        self.assertTrue(jwt2, "second response must include node_jwt")

    def test_b_canonical_e2e_node_establishes_shared_jwt(self):
        """Register ``test-e2e-node`` (helper slug); set state.NODE_JWT for downstream tests."""
        url = config.CONTROL_PLANE_API + "/v1/nodes/register"
        body = json.dumps(_registration_payload("test-e2e-node"))
        code, resp = helpers.run_curl_with_status("POST", url, data=body, timeout=60)
        self.assertIn(code, (200, 201), f"canonical register: {resp[:500]!r}")
        data = helpers.parse_json_safe(resp) or {}
        jwt = (data.get("auth") or {}).get("node_jwt")
        self.assertTrue(jwt, "no node_jwt in response")
        state.NODE_JWT = jwt

    def test_c_canonical_slug_re_register_returns_200(self):
        """Second register of ``test-e2e-node`` must hit existing-node path (200 only)."""
        self.assertTrue(
            getattr(state, "NODE_JWT", None),
            "NODE_JWT missing; test_b must run before test_c in this class",
        )
        url = config.CONTROL_PLANE_API + "/v1/nodes/register"
        body = json.dumps(_registration_payload("test-e2e-node"))
        code, resp = helpers.run_curl_with_status("POST", url, data=body, timeout=60)
        self.assertEqual(code, 200, f"re-register canonical slug expected 200: {resp[:500]!r}")
        data = helpers.parse_json_safe(resp) or {}
        jwt = (data.get("auth") or {}).get("node_jwt")
        self.assertTrue(jwt, "re-register must return node_jwt")
        state.NODE_JWT = jwt

    def test_d_unregister_then_same_slug_registers_201(self):
        """Ephemeral slug: register 201 → DELETE node_self_unregister_url → register same slug 201."""
        slug = f"e2e-unreg-{uuid.uuid4().hex}"
        reg_url = config.CONTROL_PLANE_API + "/v1/nodes/register"
        body = json.dumps(_registration_payload(slug))

        code1, resp1 = helpers.run_curl_with_status("POST", reg_url, data=body, timeout=60)
        self.assertEqual(code1, 201, f"first register expected 201: {resp1[:500]!r}")
        data1 = helpers.parse_json_safe(resp1) or {}
        jwt1 = (data1.get("auth") or {}).get("node_jwt")
        self.assertTrue(jwt1, "first response must include node_jwt")
        unreg_url = (data1.get("orchestrator") or {}).get("endpoints", {}).get(
            "node_self_unregister_url"
        )
        self.assertTrue(
            unreg_url,
            "bootstrap must include orchestrator.endpoints.node_self_unregister_url",
        )

        headers = {"Authorization": f"Bearer {jwt1}"}
        code_del, resp_del = helpers.run_curl_with_status(
            "DELETE", unreg_url, headers=headers, timeout=60
        )
        self.assertEqual(
            code_del,
            204,
            f"unregister expected 204: {resp_del[:500]!r}",
        )

        code3, resp3 = helpers.run_curl_with_status("POST", reg_url, data=body, timeout=60)
        self.assertEqual(
            code3,
            201,
            f"register after unregister expected 201: {resp3[:500]!r}",
        )
        # Remove ephemeral node so only the canonical worker remains; otherwise lexicographically
        # earlier slugs can steal PMA host selection from test-e2e-node (managed_services/PMA E2E).
        data3 = helpers.parse_json_safe(resp3) or {}
        jwt3 = (data3.get("auth") or {}).get("node_jwt")
        self.assertTrue(jwt3, "re-register response must include node_jwt")
        code4, _ = helpers.run_curl_with_status(
            "DELETE",
            unreg_url,
            headers={"Authorization": f"Bearer {jwt3}"},
            timeout=60,
        )
        self.assertEqual(code4, 204, "cleanup unregister expected 204")
