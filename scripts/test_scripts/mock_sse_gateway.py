# Mock SSE gateway for contract validation and unit tests.
# Serves configurable SSE streams. E2E tests (e2e_202, e2e_203, e2e_204) use real stack.

import json
import threading
import time
from http.server import BaseHTTPRequestHandler, HTTPServer


def _sse_line(prefix, value):
    """Format one SSE line (event: or data:)."""
    return f"{prefix} {value}\n".encode("utf-8")


def _sse_event(event_type, data_obj):
    """Yield bytes for one named SSE event."""
    yield _sse_line("event:", event_type)
    yield _sse_line("data:", json.dumps(data_obj))
    yield b"\n"


def _sse_data(data_obj):
    """Yield bytes for one unnamed data line."""
    yield _sse_line("data:", json.dumps(data_obj))
    yield b"\n"


class MockSSEGatewayHandler(BaseHTTPRequestHandler):
    """Handles POST /v1/chat/completions and /v1/responses with stream=true.

    Response shape is controlled by the handler class attribute `stream_mode`:
    - "amendment_before_done": amendment event then chunk then [DONE]
    - "heartbeat_then_content": heartbeat then content chunks then [DONE]
    - "iteration_and_response_id": iteration_start then response_id then [DONE]
    - "default": one chunk then [DONE]
    """

    stream_mode = "default"
    response_id = "mock-response-id-001"

    def log_message(self, format, *args):  # pylint: disable=arguments-differ,redefined-builtin
        pass  # Silence logging; BaseHTTPRequestHandler requires these params

    def do_POST(self):  # pylint: disable=invalid-name
        if self.path in ("/v1/chat/completions", "/v1/responses"):
            # Stream SSE (caller sends stream=true in body)
            self.send_response(200)
            self.send_header("Content-Type", "text/event-stream")
            self.end_headers()
            self._write_stream()
            return
        self.send_response(404)
        self.end_headers()

    def _write_chunk(self, chunk):
        """Write chunk parts; return False on BrokenPipe/ConnectionReset."""
        for part in chunk:
            try:
                self.wfile.write(part)
                self.wfile.flush()
            except (BrokenPipeError, ConnectionResetError):
                return False
        return True

    def _write_stream(self):
        mode = getattr(self.__class__, "stream_mode", "default")
        rid = getattr(self.__class__, "response_id", "mock-response-id-001")

        def wc(c):
            return self._write_chunk([c])

        def wc_multi(parts):
            return all(wc(p) for p in parts)

        if mode == "amendment_before_done":
            self._stream_amendment_before_done(wc, wc_multi)
        elif mode == "heartbeat_then_content":
            self._stream_heartbeat_then_content(wc, wc_multi)
        elif mode == "iteration_and_response_id":
            self._stream_iteration_and_response_id(rid, wc, wc_multi)
        else:
            self._stream_default(wc, wc_multi)

    def _stream_amendment_before_done(self, wc, wc_multi):
        for part in _sse_event("cynodeai.amendment", {"redacted": "visible"}):
            if not wc(part):
                return
        chunk = {"object": "chat.completion.chunk", "choices": [{"delta": {"content": "ok"}}]}
        if not wc_multi(_sse_data(chunk)):
            return
        wc(b"data: [DONE]\n\n")

    def _stream_heartbeat_then_content(self, wc, wc_multi):
        for part in _sse_event("cynodeai.heartbeat", {}):
            if not wc(part):
                return
        chunk = {"object": "chat.completion.chunk", "choices": [{"delta": {"content": "done"}}]}
        if not wc_multi(_sse_data(chunk)):
            return
        wc(b"data: [DONE]\n\n")

    def _stream_iteration_and_response_id(self, rid, wc, wc_multi):
        for part in _sse_event("cynodeai.iteration_start", {}):
            if not wc(part):
                return
        if not wc_multi(_sse_data({"response_id": rid, "choices": [{"delta": {"content": "hi"}}]})):
            return
        if not wc_multi(_sse_data({"response_id": rid, "choices": [{"delta": {"content": ""}}]})):
            return
        wc(b"data: [DONE]\n\n")

    def _stream_default(self, wc, wc_multi):
        chunk = {"object": "chat.completion.chunk", "choices": [{"delta": {"content": "ok"}}]}
        wc_multi(_sse_data(chunk))
        wc(b"data: [DONE]\n\n")


def start_mock_sse_gateway(stream_mode="default", response_id="mock-response-id-001"):
    """Start mock SSE gateway in a daemon thread. Returns (base_url, server, thread)."""
    handler = type(
        "Handler",
        (MockSSEGatewayHandler,),
        {"stream_mode": stream_mode, "response_id": response_id},
    )
    server = HTTPServer(("127.0.0.1", 0), handler)
    port = server.server_address[1]
    base_url = f"http://127.0.0.1:{port}"
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    time.sleep(0.1)
    return base_url, server, thread
