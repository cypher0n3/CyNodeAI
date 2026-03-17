# Failed E2E Report: e2e_0420_task_create.test_task_create_with_attachments

## 1 Summary

Test `e2e_0420_task_create.TestTaskCreate.test_task_create_with_attachments` failed because the `cynork task create` command with `--attach` flags did not complete within the 120-second timeout.
The test never received a successful response (ok False on all attempts), so it failed with an assertion that task create with attachments failed.

## 2 Why the Failure Occurred

- **Observed:** `AssertionError: task create with attachments failed: ... Command '...' timed out after 120 seconds`
- **Root cause:** Same as [2026-03-16_e2e_050_test_task_create.md](2026-03-16_e2e_050_test_task_create.md): `helpers.run_cynork()` uses a 120-second default timeout.
  The subprocess running `cynork task create -p "Read the attached files and acknowledge receipt." --attach tmp/att1.txt --attach tmp/att2.txt -o json` was killed by `subprocess.TimeoutExpired` before the CLI returned.
- **Effect:** The test loop (up to 3 attempts) never got `ok` True; on attempt 3 it called `self.fail(...)` at line 124.

## 3 Specific Code Paths Involved

Relevant code paths from test to backend:

### 3.1 Python Test Path

- [e2e_0420_task_create.py](../../../scripts/test_scripts/e2e_0420_task_create.py) lines 93-124: `test_task_create_with_attachments` creates tmp files, then loops up to 3 times calling `helpers.run_cynork(["task", "create", "-p", "...", "--attach", "tmp/att1.txt", "--attach", "tmp/att2.txt", "-o", "json"], ...)`.
  It fails when `ok` is False on the third attempt (line 124).

### 3.2 Helper Layer

- [helpers.py](../../../scripts/test_scripts/helpers.py) lines 17-37: `run_cynork(..., timeout=120)`; on `TimeoutExpired` returns `(False, "", str(e))`.

### 3.3 CLI and Gateway

- [cynork/cmd/task.go](../../../cynork/cmd/task.go): `runTaskCreate` builds the request including attachments via `buildCreateTaskRequest(..., attachments)` and calls `client.CreateTask(&req)`.
- [cynork/internal/gateway/client.go](../../../cynork/internal/gateway/client.go): `CreateTask` sends `POST /v1/tasks` with the create payload (including attachments) to the user gateway.

### 3.4 Backend Path

- Gateway and orchestrator handle task create with attachments; end-to-end response again exceeds 120 seconds in the E2E environment.

## 4 Related Specs, Requirements, and Feature Files

Traceability for this test:

### 4.1 Requirements References

- [REQ-ORCHES-0122](../../requirements/orches.md): Authenticated user clients MUST be able to create tasks through the User API Gateway.
- [REQ-ORCHES-0126](../../requirements/orches.md): Task creation MUST accept a natural-language user prompt and task input.

### 4.2 Tech Specs

- [cli_management_app_commands_tasks.md](../../tech_specs/cli_management_app_commands_tasks.md): `cynork task create` with `--attach` paths; attachments included in the task create request and returned in create/get responses.

### 4.3 Feature Files

- [cynork_tasks.feature](../../../features/cynork/cynork_tasks.feature): Scenario "I run cynork task create with prompt ... and attachments ..." and assertions on attachments in create/get.

## 5 Implementation Deviation

- **Spec/requirement intent:** Task create with attachments MUST complete and return a task identifier and attachment list so the test can assert on create and get responses.
- **Observed behavior:** The create path with attachments does not respond within 120 seconds; the CLI times out.
- **Deviation:** Same as the plain task-create case: the system does not respond within the test timeout (environment or gateway/orchestrator slowness/blocking).

## 6 What Needs to Be Fixed in the Implementation

- **Same root cause as plain task create:** [orchestrator/internal/handlers/tasks.go](../../../orchestrator/internal/handlers/tasks.go) `CreateTask` uses the same blocking path for prompt mode when `h.inferenceURL != ""`: it calls `tryCompleteWithOrchestratorInference`, which blocks on `inference.CallGenerate`.
  Attachments are persisted via `persistTaskAttachments` before that and do not change the blocking branch.
  Apply the **same fix** as in [2026-03-16_e2e_050_test_task_create.md](2026-03-16_e2e_050_test_task_create.md): stop blocking on orchestrator inference for create (return 201 immediately, run inference async) or run E2E with inference URL unset so the sandbox-job path returns 201 immediately.
- **Attachments:** No separate missing implementation; attachment handling exists.
  Ensure attachment persistence does not add synchronous work that delays the response.
