# Unit and functional tests for the worker managed-service proxy path (proxy + PMA).
# Minimal services: worker-api (proxy) and PMA; orchestrator is emulated by calling the proxy
# endpoint directly with the same request shape the orchestrator would send.
# Traces: worker_api.md managed service proxy; REQ-ORCHES-0162 PMA routing via worker.

import base64
import json
import os
import subprocess
import time
import unittest

from scripts.test_scripts import config, helpers


# --- Proxy protocol (matches worker_api main.go managedProxyRequest / managedProxyResponse) ---

def build_proxy_request(method, path, body_bytes, headers=None):
    """Build JSON body for POST /v1/worker/managed-services/{id}/proxy:http."""
    if headers is None:
        headers = {"Content-Type": ["application/json"]}
    return {
        "version": 1,
        "method": method,
        "path": path,
        "headers": headers,
        "body_b64": base64.standard_b64encode(body_bytes).decode("ascii"),
    }


def parse_proxy_response(resp_body_str):
    """Parse proxy response JSON; return (status, body_bytes from body_b64)."""
    data = json.loads(resp_body_str)
    status = data.get("status", 0)
    b64 = data.get("body_b64", "")
    raw = base64.standard_b64decode(b64) if b64 else b""
    return status, raw


def build_chat_completion_body(messages):
    """Build body for POST /internal/chat/completion (PMA handoff)."""
    return json.dumps({"messages": messages}).encode("utf-8")


# --- Unit tests (no processes) ---


class TestProxyPayloadEncoding(unittest.TestCase):
    """Unit tests: proxy request/response encoding and decoding."""

    tags = ["suite_proxy_pma", "suite_worker_node"]

    def test_build_proxy_request_shape(self):
        """Proxy request has version, method, path, headers, body_b64."""
        body = build_chat_completion_body([{"role": "user", "content": "Hi"}])
        req = build_proxy_request("POST", "/internal/chat/completion", body)
        self.assertEqual(req["version"], 1)
        self.assertEqual(req["method"], "POST")
        self.assertEqual(req["path"], "/internal/chat/completion")
        self.assertIn("Content-Type", req["headers"])
        self.assertIn("body_b64", req)
        decoded = base64.standard_b64decode(req["body_b64"])
        self.assertEqual(json.loads(decoded), {"messages": [{"role": "user", "content": "Hi"}]})

    def test_parse_proxy_response(self):
        """Parse proxy response status and body_b64."""
        content = '{"content":"hello"}'
        payload = {
            "version": 1,
            "status": 200,
            "body_b64": base64.standard_b64encode(content.encode()).decode("ascii"),
        }
        status, raw = parse_proxy_response(json.dumps(payload))
        self.assertEqual(status, 200)
        self.assertEqual(json.loads(raw.decode()), {"content": "hello"})


# --- Functional tests (minimal services: PMA + worker-api proxy) ---


class TestProxyPmaFunctional(unittest.TestCase):
    """Functional tests: start PMA and worker-api, call proxy, assert forwarding.

    Emulates the orchestrator by POSTing to the worker's proxy endpoint with the
    same payload shape (method, path, body_b64) used for PMA chat handoff.
    """

    tags = ["suite_proxy_pma", "suite_worker_node"]

    _pma_proc = None
    _worker_proc = None
    _worker_base = None
    _pma_base = None

    @classmethod
    def setUpClass(cls):
        cls._pma_base = f"http://127.0.0.1:{config.PROXY_PMA_TEST_PMA_PORT}"
        cls._worker_base = f"http://127.0.0.1:{config.PROXY_PMA_TEST_WORKER_PORT}"
        if not os.path.isfile(config.PMA_BIN):
            raise unittest.SkipTest(f"PMA binary not found: {config.PMA_BIN}")
        if not os.path.isfile(config.WORKER_API_BIN):
            raise unittest.SkipTest(f"worker-api binary not found: {config.WORKER_API_BIN}")

        # Start PMA (actual agent app) on fixed port
        env_pma = os.environ.copy()
        env_pma["PMA_LISTEN_ADDR"] = f"127.0.0.1:{config.PROXY_PMA_TEST_PMA_PORT}"
        env_pma["PMA_ROLE"] = "project_manager"
        agents_dir = os.path.join(config.PROJECT_ROOT, "agents")
        cls._pma_proc = subprocess.Popen(
            [
                config.PMA_BIN,
                "--listen",
                f"127.0.0.1:{config.PROXY_PMA_TEST_PMA_PORT}",
                "--role",
                "project_manager",
            ],
            cwd=agents_dir,
            env=env_pma,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.PIPE,
        )
        if not _wait_http(f"{cls._pma_base}/healthz", timeout=15):
            cls._pma_proc.terminate()
            cls._pma_proc.wait(timeout=5)
            raise unittest.SkipTest("PMA did not become ready (healthz)")

        # Start worker-api with single managed service target pointing at PMA
        targets_json = json.dumps({"pma-main": cls._pma_base})
        env_worker = os.environ.copy()
        env_worker["WORKER_API_BEARER_TOKEN"] = "proxy-test-token"
        env_worker["WORKER_MANAGED_SERVICE_TARGETS_JSON"] = targets_json
        env_worker["LISTEN_ADDR"] = f"127.0.0.1:{config.PROXY_PMA_TEST_WORKER_PORT}"
        cls._worker_proc = subprocess.Popen(
            [config.WORKER_API_BIN],
            cwd=config.PROJECT_ROOT,
            env=env_worker,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.PIPE,
        )
        if not _wait_http(f"{cls._worker_base}/healthz", timeout=10):
            cls._worker_proc.terminate()
            cls._worker_proc.wait(timeout=5)
            if cls._pma_proc:
                cls._pma_proc.terminate()
                cls._pma_proc.wait(timeout=5)
            raise unittest.SkipTest("worker-api did not become ready (healthz)")

    @classmethod
    def tearDownClass(cls):
        if cls._worker_proc:
            cls._worker_proc.terminate()
            cls._worker_proc.wait(timeout=5)
        if cls._pma_proc:
            cls._pma_proc.terminate()
            cls._pma_proc.wait(timeout=5)

    def test_proxy_forwards_to_pma_healthz(self):
        """Proxy forwards GET /healthz to PMA and returns 200 with body."""
        body = build_proxy_request("GET", "/healthz", b"")
        code, resp_body = helpers.run_curl_with_status(
            "POST",
            f"{self._worker_base}/v1/worker/managed-services/pma-main/proxy:http",
            data=json.dumps(body),
            headers={
                "Content-Type": "application/json",
                "Authorization": "Bearer proxy-test-token",
            },
            timeout=10,
        )
        self.assertEqual(code, 200, f"proxy returned {code}: {resp_body!r}")
        status, raw = parse_proxy_response(resp_body)
        self.assertEqual(status, 200)
        self.assertIn(b"ok", raw)

    def test_proxy_forwards_chat_completion(self):
        """Proxy forwards POST /internal/chat/completion to PMA; upstream 200 with content or 500 if no inference."""
        chat_body = build_chat_completion_body([{"role": "user", "content": "Reply with OK"}])
        proxy_body = build_proxy_request("POST", "/internal/chat/completion", chat_body)
        code, resp_body = helpers.run_curl_with_status(
            "POST",
            f"{self._worker_base}/v1/worker/managed-services/pma-main/proxy:http",
            data=json.dumps(proxy_body),
            headers={
                "Content-Type": "application/json",
                "Authorization": "Bearer proxy-test-token",
            },
            timeout=15,
        )
        self.assertEqual(code, 200, f"proxy returned {code}: {resp_body!r}")
        status, raw = parse_proxy_response(resp_body)
        self.assertIn(status, (200, 500), f"upstream status {status}: {raw!r}")
        out = json.loads(raw.decode())
        self.assertIn("content", out)
        if status == 500:
            self.skipTest("PMA returned 500 (no inference in minimal env); proxy path is verified")
        self.assertTrue(isinstance(out["content"], str))

    def test_proxy_requires_bearer(self):
        """Proxy returns 401 without valid bearer token."""
        body = build_proxy_request("GET", "/healthz", b"")
        code, _ = helpers.run_curl_with_status(
            "POST",
            f"{self._worker_base}/v1/worker/managed-services/pma-main/proxy:http",
            data=json.dumps(body),
            headers={"Content-Type": "application/json"},
            timeout=5,
        )
        self.assertEqual(code, 401)

    def test_proxy_unknown_service_returns_404(self):
        """Proxy returns 404 for unknown service_id."""
        body = build_proxy_request("GET", "/healthz", b"")
        code, _ = helpers.run_curl_with_status(
            "POST",
            f"{self._worker_base}/v1/worker/managed-services/unknown-svc/proxy:http",
            data=json.dumps(body),
            headers={
                "Content-Type": "application/json",
                "Authorization": "Bearer proxy-test-token",
            },
            timeout=5,
        )
        self.assertEqual(code, 404)


def _wait_http(url, timeout=10):
    """Return True when GET url returns 200 within timeout."""
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        code, _ = helpers.run_curl_with_status("GET", url, timeout=2)
        if code == 200:
            return True
        time.sleep(0.2)
    return False
