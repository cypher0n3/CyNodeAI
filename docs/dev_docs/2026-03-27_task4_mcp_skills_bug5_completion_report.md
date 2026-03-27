# Task 4 Completion Report: MCP Skills and Bug 5

## Summary

Bug 5 described `skills.*` MCP tool calls returning `400 {"error":"task_id required"}` despite valid `user_id` in `arguments`.
Code review and tests show the current MCP gateway enforces **user_id only** for `skills.*` via `requiredScopedIds` and `ValidateRequiredScopedIds`.
`task_id required` is returned only for tools that declare `TaskID: true` (for example `preference.effective`).
Direct `POST /v1/mcp/tools/call` is handled by the control-plane and does not pass through api-egress.

**Conclusion:** No handler change was required for scoped-ID validation.
Closure work adds **regression tests** (Go + BDD) and updates **`docs/dev_docs/_bugs.md`** so the issue is documented as closed with traceability.

**Date:** 2026-03-27

## What Changed

- **`orchestrator/internal/mcpgateway/handlers_test.go`:** Table case for extraneous `task_id` on `skills.create`; `TestValidateRequiredScopedIds_SkillsNeverRequireTaskID`.
- **`features/orchestrator/orchestrator_mcp_skills.feature`** + **`orchestrator/_bdd/steps_orchestrator_workflow_egress_artifacts.go`:** PM agent calls `skills.create` with `user_id`, `content`, and an extra `task_id` in `arguments`; expect `200`.
- **`docs/dev_docs/_bugs.md`:** Bug 5 marked closed with request-path notes.

## Tests Run (This Session)

- `go test ./orchestrator/internal/mcpgateway/...`
- `go test ./orchestrator/_bdd`

## Follow-Up (Environment)

- Run **`just e2e --tags control_plane`** (and optionally **`e2e_0812`** with required env) against a live dev stack to satisfy remaining plan checkboxes for Python E2E.
- **`e2e_0812`** may still skip without agent token env; unchanged by this task.

## E2E `task_id` Prereq (`ensure_e2e_task`) - 2026-03-27 Addendum

The `task_id` prereq could fail while **`cynork task create` was never invoked**: auth left **tokens in `e2e_gateway_session.json` only**, with **no `config.yaml`**, and `ensure_e2e_task` exited early on `os.path.isfile(config_path)`.
Fixed by writing minimal `gateway_url` YAML via `ensure_minimal_gateway_config_yaml`, dropping the early `isfile` check, and hardening JSON / `id` handling. **`helpers.gateway_post_task_no_inference`** reduces flakes on `POST /v1/tasks` after long MCP matrices (`e2e_0810`, `e2e_0812`).
See **Task 4 - `cynork task create` / `ensure_e2e_task`** in `docs/dev_docs/2026-03-27_consolidated_refactor_and_outstanding_work_plan_remaining_tasks.md`.
