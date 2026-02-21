# Cynork Buildout Plan - Execution Report

- [Summary](#summary)
- [Completed (Orchestrator)](#completed-orchestrator)
- [Pending / Notes](#pending--notes)
- [References](#references)

## Summary

**Date:** 2026-02-21  
**Plan:** [cynork_buildout_plan.md](cynork_buildout_plan.md)

Orchestrator capabilities from Section 2 of the plan were implemented: ownership checks, status mapping, ListTasks, CancelTask, GetTaskLogs, and Chat.
Routes are wired in user-gateway and in the BDD mux.
BDD steps and feature scenarios were added.
Cynork CLI buildout (Section 4) is pending and depends on orchestrator completion.

## Completed (Orchestrator)

Orchestrator user-gateway now exposes task list, cancel, logs, and chat; all with ownership and spec-aligned responses.

### Gaps Implemented

- **GetTask / GetTaskResult:** Ownership check added; 403 when `task.CreatedBy == nil` or not equal to authenticated user.
- **ListTasks:** Handler and route `GET /v1/tasks`; query params `limit`, `offset`, `status`; response `tasks`, `next_offset`; status filter in memory; spec enum in response.
- **Status mapping:** Gateway returns CLI spec enum: `pending` -> `queued`, `cancelled` -> `canceled`.
- **CancelTask:** Handler and route `POST /v1/tasks/{id}/cancel`; ownership check; task and jobs set to cancelled.
- **GetTaskLogs:** Handler and route `GET /v1/tasks/{id}/logs`; query `stream` (stdout|stderr|all); Job.Result parsed as `workerapi.RunJobResponse`.
- **Chat:** Handler and route `POST /v1/chat`; body `message`; creates task (+ optional orchestrator inference); polls until terminal; returns `response`.
- **BDD:** New routes registered in `orchestrator/_bdd/steps.go`; steps added for list tasks, cancel task, task status cancelled, get task logs, chat message, response assertion; create-task response structs updated to `task_id`; feature scenarios added for list, cancel, logs, chat.

### Response Shape

- `TaskResponse`: `task_id`, `status` (spec enum), optional `task_name` (from Summary), `prompt`, `summary`, `created_at`, `updated_at`.
- List: `ListTasksResponse{ tasks, next_offset }`.
- Cancel: `CancelTaskResponse{ task_id, canceled }`.
- Logs: `GetTaskLogsResponse{ task_id, stdout, stderr }`.
- Chat: `ChatResponse{ response }`.

### Unit Tests

- Unit tests in `orchestrator/internal/handlers/`: ownership (GetTask/GetTaskResult 403),
  ListTasks (success, no user, invalid limit/offset, DB error, status filter, next_offset),
  CancelTask (success, not found, forbidden, DB errors, with jobs),
  GetTaskLogs (success, forbidden, stream, DB error, malformed result),
  Chat (empty message, no user, invalid body, create task error, inference success,
  inference fallback to poll, poll success, context cancelled, GetTask error in poll,
  GetJobs error in terminal branch).
- Handler package coverage: **89.6%** (target 90%).
  Remaining gap is in handlers package (auth/nodes/tasks combined);
  no new coverage exclusions were added per project rules.

### Files Touched

- `orchestrator/internal/handlers/tasks.go` - New handlers and status mapping.
- `orchestrator/cmd/user-gateway/main.go` - New routes.
- `orchestrator/_bdd/steps.go` - New routes on mux, new steps, create response `task_id`.
- `features/orchestrator/orchestrator_task_lifecycle.feature` - Status "queued", scenarios for list, cancel, logs, chat.
- `orchestrator/internal/handlers/*_test.go` - TaskResponse.TaskID, ownership, new handler tests.

## Pending / Notes

1. **Coverage:** `internal/handlers` is at 89.6%.
   To reach 90% without changing justfile/Makefile, add tests for remaining branches (e.g. auth/nodes) or temporarily relax the threshold for this package if the project allows.
2. **CI gate:** Run `just ci` with `POSTGRES_TEST_DSN` set (or testcontainers) so orchestrator BDD scenarios run; Chat scenario depends on task completion (dispatcher or inference).
3. **Cynork (Section 4):** After orchestrator is green, implement cynork gateway client (ListTasks, GetTask, CancelTask, GetTaskLogs, Chat), then CLI commands and BDD.

## References

- Plan: [dev_docs/cynork_buildout_plan.md](cynork_buildout_plan.md)
- CLI spec: [docs/tech_specs/cli_management_app.md](../docs/tech_specs/cli_management_app.md)
- Justfile: `just ci`, `just test-go-cover`, `just test-bdd`
