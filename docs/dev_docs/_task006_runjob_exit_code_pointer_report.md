# Task 6 completion: `RunJobResponse.ExitCode` as `*int`

**Date:** 2026-03-29

## Changes

- `go_shared_libs/contracts/workerapi/workerapi.go`: `ExitCode *int` with `json:"exit_code,omitempty"`; helper `ExitCodePtr(int) *int` so zero is serialized.
- `go_shared_libs/contracts/workerapi/workerapi_test.go`: `TestExitCodeZero` plus coverage for `ValidateRequest`, `DefaultSandboxSpec`, `ExitCodePtr`, and diagnostics JSON round-trip.
- `worker_node/internal/executor/executor.go`: all assignments use `workerapi.ExitCodePtr(...)`; failure paths compare or set via nil-safe checks.
- `worker_node/internal/executor/executor_test.go`: `exitCodeVal` helper for assertions.
- `worker_node/internal/executor/executor_runjob_sba_test.go`: assertions use `exitCodeVal`.
- `worker_node/_bdd/steps.go`: exit-code step dereferences `*int`.
- `orchestrator/internal/dispatcher/run_test.go`, `orchestrator/cmd/control-plane/main_test.go`, `orchestrator/cmd/control-plane/main_run_extra_test.go`: stubs use `workerapi.ExitCodePtr(...)`.
- `orchestrator/internal/handlers/tasks.go`, `orchestrator/_bdd/steps_orchestrator_tasks_dispatch_chat.go`: synthetic responses use `ExitCodePtr`.

## Tests

- `just lint-go`
- `just test-go-cover` (includes `go_shared_libs/contracts/workerapi` at 100% after added tests)

## Worker API server coverage

- `worker_node/internal/workerapiserver/embed_handlers_test.go`: `TestEmbedBearerOK`, `TestBuildMuxesFromEmbedConfig_NodeInfo_StoreEmptyKernelFallback` to hold package coverage at ≥ 90%.

## E2E (deviation from plan wording)

- Ran `just e2e --tags task --exclude-tags sba,inference` — task create/list/get/result/logs/cancel/status filters and prompt task (15 tests, ~87s). This targets task lifecycle and job JSON without pulling every `no_inference`-tagged module (OR semantics would add unrelated suites).

- Did **not** complete `just e2e --tags sba,no_inference` as written: every module tagged `sba` also carries `sba_inference` and/or `ollama` prereqs; `e2e_0720` blocks in `ensure_e2e_sba_task` when SBA task setup is slow. SBA run-arg contract coverage remains in unit/E2E UDS suites (`e2e_0340`) and BDD.

## Deviations

- E2E gate uses the task-focused command above instead of the plan’s comma-separated `task,no_inference` / `sba,no_inference` lines, for the same OR-tag reason documented in Task 5’s report.
