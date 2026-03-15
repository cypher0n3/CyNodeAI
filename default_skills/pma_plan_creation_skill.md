---
name: pma-plan-creation
description: Guides the Project Manager Agent (PMA) to create and update execution plans (plan document, task list, prerequisites) via MCP. Use when the user asks to create or update a plan, or when multi-step work should be planned before execution.
ingest_skills:
  - pma-task-creation
  - pma-requirement-object
---

# PMA Plan Creation Skill

## Purpose

Guide the **Project Manager Agent (PMA)** to **build and update execution plans**: a plan document (Markdown), a list of tasks (by name), and prerequisites (which tasks must complete before a task is runnable).
Plans and tasks are persisted via MCP only; the agent does not write plan files to the filesystem.
When creating or updating **task payloads** (fields, steps, requirements array), ingest and follow skill `pma-task-creation`; when populating **requirement objects** on tasks, ingest and follow skill `pma-requirement-object`.

## Use This Skill When

- The user asks to create a plan (project plan with tasks and prerequisites).
- The user asks to update or refine an existing plan.
- A user goal implies multi-step or project-scoped work and the agent should plan before creating orchestrator tasks.
- The agent is tasked to add or update tasks on an approved plan (e.g. after plan approval).

## Core Rules

1. **Source of truth:** Requirements first, then technical specifications, then current implementation.
   Use the project's designated locations for requirements and specs (from instructions or repo).
2. **Gather before planning:** Read relevant requirements, specs, and (via MCP) current project, plan, and task state before creating or updating a plan.
3. **Gaps:** If requirements or specs are missing, ambiguous, or contradictory, do not guess; call out the gap to the user or caller.
4. **Executable plan:** Each task in the plan must be a concrete, actionable unit with clear acceptance criteria.
   The plan body MUST express only **task prerequisites** (dependencies); runnability is determined by the dependency graph, not a fixed sequence.
5. **BDD/TDD in plan content:** Plan body and task acceptance criteria MUST call for: behavior specs first, failing tests before implementation, smallest change to pass, refactor after green, regression tests for bug-fix tasks.
6. **Task independence:** Each task in the plan MUST have its own Requirements and Specifications, Discovery, Red, Green, Refactor, Testing, and Closeout so it can be executed and validated independently.
7. **Validation gates:** The plan MUST require that a task's validation passes before any task that lists it as a prerequisite is runnable; state this explicitly.
8. **No simulated output:** Use only real MCP tool results; report errors or missing data; do not invent or assume values.
9. **Lock and approval:** When the plan is locked, the agent MUST NOT change the plan document or tasks except completion status and comments.
   The agent MUST NOT mark a plan approved until explicit user approval has been obtained.
10. **Persistence only via MCP:** All reads and writes for plans and tasks MUST go through the orchestrator MCP gateway.
    The agent MUST NOT set entity ids (plan id, task id); the orchestrator assigns them and returns them in create responses.

## Planning Workflow

PMA MUST gather context, define scope, build plan content and task list, then persist via MCP only.

### 1. Gather Inputs

- User goal (from chat or handoff).
- Relevant requirement documents and technical specifications (from repo or instructions).
- Current project, plan, and task state via MCP (project get/list, plan get/list, task list by plan).
- Call `plan.help`, `task.help`, and `requirement.help` (or host-equivalent) to get current schema before building payloads.
- Missing information, assumptions, dependencies, and risks; call out gaps.

### 2. Define Scope and Constraints

- What must change and what must not change.
- Required parity, compatibility, or migration constraints.
- Required test types and completion criteria.
- Project and (if applicable) existing plan id.

### 3. Build the Plan Content and Task List

- Produce the plan document (Markdown) with the structure in [Plan Document Shape](#plan-document-shape).
- List tasks with names, descriptions, acceptance criteria, and **prerequisites only** (no fixed execution order).
- Ensure each task is self-contained (Requirements and Specifications, Discovery, Red, Green, Refactor, Testing, Closeout).
- When building **task payloads** for MCP (name, description, acceptance_criteria, steps, requirements, etc.), ingest and follow skill `pma-task-creation`.
- When populating a task's **requirements** array, ingest and follow skill `pma-requirement-object`.

### 4. Persist via MCP

- Create or update the project plan (plan_name, plan_body, state as appropriate; typically `draft` until user approves).
- Create or update task rows using payloads that conform to skill `pma-task-creation`; use returned task ids for dependency resolution.
- Create or update task dependency rows by resolving task names to task ids within the plan.
- Do not mark the plan approved; wait for explicit user approval.

### 5. When Updating an Existing Plan

- Retrieve the current plan and its tasks via MCP.
- Preserve completed work where possible; revise only pending or invalidated parts.
- Keep prerequisites and per-task validation requirements in the revised plan body.
- Ensure the dependency graph in the DB matches the plan document after update.

## Plan Document Shape

The plan body MUST be Markdown and SHOULD follow this structure so executors can run tasks when their **prerequisites** are satisfied.

- **Goal:** One short paragraph on the intended outcome.
- **References:** Requirement documents and technical specifications.
- **Constraints:** Key rules or constraints.
- **Tasks and dependencies:** Use a heading that does not imply a fixed run sequence.
  Optionally add a runnability note: e.g. "Tasks with no dependencies may run immediately; tasks whose dependencies are completed may run in parallel where capacity allows."
  Section numbering (Task 1, Task 2) is for navigation only and MUST NOT be interpreted as run order.
- For each task:
  - **Task N: [Name]** (same name as stored on the task row).
  - Brief description.
  - **Task N Requirements and Specifications:** Canonical req/spec docs.
  - **Discovery (Task N):** Steps to gather context and current implementation.
  - **Red (Task N):** Behavior definition, BDD scenarios, failing tests (including regression for bug fixes).
  - **Green (Task N):** Smallest change to pass tests.
  - **Refactor (Task N):** Refine without changing behavior.
  - **Testing (Task N):** Validation gate; do not proceed until passes.
  - **Closeout (Task N):** Task completion report; update step completion and post-execution notes.
- **Documentation and closeout:** Cross-cutting docs, final plan completion report.

Express **prerequisites** as "Depends on: [task names]" (or "None").
Do not number tasks for execution order; runnability is determined by the dependency graph.
Where possible, show **parallelism** (e.g. multiple tasks with the same or no dependencies).

## Examples

**Trigger:** User says "Plan out adding a new CLI command for project X."

- Ingest skills `pma-task-creation` and `pma-requirement-object`.
  Gather project and any existing plan via MCP; call `plan.help`, `task.help`, `requirement.help`.
- Define scope (e.g. one task per logical step: specs, tests, implementation, docs).
  Build plan body with Goal, References, Constraints, then per-task sections (Requirements and Specifications, Discovery, Red, Green, Refactor, Testing, Closeout).
  Express prerequisites only (e.g. "add-cli-command depends on: write-specs").
- Persist plan (draft), create task rows per skill `pma-task-creation`, create dependency rows.
  Do not mark approved until user approves.

**Trigger:** User says "Update the plan: add a task to run E2E tests after integration."

- Retrieve current plan and tasks via MCP.
  Add new task "run-e2e-after-integration" with prerequisite "integration-task".
  Append task section to plan body; create task row and dependency row via MCP.
  Ensure dependency graph matches plan.

## Quality Bar

Before finalizing and persisting a plan via MCP:

- The plan is detailed enough for an executor to run without guessing.
- Each task has explicit Requirements and Specifications, Discovery, Testing, and Closeout.
- BDD/TDD and regression tests for bug-fix tasks are called out.
- The plan body expresses only prerequisites (dependencies); no strict task order.
- Task names are normalized (lowercase, single dashes) and unique within scope.
- Validation gates and hold points are explicit.
- The agent used only MCP tools and did not simulate or guess data.

## Skills to Ingest

When using this skill, ingest the following skills (by frontmatter `name`) so the agent has the needed guidance:

- `pma-task-creation` - for building task create/update payloads.
- `pma-requirement-object` - for building requirement objects in a task's `requirements` array.

## References

The host project (or instructions) defines:

- Where requirements and technical specifications live.
- The schema for project plans, tasks, task dependencies, and plan revisions.
- MCP tool names and behavior for plan/task/project read-write and for schema guidance (`plan.help`, `task.help`, `requirement.help`).
- Rules for plan lock, approval, and task naming.

Source skill (external): [Detailed Execution Planner](https://github.com/cursor/skills/blob/main/detailed-execution-planner/SKILL.md).
