# Plan 004 - Final Report (Medium-Severity Improvements)

- [Overview](#overview)
- [Validation](#validation)
- [Follow-Up: Cynork Task Ready HTTP Client](#follow-up-cynork-task-ready-http-client)
- [Tasks 5-11](#tasks-5-11-concise)
- [Remaining risks](#remaining-risks)

## Overview

**Date:** 2026-03-30.

**Plan:** [`_plan_004_planned.md`](_plan_004_planned.md).

Tasks 1-4 (orchestrator transactions, store sub-interfaces, pagination, batch queries) were completed in earlier work.

This report covers completion of **tasks 5-11**: secure store crypto hardening, telemetry indexes, PMA dependency injection, TUI scrollback test, contract validation, BDD vs unit coverage reporting, and documentation closeout.

## Validation

- `just lint-go` - pass (including line-count gate; large tests split into `store_kem_pq_test.go`, `chat_deps_test.go`).
- `just test-go-cover` - pass (including `worker_node/internal/securestore` at >=90%).
- Task 10: `just test-coverage-bdd-vs-unit` documents that BDD coverage stays separate from unit-test profiles; full BDD still runs via `just test-bdd` / `just bdd-ci`.
- Cynork: `go test ./cynork/...` pass after the task-ready client change; rebuild with `just build-cynork-dev` before E2E so `scripts/test_scripts/run_e2e.py` uses the updated `cynork/bin/cynork-dev` (using `--no-build` without a rebuild can mask fixes).
- E2E spot-check (stack up): `run_e2e.py --single scripts.test_scripts.e2e_0500_workflow_api.TestWorkflowAPI.test_workflow_start_same_holder_returns_200_already_running` pass after rebuild.
- **2026-03-30 (full stack, gateway + Ollama):** after `just build-cynork-dev`, `just e2e --no-build --tags no_inference`: 121 tests, OK (4 skipped; optional MCP/workflow-token cases when env unset).
- **Same session:** `just e2e --no-build --tags pma_inference`: 19 tests, OK (3 skipped; optional stream-shape cases).
- **Same session:** `just e2e --no-build --tags tui_pty`: 52 tests, OK (2 skipped; optional heartbeat/amendment stream cases).
- **Same session:** `just test-bdd 45m`: all modules with `_bdd/` pass (orchestrator, agents, worker_node, etc.).

## Follow-Up: Cynork Task Ready HTTP Client

**Status:** Resolved in tree; validated by cynork tests, workflow E2E spot-check, and the tag runs above (many exercises `cynork task ready`).

**Problem:** `POST /v1/tasks/{id}/ready` can block until the orchestrator finishes work that includes inference; the cynork gateway `http.Client` used a **30s** default for all calls, so `cynork task ready` failed with `Client.Timeout exceeded while awaiting headers` under load or slow inference.

**Change:** `cynork/internal/gateway` uses a dedicated long-timeout client (five minutes) for `PostTaskReady`, via `doPostJSONWithHTTPClient` and `httpClientForTaskReady()`, while other gateway methods keep the default timeout.

**Files:** `cynork/internal/gateway/client.go`, `cynork/internal/gateway/client_http.go`.

## Tasks 5-11 (Concise)

- **Task:** 5
  - outcome: AAD + HKDF + migration-on-read in `worker_node/internal/securestore`; extra tests in `store_kem_pq_test.go` for coverage.
- **Task:** 6
  - outcome: GORM index tags on telemetry models; `gorm_indexes_test.go`.
- **Task:** 7
  - outcome: `ChatDeps` + `NewChatDepsFromEnv()`; `ChatCompletionHandler` takes deps; `TestHandlerDI_UsesInjectedDeps` in `chat_deps_test.go`.
- **Task:** 8
  - outcome: Unified scrollback covered by `cynork/internal/tui/unified_scrollback_test.go`.
- **Task:** 9
  - outcome: `workerapi.ValidateRequest`, `nodepayloads` validation, executor/server wiring.
- **Task:** 10
  - outcome: `just test-coverage-bdd-vs-unit` plus justfile comments.
- **Task:** 11
  - outcome: This report; `_todo.md` section 4 updated; plan document checkboxes refreshed for tasks 5-11.

## Remaining Risks

- **Coverage metrics:** BDD and unit coverage remain separate profiles by design (`just test-coverage-bdd-vs-unit`).
  A single merged percentage still requires ad hoc `-coverprofile` runs if needed.
  This is unchanged but now re-validated via `just test-bdd`.
- **Optional E2E skips:** Some tests skip when optional env is unset (e.g. MCP agent bearer tokens, workflow runner token, amendment/heartbeat-dependent stream assertions).
  That is expected.
  Enable the relevant env on the stack to exercise them.
- **Reproducing this validation:** Tag runs need a healthy user-gateway (`readyz`), auth, and for `pma_inference` a reachable Ollama/PMA path as in `scripts/test_scripts/run_e2e.py` prereqs.
