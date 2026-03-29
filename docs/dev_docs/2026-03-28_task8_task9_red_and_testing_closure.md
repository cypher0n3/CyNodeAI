# Task 8 and Task 9: Red-Phase Audit and Testing Closure

<!-- dev_docs closure for Task 8 and Task 9 -->

## Summary

**Date:** 2026-03-28.

This document closes the **Red** checklist items and **Testing** gates for Task 8 and Task 9 in
`2026-03-27_consolidated_refactor_and_outstanding_work_plan_remaining_tasks.md` without reordering
work that already shipped.

**Green** implementation and **Testing** validation are the source of truth.

Red items are satisfied by **retroactive audit** and **current test evidence** (the plan's
"fail before fix" Red runs were superseded once Green landed).

## Validation Runs (2026-03-28)

- **Command:** `just bdd-ci`
  - result: PASS
- **Command:** `just test-bdd` (via `just ci`)
  - result: PASS
- **Command:** `just e2e --tags auth`
  - result: PASS (5 tests)
- **Command:** `just e2e --tags tui_pty`
  - result: PASS (51 tests, 2 skipped)
- **Command:** `just e2e` (full suite)
  - result: PASS (150 tests, 8 skipped)
- **Command:** `just ci`
  - result: PASS

## Task 8 - Red Phase (Audit)

Audit notes for Task 8 Red checklist items (retroactive evidence).

### Python E2E Matrix (`e2e_0610`, `e2e_0620`, `e2e_0630`, `e2e_0640`, `e2e_0650`, `e2e_0750`, `e2e_0760`)

- **Skips** are documented in-module: `E2E_SKIP_INFERENCE_SMOKE`, gateway non-200, or optional
  stream shapes (heartbeat/amendment) when upstream does not emit them.
- **No unimplemented gaps** remain for Phase 6 streaming scope beyond those skips and the two
  MCP allowlist tests that require optional env tokens.

### BDD `godog.ErrPending`

- Streaming and TUI-deferred steps are implemented in `cynork/_bdd/steps_cynork_streaming_bdd.go`
  and related registrars; remaining `ErrPending` entries are **intentional**: PTY-only observation,
  queue-draft not implemented, web login not implemented, or second mock gateway (see file
  comments).
- **`just bdd-ci`** (strict) passes.

### Go Unit Tests for BDD Helpers

- Shared simulation lives in `cynork/internal/tui` (`stream_bdd_sim.go`, `bdd_auth.go`) with unit
  tests; `_bdd` is covered by integration tests in `cynork/_bdd/suite_test.go` and `just bdd-ci`.

## Task 9 - Red Phase (Coverage Mapping)

- **Plan line:** PTY startup auth recovery
  - evidence: `e2e_0765_tui_composer_editor.test_tui_empty_env_tokens_shows_login_overlay`
- **Plan line:** In-session auth recovery (401 path)
  - evidence: `cynork/internal/tui/model_unauthorized_recovery_test.go`, BDD steps in `steps_cynork_extra_tui_deferred.go`, `features/cynork/cynork_tui_auth.feature`
- **Plan line:** Project / model in-session
  - evidence: `e2e_0760_tui_slash_commands` (`/project`, `/model`)
- **Plan line:** Thread create/switch/rename, thinking
  - evidence: `e2e_0750_tui_pty`, `e2e_0760` slash commands
- **Plan line:** BDD scenarios
  - evidence: `features/cynork/cynork_tui_auth.feature` and related cynork features
- **Plan line:** Go unit tests
  - evidence: `model_unauthorized_recovery_test.go`, `model_credential_redaction_test.go`, `bdd_auth_test.go`

## Task 9 - Testing Gate

- **Python E2E:** `just e2e --tags auth` and `just e2e --tags tui_pty` green; full `just e2e` green.
- **`just ci`:** green.

## Follow-Up (Optional, Not Blocking)

- Dedicated **live PTY** test that forces HTTP 401 mid-stream against the real gateway would
  duplicate BDD in-memory and unit coverage unless the stack adds a fault-injection hook; not
  required for the Task 9 bar once BDD + unit + startup PTY E2E pass.
