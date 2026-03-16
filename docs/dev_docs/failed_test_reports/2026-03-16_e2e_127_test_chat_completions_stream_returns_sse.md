# Failed E2E Report: e2e_127_sse_streaming.test_chat_completions_stream_returns_sse

## 1 Summary

Test `e2e_127_sse_streaming.TestSSEStreaming.test_chat_completions_stream_returns_sse` failed because the first SSE event data parsed as a JSON object had `object` not equal to `chat.completion.chunk`; it was an error object: `stream_error` with message "context deadline exceeded (Client.Timeout or context cancellation while reading body)".
The test expects a 200 response, SSE stream ending with [DONE], and each event to be a `chat.completion.chunk`; the stream included an error chunk instead.

## 2 Why the Failure Occurred

- **Observed:** `AssertionError: None != 'chat.completion.chunk' : chunk.object wrong: {'error': {'code': 'stream_error', 'message': 'context deadline exceeded (Client.Timeout or context cancellation while reading body)', 'type': 'cynodeai_error'}}`
- **Root cause:** The gateway (or upstream PMA/inference) hit a context deadline or client timeout while reading the completion body; it sent an SSE event with an error payload instead of (or in addition to) normal chunks.
- **Effect:** The test iterates over events and asserts each chunk has `object == "chat.completion.chunk"`; the first (or an early) chunk was the error object, so the assertion failed.

## 3 Specific Code Paths Involved

Relevant code paths:

### 3.1 Python Test Path

- [e2e_127_sse_streaming.py](../../../scripts/test_scripts/e2e_127_sse_streaming.py) lines 88-154: POST `/v1/chat/completions` with stream=true, parse SSE, assert chunk.object is `chat.completion.chunk` (line 136), accumulate content, assert non-empty and no think blocks.

### 3.2 Gateway and Streaming

- User API Gateway: `POST /v1/chat/completions` with stream=true; relays SSE from PMA or inference backend.
- Orchestrator/PMA: streaming completion; context deadline or timeout while reading from upstream causes gateway to emit stream_error.

### 3.3 Backend Path

- PMA streaming and inference backend; if the upstream stream is slow or the gateway cancels the context (e.g. client disconnect or server-side timeout), the gateway returns a cynodeai_error stream_error event.

## 4 Related Specs, Requirements, and Feature Files

Traceability for this test:

### 4.1 Requirements References

- [REQ-USRGWY-0149](../../requirements/usrgwy.md): Streaming chat responses for POST /v1/chat/completions when stream=true.
- [REQ-USRGWY-0150](../../requirements/usrgwy.md): Streaming behavior and event shape (as referenced in test docstring).

### 4.2 Tech Specs

- [openai_compatible_chat_api.md](../../tech_specs/openai_compatible_chat_api.md): Chat completions streaming and SSE format.
- [user_api_gateway.md](../../tech_specs/user_api_gateway.md): Streaming and error handling.
- [2026-03-15_streaming_specs_implementation_plan.md](../2026-03-15_streaming_specs_implementation_plan.md): PMA streaming relay and gateway contract.

### 4.3 Feature Files

- E2E and feature coverage for chat completions streaming (e2e_127, streaming tags).

## 5 Implementation Deviation

- **Spec/requirement intent:** Stream MUST return SSE events with `object: "chat.completion.chunk"` and non-empty content until [DONE]; errors should be handled without replacing chunk shape with an error object in the same stream.
- **Observed behavior:** The stream contained an event whose data was an error object (stream_error, context deadline exceeded); the test correctly rejects it as not a chunk.
- **Deviation:** Gateway (or relay) emits a stream_error event when context is cancelled or timeout occurs while reading body; the implementation may not complete the stream with valid chunks before the deadline, or may inject the error event in a way that breaks the chunk-only expectation of the test.

## 6 What Needs to Be Fixed in the Implementation

The following describes the exact implementation behavior and required changes.

### 6.1 Root Cause (Implemented Behavior)

- In [orchestrator/internal/handlers/openai_chat.go](../../../orchestrator/internal/handlers/openai_chat.go), when `stream=true` and model is `cynodeai.pm`, the handler calls `tryPMAStream` (line 185), which calls `completeViaPMAStream` (line 325).
  If that returns an error, the handler calls `writeSSEError(w, "stream_error", err.Error())` (line 138).
  So any error from the PMA stream (including context deadline exceeded) is sent as a single SSE data line containing a JSON error object, then [DONE].
  The test uses a 120s read timeout; the gateway uses `timeoutCtx` with `chatCompletionTimeout` (200s) for the overall completion.
  The PMA client in [orchestrator/internal/pmaclient/client.go](../../../orchestrator/internal/pmaclient/client.go) uses `defaultTimeout = 120 * time.Second` for the HTTP client (lines 18, 111).
  So when the gateway calls `CallChatCompletionStreamWithCallbacks`, the outbound request to the PMA has a **120s** HTTP client timeout.
  If the PMA (or worker proxy) does not start streaming within 120s, or the first token takes longer than 120s, the client returns an error (e.g. context deadline exceeded or read timeout).
  The gateway then writes `stream_error` with that error message.
  So the **exact** cause is: completion does not produce data within the **pmaclient** 120s timeout, so the gateway never sends normal chunks and only sends the error event.

### 6.2 What is Not Implemented or is Misconfigured

- **Timeout mismatch:** The gateway allows 200s for completion (`chatCompletionTimeout`) but the PMA client uses 120s.
  So the upstream (PMA + inference) must produce the first token and stream to completion within 120s or the client times out and the gateway emits stream_error.
  Either: (1) increase the PMA client timeout (e.g. to 200s or match chatCompletionTimeout) when called from the gateway so that the stream has time to complete, or (2) ensure the PMA and inference backend respond and stream within 120s (e.g. model loaded, no long blocking before first token).
- **Error handling contract:** The spec expects the stream to deliver only `chat.completion.chunk` events until [DONE].
  When an error occurs, the implementation correctly sends a structured error, but the test treats any non-chunk event as failure.
  So either the stream must complete successfully within the allowed time (fix timeouts/performance), or the test could be relaxed to accept a terminal error event after some chunks (not required by current spec).

### 6.3 Exact Code or Config Changes Required

- **Increase pmaclient timeout when used from gateway:** In `pmaclient.CallChatCompletionStreamWithCallbacks`, the default `http.Client{Timeout: defaultTimeout}` (120s) is used when `client == nil`.
  The gateway passes `nil` for the client.
  So either: (1) have the gateway pass an HTTP client with timeout of at least `chatCompletionTimeout` (200s) when calling the PMA stream, or (2) increase `defaultTimeout` in pmaclient to match (with care for other callers).
  That gives the PMA stream up to 200s to complete before the gateway reports stream_error.
- **Ensure PMA/inference respond in time:** If the PMA or inference backend is slow (cold model, network), ensure they are warmed up before the test or that the first token is sent well within 120s.
  Misconfiguration (e.g. wrong PMA URL, worker not ready) can also cause long waits or connection failures that surface as timeouts.
