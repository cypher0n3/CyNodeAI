# 2026-03-02 Feature Files, BDD, and E2E Additions

- [Summary](#summary)
- [Feature files](#feature-files)
- [BDD steps and scenarios](#bdd-steps-and-scenarios)
- [Python E2E](#python-e2e)
- [Validation](#validation)

## Summary

Added feature scenarios, BDD step definitions, and Python E2E tests for recent code changes (PMA gating, chat reliability, task create with optional task name).
All changes follow copilot-instructions, meta.md, and justfile; no Makefile/Justfile or linter rule changes.
`just ci` is green.

## Feature Files

- **features/orchestrator/orchestrator_startup.feature**
  - New scenario: "Orchestrator becomes ready when inference path is available" (REQ-ORCHES-0150).
  - One When step: node active with worker_api config and request readyz; Then orchestrator enters ready state.
- **features/orchestrator/chat_completion_reliability.feature** (new)
  - Single scenario: "Chat completion returns 200 or acceptable error status" (REQ-ORCHES-0131, REQ-ORCHES-0132).
  - Asserts response status is one of 200, 502, 504.
- **features/orchestrator/orchestrator_task_lifecycle.feature**
  - New scenario: "Create task with optional task name" (POST with task_name, GET task, assert task name).
- **features/cynork/cynork_tasks.feature**
  - New scenario: "Create task with optional task name" (cynork task create with --task-name, task get shows task name).

## BDD Steps and Scenarios

- **orchestrator/_bdd/steps.go**
  - "I request the readyz endpoint" (store status); "the orchestrator enters ready state" (GET readyz, expect 200).
  - Compound step: "a registered node X is active with worker_api config and I request the readyz endpoint" (only-one-when).
  - "I create a task with prompt X and task name Y"; "the task name is Y" (GET task, assert task_name).
  - "the response status is one of 200, 502, 504" (chat reliability).
  - Chat mock enabled for scenario "Chat completion returns 200 or acceptable error status".
- **cynork/_bdd/steps.go**
  - Mock POST /v1/tasks accepts `task_name`; GET /v1/tasks and GET /v1/tasks/{id} return task_name.
  - "I run cynork task create with prompt X and task name Y"; "cynork task get shows task name Y".

## Python E2E

- **scripts/test_scripts/e2e_050_task_create.py**
  - New test: `test_task_create_with_task_name`: create with `--task-name e2e-task-name`, get task, assert `task_name` in JSON response (normalized).

## Validation

- `just validate-feature-files`: OK.
- `just lint-gherkin`: OK (no Background with single scenario; only-one-when satisfied).
- `just test-bdd`: all orchestrator and cynork scenarios pass.
- `just ci`: green.
