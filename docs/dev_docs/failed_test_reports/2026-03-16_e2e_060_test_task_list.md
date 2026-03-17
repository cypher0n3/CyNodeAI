# Failed E2E Report: e2e_0430_task_list.test_task_list

## 1 Summary

Test `e2e_0430_task_list.TestTaskList.test_task_list` failed because `state.TASK_ID` was None when the test ran.
The test requires that e2e_0420 (task create) has already set `state.TASK_ID`; it asserts that at line 17 and fails with "state.TASK_ID must be set by e2e_0420 (task create); run tests in order".

## 2 Why the Failure Occurred

- **Observed:** `AssertionError: unexpectedly None : state.TASK_ID must be set by e2e_0420 (task create); run tests in order`
- **Root cause:** Cascade failure.
  e2e_0420_task_create tests (e.g. `test_task_create`) did not complete successfully in this run; they timed out before returning a `task_id`, so `state.TASK_ID` was never set.
- **Effect:** e2e_0430 assumes the suite runs in order and that at least one task was created; it fails immediately on the prerequisite check before calling `task list`.

## 3 Specific Code Paths Involved

Relevant code paths:

### 3.1 Python Test Path

- [e2e_0430_task_list.py](../../../scripts/test_scripts/e2e_0430_task_list.py) lines 15-19: `test_task_list` first asserts `state.TASK_ID` is not None (line 17), then runs `cynork task list -o json -l 10` and asserts the returned tasks list contains `state.TASK_ID`.

### 3.2 Shared State

- [e2e_state.py](../../../scripts/test_scripts/e2e_state.py): Module-level `TASK_ID` is set by e2e_0420 when task create returns a `task_id`; when e2e_0420 times out, it remains None.

### 3.3 CLI and Gateway

- [cynork/cmd/task.go](../../../cynork/cmd/task.go): `taskListCmd` / `runTaskList` call the gateway `GET /v1/tasks`.
- [cynork/internal/gateway/client.go](../../../cynork/internal/gateway/client.go): List tasks API.

### 3.4 Backend Path

- User API Gateway and orchestrator serve task list; not exercised in this run because the test exits on the prerequisite assertion.

## 4 Related Specs, Requirements, and Feature Files

Traceability for this test:

### 4.1 Requirements References

- [REQ-ORCHES-0125](../../requirements/orches.md): Authorized clients MUST be able to read task state (including status) through the User API Gateway.

### 4.2 Tech Specs

- [cli_management_app_commands_tasks.md](../../tech_specs/cli_management_app_commands_tasks.md): Task list subcommand and output format.

### 4.3 Feature Files

- [cynork_tasks.feature](../../../features/cynork/cynork_tasks.feature): Task list scenarios (when run after task create).

## 5 Implementation Deviation

- **Spec/requirement intent:** Clients MUST be able to list tasks and see task state; the E2E suite assumes one task exists (from e2e_0420) and asserts the list contains it.
- **Observed behavior:** The test does not reach the task-list API call; it fails on the prerequisite that `state.TASK_ID` is set.
- **Deviation:** No deviation in the task list implementation itself.
  The failure is a cascade from e2e_0420 not completing (task create timeout); fixing the task-create path so it returns within the test timeout would allow this test to run and validate task list behavior.

## 6 What Needs to Be Fixed in the Implementation

- **No change in this component.**
  The task list API and CLI work as specified.
  The test fails because it depends on `state.TASK_ID` from the previous test (e2e_0420).
  e2e_0420 fails due to task create blocking on orchestrator inference (see [2026-03-16_e2e_050_test_task_create.md](2026-03-16_e2e_050_test_task_create.md) section 6).
  Fix the task-create path (return 201 promptly without blocking on inference, or run E2E with inference URL unset); then e2e_060 will have a valid TASK_ID and can assert on the list.
