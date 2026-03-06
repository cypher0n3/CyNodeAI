# Task Creation Spec and Requirements Deviation Report

- [1 Summary](#1-summary)
- [2 Gateway and Orchestrator Deviations](#2-gateway-and-orchestrator-deviations)
- [3 CLI Deviations](#3-cli-deviations)
- [4 Remediation Plan](#4-remediation-plan)
- [5 E2E Test Failures and Remediation](#5-e2e-test-failures-and-remediation)
- [6 References](#6-references)

## 1 Summary

The current implementation supports basic task create (prompt, task name, attachments as paths, input modes, SBA, orchestrator inference).

It deviates from specs and requirements in project association, task closed consistency, gateway contract (project_id, cursor pagination), CLI task input modes and flags, path and size validation, and status enum (superseded; spelling canonical: canceled).

This document describes deviations and a remediation plan only; no code changes.

## 2 Gateway and Orchestrator Deviations

Deviations in the User API Gateway and orchestrator task-create flow.

### 2.1 Project Association (REQ-ORCHES-0133, REQ-PROJCT-0104, REQ-USRGWY)

Task creation MUST accept optional `project_id` per REQ-ORCHES-0133 and REQ-PROJCT-0104.

When omitted, the task MUST be associated with the creating user's default project.

The gateway MUST associate the task with the default project when no project is set (usrgwy.md).

- **Spec / requirement:** Task creation MUST accept optional `project_id`.
  When omitted, task MUST be associated with the creating user's default project.
  Gateway MUST associate task with default project when no project set (usrgwy.md).
- **Current implementation:** `CreateTaskRequest` has no `project_id`.
  `database.CreateTask` does not accept or set `project_id`.
  Task model has `ProjectID` but it is never set on create.
- **Deviation:** Gateway does not accept `project_id`; tasks are never associated with a project or default project.

#### 2.1.1 Steps for Project Association

- Add `project_id` (optional UUID) to `userapi.CreateTaskRequest` and to handler decode.
- In handler: resolve effective project (request `project_id` if present, else user's default project per REQ-PROJCT-0104).
  Require projects and default-project resolution to be implemented (or stubbed) before setting `task.ProjectID`.
- Extend `database.CreateTask` (or equivalent) to accept optional `projectID *uuid.UUID` and persist it on the task row.

### 2.2 Task Closed Flag (`CYNAI.SCHEMA.TaskStatusAndClosed`, `postgres_schema.md`)

When status becomes completed, failed, canceled, or superseded, `tasks.closed` MUST be set to `true` (postgres_schema.md).

- **Spec:** When status becomes completed, failed, canceled, or superseded, `tasks.closed` MUST be set to `true` (postgres_schema.md).
- **Current implementation:** `UpdateTaskStatus` updates only `status`; it does not set `closed`.
- **Deviation:** `closed` is never updated; it can remain false for terminal statuses.

#### 2.2.1 Steps for Closed Flag

- Whenever task status is set to a terminal value (completed, failed, canceled, superseded), also set `closed = true`.
  Options: (a) extend `UpdateTaskStatus` to accept or derive `closed` and update both columns; (b) add a helper that sets status and closed together; (c) use a DB trigger.
  Ensure all call sites (handlers, dispatcher) use the same rule.

### 2.3 Status Enum: Superseded and Canceled

Task status enum in spec includes `superseded` (cli_management_app_commands_tasks.md, postgres_schema.md).

Terminal statuses are completed, failed, canceled, superseded.

- **Spec:** Task status enum includes `superseded`.
  Terminal statuses: completed, failed, canceled, superseded.
- **Current implementation:** userapi uses `StatusCanceled` = "canceled"; handler maps internal `TaskStatusCanceled` to `userapi.StatusCanceled`.
  Models use `TaskStatusCanceled` = "canceled".
    BDD expects "canceled".
  No handling of "superseded".
- **Deviation:** "superseded" is not in userapi or handler mapping; status spelling is aligned (canceled).

#### 2.3.1 Steps for Status Enum

- Add "superseded" to userapi and to `taskStatusToSpec` (and any list/result filters).
  Status "canceled" is already aligned.
  Ensure task list/result and CLI treat superseded as terminal and display it correctly.

### 2.4 List Tasks Pagination (`cli_management_app_commands_tasks.md`)

Task list JSON in spec: `{"tasks":[...],"next_cursor":"<opaque>"}`.

Optional `--cursor <opaque>`.

- **Spec:** Task list JSON MUST use `next_cursor`; optional `--cursor <opaque>`.
- **Current implementation:** ListTasksResponse has `NextOffset *int`; handler uses offset-based pagination.
  CLI uses `--offset` and `--limit`.
- **Deviation:** Spec requires cursor-based pagination for task list; implementation uses offset.

#### 2.4.1 Steps for List Pagination

- Either (a) extend gateway and CLI to support cursor-based task list (opaque cursor, next_cursor in response) and keep offset as legacy, or (b) update spec to allow offset-based pagination and document `next_offset` in the API contract.
  Recommendation: align to spec with cursor for consistency with other list endpoints.

### 2.5 Attachments: File Upload Path (REQ-ORCHES-0127)

REQ-ORCHES-0127: clients MAY supply attachments as path strings (CLI) or via file upload (web console); gateway and orchestrator define how payloads are ingested.

- **Spec:** "Clients MAY supply attachments as path strings (CLI) or via file upload (web console); the gateway and orchestrator define how attachment payloads are ingested" (REQ-ORCHES-0127).
- **Current implementation:** Gateway accepts path strings only; persists them as task artifact path references.
  No file upload or ingestion of file contents.
- **Deviation:** Path-string acceptance path is implemented; file upload path is not.

#### 2.5.1 Steps for Attachments

- Document as known gap: path-only acceptance is implemented; file upload and ingestion (e.g. for web console) are future work.
  Optionally add a short "Attachment acceptance paths" subsection in the gateway or task spec stating current (path-only) vs planned (upload) behavior.

## 3 CLI Deviations

Deviations in the cynork CLI task create command and flags.

### 3.1 Task Input Modes: Exactly One and Full Set (`CYNAI.CLIENT.CliTaskCreatePrompt`)

Exactly one task input mode per create: `-t/--task` or `-p/--prompt`, or `-f/--task-file`, or `-s/--script`, or `--command` (repeatable), or `--commands-file`.

Reject zero or more than one (exit 2).

- **Spec:** Exactly one task input mode per create.
  Modes: `-t/--task` or `-p/--prompt`, or `-f/--task-file`, or `-s/--script`, or `--command` (repeatable), or `--commands-file`.
  Reject zero or more than one (exit 2).
- **Current implementation:** Single required `--prompt` and `--input-mode` (prompt/script/commands).
  No `--task`, `--task-file`, `--script`, or `--commands-file`.
  Content for script/commands is passed via prompt.
- **Deviation:** No "exactly one mode" check; only one effective mode (prompt + input-mode); no file-based modes.

#### 3.1.1 Steps for Input Modes

- Introduce distinct flags: `--task`/`--prompt` (inline), `--task-file`, `--script`, `--command` (repeatable), `--commands-file`.
  Validate exactly one mode is set; otherwise exit 2.
- For `--task-file`/`--script`/`--commands-file`, read file contents and send as prompt/script/commands payload as per spec.
  Keep `input_mode` in the request so gateway semantics stay correct.

### 3.2 Optional Flags: Name, Result, Project-Id (`cli_management_app_commands_tasks.md`)

Optional `--name <string>` for task name; include in task create request.

`--result`: after create, poll until terminal status, then print result in same format as `task result`.

`--project-id <project_id>`: include in request when provided; when omitted gateway uses default project.

- **Spec:** Optional `--name <string>` for task name; include in request.
  `--result`: after create, poll until terminal status, then print result like `task result`.
  `--project-id`: include when provided; when omitted gateway uses default project.
- **Current implementation:** CLI has `--task-name`, not `--name`.
  No `--result` on create.
  No `--project-id` on task create.
- **Deviation:** Flag name mismatch (spec `--name`, impl `--task-name`); missing `--result`; missing `--project-id` (depends on gateway 2.1).

#### 3.2.1 Steps for Optional Flags

- Add `--name` as spec alias (or rename to `--name` and keep `--task-name` as alias) and map to existing request `task_name`.
- Implement `--result`: after successful create, poll GET task result until terminal status, then output same format as `cynork task result`; on interrupt, exit without printing result.
- Add `--project-id` and include in CreateTaskRequest when gateway supports it (after 2.1).

### 3.3 Path and File Validation (`cli_management_app_commands_tasks.md`)

For `--task-file`, `--script`, `--commands-file`, each `--attach`: validate path exists, is regular file, readable; reject directories and symlinks; exit 2 on failure.

- **Spec:** For `--task-file`, `--script`, `--commands-file`, each `--attach`: validate path exists, is regular file, readable; reject directories and symlinks; exit 2 on failure.
- **Current implementation:** Attachment paths are sent to gateway without local validation.
- **Deviation:** No pre-request path validation.

#### 3.3.1 Steps for Path Validation

- Before sending create request: for every path used (task-file, script, commands-file, attach), check existence, regular file, readable, and that it is not a directory or symlink; on failure exit 2 with clear message.

### 3.4 Size Limits (`cli_management_app_commands_tasks.md`)

`--task-file` <= 1 MiB; `--script` <= 256 KiB; `--commands-file` <= 64 KiB; each `--attach` <= 10 MiB; <= 16 attach occurrences.

Exit 2 if exceeded.

- **Spec:** `--task-file` <= 1 MiB; `--script` <= 256 KiB; `--commands-file` <= 64 KiB; each `--attach` <= 10 MiB; <= 16 attach occurrences.
  Exit 2 if exceeded.
- **Current implementation:** No size or count checks.
- **Deviation:** Limits not enforced.

#### 3.4.1 Steps for Size Limits

- After reading file contents (for task-file, script, commands-file), enforce size limits; for attachments, check file size and attachment count (<= 16).
  Exit 2 when a limit is exceeded.

## 4 Remediation Plan

Prioritized list (P1 highest).

- **P1.**
  Set `closed = true` when task status becomes terminal (2.2).
  Area: Orchestrator/DB.
  Dependency: None.
- **P1.**
  Add "superseded" to status handling and canonical status spelling: canceled (2.3).
  Area: Gateway / userapi.
  Dependency: None.
- **P2.**
  Gateway: accept and persist `project_id`; default project association (2.1).
  Area: Gateway, DB, projects.
  Dependency: Default-project resolution (may be stubbed).
- **P2.**
  CLI: path validation for attachments and future task-file/script/commands-file (3.3).
  Area: CLI.
  Dependency: None.
- **P2.**
  CLI: `--result` on task create (3.2).
  Area: CLI.
  Dependency: None.
- **P3.**
  Task list: cursor-based pagination (2.4).
  Area: Gateway, CLI.
  Dependency: Optional.
- **P3.**
  CLI: exactly one input mode and full mode set (3.1).
  Area: CLI.
  Dependency: None.
- **P3.**
  CLI: `--name` alias and `--project-id` (3.2).
  Area: CLI.
  Dependency: 2.1 for project-id.
- **P3.**
  CLI: size limits (3.4).
  Area: CLI.
  Dependency: None.
- **P4.**
  Document attachment path vs file-upload (2.5).
  Area: Docs.
  Dependency: None.
- **P2 (E2E).**
  SBA E2E: ensure inference path for SBA jobs (5.1).
  Area: Worker node / E2E setup.
  Dependency: Inference proxy or network access from SBA container to Ollama.
- **P2 (E2E).**
  User-gateway readyz endpoint (5.2).
  Area: User-gateway.
  Dependency: None.
- **P3 (E2E).**
  Task list E2E: assert on `tasks` key (5.3).
  Area: E2E test scripts.
  Dependency: None.

## 5 E2E Test Failures and Remediation

Remediation for current E2E failures observed in the test run (scripts/test_scripts, e2e_*).

### 5.1 SBA Task Failures: Inference Unreachable (E2e_123_123, E2e_140_140, E2e_145_145)

Tests: `test_sba_task`, `test_sba_task_with_inference_prompt`, `test_sba_inference_reply_current_time`.

Failure: SBA task completes with status `failed`; `sba_result.failure_message` is connection refused to `http://localhost:11434/api/chat`.

Root cause: SBA container is run with `--network=none`.
The SBA inside the container cannot reach Ollama on the host (localhost:11434).
[REQ-SBAGNT-0109](../requirements/sbagnt.md#req-sbagnt-0109) requires the SBA to have access to at least one model via worker proxy or orchestrator-mediated API Egress; the runtime MUST inject the appropriate inference endpoint(s) into the sandbox.
[cynode_sba.md](../tech_specs/cynode_sba.md) (CYNAI.SBAGNT.WorkerProxies, CYNAI.SBAGNT.JobInferenceModel) specifies that inference is via node-local inference proxy or API Egress and that the orchestrator and node MUST inject the inference endpoint(s) into the sandbox.

- **Observed:** Job run uses `podman run ... --network=none ... cynodeai-cynode-sba:dev`; SBA tries to call Ollama at localhost:11434 and fails.
- **Deviation:** The inference path is supposed to be supplied via the worker proxy (per spec).
  The worker node must expose inference to the SBA container through that proxy so the container can reach it without host localhost.

#### 5.1.1 Steps for SBA Inference Path

- Supply the inference path via the worker proxy per spec.
  The worker node must provide the SBA container with a proxy URL (e.g. via env such as `OLLAMA_BASE_URL`) that is reachable from inside the container (e.g. proxy bound to a container-visible address when using `--network=none`, or shared network).
  SBA jobs that use inference (e.g. `agent_inference` execution mode) must receive this proxy endpoint so they do not try localhost:11434 from inside the container.
- Align with [worker managed services implementation plan](2026-03-04_worker_managed_services_implementation_plan.md) so E2E and production use the same inference-path contract.

### 5.2 User-Gateway Readyz Returns 404 (E2e_195_195)

Test: `test_readyz_200_or_503`.

Failure: GET readyz returns 404 (page not found); test expects 200 or 503.

Root cause: User-gateway does not expose a `/readyz` route (or it is mounted under a different path).

[go_rest_api_standards.md](../tech_specs/go_rest_api_standards.md) requires all APIs to offer `GET /readyz`: 200 when ready to accept traffic, 503 otherwise with an actionable reason.

- **Observed:** Test calls user-gateway readyz; response is 404.
- **Deviation:** User-gateway does not implement the required readyz endpoint.

#### 5.2.1 Steps for Readyz

- Add GET `/readyz` (or the path chosen by gateway spec) to the user-gateway server.
  Handler must return 200 when the service is ready to accept traffic and 503 when not (e.g. DB or dependency unavailable).
  Ensure E2E test targets the same path as the one implemented.
- If readyz is implemented elsewhere (e.g. control-plane only), document the contract and either add readyz to user-gateway or update the E2E test to call the correct service and path.

### 5.3 Task List Status Filter E2E Expects Wrong Key (E2e_196_196)

Test: `test_task_list_status_completed`.

Failure: Test asserts `"data" in data`; API response has `tasks` and `next_offset`, not `data`.

Root cause: E2E test expects a `data` key; [cli_management_app_commands_tasks.md](../tech_specs/cli_management_app_commands_tasks.md) specifies task list JSON as `{"tasks":[...],"next_cursor":"<opaque>"}` (key is `tasks`, not `data`).

- **Observed:** Response is `{"tasks": [...], "next_offset": 50}`; test does `assertIn("data", data)` and fails.
- **Deviation:** Test assertion does not match API/spec; API correctly returns `tasks`.

#### 5.3.1 Steps for Task List E2E

- Update E2E test (e.g. `e2e_196_task_list_status_filter.py`) to accept the actual API shape: require `tasks` (and optionally `next_offset` or `next_cursor`) and assert on that.
  For example: assert `"tasks" in data` and that the value is a list; drop or relax the requirement for a `data` key unless the API is explicitly extended to add it.

## 6 References

Canonical requirements and tech specs cited in this report; implementation paths and related dev docs.

- **Requirements:** [orches.md](../requirements/orches.md) (REQ-ORCHES-0122, 0126, 0127, 0128, 0133), [projct.md](../requirements/projct.md) (REQ-PROJCT-0104), [usrgwy.md](../requirements/usrgwy.md), [client.md](../requirements/client.md) (REQ-CLIENT-0151, 0153, 0157), [sbagnt.md](../requirements/sbagnt.md) (REQ-SBAGNT-0109).
- **Tech specs:** [cli_management_app_commands_tasks.md](../tech_specs/cli_management_app_commands_tasks.md), [postgres_schema.md](../tech_specs/postgres_schema.md), [projects_and_scopes.md](../tech_specs/projects_and_scopes.md), [cynode_sba.md](../tech_specs/cynode_sba.md), [go_rest_api_standards.md](../tech_specs/go_rest_api_standards.md).
- **Implementation:** `orchestrator/internal/handlers/tasks.go`, `orchestrator/internal/database/tasks.go`, `go_shared_libs/contracts/userapi/userapi.go`, `cynork/cmd/task.go`.
- **Other:** [2026-03-04_worker_managed_services_implementation_plan.md](2026-03-04_worker_managed_services_implementation_plan.md).
