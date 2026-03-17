# Failed E2E Report: e2e_0550_chat_sequential_messages.test_chat_sequential_two_turns

## 1 Summary

Test `e2e_0550_chat_sequential_messages.TestChatSequentialMessages.test_chat_sequential_two_turns` failed because the first POST to `/v1/chat/completions` (two-turn chat: first message "Say one word: first") did not return success; the response body contained an error with code `cynodeai_completion_timeout` and message "Completion did not finish before the maximum wait duration".
The test asserts ok (2xx) at line 68; ok was False and body contained the structured error.

## 2 Why the Failure Occurred

- **Observed:** `AssertionError: False is not true : first chat request failed: {"error":{"code":"cynodeai_completion_timeout","message":"Completion did not finish before the maximum wait duration","param":null,"type":"cynodeai_error"}}`
- **Root cause:** The gateway (or PMA) returned HTTP non-2xx with a structured error: completion did not finish before the maximum wait duration (server-side timeout).
- **Effect:** The test correctly received a structured error but treats non-ok as failure for the first turn; it never reached the second turn or content assertions.

## 3 Specific Code Paths Involved

Relevant code paths:

### 3.1 Python Test Path

- [e2e_0550_chat_sequential_messages.py](../../../scripts/test_scripts/e2e_0550_chat_sequential_messages.py) lines 42-68: First _chat_request with messages [{"role":"user","content":"Say one word: first"}]; asserts ok at line 68, then extracts first_content and asserts non-empty.

### 3.2 Gateway and Completion

- User API Gateway: POST /v1/chat/completions; when completion exceeds server-side max wait duration, returns error code cynodeai_completion_timeout.
- REQ-USRGWY-0130 and chat threads/messages; completion timeout is a valid structured error but the test expects a successful first turn.

## 4 Related Specs, Requirements, and Feature Files

Traceability for this test:

### 4.1 Requirements References

- [REQ-USRGWY-0130](../../requirements/usrgwy.md): Chat and thread/message handling (per test docstring).

### 4.2 Tech Specs

- [openai_compatible_chat_api.md](../../tech_specs/openai_compatible_chat_api.md): Chat completions and error format.
- [chat_threads_and_messages.md](../../tech_specs/chat_threads_and_messages.md): Sequential message handling.

### 4.3 Feature Files

- Sequential chat E2E coverage.

## 5 Implementation Deviation

- **Spec/requirement intent:** Sequential two-turn chat MUST return a successful completion for each turn (or a clear error that the test can skip on inference-unavailable).
- **Observed behavior:** First turn returned a structured error (cynodeai_completion_timeout) instead of completion content; test does not treat this as skip (only orchestrator_inference_failed, completion failed, model_unavailable trigger skip).
- **Deviation:** Server-side completion timeout is hit before the first turn completes; the implementation returns the correct error type but the test expects a successful reply for the reliability/sequential scenario.

## 6 What Needs to Be Fixed in the Implementation

The following describes why the first turn times out and what to change.

### 6.1 Root Cause (Implemented Behavior)

- The first turn returns **cynodeai_completion_timeout**, so the gateway does return a structured error.
  The completion path in [orchestrator/internal/handlers/openai_chat.go](../../../orchestrator/internal/handlers/openai_chat.go) and [orchestrator/internal/pmaclient/client.go](../../../orchestrator/internal/pmaclient/client.go) uses a **120s** HTTP client timeout; when the PMA does not complete in time, the gateway surfaces that as a timeout error.
  The test expects a successful reply (or a skip only for orchestrator_inference_failed, completion failed, model_unavailable); it does not treat cynodeai_completion_timeout as skip.

### 6.2 What is Not Implemented or is Misconfigured

- **Completion too slow:** The PMA/inference backend does not complete within 120s.
  Fix: increase the PMA client timeout (e.g. to 200s) when used from the gateway, or warm up inference so the first turn completes in time.
  **Test skip logic (optional):** If the spec allows treating completion_timeout as "inference unavailable" for this test, add cynodeai_completion_timeout to the skip conditions so the test can pass when the environment is slow.

### 6.3 Exact Code or Config Changes

- Same as e2e_0530: use a longer HTTP timeout for pmaclient when called from the gateway.
- Optionally extend the test's skip list to include cynodeai_completion_timeout so sequential test is resilient to slow environments.
