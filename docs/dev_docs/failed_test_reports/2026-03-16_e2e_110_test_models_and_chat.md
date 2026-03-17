# Failed E2E Report: e2e_0530_task_models_and_chat.test_models_and_chat

## 1 Summary

Test `e2e_0530_task_models_and_chat.TestModelsAndChat.test_models_and_chat` failed because the one-shot chat command (`cynork chat --message ping --plain`) did not complete successfully within the test's retries.
The test first asserts models list returns a list, then (unless inference smoke is skipped) runs chat up to 3 times; the final assertion at line 68 failed with "one-shot chat failed after retries" and stderr showing the chat command timed out after 120 seconds.

## 2 Why the Failure Occurred

- **Observed:** `AssertionError: False is not true : one-shot chat failed after retries: stdout='' stderr="Command '...' timed out after 120 seconds"`
- **Root cause:** `helpers.run_cynork(["chat", "--message", "ping", "--plain"], ..., timeout=120)` hit `subprocess.TimeoutExpired`; the chat CLI did not return within 120 seconds.
- **Effect:** `chat_ok` remained False; the test asserted at line 68 and failed.

## 3 Specific Code Paths Involved

Relevant code paths:

### 3.1 Python Test Path

- [e2e_0530_task_models_and_chat.py](../../../scripts/test_scripts/e2e_0530_task_models_and_chat.py) lines 23-68: Runs `models list -o json` (asserts ok and list), then up to 3 attempts of `chat --message ping --plain`; asserts chat_ok (non-empty stdout, no error) at line 68.

### 3.2 Helper and CLI

- [helpers.py](../../../scripts/test_scripts/helpers.py): `run_cynork` (timeout=120).
- [cynork/cmd/chat_slash.go](../../../cynork/cmd/chat_slash.go) and chat command: invoke gateway chat/completions API.

### 3.3 Gateway and Inference

- User API Gateway: chat/completions endpoint; routes to PMA or inference backend.
- Orchestrator and PMA: completion handling; inference backend (e.g. Ollama) may be slow or unavailable, causing the gateway to block until timeout.

## 4 Related Specs, Requirements, and Feature Files

Traceability for this test:

### 4.1 Requirements References

- [REQ-USRGWY-0121](../../requirements/usrgwy.md), [REQ-USRGWY-0127](../../requirements/usrgwy.md): Chat and models API.
- [REQ-CLIENT-0161](../../requirements/client.md): CLI chat parity.

### 4.2 Tech Specs

- [openai_compatible_chat_api.md](../../tech_specs/openai_compatible_chat_api.md): Chat completions API.
- [user_api_gateway.md](../../tech_specs/user_api_gateway.md): Models list and chat.
- [cynork_cli.md](../../tech_specs/cynork_cli.md): Chat command.

### 4.3 Feature Files

- Chat and models E2E/feature coverage where one-shot chat and models list are exercised.

## 5 Implementation Deviation

- **Spec/requirement intent:** Clients MUST be able to list models and run one-shot chat; chat MUST return a timely completion or a clear error.
- **Observed behavior:** Chat command does not return within 120 seconds; stdout empty, stderr shows timeout.
- **Deviation:** The chat path (gateway, PMA, or inference) does not complete within the test timeout; may be environment (inference startup/latency) or implementation (blocking wait, no client-side timeout handling).

## 6 What Needs to Be Fixed in the Implementation

The following describes the chat path and timeout cause.

### 6.1 Root Cause (Implemented Behavior)

- Cynork chat sends POST /v1/chat/completions (stream=false) to the gateway.
  The gateway in [orchestrator/internal/handlers/openai_chat.go](../../../orchestrator/internal/handlers/openai_chat.go) calls `tryPMACompletion` (non-stream) or equivalent, which uses [orchestrator/internal/pmaclient/client.go](../../../orchestrator/internal/pmaclient/client.go) `CallChatCompletion`.
  That uses `http.Client{Timeout: defaultTimeout}` with **defaultTimeout = 120s**.
  The test also uses a 120s subprocess timeout.
  So if the PMA or inference backend does not respond within 120s, the gateway returns an error and the CLI sees failure or timeout.

### 6.2 What is Not Implemented or is Misconfigured

- **Timeout alignment:** Gateway handler may use a longer context (e.g. chatCompletionTimeout 200s) but the PMA client's HTTP client is fixed at 120s.
- So the upstream (PMA + inference) must complete within 120s.
- Either: (1) pass an HTTP client with timeout >= chatCompletionTimeout when calling the PMA from the gateway, or (2) ensure inference is warm and PMA responds within 120s.

### 6.3 Exact Code or Config Changes

- In the gateway, when building the PMA client or calling `pmaclient.CallChatCompletion`, use an HTTP client with timeout of at least 200s (or match chatCompletionTimeout) so that the completion has time to finish before the client reports timeout.
  Alternatively, ensure E2E environment has inference and PMA ready so completion finishes within 120s.
