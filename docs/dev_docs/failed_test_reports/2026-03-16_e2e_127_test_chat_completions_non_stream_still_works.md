# Failed E2E Report: e2e_127_sse_streaming.test_chat_completions_non_stream_still_works

## 1 Summary

Test `e2e_127_sse_streaming.TestSSEStreaming.test_chat_completions_non_stream_still_works` failed because the non-streaming POST to `/v1/chat/completions` (stream=false) did not complete within the 120-second read timeout.
The test retries up to 3 times; after retries it failed with "Non-stream request failed after retries: ...
Read timed out. (read timeout=120)".

## 2 Why the Failure Occurred

- **Observed:** `AssertionError: Non-stream request failed after retries: HTTPConnectionPool(host='localhost', port=12080): Read timed out. (read timeout=120)`
- **Root cause:** `requests.post(..., timeout=_SSE_TIMEOUT_SEC)` with _SSE_TIMEOUT_SEC=120; the server did not respond with a complete JSON body within 120 seconds.
- **Effect:** requests raised ReadTimeout on all attempts; the test failed before asserting status code or response body.

## 3 Specific Code Paths Involved

Relevant code paths:

### 3.1 Python Test Path

- [e2e_127_sse_streaming.py](../../../scripts/test_scripts/e2e_127_sse_streaming.py) lines 219-242: POST `/v1/chat/completions` with stream=false (no stream key or stream=False), timeout=120; on RequestException retry; after 3 attempts fail with last_exc.

### 3.2 Gateway and Completion

- User API Gateway: POST /v1/chat/completions without stream returns a single JSON response; gateway waits for full completion from PMA/inference.
- PMA and inference: non-streaming path blocks until completion; if inference or PMA is slow, the response exceeds 120s and the client times out.

## 4 Related Specs, Requirements, and Feature Files

Traceability for this test:

### 4.1 Requirements References

- [REQ-USRGWY-0149](../../requirements/usrgwy.md): Streaming support; non-stream remains valid.
- Chat completions API: non-stream MUST still return JSON (regression test).

### 4.2 Tech Specs

- [openai_compatible_chat_api.md](../../tech_specs/openai_compatible_chat_api.md): Chat completions with and without stream.
- [user_api_gateway.md](../../tech_specs/user_api_gateway.md): Completion timeout and error handling.

### 4.3 Feature Files

- E2E streaming suite; non-stream regression.

## 5 Implementation Deviation

- **Spec/requirement intent:** POST /v1/chat/completions without stream=true MUST return a complete JSON response (choices, message content) within a reasonable time.
- **Observed behavior:** The server did not respond within 120 seconds; client read timeout.
- **Deviation:** Non-streaming completion path (gateway/PMA/inference) takes longer than 120s in the E2E environment; may need longer server-side timeout or faster completion path.

## 6 What Needs to Be Fixed in the Implementation

The following describes the non-streaming path and timeout.

### 6.1 Root Cause (Implemented Behavior)

- Non-streaming POST /v1/chat/completions is handled in [orchestrator/internal/handlers/openai_chat.go](../../../orchestrator/internal/handlers/openai_chat.go) and forwarded via [orchestrator/internal/pmaclient/client.go](../../../orchestrator/internal/pmaclient/client.go) `CallChatCompletion`, which uses `http.Client{Timeout: defaultTimeout}` with **defaultTimeout = 120s**.
  The test client also has a 120s read timeout.
  If the PMA or inference backend does not return the full completion within 120s, the gateway never responds in time.

### 6.2 What is Not Implemented or is Misconfigured

- **PMA client timeout:** 120s may be too short for a full non-streaming completion (e.g. long reply).
- The gateway's chatCompletionTimeout (200s) is not applied to the PMA client's HTTP transport.

### 6.3 Exact Code or Config Changes

- Use an HTTP client with timeout of at least 200s (or match chatCompletionTimeout) when calling `pmaclient.CallChatCompletion` from the gateway, so non-streaming completions have time to finish.
  Alternatively, ensure E2E inference and PMA respond within 120s.
