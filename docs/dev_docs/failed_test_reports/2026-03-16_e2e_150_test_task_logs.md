# Failed E2E Report: e2e_0460_task_logs.test_task_logs

## 1 Summary

Test `e2e_0460_task_logs.TestTaskLogs.test_task_logs` failed because `state.TASK_ID` was None when the test ran.
The test requires that e2e_0420 (task create) has set `state.TASK_ID`; it asserts that at lines 27-29 and fails with "state.TASK_ID must be set by e2e_0420 (task create); run tests in order".

## 2 Why the Failure Occurred

- **Observed:** `AssertionError: unexpectedly None : state.TASK_ID must be set by e2e_0420 (task create); run tests in order`
- **Root cause:** Cascade failure: e2e_0420 task create tests timed out and never set `state.TASK_ID`.
- **Effect:** The test exits at the prerequisite check before calling `task logs`.

## 3 Specific Code Paths Involved

Relevant code paths:

### 3.1 Python Test Path

- [e2e_0460_task_logs.py](../../../scripts/test_scripts/e2e_0460_task_logs.py) lines 25-34: `test_task_logs` asserts `state.TASK_ID` is not None (lines 27-29), then runs `cynork task logs <task_id> -o json` and asserts response has task_id, stdout, and stderr.

### 3.2 Shared State

- [e2e_state.py](../../../scripts/test_scripts/e2e_state.py): `TASK_ID` set by e2e_0420; remains None when e2e_0420 times out.

### 3.3 CLI and Gateway

- [cynork/cmd/task.go](../../../cynork/cmd/task.go): `taskLogsCmd` / `runTaskLogs` call `GET /v1/tasks/{id}/logs`.
- [cynork/internal/gateway/client.go](../../../cynork/internal/gateway/client.go): Task logs API.

### 3.4 Backend Path

- User API Gateway and orchestrator serve task logs; not exercised in this run due to prerequisite failure.

## 4 Related Specs, Requirements, and Feature Files

Traceability for this test:

### 4.1 Requirements References

- [REQ-ORCHES-0124](../../requirements/orches.md): Task logs and result access through the User API Gateway.

### 4.2 Tech Specs

- [cli_management_app_commands_tasks.md](../../tech_specs/cli_management_app_commands_tasks.md): Task logs subcommand and response (task_id, stdout, stderr).

### 4.3 Feature Files

- Task logs behavior covered in task-lifecycle feature scenarios where applicable.

## 5 Implementation Deviation

- **Spec/requirement intent:** Clients MUST be able to get task logs by ID with task_id, stdout, and stderr; the E2E test assumes a task was created in e2e_0420.
- **Observed behavior:** Test fails on prerequisite before calling the API.
- **Deviation:** No deviation in task logs implementation; failure is a cascade from e2e_0420 task create timeout.

## 6 What Needs to Be Fixed in the Implementation

- **No change in this component.**
  Task logs API and CLI are correct.
  The test fails because `state.TASK_ID` is None (e2e_0420 task create timed out).
  Fix task create as in [2026-03-16_e2e_050_test_task_create.md](2026-03-16_e2e_050_test_task_create.md) section 6; then e2e_0460 will have a task_id and can assert on logs.
