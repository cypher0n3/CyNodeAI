# Task 5 Completion: Constant-Time Bearer / Token Compare

- [Changes](#changes)
- [Tests](#tests)
- [E2E (Deviation From Plan Wording)](#e2e-deviation-from-plan-wording)
- [Deviations](#deviations)

## Changes

- `go_shared_libs/secretutil/token_equal.go`: `TokenEquals` wraps `crypto/subtle.ConstantTimeCompare`.
- `go_shared_libs/secretutil/secretutil_test.go`: `TestTokenEquals`.
- `orchestrator/cmd/api-egress/main.go`: bearer check uses `TokenEquals` on trimmed bearer secret.
- `orchestrator/internal/middleware/auth.go`: workflow runner auth uses `TokenEquals`.
- `orchestrator/internal/middleware/auth_test.go`: `TestWorkflowAuth` (plan name `TestWorkflowAuth`).
- `orchestrator/cmd/api-egress/main_test.go`: `TestTokenAuth`.
- `worker_node/internal/workerapiserver/embed_handlers.go`: `embedBearerOK` and managed-proxy / telemetry auth paths use `secretutil.TokenEquals`.

**Old pattern:** string inequality on bearer material.

**New pattern:** `secretutil.TokenEquals(got, expected)` after normalizing the `Bearer` prefix and trimming spaces where applicable.

**Date:** 2026-03-29.

## Tests

- `just lint-go`
- `just test-go-cover` (full workspace)
- Targeted: `TestTokenAuth`, `TestWorkflowAuth`, `TestBearerAuth`, `TestEmbedBearerOK`

## E2E (Deviation From Plan Wording)

The runner matches **any** listed tag on a test (OR), not AND.
The combination `auth,no_inference` still selected `e2e_0720_sba_task_result_contract` (it carries both `no_inference` and an `ollama` prereq), which blocked for a long time on SBA setup.

### Executed Instead (Same Components, Bounded Runtime)

- `just e2e --tags auth --exclude-tags sba` - cynork auth paths (login, negative whoami, whoami, refresh, logout).
- `just e2e --tags worker --exclude-tags sba` - Worker API health, telemetry, node-manager logs (bearer exercised end-to-end).

## Deviations

- None for product code; E2E commands above substitute for the plan's comma-separated tag lines to avoid OR-tag selection of SBA/Ollama-heavy tests.
