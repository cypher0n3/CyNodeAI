# Plan `_plan_002_bugs.md` -- Final Completion

- [Outcome](#outcome)
- [Validation](#validation)
- [Remaining risks](#remaining-risks)

## Outcome

- **Task 1 (Bug 3):** Logout clears `CurrentThreadID`; ensure-thread landmarks use `createdNew` / `resumedFromCache`; tests and Task 1 report in `docs/dev_docs/_plan_002_bugs_task1_report.md`.
- **Task 2 (Bug 4):** Streaming-aware queue, Ctrl+Q explicit queue, send-now (Ctrl+S + `ctrl+enter` string); tests, E2E `e2e_0650_tui_queue_model.py`, Task 2 report in `docs/dev_docs/_plan_002_bugs_task2_report.md`.
- **Task 3:** `_todo.md` updated; this file; plan checklists marked complete.

## Validation

- `just ci` (lint, coverage including `cynork/internal/tui` >=90%, race tests), `just docs-check`, and `just e2e --tags tui_pty,no_inference` passed at closeout (see task reports for detail).

## Remaining Risks

- Send-now chord relies on **Ctrl+S** where **Ctrl+Enter** cannot be distinguished from **Enter** in bubbletea; terminals vary.
- Queue UX is minimal (no visible draft list pane per full spec).
- Product decisions called out in Task 1 report (in-TUI login transcript scope) remain open.
