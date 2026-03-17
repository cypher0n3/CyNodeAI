# Failed E2E Report: e2e_0450_task_result.test_task_result

## 1 Summary

Test `e2e_0450_task_result.TestTaskResult.test_task_result` failed because `state.TASK_ID` was None when the test ran.
The test requires that e2e_0420 (task create) has set `state.TASK_ID`; it asserts that at line 27 and fails before calling `task result`.

## 2 Why the Failure Occurred

- **Observed:** `AssertionError: unexpectedly None`
- **Root cause:** Cascade failure: e2e_0420 task create tests timed out and never set `state.TASK_ID`.
- **Effect:** The test exits at `self.assertIsNotNone(state.TASK_ID)` (line 27) and never exercises the task result API.

## 3 Specific Code Paths Involved

Relevant code paths:

### 3.1 Python Test Path

- [e2e_0450_task_result.py](../../../scripts/test_scripts/e2e_0450_task_result.py) lines 25-39: `test_task_result` asserts `state.TASK_ID` is not None (line 27), then runs `cynork task result <task_id> -o json` and asserts response includes task_id, status, and terminal stdout/stderr fields.

### 3.2 Shared State

- [e2e_state.py](../../../scripts/test_scripts/e2e_state.py): `TASK_ID` set by e2e_0420; remains None when e2e_0420 times out.

### 3.3 CLI and Gateway

- [cynork/cmd/task.go](../../../cynork/cmd/task.go): `taskResultCmd` / `runTaskResult` call `GET /v1/tasks/{id}/result`.
- [cynork/internal/gateway/client.go](../../../cynork/internal/gateway/client.go): Task result API.

### 3.4 Backend Path

- User API Gateway and orchestrator serve task result; not exercised in this run due to prerequisite failure.

## 4 Related Specs, Requirements, and Feature Files

Traceability for this test:

### 4.1 Requirements References

- [REQ-ORCHES-0124](../../requirements/orches.md), [REQ-ORCHES-0125](../../requirements/orches.md): Task result and task state read through the User API Gateway.

### 4.2 Tech Specs

- [cli_management_app_commands_tasks.md](../../tech_specs/cli_management_app_commands_tasks.md): Task result subcommand and canonical task-result JSON contract.

### 4.3 Feature Files

- [cynork_tasks.feature](../../../features/cynork/cynork_tasks.feature): Task result scenarios.

## 5 Implementation Deviation

- **Spec/requirement intent:** Clients MUST be able to get task result by ID with required keys and terminal stdout/stderr; the E2E test assumes a task was created in e2e_0420.
- **Observed behavior:** Test fails on prerequisite before calling the API.
- **Deviation:** No deviation in task result implementation; failure is a cascade from e2e_0420 task create timeout.

## 6 What Needs to Be Fixed in the Implementation

- **No change in this component.**
  Task result API and CLI are correct.
  The test fails on the prerequisite that `state.TASK_ID` is set (e2e_0420 did not complete).
  Fix task create per [2026-03-16_e2e_050_test_task_create.md](2026-03-16_e2e_050_test_task_create.md) section 6; then e2e_0450 can poll GET /v1/tasks/{id}/result and assert on stdout/stderr.
