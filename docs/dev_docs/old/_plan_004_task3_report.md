# Plan 004 - Task 3 Report (Pagination)

## Summary

Implemented bounded pagination for task jobs, skills, and chat messages; added shared query parsing and `TestPagination`.
Worker node DB list paths were already bounded (MCP uses `limit+1`); `ListNodes` and `ListChatThreads` now apply default/clamped limits when callers pass `limit<=0`.

## Changes

- **Jobs:** `database.ListJobsForTask` (+ count); `GetJobsByTaskID` loads in fixed-size chunks.
  `TaskStore` extended.
  REST and MCP use `limit`/`offset`; `userapi.TaskResultResponse` / `TaskLogsResponse` include `total_count`, `next_offset`, `next_cursor` (logs aggregate **only the requested job page**).
- **Skills:** `ListSkillsForUser` paginated with count; GET `/v1/skills` and MCP `skills.list` return `total_count` and optional `next_offset` / `next_cursor`.
- **Chat:** `ListChatMessages` now takes `limit, offset`, returns total count; thread messages API returns `total_count` and optional next fields.
  `ListChatThreads` defaults `limit` when unset.
- **Handlers:** `pagination.go` + `TestPagination` for shared `parseLimitOffsetQuery`.
- **Mocks:** `ListJobsForTask`; updated `ListChatMessages` / `ListSkillsForUser`; `errorOnJobsMockDB` implements `ListJobsForTask` for error tests.

## Verification

Use **justfile** targets (not raw `go test` / scripts).
Typical checks:

- `just test-go-race` - race detector across `go_modules`.
- `just test-go-cover` - coverage thresholds (needs Podman for orchestrator integration tests).
- `just lint-go` - vet, staticcheck, line-count guard (may fail on pre-existing &gt;1000-line files).

Last run: `just test-go-race` completed all modules until `orchestrator/cmd/control-plane`, which failed with a **data race** in `TestWaitForInferencePath_NodeAvailable` (`main_test.go` / `waitForInferencePath`) - appears **pre-existing**, not caused by pagination work.

## Follow-Up

- Run `just e2e --tags no_inference` against a live stack when available.
- Clients that relied on full job lists from GET `/v1/tasks/{id}/result` or `/logs` must paginate or repeat requests using `next_cursor` / `offset`.
