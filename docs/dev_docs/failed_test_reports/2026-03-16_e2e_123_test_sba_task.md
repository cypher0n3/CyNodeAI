# Failed E2E Report: e2e_0710_sba_task.test_sba_task

## 1 Summary

Test `e2e_0710_sba_task.TestSbaTask.test_sba_task` failed because no `task_id` was returned from SBA task create.
The test calls `helpers.create_and_poll_sba_task()` with args for `task create -p "echo from SBA" --use-sba --use-inference -o json`; at line 34 it asserts task_id is not None with "SBA task create failed".

## 2 Why the Failure Occurred

- **Observed:** `AssertionError: unexpectedly None : SBA task create failed`
- **Root cause:** `create_and_poll_sba_task` runs task create and then polls task result; the create step did not yield a task_id (create command timed out or returned output that did not parse to a task_id).
- **Effect:** The test failed at the first assertion before checking status or sba_result.

## 3 Specific Code Paths Involved

Relevant code paths:

### 3.1 Python Test Path

- [e2e_0710_sba_task.py](../../../scripts/test_scripts/e2e_0710_sba_task.py) lines 24-34: Calls `helpers.create_and_poll_sba_task(create_args, state.CONFIG_PATH)`, then asserts task_id is not None.
- [helpers.py](../../../scripts/test_scripts/helpers.py): `create_and_poll_sba_task` runs cynork task create, parses task_id, then polls task result; returns (task_id, status, result_data).

### 3.2 CLI and Gateway

- [cynork/cmd/task.go](../../../cynork/cmd/task.go): Task create with `--use-sba` and `--use-inference`.
- User API Gateway and orchestrator: SBA task create and dispatch to SBA agent/worker.

### 3.3 Backend Path

- SBA agent and worker integration (REQ-SBAGNT-0001, 0106); task create path same timeout/response issues as other task create in this run.

## 4 Related Specs, Requirements, and Feature Files

Traceability for this test:

### 4.1 Requirements References

- [REQ-SBAGNT-0001](../../requirements/sbagnt.md), [REQ-SBAGNT-0106](../../requirements/sbagnt.md): SBA agent and result contract.
- [REQ-SBAGNT-0109](../../requirements/sbagnt.md): Inference reachable for SBA.

### 4.2 Tech Specs

- [cynode_sba.md](../../tech_specs/cynode_sba.md): SBA task execution and result contract.
- [cli_management_app_commands_tasks.md](../../tech_specs/cli_management_app_commands_tasks.md): Task create with --use-sba.

### 4.3 Feature Files

- SBA and worker integration feature scenarios where SBA task create and sba_result are exercised.

## 5 Implementation Deviation

- **Spec/requirement intent:** SBA task create MUST return a task identifier; the test then polls until terminal status and asserts sba_result in job result.
- **Observed behavior:** Task create did not return a task_id (timeout or non-JSON/empty response).
- **Deviation:** Same pattern as e2e_050/e2e_100: create path does not complete within the test timeout in the E2E environment.

## 6 What Needs to Be Fixed in the Implementation

- **Same root cause as e2e_050.**
  SBA task create uses the same gateway/orchestrator create path; when prompt mode and inference URL are set, [orchestrator/internal/handlers/tasks.go](../../../orchestrator/internal/handlers/tasks.go) blocks on `tryCompleteWithOrchestratorInference`.
  The test does not get task_id in time.
  Fix: same as [2026-03-16_e2e_050_test_task_create.md](2026-03-16_e2e_050_test_task_create.md) section 6 (return 201 promptly without blocking on inference, or E2E config with inference URL unset).
