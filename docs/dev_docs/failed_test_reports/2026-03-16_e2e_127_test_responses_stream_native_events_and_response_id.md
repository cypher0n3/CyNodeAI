# Failed E2E Report: e2e_0610_sse_streaming.test_responses_stream_uses_native_responses_events_and_exposes_streamed_response_id

## 1 Summary

Test `e2e_0610_sse_streaming.TestSSEStreaming.test_responses_stream_uses_native_responses_events_and_exposes_streamed_response_id` failed with a ReadTimeout (ERROR in the test run): the POST to `/v1/responses` with stream=true did not complete within 120 seconds.
The test expects 200, typed SSE events, [DONE], and at least one response_id in the stream; the HTTP connection read timed out before the test could finish parsing.

## 2 Why the Failure Occurred

- **Observed:** `requests.exceptions.ReadTimeout: HTTPConnectionPool(host='localhost', port=12080): Read timed out. (read timeout=120)` (traceback from requests.post at line 350 with timeout=_SSE_TIMEOUT_SEC).
- **Root cause:** The stream response from `/v1/responses` did not complete (e.g. [DONE] not received) within 120 seconds; the client closed the connection on read timeout.
- **Effect:** The test raised ReadTimeout before asserting on event shape or response_id; reported as an ERROR rather than FAIL.

## 3 Specific Code Paths Involved

Relevant code paths:

### 3.1 Python Test Path

- [e2e_0610_sse_streaming.py](../../../scripts/test_scripts/e2e_0610_sse_streaming.py) lines 336-363 (approx): POST `/v1/responses` stream=true, timeout=120; parse_sse_stream_typed(resp); assert found_done, collect response_ids from events, assert at least one response_id.
  The request itself timed out before response was fully read.

### 3.2 Gateway and Responses Stream

- User API Gateway: POST /v1/responses stream; must use native responses event shape and expose streamed response_id per spec.
- PMA streaming: if upstream does not finish within 120s or gateway does not send [DONE], the client will timeout.

## 4 Related Specs, Requirements, and Feature Files

Traceability for this test:

### 4.1 Requirements References

- [REQ-USRGWY-0149](../../requirements/usrgwy.md), [REQ-USRGWY-0150](../../requirements/usrgwy.md): Streaming and event shape for /v1/responses.

### 4.2 Tech Specs

- [user_api_gateway.md](../../tech_specs/user_api_gateway.md): CYNAI.USRGWY.OpenAIChatApi.StreamingPerEndpointSSEFormat; native responses events and response_id.
- [2026-03-15_streaming_specs_implementation_plan.md](../2026-03-15_streaming_specs_implementation_plan.md): Responses stream contract.

### 4.3 Feature Files

- E2E streaming for /v1/responses.

## 5 Implementation Deviation

- **Spec/requirement intent:** Responses stream MUST use native event shape and expose streamed response_id and MUST complete with [DONE] within a reasonable time.
- **Observed behavior:** The stream did not complete within 120 seconds; client read timeout.
- **Deviation:** The /v1/responses stream path (gateway/PMA/inference) does not finish within the test timeout; same underlying completion/timeout issue as other streaming tests.

## 6 What Needs to Be Fixed in the Implementation

The following describes the /v1/responses stream path and timeout.

### 6.1 Root Cause (Implemented Behavior)

- The /v1/responses stream is served by the same handler path as chat completions streaming; it uses [orchestrator/internal/pmaclient/client.go](../../../orchestrator/internal/pmaclient/client.go) with **120s** HTTP client timeout.
  response_id and native event shape are already emitted by `completeViaPMAStream` (response_id early, then chunks).
  The stream does not complete within 120s, so the client times out.

### 6.2 What is Not Implemented or is Misconfigured

- **Timeout:** PMA client 120s timeout; completion takes longer in E2E.

### 6.3 Exact Code or Config Changes

- Increase the PMA client timeout when used for /v1/responses streaming (e.g. to 200s) so the stream can complete and the test can assert on response_id and [DONE].
  Same fix as other e2e_0610 streaming reports.
