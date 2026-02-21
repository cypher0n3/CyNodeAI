# Cynork Buildout Plan (Orchestrator-Aligned)

- [1. Current State of the Orchestrator](#1-current-state-of-the-orchestrator)
  - [1.1 User API Gateway (User-Gateway)](#11-user-api-gateway-user-gateway)
  - [1.2 What Cynork Currently Implements](#12-what-cynork-currently-implements)
  - [1.3 Feature File and BDD Gap](#13-feature-file-and-bdd-gap)
- [2. Orchestrator Capabilities Buildout (First)](#2-orchestrator-capabilities-buildout-first)
  - [2.0 Alignment with current implementation and specs](#20-alignment-with-current-implementation-and-specs)
  - [2.1 Task List, Get, Cancel, Logs](#21-task-list-get-cancel-logs)
    - [2.1.1 Task List](#211-task-list)
    - [2.1.2 Task Get (Verify Shape and Ownership)](#212-task-get-verify-shape-and-ownership)
    - [2.1.3 Task Cancel](#213-task-cancel)
    - [2.1.4 Task Logs](#214-task-logs)
    - [2.1.5 Task Artifacts (Skipped)](#215-task-artifacts-skipped)
  - [2.2 Chat Endpoint](#22-chat-endpoint)
  - [2.3 Testing and BDD Requirements (Orchestrator)](#23-testing-and-bdd-requirements-orchestrator)
- [3. Scope: What Cynork Can Build Once Orchestrator is Ready](#3-scope-what-cynork-can-build-once-orchestrator-is-ready)
- [4. Implementation Plan (Cynork)](#4-implementation-plan-cynork)
  - [4.1 Orchestrator Changes Summary](#41-orchestrator-changes-summary)
  - [4.2 Cynork CLI Buildout](#42-cynork-cli-buildout)
  - [4.3 Feature Files and BDD](#43-feature-files-and-bdd)
  - [4.4 Unit Tests](#44-unit-tests)
  - [4.5 Lint and CI](#45-lint-and-ci)
- [5. Implementation Order (Suggested)](#5-implementation-order-suggested)
- [6. Out of Scope (Current Plan)](#6-out-of-scope-current-plan)
- [7. References](#7-references)

## 1. Current State of the Orchestrator

This plan builds out `cynork` to the extent supported by the current orchestrator, including interactive mode and chat, plus feature files, BDD, and unit tests so that `just ci` succeeds.

References: [meta.md](../meta.md), [docs/tech_specs/cli_management_app.md](../docs/tech_specs/cli_management_app.md), [docs/mvp_plan.md](../docs/mvp_plan.md), [.github/copilot-instructions.md](../.github/copilot-instructions.md).

### 1.1 User API Gateway (User-Gateway)

Current routes and implementation details (see `orchestrator/cmd/user-gateway/main.go`).

- `GET /healthz` (no auth)
- `POST /v1/auth/login`, `POST /v1/auth/refresh` (no auth)
- `POST /v1/auth/logout`, `GET /v1/users/me` (auth required)
- `POST /v1/tasks`, `GET /v1/tasks/{id}`, `GET /v1/tasks/{id}/result` (auth required)

**Handlers:** `TaskHandler` is created with `NewTaskHandler(store, logger, cfg.InferenceURL, cfg.InferenceModel)`; inference is used for prompt-mode tasks when configured.
`GetTask` and `GetTaskResult` do **not** enforce task ownership (any authenticated user can read any task by ID); they should be updated to return 403 when `task.CreatedBy != userID`.

**Database (orchestrator/internal/database):** `ListTasksByUser(ctx, userID, limit, offset)` exists; no status filter in DB.
`UpdateTaskStatus`, `UpdateJobStatus`, `GetTaskByID`, `GetJobsByTaskID` exist.
Job `Result` is stored as JSON of `workerapi.RunJobResponse` (Stdout, Stderr, Status, etc.) by `dispatcher/run.applyJobResult`.

**Models (orchestrator/internal/models):** Task statuses: `pending`, `running`, `completed`, `failed`, `cancelled` (UK spelling).
CLI spec expects: `queued`, `running`, `completed`, `failed`, `canceled` (US spelling).
No `task_name` field on Task; optional display name can come from `Summary` or be omitted for MVP.

**Not implemented (no routes):** task list, task cancel, task logs, task artifacts, chat, credentials, preferences, system settings, nodes, skills, audit.
These capabilities are critical for properly testing cynork functionality.
The plan below builds them out in the orchestrator first, with the same testing and BDD requirements (unit tests, feature files, BDD steps, coverage, `just ci`), before implementing cynork against them.

### 1.2 What Cynork Currently Implements

Commands: `version`, `status`, `auth login` / `logout` / `whoami`, `task create`, `task result`.

Config: file plus env (`CYNORK_GATEWAY_URL`, `CYNORK_TOKEN`); token resolution (no credential helper in code yet).

Gateway client: `Health`, `Login`, `GetMe`, `CreateTask`, `GetTaskResult`.
No `ListTasks`, `GetTask`, `CancelTask`, or chat API.

### 1.3 Feature File and BDD Gap

`features/cynork/cynork_cli.feature` already has scenarios for: status, auth, task create/result, task list, task get, task cancel, task logs, creds list, prefs set/get, settings set/get, nodes list, skills load, audit list, session persistence, interactive mode (tab completion), chat (no-token fails; with session accepts `/exit`).

`cynork/_bdd/steps.go` implements only: mock gateway, status, auth login/whoami, task create, store task id, task result.
Missing steps for: task list, task get, task cancel, task logs, task-file/script/commands/attachments, creds/prefs/settings/nodes/skills/audit, session persistence (whoami with stored config), shell, chat.

Mock gateway in BDD: only `GET /healthz`, `POST /v1/auth/login`, `GET /v1/users/me`, `POST /v1/tasks`, `GET /v1/tasks/{id}/result`.
Missing: `GET /v1/tasks`, `GET /v1/tasks/{id}`, cancel, and stubs for creds/prefs/nodes/skills/audit/settings so those scenarios pass against the mock.

## 2. Orchestrator Capabilities Buildout (First)

Build out the following capabilities in the User API Gateway (orchestrator) first.
This section is scoped to MVP-critical capabilities; some non-critical functionality has been intentionally omitted and can be added in a later phase.
Each capability below is specified with explicit routes, request/response shapes, handler and DB usage, unit test cases, feature scenarios, and BDD steps.
Deliverables must achieve at least 90% package-level coverage and pass `just ci` before cynork is implemented against these endpoints.

### 2.0 Alignment With Current Implementation and Specs

**Ownership:** `GetTask` and `GetTaskResult` currently do not check `task.CreatedBy` vs the authenticated user.
All task endpoints that return or mutate a single task MUST enforce ownership: return 403 if `task.CreatedBy == nil` or `*task.CreatedBy != userID` from `getUserIDFromContext(ctx)`.
Apply this to GetTask, GetTaskResult, CancelTask, and GetTaskLogs so list/get/cancel/logs are consistent.

**Status enum:** CLI spec ([cli_management_app.md](../docs/tech_specs/cli_management_app.md)) uses `queued`, `running`, `completed`, `failed`, `canceled`.
Orchestrator models use `pending`, `cancelled` (UK).
Gateway responses (list, get, result, cancel) SHOULD use the CLI spec values so cynork can consume them as-is; either return spec values from handlers (e.g. map `pending` to `queued`, `cancelled` to `canceled`) or document that cynork MUST normalize.

**Response field names:** CLI spec requires `task_id` (and optionally `task_name`) in list/get responses.
List response: each task object MUST include `task_id`, `status`; include `task_name` when the system provides one (e.g. from `Summary` or omit for MVP).
GetTask response: same; ensure `TaskResponse` or list item struct uses JSON tag `task_id` for the id field (or cynork maps `id` to task_id).

**BDD test server:** The orchestrator BDD suite (`orchestrator/_bdd/steps.go`) runs its own HTTP server with the same handlers as user-gateway (auth, tasks, nodes).
When adding new user-gateway routes (list, cancel, logs, chat), register the same routes and handler methods on that BDD mux so new scenarios can call them; use the same `taskHandler` instance and auth middleware.

**Chat and CreateTask:** Chat must create a task and wait for a terminal result.
Either: (a) add a non-HTTP helper (e.g. `createTaskAndWaitForResult`) used by Chat and optionally by CreateTask, or (b) have Chat call `db.CreateTask`, `db.CreateJob`, then poll `GetTaskByID` / `GetJobsByTaskID` until task status is terminal, then build response from job Result.
`TaskHandler` already has `inferenceURL` and `inferenceModel`; Chat can call the same orchestrator-side inference path when configured.
Wire `POST /v1/chat` with `limitBody(maxBodyBytes, ...)` in main.go like other POST handlers.

**Job Result format:** `Job.Result` is stored as JSON of `workerapi.RunJobResponse` (Stdout, Stderr, Status, etc.).
GetTaskLogs MUST unmarshal each job's Result into that struct (or a struct with Stdout/Stderr) and aggregate per the `stream` query param.

#### 2.0.1 Gaps Checklist (Implement in Order)

- [ ] **GetTask / GetTaskResult:** Add ownership check; return 403 if not task owner.
- [ ] **ListTasks:** New handler and route; response shape with `task_id`, `status`, `task_name` (optional); status filter in memory; pagination via offset/limit.
- [ ] **Status mapping:** Decide and implement: gateway returns spec enum (queued, canceled) or cynork maps (pending, cancelled).
- [ ] **CancelTask:** New handler and route; ownership check; UpdateTaskStatus + UpdateJobStatus for each job; response `canceled: true`.
- [ ] **GetTaskLogs:** New handler and route; ownership check; parse Job.Result JSON; aggregate stdout/stderr; `stream` query param.
- [ ] **Chat:** New handler/method; create task + job; wait for terminal; return response body; wire with limitBody.
- [ ] **BDD:** Register new routes in `orchestrator/_bdd/steps.go` mux; add step definitions and scenarios for list, get (if missing), cancel, logs, chat.
- [ ] **Cynork:** After orchestrator is done, implement cynork commands and gateway client methods (section 4); extend cynork BDD mock or run against real gateway.

### 2.1 Task List, Get, Cancel, Logs

The following subsections specify task list, get, cancel, logs, and artifacts (skipped).

#### 2.1.1 Task List

**Route:** `GET /v1/tasks` (auth required).

**Query parameters:** `limit` (optional, default 50, min 1, max 200); `cursor` or `offset` (optional, for pagination); `status` (optional, filter: queued/running/completed/failed/cancelled).

**Response:** 200 OK, body `{"tasks":[...],"next_cursor":"opaque"}` or `{"tasks":[...],"next_offset":N}` with integer N (DB has offset only).
Each task object MUST include `task_id`, `status` per CLI spec; include `prompt`, `summary`, `created_at`, `updated_at`, and `task_name` when supported (task_name from Summary or omit for MVP).

**Handler:** New method `ListTasks` on `TaskHandler` in `orchestrator/internal/handlers/tasks.go`.
Obtain user ID from context (`getUserIDFromContext`); call `db.ListTasksByUser(ctx, userID, limit, offset)`; filter by status in memory if needed; return 500 on DB error.
List is already scoped to user so no separate ownership check.

**Wire:** In `orchestrator/cmd/user-gateway/main.go`: `mux.Handle("GET /v1/tasks", authMiddleware.RequireUserAuth(http.HandlerFunc(taskHandler.ListTasks)))`.
Register the same route and handler in `orchestrator/_bdd/steps.go` on the BDD test server mux.

**Unit tests:** Success (returns list), empty list, invalid limit, DB error (500); use mock DB; table-driven.
**Feature file:** Add scenario(s) to `features/orchestrator/orchestrator_task_lifecycle.feature` (e.g. "List tasks as authenticated user").
**BDD steps:** In `orchestrator/_bdd/steps.go`, add steps for "I list tasks with limit N", "I receive a list of tasks" / response contains task_id and status.
**Done:** `just test-go-cover` and `just test-bdd` pass.

#### 2.1.2 Task Get (Verify Shape and Ownership)

**Route:** Existing `GET /v1/tasks/{id}`.
**Required:** (1) Enforce ownership: after `GetTaskByID`, return 403 if `task.CreatedBy == nil` or `*task.CreatedBy != *getUserIDFromContext(ctx)`.
(2) Response shape per CLI spec: include `task_id` (or `id`; cynork expects task_id), `status`, `prompt`, `summary`, `created_at`, `updated_at`; add `task_name` to `TaskResponse` when supported (e.g. from Summary).
(3) Add/extend unit test for response JSON shape and for 403 when not owner; BDD step for "get task details" if missing.

#### 2.1.3 Task Cancel

**Route:** `POST /v1/tasks/{id}/cancel` (auth required), no body.
**Response:** 200 OK, body `{"task_id":"uuid","canceled":true}`; 404 not found; 403 not owner.

**Handler:** New method `CancelTask` on `TaskHandler`.
Parse task ID from path; get user ID from `getUserIDFromContext(ctx)`; `GetTaskByID` (404 if not found); return 403 if `task.CreatedBy == nil` or `*task.CreatedBy != userID`; `UpdateTaskStatus(ctx, taskID, models.TaskStatusCancelled)`; for each job from `GetJobsByTaskID`, `UpdateJobStatus(ctx, jobID, models.JobStatusCancelled)`; return 200 with body `{"task_id": taskID.String(), "canceled": true}` (CLI spec uses "canceled").

**Wire:** `mux.Handle("POST /v1/tasks/{id}/cancel", authMiddleware.RequireUserAuth(http.HandlerFunc(taskHandler.CancelTask)))`.
Register same route in `orchestrator/_bdd/steps.go`.

**Unit tests:** Success; not found (404); wrong user (403); DB error (500).
**Feature file:** Scenario "Cancel task as owner".
**BDD steps:** "I cancel the task", "the task status is cancelled" (POST with auth; optionally GET to assert).
**Done:** Coverage and BDD pass.

#### 2.1.4 Task Logs

**Route:** `GET /v1/tasks/{id}/logs` (auth required).
**Query:** `stream` (optional): stdout | stderr | all (default).
**Response:** 200 OK, body e.g. `{"task_id":"id","stdout":"content","stderr":"content"}`; 404/403 as above.

**Handler:** New method `GetTaskLogs` on `TaskHandler`.
Get task (404 if not found); return 403 if not owner (`task.CreatedBy == nil` or `*task.CreatedBy != userID`); `GetJobsByTaskID`; parse each job `Result` as JSON (e.g. unmarshal into struct with Stdout, Stderr matching `workerapi.RunJobResponse`); aggregate per `stream` query; return body.

**Wire:** `mux.Handle("GET /v1/tasks/{id}/logs", authMiddleware.RequireUserAuth(http.HandlerFunc(taskHandler.GetTaskLogs)))`.
Register same route in `orchestrator/_bdd/steps.go`.

**Unit tests:** Success; not found; not owner; no jobs (empty); malformed result (graceful).
**Feature file:** Scenario "Get task logs".
**BDD steps:** "I get the task logs", "the response contains stdout/stderr".
**Done:** Coverage and BDD pass.

#### 2.1.5 Task Artifacts (Skipped)

Skipped for MVP; add list/download endpoints in a later phase.

### 2.2 Chat Endpoint

**Route:** `POST /v1/chat` (auth required).
**Request body:** `{"message":"user message"}`, Content-Type application/json.
**Response:** 200 OK, body `{"response":"model or system response"}`.
MVP: create task with message as prompt (same as task create prompt path); wait until task reaches terminal status (poll or block); read result from task result (e.g. job stdout/summary); return as `response`.
400 if message missing/empty; 401/403 from middleware; 500 on error.

**Handler:** New method `Chat` on `TaskHandler` (or dedicated handler that has access to store and inference); parse body; create task + job (reuse or extract CreateTask logic); wait for terminal status (poll GetTaskByID / GetJobsByTaskID); build response from job Result (e.g. stdout/summary); return JSON.
When `TaskHandler.inferenceURL` is set, use same orchestrator-side inference path as prompt-mode CreateTask.

**Wire:** In main.go: `mux.Handle("POST /v1/chat", authMiddleware.RequireUserAuth(http.HandlerFunc(limitBody(maxBodyBytes, taskHandler.Chat))))`.
Register same route in `orchestrator/_bdd/steps.go` (with limitBody if BDD server uses it).

**Unit tests:** Success (message in, response out); empty message (400); auth; internal error (500); mock DB and optionally mock task completion.
**Feature file:** Scenario "Chat returns model response" (e.g. in `orchestrator_task_lifecycle.feature` or new `orchestrator_chat.feature`).
**BDD steps:** "I send a chat message" with text; "I receive a 200 response with non-empty response field"; POST `/v1/chat` with body and auth.
**Done:** Coverage and BDD pass.

Credentials API is skipped for now and can be added in a later phase.

### 2.3 Testing and BDD Requirements (Orchestrator)

Apply to every new or touched handler above.

**Unit tests (explicit):** File: `orchestrator/internal/handlers/` (e.g. `handlers_mockdb_test.go`, `tasks_test.go`, `chat_test.go`).
For each handler: success, not found (404), forbidden (403), bad request (400), internal error (500).
Table-driven where appropriate; use `testutil.MockDB`; no new coverage exceptions; 90%+ per package.

**Feature files (explicit):** Directory `features/orchestrator/`; extend `orchestrator_task_lifecycle.feature` for list, get, cancel, logs; add chat scenario there or in new file.
Each scenario: one `@suite_orchestrator`; tags `@req_*`, `@spec_*` per [features/README.md](../features/README.md); clear Given/When/Then.

**BDD steps (explicit):** File `orchestrator/_bdd/steps.go`.
Add step functions for every new When/Then (e.g. "I list tasks with limit N", "I cancel the task", "the task status is cancelled", "I get the task logs", "I send a chat message" with text).
Steps call test HTTP server with method, path, query, body, auth; reuse existing login/token setup.
Run `just test-bdd`; all orchestrator scenarios must pass.

**CI gate:** After each capability or batch, run `just ci`.
Fix lint (`just lint-go`, `just lint-go-ci`), coverage (`just test-go-cover`), BDD (`just test-bdd`), vuln (`just vulncheck-go`).
Do not proceed to cynork until `just ci` passes for the orchestrator buildout.

## 3. Scope: What Cynork Can Build Once Orchestrator is Ready

| Capability                              | Orchestrator support                             | Cynork action                                                                         |
| --------------------------------------- | ------------------------------------------------ | ------------------------------------------------------------------------------------- |
| Task list                               | Build in orchestrator first (see section 2)      | Cynork `task list` once `GET /v1/tasks` exists                                        |
| Task get                                | Exists; ensure spec shape                        | Add cynork `task get`                                                                 |
| Task result                             | Exists                                           | Already done; align output with spec                                                  |
| Task cancel                             | Build in orchestrator first (see section 2)      | Cynork `task cancel` once endpoint exists                                             |
| Task logs                               | Build in orchestrator first (see section 2)      | Cynork `task logs` once `GET /v1/tasks/{id}/logs` exists                              |
| Task artifacts                          | Omitted from MVP (section 2); add in later phase | Cynork stub or mock until orchestrator supports them                                  |
| Chat                                    | Build in orchestrator first (see section 2)      | Cynork `chat` calls gateway chat endpoint (or task+result loop until endpoint exists) |
| Interactive mode                        | No gateway change                                | Add `cynork shell` (REPL) and optional `shell -c "..."`                               |
| Creds/prefs/nodes/skills/audit/settings | Omitted from MVP (section 2); add in later phase | Cynork stub or mock until orchestrator supports them                                  |

## 4. Implementation Plan (Cynork)

The following sections summarize orchestrator changes (detailed in section 2), then cynork CLI buildout, feature files and BDD, and unit tests.
Cynork implementation follows completion of the orchestrator capabilities in section 2.

### 4.1 Orchestrator Changes Summary

Section 2 defines the MVP orchestrator buildout (task list/cancel/logs, chat) with tests and BDD; task artifacts, credentials, prefs, settings, nodes, skills, and audit are omitted for MVP.
Summary of endpoints cynork will use: `GET /v1/tasks` (list), `POST /v1/tasks/{id}/cancel` (cancel), `GET /v1/tasks/{id}/logs` (logs), and chat as in section 2.

### 4.2 Cynork CLI Buildout

Implement commands, output format, and input modes per CLI spec.

#### 4.2.1 Output and Exit Codes

- Align `task create` output with spec: table mode `task_id=<id>`; JSON mode `{"task_id":"<id>"}`.
- Use exit codes per spec (2 usage, 3 auth, 4 not found, 5 conflict, 6 validation, 7 gateway, 8 internal).

#### 4.2.2 Task Commands

- **task list:** `GET /v1/tasks`; flags `--status`, `-l/--limit`, `--cursor`; table/JSON output per spec.
- **task get:** `GET /v1/tasks/{id}`; output `task_id`, `status`, `task_name` if present.
- **task cancel:** `POST /v1/tasks/{id}/cancel` (or PATCH); `-y/--yes` to skip confirmation; confirmation prompt per spec.
- **task result:** Already present; add `-w/--wait` to poll until terminal status; ensure table/JSON format matches spec (stdout/stderr in terminal status).
- **task logs:** If orchestrator adds logs endpoint, implement; else stub or skip and BDD mock only.

#### 4.2.3 Task Create Input Modes (Spec Alignment)

- Support exactly one of: `-p/--prompt`, `-f/--task-file`, `-s/--script`, `--command` (repeatable), `--commands-file`.
- Support `-a/--attach` (repeatable, with size/count limits per spec).
- Path validation (file exists, regular file, readable); exit 2 on validation failure.
- Size limits: task-file 1 MiB, script 256 KiB, commands-file 64 KiB, attach 10 MiB each, max 16 attachments.

#### 4.2.4 Chat Command

- **cynork chat:** Resolve config/token; if empty exit 3.
- Loop: read line from stdin; if `/exit`, `/quit`, or EOF, exit 0.
- Otherwise send message via gateway.
- With current orchestrator: implement as create task (prompt = line) plus poll `GET /v1/tasks/{id}/result` until terminal status, then print result to user.
- No new gateway endpoint.
- Future: dedicated chat endpoint can replace this.

#### 4.2.5 Interactive Mode (Shell)

- **cynork shell:** REPL: prompt (e.g. show gateway URL or label; optionally whoami handle); read line; parse as cynork argv (split on spaces, respect quotes); run same command surface (version, status, auth, task, etc.); repeat.
- No history of secrets (REQ-CLIENT-0140).
- **cynork shell -c "command":** Run single command and exit with its exit code.
- Tab completion: at least for commands/subcommands and flags; task-name completion can call `GET /v1/tasks` and suggest names (spec allows gateway-backed completion).

#### 4.2.6 Commands Calling Real Gateway

- **creds, prefs, nodes, skills, audit, settings:** CLI calls gateway paths per tech spec.
- Once orchestrator capabilities are built (section 2), cynork uses real endpoints; BDD can run against real gateway or a mock that mirrors the real API shape.

### 4.3 Feature Files and BDD

Keep feature files and BDD steps in sync with new behavior.

#### 4.3.1 Feature Files

- **features/cynork/cynork_cli.feature:** Already has scenarios; add any missing ones for task list/get/cancel/logs, chat, shell.
- Ensure traceability tags (`@req_*`, `@spec_*`) and single `@suite_cynork` per file; keep under `features/cynork/`.
- **features/orchestrator:** If new routes (list, cancel) are added, add or extend scenarios in existing orchestrator feature files (e.g. task lifecycle) for list and cancel; keep tag/location rules from [features/README.md](../features/README.md).

#### 4.3.2 BDD Steps (Cynork `_bdd/steps.go`)

- Implement all steps referenced by cynork_cli.feature: status (with shorthand `-o json`), login, whoami (with/without stored config), task create (prompt, task-file, script, commands, attachments), store task id from stdout (parse `task_id=` if table mode), task result, task list, task get, task cancel (with `--yes`), task logs.
- Steps for: creds list, prefs set/get, settings set/get, nodes list, skills load, audit list, session persistence (whoami using stored config), shell (interactive mode, optional `-c`), chat (no token yields exit 3; with session, send `/exit` yields exit 0).
- Extend mock gateway: add `GET /v1/tasks` (return list), `GET /v1/tasks/{id}` (return single task), `POST /v1/tasks/{id}/cancel` (return success), `GET /v1/tasks/{id}/logs` (stub), and stub endpoints for creds, prefs, settings, nodes, skills, audit so those scenarios pass.

#### 4.3.3 Task ID in BDD

- Cynork task create should output `task_id=<id>` in table mode so step "I store the task id from cynork stdout" can parse it (e.g. extract value after `task_id=`).
- JSON mode: parse `{"task_id":"..."}`.

### 4.4 Unit Tests

Add and maintain tests for new handlers and CLI commands.
Orchestrator handler tests are covered in section 2.9.

#### 4.4.1 Cynork Tests

- **internal/gateway:** Add tests for `ListTasks`, `GetTask`, `CancelTask` (and logs if implemented); mock HTTP responses; cover 4xx/5xx and parseError.
- **cmd:** Test root and subcommands (task list, get, cancel, result with --wait, shell -c, chat exit behavior) with mocked gateway or env; test exit codes and output format (table/JSON).
- **internal/config:** Already tested; ensure token resolution and env overrides stay covered.

#### 4.4.2 Test Coverage

- Maintain at least 90% package-level coverage for orchestrator and cynork; no new broad exclusions.
- Use `just test-go-cover`; fix any package below threshold.

### 4.5 Lint and CI

- Run `just fmt-go`, `just lint-go`, `just lint-go-ci`, `just lint-md` (on changed docs), `just validate-feature-files`, `just validate-doc-links`.
- Run `just test-go-cover` (all modules) and `just test-bdd` (orchestrator, worker_node, cynork).
- Run `just vulncheck-go`.
- Final gate: **`just ci`** must pass (all of the above).

## 5. Implementation Order (Suggested)

1. **Orchestrator capabilities (section 2), in dependency order:** Task list, cancel, logs (handlers, routes, DB as needed); then chat (task artifacts, credentials, prefs, settings, nodes, skills, audit omitted for MVP).
   For each capability: add handlers and routes, unit tests, orchestrator feature file scenarios, BDD steps in `orchestrator/_bdd/steps.go`; run `just test-go-cover` and `just test-bdd`; ensure `just ci` passes before moving to the next (or batch).
2. Cynork gateway client: Add methods for new MVP endpoints (list, get, cancel, logs, chat); unit tests.
3. Cynork task commands: task list, get, cancel, result (with --wait), logs; task create output format and input modes; stub or skip task artifacts until orchestrator supports them; unit tests.
4. Cynork chat: Call gateway chat endpoint (or task+result loop fallback); unit tests.
5. Cynork shell: REPL and `shell -c`; optional tab completion; unit tests.
6. Cynork creds/prefs/settings/nodes/skills/audit commands: Stub or mock until orchestrator supports them; BDD mock gateway can return stub responses.
7. Cynork BDD: Implement all missing steps; mock gateway can mirror real API or run against real gateway; run `just test-bdd`.
8. Feature files: Add or adjust cynork and orchestrator scenarios; run `just validate-feature-files`.
9. Full CI: `just ci`; fix coverage, lint, and any remaining BDD failures.

## 6. Out of Scope (Current Plan)

- Real credential helper protocol (optional; can stub in BDD).
- Dedicated streaming chat protocol beyond the MVP chat endpoint (section 2.2).

## 7. References

- [docs/tech_specs/cli_management_app.md](../docs/tech_specs/cli_management_app.md): CLI spec (chat, interactive mode, task commands, output, exit codes).
- [docs/tech_specs/user_api_gateway.md](../docs/tech_specs/user_api_gateway.md): Gateway capabilities.
- [docs/requirements/client.md](../docs/requirements/client.md): Client requirements.
- [docs/mvp_plan.md](../docs/mvp_plan.md): MVP phases, BDD, feature files, coverage.
- [features/README.md](../features/README.md): Feature file and suite tag rules.
- [justfile](../justfile): `just ci`, `just test-bdd`, `just test-go-cover`.
