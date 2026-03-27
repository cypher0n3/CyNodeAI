# Task 2 Completion Report: TUI Bug 3/4 and Spec Alignment (2026-03-27)

## Summary

- **Bug 3:** `applyEnsureThreadResult` uses `ensureThreadScrollbackLine` so a stable thread id emits `[CYNRK_THREAD_READY]` instead of a misleading `[CYNRK_THREAD_SWITCHED]` when the id did not change.
- **Bug 4:** `handleEnterKey` blocks only **plain** chat while `Loading`; lines starting with `/` or `!` still dispatch.
- **Tests:** Unit coverage in `cynork/internal/tui` (`TestEnsureThreadScrollbackLine`, slash/shell-while-loading tests); BDD `features/cynork/cynork_tui_bugfixes.feature` with steps calling `EnsureThreadScrollbackSystemLine` and `EnterBlockedWhileLoading`; PTY E2E in `e2e_0750_tui_pty.py` (`LANDMARK_THREAD_READY` in harness; slash `/version` during in-flight; conditional check when `THREAD_READY` appears after startup).
- **Specs:** TUI delta items were already largely merged in `cynork_tui.md` / `cynork_tui_slash_commands.md` / `client.md` (REQ-CLIENT-0206).
  Added an optional **Auth Recovery** layout bullet in `cynork_tui.md` (centering, label column, true-black panel, reverse-video carets, password field Ctrl+word disabled).

## What Passed

- `go test ./cynork/internal/tui/...`
- `go test ./cynork/_bdd` (includes new bugfix scenarios)
- `just test-bdd` (all modules with `_bdd`)
- `just -f scripts/justfile e2e --single e2e_0750_tui_pty --no-build --timeout 0` (PTY module, including Bug 3/4 tests)
- `just test-go-cover` (cynork packages meet thresholds)
- `just lint-go` / `just lint-go-ci` (repo-wide, this session)
- `just lint-md` on edited `.md` files; `just docs-check`

## Deviations / Notes

- BDD scenarios for Bug 3/4 use **exported helpers** (`EnsureThreadScrollbackSystemLine`, `EnterBlockedWhileLoading`) so Godog does not duplicate TUI internals.
- **PTY Bug 3 E2E** checks that when `[CYNRK_THREAD_READY]` is present after startup, `[CYNRK_THREAD_SWITCHED]` is absent (stable ensure).
  Full `/auth login` overlay typing is not automated in PTY.
- **PTY Bug 4 E2E** uses `/version` during an in-flight turn; `/help`, `/copy`, and `!ls` are covered by Go/BDD rather than additional PTY cases in this pass.
