# E2E: scope-partitioned artifacts REST API (POST/GET/PUT/DELETE/GET list) and MinIO backend.
# Traces: docs/tech_specs/orchestrator_artifacts_storage.md; REQ-SCHEMA-0114; REQ-ORCHES-0127.

import base64
import json
import unittest
import urllib.error
import urllib.parse
import urllib.request
import uuid

from scripts.test_scripts import config, helpers
import scripts.test_scripts.e2e_state as state


def _gateway_raw(method, path, access_token, data=None, extra_headers=None):
    """HTTP request to user gateway; data is raw bytes or None."""
    url = config.USER_API.rstrip("/") + path
    headers = {}
    if access_token:
        headers["Authorization"] = "Bearer " + access_token.strip()
    if extra_headers:
        headers.update(extra_headers)
    req = urllib.request.Request(url, data=data, method=method, headers=headers)
    try:
        with urllib.request.urlopen(req, timeout=120) as resp:
            return resp.status, resp.read()
    except urllib.error.HTTPError as e:
        try:
            raw = e.read() if e.fp else b""
            return e.code, raw
        finally:
            e.close()
    except (urllib.error.URLError, OSError, ValueError):
        return 0, b""


class TestArtifactsCRUD(unittest.TestCase):
    """Exercise /v1/artifacts against a stack with ARTIFACTS_S3_* configured."""

    tags = ["suite_orchestrator", "full_demo", "artifacts"]
    prereqs = ["gateway", "config", "auth"]

    def setUp(self):
        ok, detail = helpers.prepare_e2e_cynork_auth()
        self.assertTrue(ok, detail)

    def test_artifacts_user_scope_round_trip(self):
        token = helpers.read_token_from_config(state.CONFIG_PATH)
        self.assertTrue(token, "access token required")

        st, me_body = helpers.gateway_request("GET", "/v1/users/me", token, timeout=60)
        self.assertEqual(st, 200, me_body)
        me = helpers.parse_json_safe(me_body) or {}
        uid = me.get("id")
        self.assertTrue(uid, me)

        rel_path = "e2e/" + uuid.uuid4().hex + ".txt"
        q = "?" + urllib.parse.urlencode(
            {"scope_level": "user", "owner_user_id": uid, "path": rel_path}
        )
        body = b"hello artifacts e2e"
        st, raw = _gateway_raw(
            "POST",
            "/v1/artifacts" + q,
            token,
            data=body,
            extra_headers={"Content-Type": "text/plain"},
        )
        if st == 503:
            self.skipTest("artifacts storage not configured on gateway")
        self.assertEqual(st, 201, raw.decode("utf-8", errors="replace"))
        meta = json.loads(raw.decode("utf-8"))
        aid = meta.get("artifact_id")
        self.assertTrue(aid, meta)

        st, raw = _gateway_raw("GET", f"/v1/artifacts/{aid}", token)
        self.assertEqual(st, 200, raw.decode("utf-8", errors="replace"))
        self.assertEqual(raw, body)

        st, raw = _gateway_raw(
            "PUT",
            f"/v1/artifacts/{aid}",
            token,
            data=b"updated",
            extra_headers={"Content-Type": "text/plain"},
        )
        self.assertEqual(st, 200, raw.decode("utf-8", errors="replace"))

        st, raw = _gateway_raw("GET", f"/v1/artifacts/{aid}", token)
        self.assertEqual(st, 200, raw.decode("utf-8", errors="replace"))
        self.assertEqual(raw, b"updated")

        st, raw = helpers.gateway_request(
            "GET",
            "/v1/artifacts?"
            + urllib.parse.urlencode({"scope_level": "user", "user_id": uid}),
            token,
            timeout=60,
        )
        self.assertEqual(st, 200, raw)
        listed = helpers.parse_json_safe(raw) or {}
        arts = listed.get("artifacts") or []
        self.assertTrue(any(a.get("artifact_id") == aid for a in arts), listed)

        st, _ = _gateway_raw("DELETE", f"/v1/artifacts/{aid}", token)
        self.assertEqual(st, 204)

    def test_mcp_artifact_put_get_list(self):
        """MCP artifact.put / artifact.get / artifact.list via control-plane."""
        # Same backend and RBAC as REST.
        token = helpers.read_token_from_config(state.CONFIG_PATH)
        self.assertTrue(token, "access token required")
        st, me_body = helpers.gateway_request("GET", "/v1/users/me", token, timeout=60)
        self.assertEqual(st, 200, me_body)
        me = helpers.parse_json_safe(me_body) or {}
        uid = me.get("id")
        self.assertTrue(uid, me)
        rel_path = "e2e/mcp-" + uuid.uuid4().hex + ".txt"
        body = b"mcp artifact tools"
        args_put = {
            "user_id": uid,
            "scope": "user",
            "path": rel_path,
            "content_bytes_base64": base64.standard_b64encode(body).decode("ascii"),
            "content_type": "text/plain",
        }
        code, raw = helpers.mcp_tool_call("artifact.put", arguments=args_put, timeout=90)
        if code == 503:
            self.skipTest("artifacts storage not configured on control-plane")
        self.assertEqual(code, 200, raw)
        put_data = helpers.parse_json_safe(raw) or {}
        self.assertEqual(put_data.get("status"), "success", raw)

        code, raw = helpers.mcp_tool_call(
            "artifact.get",
            arguments={
                "user_id": uid,
                "scope": "user",
                "path": rel_path,
            },
            timeout=90,
        )
        self.assertEqual(code, 200, raw)
        get_data = helpers.parse_json_safe(raw) or {}
        self.assertEqual(get_data.get("status"), "success", raw)
        got = base64.standard_b64decode(get_data.get("content_bytes_base64") or "")
        self.assertEqual(got, body)

        code, raw = helpers.mcp_tool_call(
            "artifact.list",
            arguments={"user_id": uid, "scope": "user", "limit": 50},
            timeout=90,
        )
        self.assertEqual(code, 200, raw)
        list_data = helpers.parse_json_safe(raw) or {}
        self.assertEqual(list_data.get("status"), "success", raw)
        arts = list_data.get("artifacts") or []
        self.assertTrue(isinstance(arts, list))
        self.assertTrue(
            any(
                isinstance(a, dict) and a.get("path") == rel_path
                for a in arts
            ),
            list_data,
        )
