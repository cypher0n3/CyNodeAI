# Example PMA Plan: Go CLI Utility (Test Draft)

- [Purpose](#purpose)
- [Scope and Goal](#scope-and-goal)
- [Constraints and Assumptions](#constraints-and-assumptions)
- [Tasks (Execution Order)](#tasks-execution-order)
- [Alignment Notes](#alignment-notes)

## Purpose

This document is a **test draft** example of a project plan that the Project Manager Agent (PMA) might produce for a small development task.
It illustrates the structure and content expected of PMA-generated plans per [REQ-PMAGNT-0111](../requirements/pmagnt.md#req-pmagnt-0111), [CYNAI.AGENTS.ProjectPlanBuilding](../tech_specs/project_manager_agent.md#spec-cynai-agents-projectplanbuilding), and [REQ-PROJCT-0114](../requirements/projct.md#req-projct-0114).

The scenario is a minimal Linux command-line utility written in Go.

## Scope and Goal

Goal: deliver a single-binary Go CLI that reads a config file path from a flag, validates the file exists and is readable, and prints the first line of the file to stdout.
No subcommands; flags only.
Target platform: Linux (amd64).

## Constraints and Assumptions

- Go 1.21+; standard library only for file and flag handling.
- Config file format: plain text; only "first line" semantics are required.
- No authentication or network access.
- Plan is in **draft** until the user explicitly approves it; PMA must not mark the plan approved without explicit user approval ([REQ-AGENTS-0136](../requirements/agents.md#req-agents-0136)).

## Tasks (Execution Order)

Tasks are ordered so that dependencies are satisfied before dependents run.
Task names follow the required format: lowercase, words separated by single dashes ([project_manager_agent.md - Task Naming](../tech_specs/project_manager_agent.md#task-naming)).

### 1 Task Add-Project-And-Specs

Create a minimal Go module and document the intended behavior and contract (flags, exit codes, output).

#### 1.1 Acceptance Criteria Add-Project-And-Specs

- A Go module exists at the agreed path with `go.mod` and a single `main` package.
- A short spec or README describes: the `-config` flag (required), exit code 0 on success, non-zero on missing/unreadable file, and stdout containing exactly the first line of the file (no extra newline unless the file's first line contains one).
- No implementation of flag parsing or file reading yet.

Dependencies: None.

---

### 2 Task Add-Behavior-And-Red-Tests

Add behavior specs and failing tests for flag parsing, file validation, and first-line output.

#### 2.1 Acceptance Criteria Add-Behavior-And-Red-Tests

- Tests exist for: missing `-config` (non-zero exit), non-existent path (non-zero exit), unreadable file (non-zero exit), valid file (exit 0, stdout equals first line).
- Tests are runnable via `go test` and fail (Red phase) because the implementation does not exist yet.
- Test data (e.g. a small fixture file) is present and used by tests.

Dependencies: add-project-and-specs.

---

### 3 Task Implement-Flag-And-File-Read

Implement the main binary: parse `-config`, validate file, read and print first line.

#### 3.1 Acceptance Criteria Implement-Flag-And-File-Read

- The binary builds with `go build`.
- All tests added in add-behavior-and-red-tests pass (Green phase).
- Manual run with a valid path prints the first line only; with missing or invalid path, exits non-zero and does not print the first line to stdout.
- Exit codes match the documented contract.

Dependencies: add-behavior-and-red-tests.

---

### 4 Task Refactor-And-Lint

Refactor for clarity and run project linters; fix any issues.

#### 4.1 Acceptance Criteria Refactor-And-Lint

- Code passes `go vet` and any project-specific `just lint` or `just ci` targets that apply to this repo.
- No new linter suppressions or ignored rules introduced.
- Behavior unchanged; existing tests still pass.

Dependencies: implement-flag-and-file-read.

---

### 5 Task Document-Usage-And-Close

Finalize README with usage example and any runbook notes; mark task and plan deliverables complete.

#### 5.1 Acceptance Criteria Document-Usage-And-Close

- README includes at least one usage example (invocation and expected output).
- All prior tasks are closed and any plan-level completion notes are recorded.
- Plan may be set to completed only when all tasks are closed ([REQ-PROJCT-0121](../requirements/projct.md#req-projct-0121)).

Dependencies: refactor-and-lint.

## Alignment Notes

This example aligns with the following:

- Plan content: plan document (name, body, scope) and task list with order and acceptance criteria stored as Markdown ([REQ-PROJCT-0114](../requirements/projct.md#req-projct-0114), [CYNAI.ACCESS.ProjectPlanMarkdown](../tech_specs/projects_and_scopes.md#spec-cynai-access-projectplanmarkdown)).
- Execution order: tasks have explicit dependencies; runnability is determined by task dependencies ([REQ-PROJCT-0111](../requirements/projct.md#req-projct-0111), [REQ-PROJCT-0123](../requirements/projct.md#req-projct-0123)).
- Task naming: all task names are lowercase with single-dash separation and unique within the plan ([project_manager_agent.md - Task Naming](../tech_specs/project_manager_agent.md#task-naming)).
- PMA behavior: plan is built before creating orchestrator tasks; refinement and clarification are possible before approval ([REQ-PMAGNT-0111](../requirements/pmagnt.md#req-pmagnt-0111), [REQ-PMAGNT-0112](../requirements/pmagnt.md#req-pmagnt-0112), [REQ-PMAGNT-0113](../requirements/pmagnt.md#req-pmagnt-0113)).
- Approval: plan remains draft until the user explicitly approves; no agent-approved state without user confirmation ([REQ-AGENTS-0136](../requirements/agents.md#req-agents-0136), [CYNAI.AGENTS.PlanApprovalSeekExplicitApproval](../tech_specs/project_manager_agent.md#spec-cynai-agents-planapprovalseekexplicitapproval)).

This file is temporary and lives under `docs/dev_docs/`; it is not linked from stable docs and may be removed or moved once the example has been used for validation.
