# Plan 004 -- Task 2 Completion Report

<!-- Date: 2026-03-30 -->

## Summary

Split `database.Store` into focused sub-interfaces (`UserStore`, `TaskStore`, `NodeStore`,
`ChatStore`, `PreferenceStore`, `SkillStore`, `WorkflowStore`, `SystemSettingsStore`,
`Transactional`) in `orchestrator/internal/database/store_interfaces.go`.
`Store` embeds
all of them for backward compatibility.
Added `WorkflowHandlerDeps` and
`OpenAIChatHandlerDeps` for minimal handler surfaces.

Handlers now take the narrowest dependency: `TaskHandler` uses `TaskStore`, `NodeHandler`
uses `NodeStore`, `UserHandler` and `AuthHandler` use `UserStore`, `SkillsHandler` uses
`SkillStore`, `WorkflowHandler` uses `WorkflowHandlerDeps`, `OpenAIChatHandler` uses
`OpenAIChatHandlerDeps`.
`mcptaskbridge` list/result/cancel/logs functions take
`TaskStore` instead of full `Store`.

Compile-time `var _ SubStore = (*DB)(nil)` checks and matching `MockDB` checks were added.

## Validation

- `go build ./orchestrator/...`: pass
- `go test ./orchestrator/...`: pass
- `go test -cover ./orchestrator/internal/handlers/...`: ~90.3% statements

## Note

Plan text referenced `orchestrator/internal/store/` and `PostgresStore`; implementation
uses existing `database` package and `*DB` as in the codebase.
