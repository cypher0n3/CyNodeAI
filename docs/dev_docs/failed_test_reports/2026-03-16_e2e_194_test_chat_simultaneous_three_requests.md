# Failed E2E Report: e2e_0560_chat_simultaneous_messages.test_chat_simultaneous_three_requests

## 1 Summary

Test `e2e_0560_chat_simultaneous_messages.TestChatSimultaneousMessages.test_chat_simultaneous_three_requests` failed because fewer than 2 of the 3 concurrent POST requests to `/v1/chat/completions` succeeded; all 3 returned non-2xx (successes=0, failures: [(False, 'non-2xx'), (False, 'non-2xx'), (False, 'non-2xx')]).
The test runs three one-shot chat requests in parallel and asserts at least 2 succeed.

## 2 Why the Failure Occurred

- **Observed:** `AssertionError: 0 not greater than or equal to 2 : expected >=2 concurrent successes; failures: [(False, 'non-2xx'), (False, 'non-2xx'), (False, 'non-2xx')]`
- **Root cause:** Each of the 3 concurrent POST /v1/chat/completions requests returned a non-2xx status (e.g. 504, 500, or timeout mapped to error); _one_chat_request returns (False, "non-2xx") when ok is False and no error message is extracted.
- **Effect:** successes was 0; the assertion at line 86 failed.

## 3 Specific Code Paths Involved

Relevant code paths:

### 3.1 Python Test Path

- [e2e_0560_chat_simultaneous_messages.py](../../../scripts/test_scripts/e2e_0560_chat_simultaneous_messages.py) lines 53-90: ThreadPoolExecutor runs 3 _one_chat_request calls (POST /v1/chat/completions with different messages); counts successes, asserts successes >= 2.

### 3.2 Gateway and Concurrency

- User API Gateway: must handle concurrent completions per REQ-ORCHES-0131, 0132; in this run all 3 requests failed (timeout or server error).
- Same completion path as other chat tests; under load or shared backend all requests may time out or return 5xx.

## 4 Related Specs, Requirements, and Feature Files

Traceability for this test:

### 4.1 Requirements References

- [REQ-ORCHES-0131](../../requirements/orches.md), [REQ-ORCHES-0132](../../requirements/orches.md): Chat reliability and gateway handling of concurrent completions.

### 4.2 Tech Specs

- [user_api_gateway.md](../../tech_specs/user_api_gateway.md): CYNAI.USRGWY.OpenAIChatApi.Reliability.
- [openai_compatible_chat_api.md](../../tech_specs/openai_compatible_chat_api.md): Chat completions.

### 4.3 Feature Files

- Concurrent chat E2E coverage.

## 5 Implementation Deviation

- **Spec/requirement intent:** Gateway MUST handle concurrent chat requests; at least 2 of 3 concurrent requests should succeed in a healthy environment.
- **Observed behavior:** All 3 concurrent requests returned non-2xx; 0 successes.
- **Deviation:** Under concurrency, completion path fails for all requests (timeout or 5xx); may be resource contention, single-threaded backend, or shared timeout not allowing 2 to complete.

## 6 What Needs to Be Fixed in the Implementation

The following describes concurrency and timeout behavior.

### 6.1 Root Cause (Implemented Behavior)

- Three concurrent POST /v1/chat/completions requests are sent.
  Each is handled by the gateway and PMA client with **120s** timeout ([orchestrator/internal/pmaclient/client.go](../../../orchestrator/internal/pmaclient/client.go)).
  If the backend (PMA or inference) serializes requests or has limited capacity, some or all requests may time out or get 5xx.
  The test expects at least 2 of 3 to succeed.

### 6.2 What is Not Implemented or is Misconfigured

- **Concurrency and timeout:** (1) Ensure the gateway and PMA can handle concurrent requests (no single global lock that serializes completions). (2) Ensure each request has enough time (120s per request may not be enough if three share the same inference and run sequentially server-side). (3) If the backend is single-threaded or inference allows only one request at a time, the test environment may need to be configured for concurrent inference or the test expectation relaxed.

### 6.3 Exact Code or Config Changes

- Verify the gateway and PMA do not serialize chat completion requests (e.g. one mutex around the whole completion path).
  Increase PMA client timeout so that when multiple requests are in flight, each has a chance to complete.
  If the inference backend is single-request, document that or add a queue with fair scheduling so at least 2 of 3 can complete within the test window.
