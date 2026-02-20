# Post-Phase 1: Use_inference_inference API and CLI Wiring

- [Summary](#summary)
- [Changes](#changes)
- [Verification](#verification)
- [E2E Feature File](#e2e-feature-file)

## Summary

User-facing support for `use_inference` was added per [post_phase1_mvp_plan.md](post_phase1_mvp_plan.md) Section 2.1 and Implementation Order item 4.
Report date: 2026-02-20.

## Changes

1. **Orchestrator User API** (`orchestrator/internal/handlers/tasks.go`)
   - `CreateTaskRequest` now has `UseInference bool` with JSON tag `use_inference,omitempty`.
   - `marshalJobPayload(prompt, useInference)` includes `"use_inference": true` in the job payload when set.
   - `CreateTask` passes `req.UseInference` into `marshalJobPayload`.

2. **Cynork CLI** (`cynork/internal/gateway/client.go`, `cynork/cmd/task.go`)
   - Gateway `CreateTaskRequest` has `UseInference bool` with `use_inference,omitempty`.
   - `cynork task create` has `--use-inference` flag; when set, the request sends `use_inference: true`.

3. **Orchestrator BDD** (`orchestrator/_bdd/steps.go`)
   - New step: `I create a task with use_inference and command "..."` which POSTs to `/v1/tasks` with `{"prompt": "<cmd>", "use_inference": true}`.

### Unit Test Updates

- `handlers/tasks_test.go`: `TestCreateTaskRequestJSON` extended for `UseInference` round-trip.
- `handlers/handlers_mockdb_test.go`: `TestTaskHandler_CreateTaskWithUseInference_StoresUseInferenceInJobPayload` ensures the stored job payload contains `use_inference: true` when the request has `UseInference: true`.

## Verification

- `just ci` passes (lint-go, lint-go-ci, vulncheck-go, lint-python, lint-md, validate-doc-links, validate-feature-files, test-go-cover, test-bdd).

## E2E Feature File

`features/e2e/single_node_happy_path.feature` scenario "Single-node task execution with inference in sandbox" (`@inference_in_sandbox`) uses the step "I create a task with use_inference and command \"sh -c 'echo $OLLAMA_BASE_URL'\"".
That step is now implemented; the scenario is run by script-driven E2E (`just e2e`), not by `just test-bdd` (no e2e Godog runner).
