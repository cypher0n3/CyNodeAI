# Failed E2E Report: e2e_0610_sse_streaming.test_responses_stream_returns_sse

## 1 Summary

Test `e2e_0610_sse_streaming.TestSSEStreaming.test_responses_stream_returns_sse` failed because the accumulated content from the `/v1/responses` SSE stream was empty, and one of the events was an error object: `stream_error` with message "context deadline exceeded (Client.Timeout or context cancellation while reading body)".
The test parses events, extracts content from choices[0].delta.content, and asserts full_content.strip() is non-empty; it also asserts no think blocks.
The events list included a valid initial response_id and a chunk, then the error event; content accumulated was empty.

## 2 Why the Failure Occurred

- **Observed:** `AssertionError: '' is not true : Accumulated /v1/responses stream content is empty; events=['{"response_id":"resp_..."}', '{"id":"...","object":"chat.completion.chunk",...}', '{"error":{"code":"stream_error","message":"context deadline exceeded ...","type":"cynodeai_error"}}']`
- **Root cause:** The stream contained three events: response_id, one chat.completion.chunk (with delta.role "assistant" but likely no or minimal content), and then a stream_error.
  The upstream (PMA or inference) hit a context deadline while reading body; the gateway sent the error event and the stream did not deliver enough content before the error.
- **Effect:** full_content was empty (or only role delta); the assertion that accumulated content is non-empty failed.

## 3 Specific Code Paths Involved

Relevant code paths:

### 3.1 Python Test Path

- [e2e_0610_sse_streaming.py](../../../scripts/test_scripts/e2e_0610_sse_streaming.py) lines 156-217: POST `/v1/responses` stream=true, parse SSE, assert found_done and events length, loop events and add choices[0].delta.content to full_content, assert full_content.strip() and no think blocks.

### 3.2 Gateway and Responses API

- User API Gateway: POST /v1/responses with stream=true; native responses event shape and response_id; relays stream from PMA.
- PMA / inference: stream body; context deadline or timeout causes gateway to emit stream_error; content may not be fully streamed before deadline.

## 4 Related Specs, Requirements, and Feature Files

Traceability for this test:

### 4.1 Requirements References

- [REQ-USRGWY-0149](../../requirements/usrgwy.md), [REQ-USRGWY-0150](../../requirements/usrgwy.md): Streaming for POST /v1/responses.

### 4.2 Tech Specs

- [user_api_gateway.md](../../tech_specs/user_api_gateway.md): /v1/responses and CYNAI.USRGWY.OpenAIChatApi.Streaming.
- [2026-03-15_streaming_specs_implementation_plan.md](../2026-03-15_streaming_specs_implementation_plan.md): Responses stream and relay.

### 4.3 Feature Files

- E2E streaming and responses endpoint.

## 5 Implementation Deviation

- **Spec/requirement intent:** POST /v1/responses stream MUST return SSE events and [DONE] with non-empty accumulated content; errors should not truncate content without delivering usable content.
- **Observed behavior:** Stream delivered response_id and one chunk (role only or minimal content), then stream_error (context deadline exceeded); accumulated content was empty.
- **Deviation:** Completion body is not fully streamed before context deadline; gateway or upstream times out and sends stream_error, leaving the client with no content.

## 6 What Needs to Be Fixed in the Implementation

The following describes the exact implementation behavior and required changes.

### 6.1 Root Cause (Implemented Behavior)

- POST /v1/responses with stream uses the same streaming path as chat completions: [orchestrator/internal/handlers/openai_chat.go](../../../orchestrator/internal/handlers/openai_chat.go) and [orchestrator/internal/pmaclient/client.go](../../../orchestrator/internal/pmaclient/client.go) with **120s** HTTP client timeout.
  The handler does emit response_id and chunks via `completeViaPMAStream`, but if the PMA stream does not complete within 120s, the client returns an error and the handler calls `writeSSEError(w, "stream_error", err.Error())`, so the client sees minimal or no content and then stream_error.

### 6.2 What is Not Implemented or is Misconfigured

- Same timeout mismatch as chat completions streaming: gateway allows longer completion time but the PMA client uses 120s.

### 6.3 Exact Code or Config Changes

- Same as [2026-03-16_e2e_127_test_chat_completions_stream_returns_sse.md](2026-03-16_e2e_127_test_chat_completions_stream_returns_sse.md) section 6: increase the PMA client timeout (e.g. to 200s) when used for streaming from the gateway so the /v1/responses stream can deliver full content before [DONE].
