# Failed E2E Report: e2e_100_task_prompt.test_prompt_task

## 1 Summary

Test `e2e_100_task_prompt.TestPromptTask.test_prompt_task` failed because no `task_id` was obtained from `cynork task create` after up to 3 attempts.
The test creates a prompt-mode task ("What model are you? Reply in one short sentence."), parses the create response for `task_id`, and fails at line 31 with "prompt task create failed".

## 2 Why the Failure Occurred

- **Observed:** `AssertionError: unexpectedly None : prompt task create failed`
- **Root cause:** Same as e2e_050: task create did not return a successful response within the test's retry window.
  Each attempt uses `helpers.run_cynork(..., timeout=120)`; the command likely timed out or returned output that did not parse to a JSON object with `task_id`.
- **Effect:** `task_id` remained None after 3 attempts; the test asserted and failed before polling task result.

## 3 Specific Code Paths Involved

Relevant code paths:

### 3.1 Python Test Path

- [e2e_100_task_prompt.py](../../../scripts/test_scripts/e2e_100_task_prompt.py) lines 16-31: Loop up to 3 times running `cynork task create -p "What model are you? ..." -o json`, parse `task_id` from response; assert task_id is not None at line 31.

### 3.2 Helper and CLI

- [helpers.py](../../../scripts/test_scripts/helpers.py): `run_cynork` (timeout=120).
- [cynork/cmd/task.go](../../../cynork/cmd/task.go): `runTaskCreate` with prompt input mode.

### 3.3 Backend Path

- User API Gateway and orchestrator: prompt-mode task create (REQ-ORCHES-0122, REQ-ORCHES-0126); interpretation and inference by default.
  Same create path as e2e_050; response not returned within timeout in this run.

## 4 Related Specs, Requirements, and Feature Files

Traceability for this test:

### 4.1 Requirements References

- [REQ-ORCHES-0122](../../requirements/orches.md): Authenticated user clients MUST be able to create tasks through the User API Gateway.
- [REQ-ORCHES-0126](../../requirements/orches.md): Task creation MUST accept a natural-language user prompt; system uses inference by default.

### 4.2 Tech Specs

- [cli_management_app_commands_tasks.md](../../tech_specs/cli_management_app_commands_tasks.md): Prompt-mode task create.
- [user_api_gateway.md](../../tech_specs/user_api_gateway.md): Task create semantics and interpretation.

### 4.3 Feature Files

- [cynork_tasks.feature](../../../features/cynork/cynork_tasks.feature): Prompt task create and result scenarios.

## 5 Implementation Deviation

- **Spec/requirement intent:** Prompt-mode task create MUST complete and return a task identifier so the client can poll result and assert completed stdout.
- **Observed behavior:** Task create did not return a task_id within the test's attempts (timeout or non-JSON/empty response).
- **Deviation:** Same as e2e_050: create path does not respond within 120 seconds or does not return a valid create response in the E2E environment.

## 6 What Needs to Be Fixed in the Implementation

- **Same root cause as e2e_050.**
  Prompt-mode task create goes through [orchestrator/internal/handlers/tasks.go](../../../orchestrator/internal/handlers/tasks.go) `CreateTask` -> `tryCompleteWithOrchestratorInference` -> `inference.CallGenerate`.
- That path blocks until the inference backend responds.
- The test never receives task_id within 120s.
- Apply the **same fix** as in [2026-03-16_e2e_050_test_task_create.md](2026-03-16_e2e_050_test_task_create.md) section 6: return 201 immediately and run inference asynchronously, or run E2E with inference URL unset so the sandbox path returns 201 promptly.
