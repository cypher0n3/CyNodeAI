# Unit and functional tests for the worker managed-service proxy path (proxy + PMA).
# Minimal services: worker-api (proxy) and PMA; orchestrator is emulated by calling the proxy
# endpoint directly with the same request shape the orchestrator would send.
# Traces: worker_api.md managed service proxy; REQ-ORCHES-0162 PMA routing via worker.

import base64
import contextlib
import json
import os
import subprocess
import threading
import time
import unittest
from http.server import BaseHTTPRequestHandler, HTTPServer

from scripts.test_scripts import config, helpers


@contextlib.contextmanager
def _popen_keepalive(*args, **kwargs):
    """Context manager for Popen; process is not terminated on exit (caller must in tearDown)."""
    proc = subprocess.Popen(*args, **kwargs)
    try:
        yield proc
    finally:
        pass  # caller terminates in tearDownClass


# Bearer token for proxy PMA E2E (worker-api expects this; test-only, not a production secret).
_PROXY_PMA_E2E_BEARER = "proxy-test-token"

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

    tags = ["suite_proxy_pma", "suite_worker_node", "full_demo", "pma"]

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

    tags = ["suite_proxy_pma", "suite_worker_node", "pma"]

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
        with _popen_keepalive(
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
        ) as proc:
            cls._pma_proc = proc
            if not _wait_http(f"{cls._pma_base}/healthz", timeout=15):
                cls._pma_proc.terminate()
                cls._pma_proc.wait(timeout=5)
                raise unittest.SkipTest("PMA did not become ready (healthz)")

        # Start worker-api with single managed service target pointing at PMA
        targets_json = json.dumps({"pma-main": cls._pma_base})
        state_dir = os.path.join(config.PROJECT_ROOT, "tmp", "proxy-pma-test-state")
        os.makedirs(state_dir, 0o700, exist_ok=True)
        env_worker = os.environ.copy()
        env_worker["WORKER_API_BEARER_TOKEN"] = _PROXY_PMA_E2E_BEARER
        env_worker["WORKER_MANAGED_SERVICE_TARGETS_JSON"] = targets_json
        env_worker["LISTEN_ADDR"] = f"127.0.0.1:{config.PROXY_PMA_TEST_WORKER_PORT}"
        env_worker["WORKER_API_STATE_DIR"] = state_dir
        with _popen_keepalive(
            [config.WORKER_API_BIN],
            cwd=config.PROJECT_ROOT,
            env=env_worker,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.PIPE,
        ) as proc:
            cls._worker_proc = proc
            if not _wait_http(f"{cls._worker_base}/healthz", timeout=15):
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
                "Authorization": f"Bearer {_PROXY_PMA_E2E_BEARER}",
            },
            timeout=10,
        )
        self.assertEqual(code, 200, f"proxy returned {code}: {resp_body!r}")
        status, raw = parse_proxy_response(resp_body)
        self.assertEqual(status, 200)
        self.assertIn(b"ok", raw)

    def test_proxy_forwards_chat_completion(self):
        """Proxy forwards POST /internal/chat/completion to PMA; 200 or 500 if no inference."""
        chat_body = build_chat_completion_body([{"role": "user", "content": "Reply with OK"}])
        proxy_body = build_proxy_request("POST", "/internal/chat/completion", chat_body)
        code, resp_body = helpers.run_curl_with_status(
            "POST",
            f"{self._worker_base}/v1/worker/managed-services/pma-main/proxy:http",
            data=json.dumps(proxy_body),
            headers={
                "Content-Type": "application/json",
                "Authorization": f"Bearer {_PROXY_PMA_E2E_BEARER}",
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
                "Authorization": f"Bearer {_PROXY_PMA_E2E_BEARER}",
            },
            timeout=5,
        )
        self.assertEqual(code, 404)


# --- Mock inference (Ollama-style /api/generate) for PMA-with-inference test ---


def _make_mock_inference_handler(response_text):
    """Return a handler class for POST /api/generate with {"response": response_text}."""

    class Handler(BaseHTTPRequestHandler):
        # Method name required by BaseHTTPRequestHandler (do_<METHOD> dispatch).
        def do_POST(self):  # pylint: disable=invalid-name
            if self.path == "/api/generate" or self.path.startswith("/api/generate?"):
                self.send_response(200)
                self.send_header("Content-Type", "application/json")
                self.end_headers()
                self.wfile.write(
                    json.dumps({"response": response_text}).encode("utf-8")
                )
            else:
                self.send_response(404)
                self.end_headers()

        # Parameter name must match BaseHTTPRequestHandler.log_message(self, format, *args).
        def log_message(self, format, *args):  # pylint: disable=redefined-builtin
            pass

    return Handler


# --- Functional tests: PMA + proxy with mock inference (full chat path) ---


class TestProxyPmaWithInference(unittest.TestCase):
    """Functional tests: mock inference + PMA + worker proxy; chat returns real content.

    Starts: mock Ollama-style server (POST /api/generate) -> PMA (OLLAMA_BASE_URL=mock)
    -> worker-api proxy. Asserts proxy -> PMA -> inference path returns 200 with content.
    """

    tags = ["suite_proxy_pma", "suite_worker_node", "inference", "pma_inference", "pma"]

    _mock_server = None
    _mock_thread = None
    _pma_proc = None
    _worker_proc = None
    _worker_base = None

    @classmethod
    def setUpClass(cls):
        pma_port = config.PROXY_PMA_TEST_PMA_PORT_WITH_INFERENCE
        worker_port = config.PROXY_PMA_TEST_WORKER_PORT_WITH_INFERENCE
        mock_port = config.PROXY_PMA_TEST_MOCK_INFERENCE_PORT
        cls._worker_base = f"http://127.0.0.1:{worker_port}"
        mock_url = f"http://127.0.0.1:{mock_port}"

        if not os.path.isfile(config.PMA_BIN):
            raise unittest.SkipTest(f"PMA binary not found: {config.PMA_BIN}")
        if not os.path.isfile(config.WORKER_API_BIN):
            raise unittest.SkipTest(f"worker-api binary not found: {config.WORKER_API_BIN}")

        # Start mock inference (Ollama-style /api/generate)
        handler = _make_mock_inference_handler("OK from mock inference")
        cls._mock_server = HTTPServer(("127.0.0.1", mock_port), handler)
        cls._mock_thread = threading.Thread(target=cls._mock_server.serve_forever, daemon=True)
        cls._mock_thread.start()
        time.sleep(0.2)
        # Verify mock inference is up (POST /api/generate)
        code, _ = helpers.run_curl_with_status(
            "POST",
            f"{mock_url}/api/generate",
            data=json.dumps({"model": "x", "prompt": "hi", "stream": False}),
            headers={"Content-Type": "application/json"},
            timeout=3,
        )
        if code != 200:
            raise unittest.SkipTest("mock inference server did not respond")

        # Start PMA with OLLAMA_BASE_URL pointing at mock
        pma_base = f"http://127.0.0.1:{pma_port}"
        agents_dir = os.path.join(config.PROJECT_ROOT, "agents")
        env_pma = os.environ.copy()
        env_pma["PMA_LISTEN_ADDR"] = f"127.0.0.1:{pma_port}"
        env_pma["PMA_ROLE"] = "project_manager"
        env_pma["OLLAMA_BASE_URL"] = mock_url
        with _popen_keepalive(
            [
                config.PMA_BIN,
                "--listen",
                f"127.0.0.1:{pma_port}",
                "--role",
                "project_manager",
            ],
            cwd=agents_dir,
            env=env_pma,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.PIPE,
        ) as proc:
            cls._pma_proc = proc
            if not _wait_http(f"{pma_base}/healthz", timeout=15):
                cls._pma_proc.terminate()
                cls._pma_proc.wait(timeout=5)
                if cls._mock_server:
                    cls._mock_server.shutdown()
                raise unittest.SkipTest("PMA did not become ready (healthz)")

        # Start worker-api with target to PMA (state dir for telemetry/securestore init)
        targets_json = json.dumps({"pma-main": pma_base})
        state_dir = os.path.join(config.PROJECT_ROOT, "tmp", "proxy-pma-test-state")
        os.makedirs(state_dir, 0o700, exist_ok=True)
        env_worker = os.environ.copy()
        env_worker["WORKER_API_BEARER_TOKEN"] = _PROXY_PMA_E2E_BEARER
        env_worker["WORKER_MANAGED_SERVICE_TARGETS_JSON"] = targets_json
        env_worker["LISTEN_ADDR"] = f"127.0.0.1:{worker_port}"
        env_worker["WORKER_API_STATE_DIR"] = state_dir
        with _popen_keepalive(
            [config.WORKER_API_BIN],
            cwd=config.PROJECT_ROOT,
            env=env_worker,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.PIPE,
        ) as proc:
            cls._worker_proc = proc
            if not _wait_http(f"{cls._worker_base}/healthz", timeout=15):
                cls._worker_proc.terminate()
                cls._worker_proc.wait(timeout=5)
                if cls._pma_proc:
                    cls._pma_proc.terminate()
                    cls._pma_proc.wait(timeout=5)
                if cls._mock_server:
                    cls._mock_server.shutdown()
                raise unittest.SkipTest("worker-api did not become ready (healthz)")

    @classmethod
    def tearDownClass(cls):
        if cls._worker_proc:
            cls._worker_proc.terminate()
            cls._worker_proc.wait(timeout=5)
        if cls._pma_proc:
            cls._pma_proc.terminate()
            cls._pma_proc.wait(timeout=5)
        if cls._mock_server:
            cls._mock_server.shutdown()

    def test_proxy_pma_inference_returns_completion_content(self):
        """Proxy -> PMA -> mock inference: chat completion returns 200 with non-empty content."""
        chat_body = build_chat_completion_body([{"role": "user", "content": "Say OK"}])
        proxy_body = build_proxy_request("POST", "/internal/chat/completion", chat_body)
        code, resp_body = helpers.run_curl_with_status(
            "POST",
            f"{self._worker_base}/v1/worker/managed-services/pma-main/proxy:http",
            data=json.dumps(proxy_body),
            headers={
                "Content-Type": "application/json",
                "Authorization": f"Bearer {_PROXY_PMA_E2E_BEARER}",
            },
            timeout=15,
        )
        self.assertEqual(code, 200, f"proxy returned {code}: {resp_body!r}")
        status, raw = parse_proxy_response(resp_body)
        self.assertEqual(status, 200, f"upstream status {status}: {raw!r}")
        out = json.loads(raw.decode())
        self.assertIn("content", out)
        self.assertIsInstance(out["content"], str)
        self.assertIn("OK from mock inference", out["content"])


# --- Real Ollama container helpers for proxy+PMA+real-inference test ---


def _start_ollama_container(runtime, container_name, host_port, image="ollama/ollama"):
    """Run ollama container; return True on success. Caller must stop/rm in teardown."""
    # Remove existing container from a previous failed run
    subprocess.run(
        [runtime, "rm", "-f", container_name],
        capture_output=True,
        timeout=10,
        check=False,
    )
    r = subprocess.run(
        [runtime, "run", "-d", "--name", container_name, "-p", f"{host_port}:11434", image],
        capture_output=True,
        text=True,
        timeout=60,
        check=False,
    )
    if r.returncode:
        return False
    return True


def _stop_ollama_container(runtime, container_name):
    """Stop and remove the container."""
    subprocess.run(
        [runtime, "stop", container_name],
        capture_output=True,
        timeout=15,
        check=False,
    )
    subprocess.run(
        [runtime, "rm", "-f", container_name],
        capture_output=True,
        timeout=10,
        check=False,
    )


def _wait_ollama_ready(base_url, timeout=60):
    """Return True when GET base_url/api/tags returns 200."""
    deadline = time.monotonic() + timeout
    url = f"{base_url.rstrip('/')}/api/tags"
    while time.monotonic() < deadline:
        code, _ = helpers.run_curl_with_status("GET", url, timeout=5)
        if code == 200:
            return True
        time.sleep(1)
    return False


def _ollama_ensure_model(runtime, container_name, model):
    """Pull model if not listed, warm up (one prompt); return True if model is available."""
    r = subprocess.run(
        [runtime, "exec", container_name, "ollama", "list"],
        capture_output=True,
        text=True,
        timeout=15,
        check=False,
    )
    if model not in (r.stdout or ""):
        r = subprocess.run(
            [runtime, "exec", container_name, "ollama", "pull", model],
            capture_output=True,
            text=True,
            timeout=600,
            check=False,
        )
        if r.returncode:
            return False
    # Warm up: load model so first PMA chat request does not timeout
    subprocess.run(
        [runtime, "exec", container_name, "ollama", "run", model, "Say OK"],
        capture_output=True,
        text=True,
        timeout=60,
        check=False,
    )
    return True


# --- Functional tests: PMA + proxy with real Ollama ---


class TestProxyPmaWithRealOllama(unittest.TestCase):
    """Functional test: real Ollama + PMA + worker proxy; chat returns LLM content.

    Starts: Ollama container -> PMA -> worker-api proxy. Requires podman/docker,
    ollama/ollama image. Skips if runtime or image unavailable.
    """

    tags = ["suite_proxy_pma", "suite_worker_node", "inference", "pma_inference", "pma"]

    _runtime = None
    _ollama_container = None
    _pma_proc = None
    _worker_proc = None
    _worker_base = None

    @classmethod
    def setUpClass(cls):
        ollama_port = config.PROXY_PMA_TEST_OLLAMA_PORT
        pma_port = config.PROXY_PMA_TEST_PMA_PORT_REAL_OLLAMA
        worker_port = config.PROXY_PMA_TEST_WORKER_PORT_REAL_OLLAMA
        container_name = config.PROXY_PMA_TEST_OLLAMA_CONTAINER_NAME
        ollama_url = f"http://127.0.0.1:{ollama_port}"
        pma_base = f"http://127.0.0.1:{pma_port}"
        cls._pma_base = pma_base
        cls._worker_base = f"http://127.0.0.1:{worker_port}"

        if not os.path.isfile(config.PMA_BIN):
            raise unittest.SkipTest(f"PMA binary not found: {config.PMA_BIN}")
        if not os.path.isfile(config.WORKER_API_BIN):
            raise unittest.SkipTest(f"worker-api binary not found: {config.WORKER_API_BIN}")

        cls._runtime = helpers.container_runtime()
        if not cls._runtime:
            raise unittest.SkipTest("no container runtime (podman/docker) for Ollama")

        if not _start_ollama_container(
            cls._runtime, container_name, ollama_port
        ):
            raise unittest.SkipTest("failed to start Ollama container (image ollama/ollama?)")
        cls._ollama_container = container_name

        if not _wait_ollama_ready(ollama_url, timeout=60):
            _stop_ollama_container(cls._runtime, container_name)
            cls._ollama_container = None
            raise unittest.SkipTest("Ollama did not become ready (api/tags)")

        if not _ollama_ensure_model(
            cls._runtime, container_name, config.OLLAMA_E2E_MODEL
        ):
            _stop_ollama_container(cls._runtime, container_name)
            cls._ollama_container = None
            raise unittest.SkipTest(
                f"failed to pull model {config.OLLAMA_E2E_MODEL}"
            )

        # Start PMA with OLLAMA_BASE_URL pointing at real Ollama
        agents_dir = os.path.join(config.PROJECT_ROOT, "agents")
        env_pma = os.environ.copy()
        env_pma["PMA_LISTEN_ADDR"] = f"127.0.0.1:{pma_port}"
        env_pma["PMA_ROLE"] = "project_manager"
        env_pma["OLLAMA_BASE_URL"] = ollama_url
        env_pma["INFERENCE_MODEL"] = config.OLLAMA_E2E_MODEL
        with _popen_keepalive(
            [
                config.PMA_BIN,
                "--listen",
                f"127.0.0.1:{pma_port}",
                "--role",
                "project_manager",
            ],
            cwd=agents_dir,
            env=env_pma,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.PIPE,
        ) as proc:
            cls._pma_proc = proc
            if not _wait_http(f"{pma_base}/healthz", timeout=15):
                cls._pma_proc.terminate()
                cls._pma_proc.wait(timeout=5)
                _stop_ollama_container(cls._runtime, container_name)
                cls._ollama_container = None
                raise unittest.SkipTest("PMA did not become ready (healthz)")

        # Start worker-api with target to PMA
        state_dir = os.path.join(config.PROJECT_ROOT, "tmp", "proxy-pma-test-state")
        os.makedirs(state_dir, 0o700, exist_ok=True)
        targets_json = json.dumps({"pma-main": pma_base})
        env_worker = os.environ.copy()
        env_worker["WORKER_API_BEARER_TOKEN"] = _PROXY_PMA_E2E_BEARER
        env_worker["WORKER_MANAGED_SERVICE_TARGETS_JSON"] = targets_json
        env_worker["LISTEN_ADDR"] = f"127.0.0.1:{worker_port}"
        env_worker["WORKER_API_STATE_DIR"] = state_dir
        env_worker["WORKER_MANAGED_PROXY_UPSTREAM_TIMEOUT_SEC"] = "120"
        with _popen_keepalive(
            [config.WORKER_API_BIN],
            cwd=config.PROJECT_ROOT,
            env=env_worker,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.PIPE,
        ) as proc:
            cls._worker_proc = proc
            if not _wait_http(f"{cls._worker_base}/healthz", timeout=15):
                cls._worker_proc.terminate()
                cls._worker_proc.wait(timeout=5)
                if cls._pma_proc:
                    cls._pma_proc.terminate()
                    cls._pma_proc.wait(timeout=5)
                _stop_ollama_container(cls._runtime, container_name)
                cls._ollama_container = None
                raise unittest.SkipTest("worker-api did not become ready (healthz)")

    @classmethod
    def tearDownClass(cls):
        if cls._worker_proc:
            cls._worker_proc.terminate()
            cls._worker_proc.wait(timeout=5)
        if cls._pma_proc:
            cls._pma_proc.terminate()
            cls._pma_proc.wait(timeout=5)
        if cls._runtime and cls._ollama_container:
            _stop_ollama_container(cls._runtime, cls._ollama_container)

    def test_proxy_pma_real_ollama_returns_completion_content(self):
        """Proxy -> PMA -> real Ollama: chat completion returns 200 with non-empty LLM content."""
        chat_body = build_chat_completion_body(
            [{"role": "user", "content": "Reply with exactly the word OK and nothing else."}]
        )
        proxy_body = build_proxy_request("POST", "/internal/chat/completion", chat_body)
        code, resp_body = helpers.run_curl_with_status(
            "POST",
            f"{self._worker_base}/v1/worker/managed-services/pma-main/proxy:http",
            data=json.dumps(proxy_body),
            headers={
                "Content-Type": "application/json",
                "Authorization": f"Bearer {_PROXY_PMA_E2E_BEARER}",
            },
            timeout=120,
        )
        msg = f"proxy returned {code}: {resp_body!r}"
        if code != 200 and hasattr(self, "_pma_base") and self._pma_base:
            direct_code, direct_body = helpers.run_curl_with_status(
                "POST",
                f"{self._pma_base}/internal/chat/completion",
                data=json.dumps(chat_body),
                headers={"Content-Type": "application/json"},
                timeout=60,
            )
            msg += f"\ndirect PMA /internal/chat/completion: {direct_code} {direct_body!r}"
        self.assertEqual(code, 200, msg)
        status, raw = parse_proxy_response(resp_body)
        self.assertEqual(status, 200, f"upstream status {status}: {raw!r}")
        out = json.loads(raw.decode())
        self.assertIn("content", out)
        self.assertIsInstance(out["content"], str)
        self.assertGreater(
            len(out["content"].strip()),
            0,
            "expected non-empty LLM completion content",
        )


def _wait_http(url, timeout=10):
    """Return True when GET url returns 200 within timeout."""
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        code, _ = helpers.run_curl_with_status("GET", url, timeout=2)
        if code == 200:
            return True
        time.sleep(0.2)
    return False
