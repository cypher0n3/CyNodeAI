# Failed E2E Report: e2e_090_task_inference.test_inference_task

## 1 Summary

Test `e2e_090_task_inference.TestInferenceTask.test_inference_task` failed because the sandbox job result's `stdout` was None when the test asserted it should contain the UDS inference proxy URL (`http+unix://`).
The test creates a task with `--use-inference` and a command that echoes `$INFERENCE_PROXY_URL`, polls for task result, then asserts that the first job's result stdout contains `http+unix://`.

## 2 Why the Failure Occurred

- **Observed:** `AssertionError: None is not true : INFERENCE_PROXY_URL missing or non-UDS in stdout (stdout=None)`
- **Root cause:** The test reached line 61 after polling task result.
  The value extracted for `stdout` from the task result (path: jobs[0].result.stdout, or equivalent) was None.
  So either the task result payload did not include job result stdout, the task did not complete with a terminal status and the result structure was incomplete, or the job never ran in a sandbox that received INFERENCE_PROXY_URL.
- **Effect:** The assertion `stdout and "http+unix://" in str(stdout)` failed because `stdout` was None.

## 3 Specific Code Paths Involved

Relevant code paths:

### 3.1 Python Test Path

- [e2e_090_task_inference.py](../../../scripts/test_scripts/e2e_090_task_inference.py) lines 18-62: Creates task with `task create --command "sh -c 'echo $INFERENCE_PROXY_URL'" --use-inference -o json`, parses `task_id`, polls `task result` up to 18 times (5s apart), then reads `jobs[0].result` and asserts `stdout` contains `http+unix://`.

### 3.2 CLI and Gateway

- [cynork/cmd/task.go](../../../cynork/cmd/task.go): `runTaskCreate` with inference flag; `runTaskResult` for polling.
- [cynork/internal/gateway/client.go](../../../cynork/internal/gateway/client.go): CreateTask, task result API.

### 3.3 Backend Path

- User API Gateway and orchestrator: task create with inference, dispatch to worker (REQ-ORCHES-0123).
- Worker node: sandbox execution with inference proxy; REQ-WORKER-0114 (node inference path), REQ-WORKER-0270 (UDS boundary).
  The sandbox must receive `INFERENCE_PROXY_URL` (UDS) and the job result must expose stdout to the task result API.

### 3.4 Worker and Sandbox

- Worker node manager and sandbox container: inject INFERENCE_PROXY_URL; capture and return job stdout in the task result payload.

## 4 Related Specs, Requirements, and Feature Files

Traceability for this test:

### 4.1 Requirements References

- [REQ-WORKER-0114](../../requirements/worker.md): Node inference path.
- [REQ-WORKER-0270](../../requirements/worker.md): UDS boundary for inference proxy.
- [REQ-ORCHES-0123](../../requirements/orches.md): Dispatch to worker.

### 4.2 Tech Specs

- [worker_node_payloads.md](../../tech_specs/worker_node_payloads.md): Job result and inference proxy configuration.
- [worker_node.md](../../tech_specs/worker_node.md): Worker API and sandbox execution.
- [cli_management_app_commands_tasks.md](../../tech_specs/cli_management_app_commands_tasks.md): Task create with `--use-inference`.

### 4.3 Feature Files

- Worker and inference E2E/feature coverage in `features/worker_node/` and related suites where inference-in-sandbox is exercised.

## 5 Implementation Deviation

- **Spec/requirement intent:** When a task is created with inference enabled, the sandbox MUST receive an inference proxy URL (UDS); job result MUST include stdout so clients can verify the proxy URL was present.
- **Observed behavior:** The task result (or the parsed path into it) yielded None for the job's stdout, so the test could not verify INFERENCE_PROXY_URL.
- **Deviation:** Either (1) the task result API or worker job result does not expose stdout in the structure the test expects, (2) the task did not reach a completed state with a valid job result, or (3) the sandbox never received INFERENCE_PROXY_URL and the job result is missing or structured differently.

## 6 What Needs to Be Fixed in the Implementation

The following describes the two failure modes and required changes.

### 6.1 Root Cause (Two Possible Failure Modes)

- **Create timeout (same as e2e_050):** If the test fails before obtaining a task_id or result, the same blocking path applies: [orchestrator/internal/handlers/tasks.go](../../../orchestrator/internal/handlers/tasks.go) blocks on orchestrator inference when `inputMode == InputModePrompt` and `h.inferenceURL != ""`.
  For `--use-inference` the task may still be created as prompt mode and block.
  Fix: same as [2026-03-16_e2e_050_test_task_create.md](2026-03-16_e2e_050_test_task_create.md) (return 201 promptly or unset inference URL in E2E).
- **Result structure / INFERENCE_PROXY_URL:** If create returns and the task completes but the test fails on stdout/INFERENCE_PROXY_URL: (1) Worker payload must pass `INFERENCE_PROXY_URL` (or equivalent) into the sandbox env (see [worker_node_payloads.md](../../tech_specs/worker_node_payloads.md)); (2) Job result returned by the worker and exposed via GET /v1/tasks/{id}/result must include stdout in the path the test expects (e.g. `result.job_result.stdout` or the structure in [worker_node.md](../../tech_specs/worker_node.md)).
- Verify the test's result parsing matches the API response shape.

### 6.2 Exact Code or Config Changes

- If failure is create timeout: apply task-create fix (orchestrator handler or E2E config).
- If failure is missing stdout / INFERENCE_PROXY_URL: ensure orchestrator/worker sets inference proxy URL in job env and that task result API returns job_result with stdout; align test assertions with actual response schema.
