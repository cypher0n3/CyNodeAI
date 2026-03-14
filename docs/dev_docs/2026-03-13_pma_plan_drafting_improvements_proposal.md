# PMA Plan Drafting Improvements: Dependencies and Task Refs (Proposal)

- [Purpose](#purpose)
- [Gaps Identified](#gaps-identified)
- [Proposal 1: Dependency-Only Execution](#proposal-1-dependency-only-execution)
- [Proposal 2: Task References in Plan Drafting](#proposal-2-task-references-in-plan-drafting)
- [Proposal 3: Plan Structure and Example](#proposal-3-plan-structure-and-example)
- [Recommendations](#recommendations)

## Purpose

This proposal refines how the Project Manager Agent (PMA) drafts project plans so that:

1. Execution order is driven **only by task dependencies**, not by a hard sequence in the plan document.
2. Tasks (which are separate database entities) are referenced in the plan in a way that stays valid before and after task creation.
3. Plans can clearly represent **parallelism**: any task with no unsatisfied dependencies may run; multiple such tasks may run simultaneously.

It addresses gaps in the current example plan ([2026-03-13_pma_plan_example_go_cli.md](2026-03-13_pma_plan_example_go_cli.md)) and suggests changes to plan-drafting guidance and optional spec/requirement clarifications.

## Gaps Identified

1. **Explicit ordering in the plan.**
   The current example presents tasks as "1, 2, 3, 4, 5" with "Dependencies: &lt;prior task&gt;."
   That implies a single linear order.
   The system already defines runnability by the dependency graph ([REQ-PROJCT-0123](../requirements/projct.md#req-projct-0123), [CYNAI.SCHEMA.TaskDependenciesTable](../tech_specs/postgres_schema.md#spec-cynai-schema-taskdependenciestable)); the plan document should not reinforce a fixed sequence.
   It should list tasks and **only** state which tasks each task depends on, so that parallel execution is obvious when multiple tasks have no (or the same) dependencies.

2. **Task refs vs. DB entities.**
   Tasks are rows in `tasks` with `id` (uuid) and a human-readable name (per [project_manager_agent.md - Task Naming](../tech_specs/project_manager_agent.md#task-naming)).
   The plan body is Markdown stored in `project_plans.plan_body`; the task list and dependencies are also reflected in `tasks.plan_id`, `task_dependencies` (by task id), and plan revisions.
   When PMA **drafts** the plan, tasks may not exist yet.
   The plan must reference tasks in a way that (a) is human-readable in the document, (b) can be used when creating tasks (e.g. as the task name), and (c) can be resolved to task ids when writing `task_dependencies` and when linking from the plan body to the stored task rows.

3. **Example does not show parallelism.**
   The Go CLI example is a strict chain (specs -> tests -> impl -> refactor -> docs).
   A better example would include at least two tasks that share the same dependency set (or have none), so that "these may run in parallel" is visible in the plan.

## Proposal 1: Dependency-Only Execution

**Principle:** The plan document MUST NOT imply a single execution order.
It MUST express only **task dependencies** (task A depends on task B; A is runnable when B is completed).
Runnability and execution order are then determined solely by the dependency graph ([REQ-PROJCT-0111](../requirements/projct.md#req-projct-0111), [REQ-PROJCT-0123](../requirements/projct.md#req-projct-0123)).

### Concrete Changes

- Remove any "Execution Order" or "Tasks in order" heading that suggests a fixed sequence.
  Use a heading such as "Tasks and dependencies" or "Task list and dependencies."
- For each task, state **Depends on: &lt;list of task refs&gt;** (or "None").
  Do not number tasks for execution; numbers may be used only for document structure (e.g. section numbering) and must not be interpreted as run order.
- Optionally, add a short "Dependency graph" or "Runnability" note: e.g. "Tasks with no dependencies may run immediately; tasks that depend only on completed tasks may run in parallel where capacity allows."
  This aligns with [langgraph_mvp.md - Workflow plan order](../tech_specs/langgraph_mvp.md#spec-cynai-orches-workflowplanorder) and the fact that the orchestrator uses the `task_dependencies` table for runnability.

No schema change is required; the DB already stores dependencies by task id and does not store a plan-level "sequence."

## Proposal 2: Task References in Plan Drafting

**Principle:** In the plan document (Markdown), reference tasks by **task name** (the same human-readable name that will be stored on the task when it is created).
Task names are unique within scope ([project_manager_agent.md - Task Naming](../tech_specs/project_manager_agent.md#task-naming)); they are the natural stable ref before and after task creation.

### Draft and Persist Flow

1. **Draft phase.**
   PMA produces a plan body that lists tasks, each with a **name** (lowercase, single-dash format), description, acceptance criteria, and "Depends on: [names]."
   No task ids appear in the plan body; names are the only refs.
2. **Task creation.**
   When the user approves and the system tasks PMA to add/update tasks ([REQ-PROJCT-0122](../requirements/projct.md#req-projct-0122)), PMA (or the gateway) creates `tasks` rows with `plan_id`, `name` (or equivalent per schema), description, acceptance_criteria.
   The name used in the plan is the same name stored on the task.
3. **Dependency persistence.**
   When writing `task_dependencies`, the system resolves each "depends on" ref by **task name within the plan** (or by task id if the plan was updated to include ids after creation).
   Recommended: resolve by name within the same plan (lookup tasks where `plan_id` = this plan and normalized name = ref); then insert rows in `task_dependencies` with `task_id` and `depends_on_task_id`.
   Plan revisions that snapshot dependencies already store them by task id ([postgres_schema.md - project_plan_revisions](../tech_specs/postgres_schema.md)); that remains correct.
4. **Linking plan to tasks.**
   Clients and APIs that show "plan + task list" already resolve `tasks.plan_id` and return task list with task ids and names.
   The plan body remains human-oriented (names in prose); the canonical task list and dependency graph live in the DB and are exposed via API.
   Optionally, the gateway or PMA could augment the stored plan body with task ids in a structured block (e.g. for round-trip edits), but the normative ref in the **draft** plan is the task name.

### Edge Cases

- **Name collisions:** Uniqueness is enforced when creating tasks (normalize name, append `-2`, `-3` if needed).
  The plan should use the intended logical name; if the system appends a suffix for uniqueness, the stored task name may differ slightly (e.g. `add-specs-2`).
  PMA instructions should prefer distinct names in the plan to avoid collisions.
- **Plan body vs. DB as source of truth:** The **task list and dependencies** are authoritative in the DB (`tasks`, `task_dependencies`).
  The plan body is a readable description; when they diverge (e.g. after an edit via API that changes dependencies), the DB wins.
  Revisions capture snapshots so the plan body can be updated to match when the plan is edited through the gateway.

**Spec/requirement clarification (optional):** Add an explicit note in the Project Manager Agent or projects_and_scopes spec: "In plan documents, tasks are referenced by human-readable task name (unique within scope).
When persisting task_dependencies, the system resolves task name to task id within the same plan."

## Proposal 3: Plan Structure and Example

The following suggests a consistent structure for the tasks section and an example that exhibits parallel runnability.

### Suggested Structure for the "Tasks and Dependencies" Section

1. **Intro sentence.**
   "The following tasks are part of this plan.
   Runnability is determined only by dependencies: a task may run when all tasks it depends on have status completed; tasks with no dependencies may run immediately; multiple tasks may run in parallel when their dependencies are satisfied."
2. **Task list.**
   For each task:
   - **Task name** (the ref used everywhere: `add-project-and-specs`, `implement-flag-and-file-read`, etc.)
   - Short description (prose).
   - Acceptance criteria (list).
   - **Depends on:** list of task names, or "None."
3. **No numbering for execution.**
   Section numbering (e.g. 1.1, 1.2) may be used for document navigation only; it must not be read as "run first, run second."

**Example that shows parallelism:** Replace or supplement the current Go CLI example with a scenario that has a clear DAG with parallel branches.
For instance:

- **setup-module** (no deps): create Go module and dir layout.
- **add-readme** (no deps): add README with goal and usage.
- **add-license** (no deps): add LICENSE file.

#### Then (Initial Tasks)

- **add-specs** (depends on: setup-module): document flags and exit codes.
- **add-tests** (depends on: setup-module): add failing tests and fixtures.

#### Then (Dependent Tasks)

- **implement-main** (depends on: add-specs, add-tests): implement binary and pass tests.
- **refactor-and-lint** (depends on: implement-main): refactor and run linters.
- **document-usage** (depends on: refactor-and-lint): README usage example and close.

Here, setup-module, add-readme, and add-license can all run in parallel at the start; add-specs and add-tests can run in parallel once setup-module is done.
The plan document would list each task with "Depends on: ..." only; no "step 1, step 2."

## Recommendations

1. **Revise the existing example plan** ([2026-03-13_pma_plan_example_go_cli.md](2026-03-13_pma_plan_example_go_cli.md)):
   - Rename "Tasks (Execution Order)" to "Tasks and dependencies."
   - Add the one-sentence runnability/dependency note at the top of that section.
   - For each task, keep "Depends on: &lt;names&gt; | None" and remove any implication that the document order is the run order.
   - Optionally, replace the linear chain with the parallel-friendly example above (or add a second subsection with that example).

2. **Document task refs in PMA/spec guidance.**
   In the Project Manager Agent tech spec (or project plan building section), state explicitly that plan documents reference tasks by **task name** and that dependencies in the plan are expressed as "Depends on: [task names]"; when creating tasks and task_dependencies, the system uses those names to create `tasks` rows and to resolve ids for `task_dependencies` rows.

3. **Leave schema as-is.**
   No change to `tasks`, `task_dependencies`, or `project_plan_revisions` is required; the proposal only clarifies how the plan **body** (Markdown) is authored and how name-to-id resolution works when persisting.

4. **Optional REQ/spec tweak.**
   If desired, add a single sentence to REQ-PROJCT or to [CYNAI.ACCESS.ProjectPlan](../tech_specs/projects_and_scopes.md#spec-cynai-access-projectplan): "Plan documents reference tasks by human-readable task name; execution order and runnability are determined solely by the task dependency graph stored in task_dependencies, not by the order of tasks in the plan document."

This file is a proposal in `docs/dev_docs/` and is not linked from stable docs.
It may be moved or refined before any normative spec or requirement change.
