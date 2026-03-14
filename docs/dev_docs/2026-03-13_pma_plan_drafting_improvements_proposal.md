# PMA Plan Drafting Improvements: Dependencies and Task Refs (Proposal)

- [Purpose](#purpose)
- [Gaps Identified](#gaps-identified)
- [Proposal 1: Dependency-Only Execution](#proposal-1-dependency-only-execution)
- [Proposal 2: Task References in Plan Drafting](#proposal-2-task-references-in-plan-drafting)
- [Proposal 2b: Task Tracking via YAML Frontmatter](#proposal-2b-task-tracking-via-yaml-frontmatter)
- [Task Steps and Artifacts](#task-steps-and-artifacts)
- [Representation: Markdown vs YAML](#representation-markdown-vs-yaml)
- [Proposal 3: Plan Structure and Example](#proposal-3-plan-structure-and-example)
- [Recommendations](#recommendations)

## Purpose

This proposal refines how the Project Manager Agent (PMA) drafts project plans so that:

1. Execution order is driven **only by task dependencies**, not by a hard sequence in the plan document.
2. Tasks (which are separate database entities) are referenced in the plan in a way that stays valid before and after task creation.
3. Plans can clearly represent **parallelism**: any task with no unsatisfied dependencies may run; multiple such tasks may run simultaneously.
4. **Task tracking refs** (task list, dependencies) MAY be expressed in **YAML frontmatter** for machine-parseable handling; steps and artifacts are clearly assigned to Markdown (narrative) vs YAML (structured) where appropriate.

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

## Proposal 2b: Task Tracking via YAML Frontmatter

**Principle:** Allow (and recommend) **YAML frontmatter** at the top of the plan document to carry task list and dependencies in a machine-parseable form.
The plan body remains Markdown; the frontmatter is the canonical source for task refs and dependency graph when present.

### Frontmatter Shape

- Plan documents MAY start with YAML between `---` delimiters.
- When present, frontmatter SHOULD include:
  - **`tasks`**: list of task definitions.
    Each item: `name` (required, task name per [Task Naming](../tech_specs/project_manager_agent.md#spec-cynai-agents-pmtasknaming)), optional `depends_on` (list of task names), optional `description`, optional `acceptance_criteria` (list of strings or structured criteria).
  - Task names in frontmatter MUST match the names used in the Markdown body and when creating `tasks` rows; `depends_on` MUST reference only task names from this list.
- Parsing: the system (gateway, PMA, or CLI) parses frontmatter first to obtain task list and dependencies; the rest of the document is the plan body (Markdown).
- If frontmatter is absent or invalid, fall back to the current behavior: task refs and "Depends on" only in Markdown body; name-to-id resolution at persist time as in Proposal 2.

### Example Frontmatter

```yaml
---
tasks:
  - name: setup-module
    depends_on: []
    description: Create Go module and dir layout.
  - name: add-specs
    depends_on: [setup-module]
  - name: add-tests
    depends_on: [setup-module]
  - name: implement-main
    depends_on: [add-specs, add-tests]
---
```

The body below the second `---` is the human-oriented plan (scope, constraints, per-task prose and acceptance criteria).
When both frontmatter and body exist, **task list and dependencies** are authoritative in the frontmatter for creation and `task_dependencies` resolution; the body remains the source for narrative and for any acceptance criteria not duplicated in frontmatter.

## Task Steps and Artifacts

This section spells out how **task steps** (executable sub-items) and **artifacts** (outputs produced by a task) are represented in the plan and persisted.

### Task Steps

- **Definition:** A step is a discrete, orderable sub-unit of work within a task (e.g. "run tests", "run linter", "commit changes").
  The PMA breaks work into steps suitable for worker nodes ([project_manager_agent.md](../tech_specs/project_manager_agent.md)); the plan may enumerate expected steps for clarity.
- **In the plan document:**
  - **Markdown:** Steps MAY be described in the plan body as prose or bullet lists under each task.
    This is human- and LLM-friendly but not machine-parseable for execution order or tooling.
  - **YAML:** For machine-parseable steps (e.g. APIs, round-trip editing, or future step-level tracking), steps SHOULD be listed in frontmatter under each task, e.g. `steps: [{ id, description }]` or a list of strings.
  - If both are present, YAML is the source of truth for step identity and order; Markdown can elaborate.
- **Persistence:** The DB does not require a dedicated "plan steps" table for MVP; steps are reflected in job payloads and workflow execution.
  Plan-level steps in frontmatter are used when creating or updating tasks and when generating job payloads; they are not stored as separate rows unless a future spec adds a step table.

### Task Artifacts (Expected vs Stored)

- **Definition:** Artifacts are outputs produced during task execution (files, reports, logs) and are stored in `task_artifacts` ([postgres_schema.md](../tech_specs/postgres_schema.md)).
- **In the plan document:**
  - **Markdown:** Expected or required artifacts MAY be described in the plan body per task (e.g. "README with usage example", "test fixtures in `testdata/`").
  - **YAML:** For machine-parseable expected artifacts (e.g. for validation or UI), list them in frontmatter per task, e.g. `artifacts: [{ path_or_type, description }]` or a list of strings.
  - If both are present, YAML is the source of truth for expected-artifact identity; Markdown can elaborate.
- **Persistence:** Actual artifacts are recorded in `task_artifacts` at runtime.
  Plan-level "expected artifacts" in frontmatter are advisory (for PMA verification and user display); they are not stored as separate plan columns unless a future spec adds them.

### Representation Summary

- **Task list and deps.**
  Prefer in plan: YAML frontmatter.
  Machine-parseable: yes (frontmatter).
  Persisted to DB: `tasks`, `task_dependencies`.
- **Task description / acceptance (narrative).**
  Prefer in plan: Markdown body.
  Machine-parseable: optional (frontmatter can mirror).
  Persisted to DB: `tasks.description`, `tasks.acceptance_criteria`.
- **Task steps.**
  Prefer in plan: YAML frontmatter if tooling needs them; else Markdown.
  Machine-parseable: YAML.
  Persisted to DB: via jobs/workflow; no dedicated step table for MVP.
- **Expected artifacts.**
  Prefer in plan: YAML frontmatter if validation/UI needs them; else Markdown.
  Machine-parseable: YAML.
  Persisted to DB: advisory only; actual artifacts in `task_artifacts`.

## Representation: Markdown vs YAML

- **YAML frontmatter** is best for:
  - Task list (names), dependencies (`depends_on`), and optionally per-task steps and expected artifacts.
  - Any field that tooling or APIs must parse without scraping prose (e.g. round-trip edit, dependency graph visualization, validation).
- **Markdown body** is best for:
  - Scope, goal, constraints, and narrative.
  - Human-readable task descriptions and acceptance criteria (and can duplicate or summarize what is in frontmatter).
- **When YAML is preferred for steps/artifacts:** When the system or UI needs to enumerate steps or expected artifacts (e.g. progress UI, validation of "all expected artifacts present"), represent them in frontmatter.
  When the plan is primarily for human and LLM consumption, Markdown-only is sufficient; frontmatter can be minimal (tasks + deps only) or omitted with fallback to body-only parsing per Proposal 2.

## Proposal 3: Plan Structure and Example

The following suggests a consistent structure for the tasks section and an example that exhibits parallel runnability.

### Suggested Structure for the "Tasks and Dependencies" Section

1. **Optional YAML frontmatter.**
   When using frontmatter (Proposal 2b), place the task list and `depends_on` (and optionally per-task steps/artifacts) at the top of the document; the body below is the narrative.
2. **Intro sentence (in body).**
   "The following tasks are part of this plan.
   Runnability is determined only by dependencies: a task may run when all tasks it depends on have status completed; tasks with no dependencies may run immediately; multiple tasks may run in parallel when their dependencies are satisfied."
3. **Task list.**
   For each task (in body and/or frontmatter):
   - **Task name** (the ref used everywhere: `add-project-and-specs`, `implement-flag-and-file-read`, etc.)
   - Short description (prose).
   - Acceptance criteria (list).
   - **Depends on:** list of task names, or "None."
   When frontmatter is used, task list and deps are authoritative there; the body may repeat for readability.
4. **No numbering for execution.**
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
   - Optionally, add YAML frontmatter with `tasks` and `depends_on` as the machine-parseable source for task refs.
   - Optionally, replace the linear chain with the parallel-friendly example above (or add a second subsection with that example).

2. **Document task refs and frontmatter in PMA/spec guidance.**
   In the Project Manager Agent tech spec (or project plan building section), state that plan documents reference tasks by **task name**; that dependencies are expressed as "Depends on: [task names]" in the body and/or as `depends_on` in YAML frontmatter; and that when frontmatter is present, task list and dependencies are taken from it for creation and `task_dependencies` resolution.
   Document that steps and expected artifacts may appear in Markdown (prose) or in frontmatter (machine-parseable); when both exist, frontmatter is the source of truth for steps and expected artifacts.

3. **Leave schema as-is.**
   No change to `tasks`, `task_dependencies`, `task_artifacts`, or `project_plan_revisions` is required; the proposal only clarifies how the plan document (frontmatter + body) is authored and how name-to-id resolution and optional steps/artifacts are handled when persisting.

4. **Optional REQ/spec tweak.**
   If desired, add a single sentence to REQ-PROJCT or to [CYNAI.ACCESS.ProjectPlan](../tech_specs/projects_and_scopes.md#spec-cynai-access-projectplan): "Plan documents reference tasks by human-readable task name; execution order and runnability are determined solely by the task dependency graph stored in task_dependencies, not by the order of tasks in the plan document.
   Task list and dependencies MAY be expressed in YAML frontmatter for machine-parseable task tracking."

This file is a proposal in `docs/dev_docs/` and is not linked from stable docs.
It may be moved or refined before any normative spec or requirement change.
