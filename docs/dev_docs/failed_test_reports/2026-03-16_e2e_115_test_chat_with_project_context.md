# Failed E2E Report: e2e_115_pma_chat_context.test_chat_with_project_context

## 1 Summary

Test `e2e_115_pma_chat_context.TestPmaChatContext.test_chat_with_project_context` failed because the chat command with `--project-id` did not complete successfully within the test's retries.
The test resolves a project ID (from state or a probe task create), then runs `cynork chat --message "Reply with exactly: OK" --project-id <id> --plain` up to 3 times; the assertion at line 99 failed with chat_ok False and stderr showing the command timed out after 120 seconds.

## 2 Why the Failure Occurred

- **Observed:** `AssertionError: False is not true : chat with project-id failed ... stdout='' stderr="Command '...' timed out after 120 seconds"`
- **Root cause:** Same as e2e_110: `run_cynork` for the chat command uses a 120-second timeout; the subprocess timed out before returning.
- **Effect:** No successful reply was received; the test asserted at line 99 and failed.

## 3 Specific Code Paths Involved

Relevant code paths:

### 3.1 Python Test Path

- [e2e_115_pma_chat_context.py](../../../scripts/test_scripts/e2e_115_pma_chat_context.py) lines 53-99: Resolves project_id via `_project_id_for_chat()` (may create a probe task), then up to 3 attempts of `chat --message "Reply with exactly: OK" --project-id <id> --plain`; asserts chat_ok at line 99.

### 3.2 Helper and CLI

- [helpers.py](../../../scripts/test_scripts/helpers.py): `run_cynork` (timeout=120).
- [cynork_cli.md](../../tech_specs/cynork_cli.md): Chat command with `--project-id` sends OpenAI-Project header to gateway.

### 3.3 Gateway and PMA

- User API Gateway: chat with project context (REQ-USRGWY-0131); PMA handoff path.
- Completion handling and inference; same timeout/blocking behavior as generic chat.

## 4 Related Specs, Requirements, and Feature Files

Traceability for this test:

### 4.1 Requirements References

- [REQ-USRGWY-0131](../../requirements/usrgwy.md): Task/project association and project context for chat.
- [REQ-CLIENT-0173](../../requirements/client.md): Project context for chat in CLI.

### 4.2 Tech Specs

- [user_api_gateway.md](../../tech_specs/user_api_gateway.md): Project-scoped chat and OpenAI-Project header.
- [cynork_cli.md](../../tech_specs/cynork_cli.md): Chat command with project-id.

### 4.3 Feature Files

- [pma_chat_and_context.feature](../../../features/agents/pma_chat_and_context.feature): PMA chat with project and task context.

## 5 Implementation Deviation

- **Spec/requirement intent:** Chat with project-id MUST succeed when inference is available and return a reply or clear error.
- **Observed behavior:** Chat with project-id does not return within 120 seconds; timeout.
- **Deviation:** Same as e2e_110: the chat path does not complete within the test timeout (environment or gateway/PMA/inference blocking).

## 6 What Needs to Be Fixed in the Implementation

- **Same root cause as e2e_110.**
  Chat with project-id uses the same gateway path: [orchestrator/internal/handlers/openai_chat.go](../../../orchestrator/internal/handlers/openai_chat.go) and [orchestrator/internal/pmaclient/client.go](../../../orchestrator/internal/pmaclient/client.go) with **120s** HTTP client timeout.
  Completion does not finish within 120s.
  Apply the same fix as in [2026-03-16_e2e_110_test_models_and_chat.md](2026-03-16_e2e_110_test_models_and_chat.md) section 6: use a longer timeout for the PMA client when calling from the gateway (e.g. 200s to match chatCompletionTimeout), or ensure PMA/inference respond in time.
