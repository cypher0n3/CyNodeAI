# Task 8 Completion Report - SBA Lifecycle Response Bodies

## Summary

`NotifyInProgress` and `NotifyCompletion` in `agents/internal/sba/lifecycle.go` now close HTTP response bodies after a successful `Do` (explicit `Close()` at end of function to satisfy `gocritic` / avoid unnecessary `defer`).

`lifecycle_test.go` adds `TestLifecycleBodyClose` with a mock server that counts `Close` on the response body.

## Validation

- `go test -v -run TestLifecycleBodyClose ./agents/internal/sba/...`
- `just lint` / `just test-go-cover` (as run with the short-term plan)

## Plan

YAML `st-079`-`st-086` and Task 8 markdown checklists updated in `docs/dev_docs/_plan_003_short_term.md`.
