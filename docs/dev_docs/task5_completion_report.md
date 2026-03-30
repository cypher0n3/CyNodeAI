# Task 5 Completion Report - Workflow Handler Auth

## Summary

Control-plane workflow routes were already wrapped with `middleware.RequireWorkflowRunnerAuth(cfg.WorkflowRunnerBearerToken)` in `orchestrator/cmd/control-plane/main.go` (start, resume, checkpoint, release).
Handler methods in `workflow.go` do not duplicate auth; enforcement is at the HTTP boundary.

Added `handlers/workflow_auth_test.go` (`package handlers_test`) to assert each workflow handler returns **401** when wrapped with a non-empty runner token and the request has no valid `Authorization: Bearer` header.
Uses an external test package to avoid an import cycle (`middleware` already depends on `handlers` for `WriteUnauthorized`).

## Validation

- `go test -v -run TestWorkflowAuth ./orchestrator/internal/handlers/...`
- `just lint-go` and `just test-go-cover` (orchestrator packages >= 90%)

## Plan

YAML `st-047`-`st-056` and Task 5 markdown checklists marked completed in `docs/dev_docs/_plan_003_short_term.md`.
