# Task 10 Completion Report

<!-- Plan: docs/dev_docs/2026-03-27_consolidated_refactor_and_outstanding_work_plan_remaining_tasks.md -->

## Summary

**Task:** Task 10 - Remaining MVP Phase 2 and Worker Deployment Docs.

**Date:** 2026-03-28 (report); verification-contract tests and CI re-verified **2026-03-29**.

Task 10 closed **documentation alignment**, **MVP plan accuracy**, and **worker deployment topology clarity** against the current repository.

Validation runs targeted `just docs-check`, `just ci`, `just e2e --tags chat`, `just e2e --tags pma`, and full `just e2e` where noted in the plan checkboxes.

**Deviation (explicit):** A first-class **Python LangGraph** library process implementing all graph nodes per [langgraph_mvp.md](../tech_specs/langgraph_mvp.md) (P2-06) is **not** in this repository.

**Delivered for the verification slice (P2-08 contract):** The orchestrator workflow API persists review-shaped state at checkpoint node `verify_step_result`; this is covered by BDD (`features/orchestrator/workflow_start_resume_lease.feature`), Python E2E (`scripts/test_scripts/e2e_0500_workflow_api.py`), and a stdlib-only reference runner (`scripts/workflow_runner_stub/minimal_runner.py`, linked from the LangGraph MVP spec).
That is **API + persistence + runner contract**, not a full multi-agent PMA/PAA production graph in Python.

Remaining high-level Phase 2 items stay tracked in [mvp_plan.md](../mvp_plan.md) where applicable.

The **orchestrator** workflow HTTP API (start, resume, checkpoint, lease) lives in `orchestrator/internal/handlers/workflow.go` with tests and BDD as above.

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
  - finding: **Workflow contract:** checkpoint at `verify_step_result` with PMA->PAA review JSON and resume round-trip is wired and tested (BDD + E2E + stub runner).
    Full LangGraph Python graph orchestration remains out of scope for this repo slice.
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

**Verification slice:** Python E2E, BDD, and the reference runner now cover the **workflow verification checkpoint** contract (review JSON in durable state).
Strict tests-first for that slice was applied in the 2026-03-29 closeout pass.

## Green

- **Item:** MCP slices beyond minimum
  - outcome: **Documented** current catalog and sandbox vs PM allowlist in [mvp_plan.md](../mvp_plan.md); no code change required for this task.
- **Item:** LangGraph graph nodes (Python LangGraph library)
  - outcome: **Not completed** as an in-tree LangGraph process; see deviation above.
- **Item:** Verification loop (PMA -> PA -> review), workflow persistence contract
  - outcome: **Completed** for orchestrator checkpoint/resume with review-shaped state (BDD + E2E + `minimal_runner.py` reference).
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

Re-verified **2026-03-29** after Gherkin fix (`workflow_start_resume_lease.feature` uses `Background:` not a Markdown heading) and doc lint fixes:

- `just docs-check` (full repo): PASS
- `just ci`: PASS (includes `lint-gherkin`, BDD, Go)
- `just setup-dev restart --force` then full `just e2e`: PASS (151 tests, 8 skipped)

**E2E hardening:** `e2e_0765_tui_composer_editor.test_tui_composer_ctrl_down_navigates_forward_in_history` failed once when an auth-recovery overlay appeared before composer history navigation; the test now dismisses that overlay and waits for prompt-ready after `/clear`.
One full-suite run had `e2e_0510_task_inference` fail with task left `queued` (transient); re-run after `just setup-dev restart --force` passed.

**Markdown CI:** `docs/dev_docs/2026-03-28_task8_task9_red_and_testing_closure.md` was adjusted for first-heading / empty-heading rules so `just ci` lint-md passes.

## Closeout

This report satisfies Task 10 **Closeout** for the completed scope.

Remaining Phase 2 implementation work (full Python LangGraph graph process, if desired) stays tracked in [mvp_plan.md](../mvp_plan.md).
The **verification checkpoint** contract is implemented and tested in-tree as described above.
