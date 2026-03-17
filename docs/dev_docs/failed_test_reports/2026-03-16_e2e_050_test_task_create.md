# Failed E2E Report: e2e_0420_task_create.test_task_create

## 1 Summary

E2E tests were renumbered; script: `e2e_0420_task_create.py`.

Test `e2e_0420_task_create.TestTaskCreate.test_task_create` failed because the `cynork task create` command did not complete within the 120-second timeout.
The test never received a `task_id` in stdout, so it failed on the third attempt with an assertion that task create failed after 3 attempts.

## 2 Why the Failure Occurred

- **Observed:** `AssertionError: task create failed after 3 attempts: ... Command '...' timed out after 120 seconds`
- **Root cause:** `helpers.run_cynork()` uses a default timeout of 120 seconds.
  The subprocess running `cynork task create -p "Please reply with exactly: hello from task create." -o json` was killed by `subprocess.TimeoutExpired` before the CLI returned.
- **Effect:** No JSON output was captured; `parse_json_safe(out)` returned nothing useful, so `task_id` stayed empty and `state.TASK_ID` was never set.
- **Possible contributing factor:** On hosts affected by [Bug 1: ROCM Ollama on Nvidia GPU](../_bugs.md#bug-1-rocm-ollama-on-nvidia-gpu), the wrong Ollama variant (rocm) may be running on an NVIDIA GPU, which can cause inference to be slow, fail, or hang.
  That would make the prompt-mode task create path (which blocks on orchestrator inference) more likely to exceed the 120s timeout.
  See [Identified Bugs](../_bugs.md) for causes, spec violations, and desired behavior.

## 3 Specific Code Paths Involved

Relevant code paths from test to backend:

### 3.1 Python Test Path

- [scripts/test_scripts/e2e_0420_task_create.py](../../../scripts/test_scripts/e2e_0420_task_create.py) lines 19-41: `test_task_create` loops up to 3 times, calls `helpers.run_cynork(["task", "create", "-p", "...", "-o", "json"], state.CONFIG_PATH)` (no explicit timeout, so 120s default), then parses stdout for `task_id` and fails if missing on attempt 3.

### 3.2 Helper Layer

- [scripts/test_scripts/helpers.py](../../../scripts/test_scripts/helpers.py) lines 17-37: `run_cynork(args, config_path, env_extra=None, timeout=120, ...)` runs the cynork binary with `subprocess.run(..., timeout=timeout)`; on `TimeoutExpired` it returns `(False, "", str(e))`.

### 3.3 CLI and Gateway

- [cynork/cmd/task.go](../../../cynork/cmd/task.go): `taskCreateCmd` invokes `runTaskCreate` (line 53); `runTaskCreate` (from line 258) builds the create request and calls `client.CreateTask(&req)` (line 276).
- [cynork/internal/gateway/client.go](../../../cynork/internal/gateway/client.go): `CreateTask` (from line 752) sends `POST /v1/tasks` to the user gateway and waits for the response.

### 3.4 Backend Path

- User API Gateway accepts `POST /v1/tasks` and forwards to the orchestrator; the orchestrator creates the task and typically waits for or coordinates with worker/PMA execution.
  The end-to-end path from CLI request to gateway response can exceed 120 seconds when inference or sandbox execution is slow or blocked.

## 4 Related Specs, Requirements, and Feature Files

Traceability for this test:

### 4.1 Requirements References

- [REQ-ORCHES-0122](../../requirements/orches.md): Authenticated user clients MUST be able to create tasks through the User API Gateway.
- [REQ-ORCHES-0126](../../requirements/orches.md): Task creation MUST accept a natural-language user prompt; the system MUST interpret the prompt and use inference by default.

### 4.2 Tech Specs

- [cli_management_app_commands_tasks.md](../../tech_specs/cli_management_app_commands_tasks.md): `cynork task create` behavior, task input modes, and traceability to REQ-ORCHES-0122, REQ-ORCHES-0126.
- [user_api_gateway.md](../../tech_specs/user_api_gateway.md): Gateway task create semantics and interpretation/inference as default.

### 4.3 Feature Files

- [features/cynork/cynork_tasks.feature](../../../features/cynork/cynork_tasks.feature): Scenarios for "I run cynork task create with prompt ..." and assertions on stdout containing `task_id=`.

## 5 Implementation Deviation

- **Spec/requirement intent:** Task create MUST complete and return a task identifier so that clients can list, get, result, and cancel the task.
- **Observed behavior:** The create path (gateway plus orchestrator plus worker/PMA) does not respond within 120 seconds in the E2E environment, so the CLI times out and the test never sees a `task_id`.
- **Deviation:** The system fails to meet the implied "timely response" needed for the E2E test; the implementation may be correct but the environment (e.g. inference startup, PMA routing, or resource contention) causes the create flow to exceed the test's timeout, or the gateway/orchestrator does not respond promptly (e.g. blocking until task completion instead of returning after task creation).
- **Related bugs:** [Identified Bugs](../_bugs.md) documents Bug 1 (ROCM Ollama on Nvidia GPU), which can cause the wrong inference backend to run and contribute to inference path slowness or failure in E2E.

## 6 What Needs to Be Fixed in the Implementation

<!-- no-empty-heading allow -->

### 6.1 Root Cause (Implemented Behavior)

In [orchestrator/internal/handlers/tasks.go](../../../orchestrator/internal/handlers/tasks.go), `CreateTask` (line 181) does the following for a prompt-mode request (`-p "..."`):

1. Creates the task in the DB and normalizes input mode to `InputModePrompt`.
2. Calls `tryCompleteWithOrchestratorInference(ctx, w, task, ...)` (line 218).
3. **That function returns true only when orchestrator-side inference succeeds.**
   It calls `createTaskWithOrchestratorInference(ctx, task.ID, prompt)` (line 159), which **blocks** on `inference.CallGenerate(ctx, nil, h.inferenceURL, h.inferenceModel, prompt)` (line 239 in the same file).
   There is no separate goroutine or early return; the HTTP handler does not send 201 until the inference call returns.
4. `tryCompleteWithOrchestratorInference` is invoked only when `inputMode == InputModePrompt` and `strings.TrimSpace(h.inferenceURL) != ""` (line 156).
   The task handler receives `inferenceURL` from gateway config: [orchestrator/cmd/user-gateway/main.go](../../../orchestrator/cmd/user-gateway/main.go) line 64 passes `cfg.InferenceURL` (and `cfg.InferenceModel`) into `NewTaskHandler`.
   Config sets `InferenceURL` from `OLLAMA_BASE_URL` or `INFERENCE_URL` (see [orchestrator/internal/config/config.go](../../../orchestrator/internal/config/config.go) around line 119).
   So in any E2E run where the user-gateway is started with Ollama or an inference URL set, **every prompt-mode task create blocks the request until the inference backend responds**.
   If the model is cold or slow, that can exceed 120 seconds.

If `tryCompleteWithOrchestratorInference` returns false (inference URL empty or inference.CallGenerate fails), the handler then calls `createSandboxJob` and returns `WriteJSON(w, http.StatusCreated, ...)` (line 233) **without** waiting for the job to complete.
So the fast path exists only when orchestrator inference is not used or fails.

### 6.2 What is Not Implemented or is Misconfigured

- **Intended flow (not implemented):** Task create should: (1) create the task; (2) send the task to PMA with instructions to execute; (3) PMA kicks off SBA to execute the task and reports back to the orchestrator that the task has been **started**; (4) orchestrator returns 201 with `task_id` immediately after receiving that acknowledgment.
  SBA then executes the task and reports results back to PMA; PMA reports task completion back to the orchestrator via MCP.
  The spec (REQ-ORCHES-0122) does not require the create response to wait for task completion.
- **Current deviation:** The implementation blocks on orchestrator-side inference (`tryCompleteWithOrchestratorInference` -> `inference.CallGenerate`) instead of handing off to PMA, receiving a "task started" ack, and returning 201.
  The correct behavior is orchestrator -> PMA (execute task) -> PMA reports "started" -> 201; PMA -> SBA (execute) -> SBA -> PMA (results) -> PMA -> orchestrator (completion via MCP).

### 6.3 Exact Code Changes or Config Changes Required

**Required fix (spec and architecture):** Implement the handoff flow so that prompt-mode task create returns 201 immediately after the task is **started** by PMA, not after inference completes.

- In `tasks.go`, for prompt mode: create the task, then send the task to PMA with instructions to execute (do not call `tryCompleteWithOrchestratorInference` / `inference.CallGenerate` in the request path).
- Orchestrator must receive from PMA an acknowledgment that the task has been **started** (PMA has handed off to SBA); then return `WriteJSON(w, http.StatusCreated, taskToResponse(...))` and return.
- PMA kicks off SBA to execute the task; SBA runs the task and reports results back to PMA; PMA reports task completion back to the orchestrator via MCP.
  Task get/result then reflect completion when the orchestrator has received that completion from PMA (e.g. via MCP or polling).
- Remove or repurpose the blocking path that currently runs orchestrator-side inference in the handler; the execution path is orchestrator -> PMA -> SBA, with completion reported asynchronously.

## 7 Recommended Spec Updates

To align specs with the intended task-create handoff flow and support implementation and E2E validation, the following updates are recommended.

### 7.1 Orchestrator ([`orchestrator.md`](../../tech_specs/orchestrator.md))

- Add a **Task create handoff** rule (or subsection under User API Gateway / task handling): for prompt-mode task create, the orchestrator MUST (1) create the task record; (2) send the task to PMA with instructions to execute; (3) wait only for PMA to acknowledge that the task has been **started** (handed off to SBA); (4) return `201 Created` with the task response (including `task_id`) immediately.
- State explicitly that the create HTTP handler MUST NOT block on task completion, inference, or sandbox job completion.
- Reference the flow: orchestrator -> PMA (start) -> 201; completion is reported asynchronously (e.g. via MCP or worker callback).

### 7.2 CyNode PMA ([`cynode_pma.md`](../../tech_specs/cynode_pma.md))

- In **Request Source and Orchestrator Handoff** (or a new subsection): define the **task execution handoff** contract.
  When the orchestrator sends a task for execution, PMA MUST accept it, kick off SBA to execute the task, and report back to the orchestrator that the task has been **started** (ack) before the orchestrator returns 201.
- Add that PMA MUST report task **completion** (and results) back to the orchestrator via MCP (or specified callback) after SBA has finished and PMA has aggregated the outcome.
- Optionally document the sequence: orchestrator sends task -> PMA -> SBA (execute) -> PMA receives SBA result -> PMA reports completion to orchestrator via MCP.

### 7.3 CyNode SBA ([`cynode_sba.md`](../../tech_specs/cynode_sba.md))

- Ensure **Job Lifecycle Reporting** (or equivalent) states that SBA reports execution results back to the caller (PMA) per the job lifecycle; no change may be needed if this is already clear.
- If the orchestrator-PMA-SBA flow is not spelled out, add a short note that SBA is invoked by PMA for task execution and reports results to PMA, which then reports to the orchestrator.

### 7.4 CLI and Gateway ([`cli_management_app_commands_tasks.md`](../../tech_specs/cli_management_app_commands_tasks.md), [`user_api_gateway.md`](../../tech_specs/user_api_gateway.md))

- In the task create section: state that `POST /v1/tasks` (and thus `cynork task create`) returns the task identifier in the response **without waiting for task completion**; clients poll task get/result for status and outcome.
- This may be a single sentence or bullet in each spec to make the "create returns promptly" expectation explicit and testable.

### 7.5 Requirements ([`orches.md`](../../requirements/orches.md))

- For REQ-ORCHES-0122 (or the task-create requirement in use): add that the create operation MUST return a task identifier in the response **within a bounded time** (e.g. without waiting for task execution to complete), so that clients can poll for status and result.
- This clarifies that "create" means "accept and start," not "create and run to completion."
