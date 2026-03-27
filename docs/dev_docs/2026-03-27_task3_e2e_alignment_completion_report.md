# Task 3 Completion Report: E2E Test Alignment Follow-Ups

## Summary

**Date:** 2026-03-27

Closed the gaps from `docs/dev_docs/2026-03-23_e2e_tech_spec_alignment_review.md` (Gaps / Follow-Ups): PTY E2E for `/copy` and `/copy all`, symmetry test for `Ctrl+Down` with sent-message history, BDD coverage for transcript extraction and history navigation, and composer footnote alignment with Alt+Enter / Ctrl+J.

## What Changed

Work spans Python E2E, Go/TUI, and BDD layers.

### Python E2E (`scripts/test_scripts/`)

- `e2e_0760_tui_slash_commands.py`: tests for `/copy last` with no assistant, `/copy last` after a sent line (scrollback feedback), and `/copy all` feedback.
- `e2e_0765_tui_composer_editor.py`: `test_tui_composer_ctrl_down_navigates_forward_in_history` (two sends, Ctrl+Up twice to older line, Ctrl+Down back to newer); footnote assertion now requires `Ctrl+J` alongside `Alt+Enter`.

### Go / TUI

- Exported `PlainTranscript`, `LastAssistantPlain`, `ScrollbackSystemLinePrefix`, `CopySelectFootnote`; renamed `NavigateInputHistory` (was private).
- Composer footnote string now includes `Ctrl+J` next to `Alt+Enter` (`view_render.go`).
- Added `TestPlainTranscript_OnlySystemLines`.
- Thread/history helpers moved to `model_thread_commands.go` so `model.go` stays under the 1000-line `lint-go` cap.

### BDD (`features/cynork/` + `cynork/_bdd/`)

- New `cynork_tui_composer_copy.feature` with four scenarios (plain transcript, last assistant, Ctrl+Up/Ctrl+Down history, footnote newline keys).
- New `steps_cynork_tui_composer_copy.go`; registered from `steps2.go`.

## Validation Run in This Session

- `go test ./...` from `cynork/` (all packages including `_bdd`).
- BDD suite completed successfully (includes new `cynork_tui_composer_copy.feature` scenarios).
- `pylint` C0301 clean on the changed E2E modules (`e2e_0760`, `e2e_0765`).

Not run to completion in this session: full `just test-go-cover`, `just e2e --tags tui_pty` (needs dev stack; a run was started but not awaited), `just setup-dev restart --force`.
`just lint-python` on the whole tree may still report other files; the modified scripts were brought under 100 columns where touched.

## Notes

- `/copy` E2E assertions allow `Copy failed` when no clipboard helper is present; they still require an explicit copy-feedback path.
- No separate PTY helper extraction: copy/history waits mirror existing patterns in the same modules (Refactor task marked done with no shared helper).
