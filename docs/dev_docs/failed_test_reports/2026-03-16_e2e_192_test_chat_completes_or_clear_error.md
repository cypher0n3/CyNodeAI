# Failed E2E Report: e2e_192_chat_reliability.test_chat_completes_or_clear_error

## 1 Summary

Test `e2e_192_chat_reliability.TestChatReliability.test_chat_completes_or_clear_error` failed because the one-shot chat command did not return a timely reply or a clear structured error within 3 attempts (each with a 150-second timeout).
The test runs `cynork chat --message ping --plain` with timeout=150s; the last attempt's last_err showed the command timed out after 150 seconds.

## 2 Why the Failure Occurred

- **Observed:** `AssertionError: chat did not return a timely reply after 3 attempts. Last: "Command '...' timed out after 150 seconds"`
- **Root cause:** `helpers.run_cynork(..., timeout=CHAT_TIMEOUT_SEC)` with CHAT_TIMEOUT_SEC=150; the subprocess was killed by TimeoutExpired on each attempt, so _chat_reply_is_clean never succeeded.
- **Effect:** The test failed at the final assert with last_err showing the timeout message.

## 3 Specific Code Paths Involved

Relevant code paths:

### 3.1 Python Test Path

- [e2e_192_chat_reliability.py](../../../scripts/test_scripts/e2e_192_chat_reliability.py) lines 42-69: Up to 3 attempts of `run_cynork(["chat", "--message", "ping", "--plain"], ..., timeout=150)`; success when _chat_reply_is_clean(out, err); otherwise fail with last_err.

### 3.2 Gateway and Chat

- Same chat path as e2e_110: User API Gateway and PMA/inference; completion does not return within 150s in this run.

## 4 Related Specs, Requirements, and Feature Files

Traceability for this test:

### 4.1 Requirements References

- [REQ-ORCHES-0131](../../requirements/orches.md), [REQ-ORCHES-0132](../../requirements/orches.md): Chat timeouts and reliability (per test docstring).

### 4.2 Tech Specs

- [user_api_gateway.md](../../tech_specs/user_api_gateway.md): CYNAI.USRGWY.OpenAIChatApi.Timeouts, Reliability.
- [openai_compatible_chat_api.md](../../tech_specs/openai_compatible_chat_api.md): Chat completions.

### 4.3 Feature Files

- Chat reliability E2E coverage.

## 5 Implementation Deviation

- **Spec/requirement intent:** Chat MUST return a timely reply or a clear structured error; extended timeout (150s) and retries are used to tolerate cold inference.
- **Observed behavior:** Chat does not complete within 150 seconds; only timeout error is observed.
- **Deviation:** Completion path exceeds even the extended timeout in the E2E environment; no structured error is returned to the CLI (only subprocess timeout).

## 6 What Needs to Be Fixed in the Implementation

The following describes the exact implementation behavior and required changes.

### 6.1 Root Cause (Implemented Behavior)

- The test uses a 150s subprocess timeout.
- The gateway's chat path uses [orchestrator/internal/pmaclient/client.go](../../../orchestrator/internal/pmaclient/client.go) with **defaultTimeout = 120s**.
- So the server-side PMA client gives up at 120s; the gateway can return a structured error (e.g. cynodeai_completion_timeout or 504).
- If the CLI then waits for stdout, it may hit its own 150s timeout before the server response is fully consumed.
- So either: (1) the completion takes >120s and the gateway returns an error, and the CLI must surface that error (not just subprocess timeout), or (2) the completion takes >150s and the subprocess is killed before any response.

### 6.2 What is Not Implemented or is Misconfigured

- **PMA client timeout:** 120s may be too short for cold inference; increase to match chatCompletionTimeout (200s) when calling from the gateway so that completions have time to finish.
  **CLI error handling:** If the gateway returns a structured error, the CLI should print it so the test can assert "clear error" instead of timing out; ensure cynork chat prints the response body on non-2xx.

### 6.3 Exact Code or Config Changes

- Increase pmaclient HTTP timeout for chat (see e2e_110 report).
  Ensure the chat CLI prints the gateway error body on failure so the test can see a structured error when completion times out server-side.
