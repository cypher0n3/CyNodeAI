# Failed E2E Report: e2e_0440_task_get.test_task_get

## 1 Summary

Test `e2e_0440_task_get.TestTaskGet.test_task_get` failed because `state.TASK_ID` was None when the test ran.
The test requires that e2e_0420 (task create) has set `state.TASK_ID`; it asserts that at line 28 and fails before calling `task get`.

## 2 Why the Failure Occurred

- **Observed:** `AssertionError: unexpectedly None`
- **Root cause:** Cascade failure: e2e_0420 task create tests timed out and never set `state.TASK_ID`.
- **Effect:** The test exits at `self.assertIsNotNone(state.TASK_ID)` (line 28) and never exercises the task get API.

## 3 Specific Code Paths Involved

Relevant code paths:

### 3.1 Python Test Path

- [e2e_0440_task_get.py](../../../scripts/test_scripts/e2e_0440_task_get.py) lines 26-34: `test_task_get` asserts `state.TASK_ID` is not None (line 28), then runs `cynork task get <task_id> -o json` and asserts response includes the task and a valid status.

### 3.2 Shared State

- [e2e_state.py](../../../scripts/test_scripts/e2e_state.py): `TASK_ID` set by e2e_0420; remains None when e2e_0420 times out.

### 3.3 CLI and Gateway

- [cynork/cmd/task.go](../../../cynork/cmd/task.go): `taskGetCmd` / `runTaskGet` call `GET /v1/tasks/{id}`.
- [cynork/internal/gateway/client.go](../../../cynork/internal/gateway/client.go): Get task API.

### 3.4 Backend Path

- User API Gateway and orchestrator serve task get; not exercised in this run due to prerequisite failure.

## 4 Related Specs, Requirements, and Feature Files

Traceability for this test:

### 4.1 Requirements References

- [REQ-ORCHES-0125](../../requirements/orches.md): Authorized clients MUST be able to read task state through the User API Gateway.

### 4.2 Tech Specs

- [cli_management_app_commands_tasks.md](../../tech_specs/cli_management_app_commands_tasks.md): Task get subcommand and response contract.

### 4.3 Feature Files

- [cynork_tasks.feature](../../../features/cynork/cynork_tasks.feature): Task get scenarios.

## 5 Implementation Deviation

- **Spec/requirement intent:** Clients MUST be able to get task details by ID; the E2E test assumes a task was created in e2e_0420.
- **Observed behavior:** Test fails on prerequisite before calling the API.
- **Deviation:** No deviation in task get implementation; failure is a cascade from e2e_0420 task create timeout.

## 6 What Needs to Be Fixed in the Implementation

- **No change in this component.**
  Task get API and CLI are correct.
  The test never calls the API because `state.TASK_ID` is None (e2e_0420 timed out).
  Fix task create as in [2026-03-16_e2e_050_test_task_create.md](2026-03-16_e2e_050_test_task_create.md) section 6; then e2e_0440 will receive a task_id and can validate GET /v1/tasks/{id}.
