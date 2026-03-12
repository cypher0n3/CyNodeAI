# PMA-First Task Routing and Task Planning State (Draft Spec Proposal)

- [1 Document Overview](#1-document-overview)
- [2 Problem Statement](#2-problem-statement)
- [3 Goals and Non-Goals](#3-goals-and-non-goals)
- [4 Proposed Model](#4-proposed-model)
- [5 API and Contract Changes](#5-api-and-contract-changes)
- [6 Task Planning State Machine](#6-task-planning-state-machine)
- [7 Routing and Algorithms](#7-routing-and-algorithms)
- [8 Data Model Changes](#8-data-model-changes)
- [9 Compatibility and Migration](#9-compatibility-and-migration)
- [10 Implementation Work Items (File-Level)](#10-implementation-work-items-file-level)
- [11 Test Plan](#11-test-plan)
- [12 References](#12-references)

## 1 Document Overview

This draft proposes a change to task routing so newly created tasks are reviewed by PMA before execution.

It introduces a task planning state machine similar to project plan state.

Only tasks in `ready` planning state are eligible for execution.

This is a draft spec in `docs/draft_specs/`.

It is intended to drive follow-on changes in requirements and then tech specs.

## 2 Problem Statement

Today, task creation is treated as "ready to be driven" immediately after create succeeds.

This makes it hard to ensure tasks have correct project context, sufficient acceptance criteria, and correct dependency modeling before execution starts.

It also makes it harder for PMA to apply project-level context and enforce "clarify before execute" expectations.

## 3 Goals and Non-Goals

This section defines intended outcomes and explicit exclusions.

### 3.1 Goals Summary

- Newly created tasks start in a non-executable planning state.

- PMA is always prompted first with task context and project context.

- PMA may enrich tasks by adding description, acceptance criteria, artifacts, and dependency edges before execution.

- Only tasks that are `ready` may be executed.

- The workflow start trigger is aligned so task execution begins only after PMA marks the task `ready`.

### 3.2 Non-Goals Summary

- This draft does not change how project plans are approved and activated.

- This draft does not define the full UI/CLI interaction model for "approve task" and "activate task."

- This draft does not define the full implementation of PMA reasoning or multi-turn clarification.

## 4 Proposed Model

Task processing is split into two phases.

### 4.1 Planning Phase (PMA Review)

The system creates a task in a planning state of `draft`.

The system routes the task to PMA for review with full context.

PMA may:

- Resolve or update `project_id` based on prompt context and user access.

- Normalize or assign task name.

- Add or refine task description and acceptance criteria.

- Attach or interpret attachment references.

- Create or update task dependency edges when the task is part of a plan.

When PMA determines the task is sufficiently specified for execution, PMA transitions the task planning state to `ready`.

### 4.2 Execution Phase (Workflow Runner and Jobs)

Only tasks in planning state `ready` are eligible to start workflow execution.

Workflow execution and job dispatch remain governed by existing gating rules (plan state, dependencies, leases).

## 5 API and Contract Changes

This draft proposes extending the task model and create semantics.

### 5.1 New Task Field: Planning State

Add a new field to the task resource:

- `planning_state`: one of `draft` or `ready`.

This state is distinct from `tasks.status` in `postgres_schema.md`.

`tasks.status` continues to represent execution lifecycle (`pending`, `running`, `completed`, `failed`, `canceled`, `superseded`).

The API must expose `planning_state` in task create, get, and list responses.

### 5.2 Task Create (User API Gateway)

Behavioral change for user task create:

- Task create persists the task and returns `planning_state=draft`.

- Task create does not start execution immediately.

- Task create enqueues a PMA review action for that task and returns immediately.

This matches the intent that PMA should see task and project context first.

### 5.3 Task Ready Transition

Introduce a task state transition operation callable by PMA (and optionally by users with explicit intent).

The concrete surface can be either:

- A dedicated gateway operation (example): `POST /v1/tasks/{task_id}/ready`.

- An MCP tool operation that updates task state and triggers workflow start for `task_id`.

This draft does not mandate which surface is used.

It mandates the semantics and gating described below.

### 5.4 PMA Review Request Contract

PMA must receive a request that includes the task and project identifiers so it can load context in the correct order.

The repository already contains an internal PMA handoff endpoint: `POST /internal/chat/completion`.

The request body type exists in code as `InternalChatCompletionRequest`.

This draft proposes using that endpoint for initial implementation of task review.

The orchestrator must send:

- `project_id` when known, or omit it when unknown and allow PMA to request clarification or resolve to default.

- `task_id` for the newly created task.

- `user_id` for audit attribution and preference resolution.

- `messages` containing the task prompt and a task-review instruction wrapper.

The orchestrator must expect a response containing a single `content` string.

The content must be structured as machine-parseable JSON or a strict Markdown-with-frontmatter block so the orchestrator can extract enrichment outputs deterministically.

## 6 Task Planning State Machine

This section defines the planning state machine and the execution gate.

### 6.1 State Definitions

- `draft`: task exists but is not eligible to start execution.

- `ready`: task is eligible to start execution when other gates allow.

### 6.2 Allowed Transitions

- `draft` => `ready`: performed by PMA after review and enrichment.

### 6.3 Execution Gate

The system MUST deny workflow start for a task unless `planning_state=ready`.

When a task is `draft`, attempts to start workflow must return a defined error (example: 409 with reason "task not ready").

## 7 Routing and Algorithms

This section defines routing rules and required gating order.

### 7.1 Routing Rule: Task Create => PMA Review

On successful task create (User API task create, CLI task create, or chat-derived task create), the orchestrator routes the task to PMA for review.

The orchestrator must supply PMA with:

- Baseline context and role instructions.

- Project-level context when `project_id` is known, or a default project context when omitted.

- Task-level context for the new task.

The ordering of context composition must follow existing PMA context rules.

### 7.2 Algorithm: PMA Review and Enrichment

PMA review is an explicit step before execution.

Minimum PMA actions for a newly created draft task:

- Confirm the effective `project_id` for the task.

- Confirm or normalize task name.

- Decide whether the task requires clarification before execution.

- Populate or refine acceptance criteria when needed.

Optional PMA actions:

- Attach the task to an existing plan or create a plan when the project implies multi-step work.

- Create dependency edges in `task_dependencies` when plan_id is set.

When PMA marks the task `ready`, it must ensure:

- The task is associated with the correct project context.

- The task has enough context for safe execution under policy.

PMA marking a task `ready` is the only supported path that enables execution in this model.

### 7.3 Algorithm: Workflow Start Trigger Update

This draft changes the meaning of "task created via User API" as a workflow start trigger.

Updated trigger behavior:

- Task create produces a draft task and triggers PMA review.

- Task execution starts only after PMA sets `planning_state=ready`.

Implementation detail:

- The workflow start request remains an explicit action initiated by PMA (or by a PMA-mediated system action).

### 7.4 Interaction With Existing Gates

Planning state gate is applied before existing gates.

After `planning_state=ready`, workflow start remains subject to:

- Plan state gating when `plan_id` is set.

- Dependency gating via `task_dependencies`.

- Single-active-workflow-per-task via lease acquisition.

## 8 Data Model Changes

This section defines the proposed schema and response-shape deltas.

### 8.1 Tasks Table

Add to tasks table:

- `planning_state` (text, not null).

Allowed values: `draft`, `ready`.

Initial value on create: `draft`.

### 8.2 API Representations

Task create/get/list responses should include `planning_state`.

Task result responses should include `planning_state` when returning task state.

## 9 Compatibility and Migration

This section describes compatibility risks and a suggested data migration approach.

### 9.1 Backward Compatibility

Existing clients assume a created task may execute immediately.

This change will modify observed behavior.

Compatibility options:

- Provide a compatibility mode that allows "legacy immediate execution" for clients that explicitly opt in.

- Provide a client-visible "ready" operation so interactive users can advance a task when PMA is not enabled.

This draft does not choose one option.

### 9.2 Data Migration

Existing tasks must be assigned a planning state during migration.

Suggested mapping:

- If `tasks.status` is `running`, `completed`, `failed`, `canceled`, or `superseded`, set `planning_state=ready`.

- If `tasks.status` is `pending`, set `planning_state=draft` or `ready` based on deployment choice.

## 10 Implementation Work Items (File-Level)

This section enumerates concrete code and doc updates required to implement this draft.

Each bullet item lists a file to update or a new file to add.

All paths are repository-relative and linked from this draft.

### 10.1 Shared Contracts

- Update [go_shared_libs/contracts/userapi/userapi.go](../../go_shared_libs/contracts/userapi/userapi.go) to add `planning_state` to `TaskResponse` and any other task response shapes.

- Update [go_shared_libs/contracts/userapi/userapi.go](../../go_shared_libs/contracts/userapi/userapi.go) to add request and response types for the task ready transition if it is exposed via the gateway.

- Update [docs/tech_specs/cli_management_app_commands_tasks.md](../tech_specs/cli_management_app_commands_tasks.md) to document `planning_state` in task create, get, list, and result output formats.

### 10.2 Orchestrator Schema and Models

- Update [orchestrator/internal/models/models.go](../../orchestrator/internal/models/models.go) to add `PlanningState string` to the `Task` model with `gorm:"column:planning_state;index"` and a default value.

- Update [orchestrator/internal/database/tasks.go](../../orchestrator/internal/database/tasks.go) so `CreateTask` persists `planning_state=draft` for all newly created tasks.

- Add a store method to update planning state, such as `UpdateTaskPlanningState(ctx, taskID, planningState)` in [orchestrator/internal/database/tasks.go](../../orchestrator/internal/database/tasks.go) and [orchestrator/internal/database/database.go](../../orchestrator/internal/database/database.go).

- Confirm schema application path in [orchestrator/internal/database/migrate.go](../../orchestrator/internal/database/migrate.go) is sufficient for the new column (GORM AutoMigrate).

- If the project continues to maintain SQL schema snapshots, add a new migration under [orchestrator/migrations/](../../orchestrator/migrations/) that adds `tasks.planning_state` and any constraint or default.

- Update [docs/tech_specs/postgres_schema.md](../tech_specs/postgres_schema.md) to include `planning_state` on the tasks table and to define its semantics relative to `status` and `closed`.

### 10.3 User-Gateway Routing and Handlers

- Update [orchestrator/cmd/user-gateway/main.go](../../orchestrator/cmd/user-gateway/main.go) to expose the new task ready transition route if using a gateway endpoint (example: `POST /v1/tasks/{id}/ready`).

- Update [orchestrator/internal/handlers/tasks.go](../../orchestrator/internal/handlers/tasks.go) `CreateTask` to stop creating an execution job immediately.

- Update [orchestrator/internal/handlers/tasks.go](../../orchestrator/internal/handlers/tasks.go) to include `planning_state` in all task responses.

- Add a new handler in [orchestrator/internal/handlers/tasks.go](../../orchestrator/internal/handlers/tasks.go) to perform the ready transition.

- Add gating in [orchestrator/internal/handlers/tasks.go](../../orchestrator/internal/handlers/tasks.go) and in any workflow start surface so workflow start is denied unless `planning_state=ready`.

### 10.4 PMA Review Enqueue and Processing

- Add a new package for task review orchestration, such as [orchestrator/internal/taskreview/](../../orchestrator/internal/) as a new directory.

- Add a new store-backed queue table and store methods if task review is processed asynchronously.

- Add a background worker loop in the control-plane process to drain the queue and call PMA.

- Implement a PMA client that calls `POST /internal/chat/completion` on the PMA service.

- Use the existing PMA request contract in [agents/internal/pma/chat.go](../../agents/internal/pma/chat.go) as the wire format for orchestrator => PMA task review.

- Add parsing logic for PMA response `content` that deterministically extracts task enrichment fields and dependency directives.

- Persist PMA enrichment results into task fields and dependencies, then set `planning_state=ready`.

### 10.5 Execution Trigger After Ready

- Decide the first execution mechanism after `planning_state=ready`.

- If using the existing queued-job mechanism initially, move job creation logic from `CreateTask` into the ready transition handler in [orchestrator/internal/handlers/tasks.go](../../orchestrator/internal/handlers/tasks.go).

- If using the workflow runner, call the workflow start contract only after ready is set and all other gates pass.

- Ensure the dispatcher in [orchestrator/internal/dispatcher/run.go](../../orchestrator/internal/dispatcher/run.go) still only observes queued jobs, which should now only exist after the ready transition.

### 10.6 Cynork CLI Changes

- Update [cynork/cmd/task.go](../../cynork/cmd/task.go) to display `planning_state` in `task create`, `task get`, and `task list` output.

- Add a `cynork task ready <task_id>` command in [cynork/cmd/task.go](../../cynork/cmd/task.go) if the ready transition is exposed through the gateway.

- Update [docs/tech_specs/cynork_cli.md](../tech_specs/cynork_cli.md) and [docs/tech_specs/cli_management_app_commands_tasks.md](../tech_specs/cli_management_app_commands_tasks.md) to document the new CLI command and output fields.

### 10.7 BDD and E2E Test Updates

- Update [features/cynork/cynork_tasks.feature](../../features/cynork/cynork_tasks.feature) to reflect that task create produces a draft task and does not execute until ready.

- Update [features/orchestrator/orchestrator_task_lifecycle.feature](../../features/orchestrator/orchestrator_task_lifecycle.feature) to assert planning state gating before execution.

- Update E2E task-create and task-result flow scripts under [scripts/test_scripts/](../../scripts/test_scripts/) to mark tasks ready or to wait for PMA to do so before asserting execution results.

- Update [scripts/test_scripts/e2e_050_task_create.py](../../scripts/test_scripts/e2e_050_task_create.py) to assert `planning_state=draft` on create.

- Update [scripts/test_scripts/e2e_080_task_result.py](../../scripts/test_scripts/e2e_080_task_result.py) and [scripts/test_scripts/e2e_100_task_prompt.py](../../scripts/test_scripts/e2e_100_task_prompt.py) to transition to ready before polling for completion.

- Update SBA-related scripts under [scripts/test_scripts/](../../scripts/test_scripts/) only after the new planning flow is stable.

## 11 Test Plan

- Add BDD scenarios that assert:

- Task create returns `planning_state=draft`.

- Workflow does not start for `planning_state=draft`.

- After PMA transitions the task to `ready`, workflow start is allowed (subject to other gates).

- Add E2E coverage that asserts PMA review occurs before task execution.

## 12 References

- [langgraph_mvp.md](../tech_specs/langgraph_mvp.md) (workflow start triggers and gates).
- [projects_and_scopes.md](../tech_specs/projects_and_scopes.md) (plan state model to mirror).
- [postgres_schema.md](../tech_specs/postgres_schema.md) (task status vs closed, dependencies).
- [project_manager_agent.md](../tech_specs/project_manager_agent.md) (task and project context composition).
- [user_api_gateway.md](../tech_specs/user_api_gateway.md) (gateway task submission semantics).
