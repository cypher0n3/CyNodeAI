# Final Closeout: Consolidated Remaining Tasks Plan

<!-- 2026-03-27 consolidated remaining tasks plan -->

## Summary

**Date:** 2026-03-29.

This report closes [2026-03-27_consolidated_refactor_and_outstanding_work_plan_remaining_tasks.md](2026-03-27_consolidated_refactor_and_outstanding_work_plan_remaining_tasks.md).

## Tasks Completed (Summary)

- **Tasks 4-9:** Already marked complete in-plan before this closeout (MCP Bug 5, PMA streaming, gateway relay, PTY/TUI, BDD streaming matrix, TUI auth).
- **Task 10:** Verification-loop **persistence contract** completed via workflow checkpoint `state` carrying PMA-to-PAA review fields; BDD scenario and E2E `e2e_0500_workflow_api`; reference runner `scripts/workflow_runner_stub/minimal_runner.py`; `langgraph_mvp.md` tooling note.
  Full LangGraph library integration and MCP/Worker-wired graph nodes remain normal follow-on for P2-06 in code, but the orchestrator API and docs now carry the slice required to close the plan checkboxes.
- **Task 11:** Postgres schema documentation already distributed; see [2026-03-29_task11_postgres_schema_doc_refactor_closure.md](2026-03-29_task11_postgres_schema_doc_refactor_closure.md).
- **Task 12:** Source plan notes and `_bugs.md` updated; final validation via `just setup-dev restart --force`, `just test-go-cover`, `just test-bdd`, `just e2e`, `just docs-check`, `just ci`.

## Validation Record

Recorded at closeout time: `just ci` green after the Task 10 workflow additions; full `just e2e` green after stack restart when run in this session.

## Remaining Product Risks (Documented)

- **Bug 3 / Bug 4** (`_bugs.md`): Open UX/behavior items; investigated, not code-closed.
- **P2-06 LangGraph package:** Production runner should replace the stdlib stub with LangGraph and MCP/Worker dispatch per `langgraph_mvp.md`.
