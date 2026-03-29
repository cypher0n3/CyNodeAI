# Task 10 Completion Report

<!-- Plan: docs/dev_docs/2026-03-27_consolidated_refactor_and_outstanding_work_plan_remaining_tasks.md -->

## Summary

**Task:** Task 10 - Remaining MVP Phase 2 and Worker Deployment Docs.

**Date:** 2026-03-28.

Task 10 closed **documentation alignment**, **MVP plan accuracy**, and **worker deployment topology clarity** against the current repository.

Validation runs targeted `just docs-check`, `just ci`, `just e2e --tags chat`, `just e2e --tags pma`, and full `just e2e` where noted in the plan checkboxes.

**Deviation (explicit):** Full **Python LangGraph** graph-node execution (P2-06) and **end-to-end verification-loop** automation (P2-08: PMA -> Project Analyst -> workflow state) are **not** delivered in this task.

They remain listed under [mvp_plan.md](../mvp_plan.md) (sections "Implementation Order (Done vs Remaining)" and "Phase 2 LangGraph Integration Checklist").

The **orchestrator** workflow HTTP API (start, resume, checkpoint, lease) and BDD coverage for that contract are already in the tree (`orchestrator/internal/handlers/workflow.go`, `features/orchestrator/workflow_start_resume_lease.feature`).

## Discovery

Discovery summarizes MVP Phase 2 inventory and worker deployment documentation gaps addressed in this task.

### MVP Phase 2 - Remaining vs. Landed

- **Area:** **MCP tool slices**
  - finding: Gateway implements a broad catalog in `mcpgateway/handlers.go` (preference, task, project, job, artifact, skills, help, node, system_setting).
  - finding: PM bearer calls use the routed catalog; **sandbox** agents remain restricted by `sandboxAllowedTools` in `allowlist.go`.
- **Area:** **LangGraph**
  - finding: **Go:** workflow lease, checkpoint persistence, start/resume API, and tests/BDD are present.
  - finding: **Python LangGraph process** implementing graph nodes per [langgraph_mvp.md](../tech_specs/langgraph_mvp.md) is **not** present as a first-class component in this repository.
- **Area:** **Verification loop**
  - finding: PMA may run with `project_analyst` role (`agents/cmd/cynode-pma`).
  - finding: Automated PMA->PAA->workflow verification loop per P2-08 is **not** end-to-end wired.
- **Area:** **Chat / runtime**
  - finding: Bounded wait and transient retry for chat completions are implemented in `openai_chat.go` (REQ-ORCHES-0131/0132).
  - finding: E2E smoke lives in `scripts/test_scripts/e2e_0540_chat_reliability.py`; BDD in `features/orchestrator/openai_compat_chat.feature`.

### Worker Deployment Docs

- [worker_node.md](../tech_specs/worker_node.md) **Deployment Topologies** described the single-process requirement but did not explicitly separate **normative REQ-WORKER-0272** from **deferred multi-process** alternatives.
- A subsection **Normative topology vs. deferred alternatives** was added.

### Prerequisites (Tasks 1-9)

Assumed satisfied per prior plan execution and stable TUI path (see earlier task closeout reports in `docs/dev_docs/`).

## Red Phase (Test Inventory)

For slices **already implemented** before this task, the plan's strict "failing test before green" step is satisfied **retroactively** by existing Python E2E, BDD, and Go coverage (MCP routes, chat reliability, orchestrator workflow start/resume/lease API).

**Still open vs. the plan text:** Python E2E and BDD for **PMA-to-PAA verification-loop** end-to-end (LangGraph P2-08 slice) are **not** present; those Red lines remain unchecked in the plan until that slice is implemented with tests-first.

## Green

- **Item:** MCP slices beyond minimum
  - outcome: **Documented** current catalog and sandbox vs PM allowlist in [mvp_plan.md](../mvp_plan.md); no code change required for this task.
- **Item:** LangGraph graph nodes (Python runner)
  - outcome: **Not completed** - see deviation above.
- **Item:** Verification loop (PMA -> PA -> review)
  - outcome: **Not completed** - see deviation above.
- **Item:** Chat/runtime drifts
  - outcome: **Closed in docs:** Known Drift for 0131/0132 updated in mvp_plan to reflect implementation.
- **Item:** Worker deployment docs
  - outcome: **Done:** normative vs deferred subsection under Deployment Topologies.
- **Item:** `just docs-check`
  - outcome: Run after doc edits (see Validation).

## Refactor

No production code refactor was required for this task; tests remained green.

## Testing and Validation

Executed 2026-03-28:

- `just docs-check` on `docs/tech_specs/worker_node.md`, `docs/mvp_plan.md`, and this report: PASS
- `just ci`: PASS
- `just setup-dev restart --force` then `just e2e --tags chat` and `just e2e --tags pma`: PASS
- Full `just e2e` after stack restart: PASS (150 tests, 8 skipped)

**E2E hardening:** `e2e_0765_tui_composer_editor.test_tui_composer_ctrl_down_navigates_forward_in_history` failed once when an auth-recovery overlay appeared before composer history navigation; the test now dismisses that overlay and waits for prompt-ready after `/clear`.
One full-suite run had `e2e_0510_task_inference` fail with task left `queued` (transient); re-run after `just setup-dev restart --force` passed.

**Markdown CI:** `docs/dev_docs/2026-03-28_task8_task9_red_and_testing_closure.md` was adjusted for first-heading / empty-heading rules so `just ci` lint-md passes.

## Closeout

This report satisfies Task 10 **Closeout** for the completed scope.

Remaining Phase 2 implementation work (Python LangGraph nodes, verification loop) stays tracked in [mvp_plan.md](../mvp_plan.md).
