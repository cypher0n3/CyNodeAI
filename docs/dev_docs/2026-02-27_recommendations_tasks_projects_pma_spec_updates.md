# Recommendations: Tasks, Projects, and PMA Spec Updates

- [1. Purpose](#1-purpose)
- [2. Current State Summary](#2-current-state-summary)
  - [2.1 Projects and Tasks Today](#21-projects-and-tasks-today)
  - [2.2 PMA Today](#22-pma-today)
  - [2.3 Agile Work Model (Draft)](#23-agile-work-model-draft)
- [3. Gaps](#3-gaps)
- [4. Recommendations for Spec Updates](#4-recommendations-for-spec-updates)
  - [4.1 Requirements Layer](#41-requirements-layer)
  - [4.2 Tech Specs Layer](#42-tech-specs-layer)
  - [4.3 Chat and Multi-Message Clarification](#43-chat-and-multi-message-clarification)
- [5. Summary Table](#5-summary-table)
- [6. Traceability](#6-traceability)
- [7. Next Steps](#7-next-steps)

## 1. Purpose

**Date:** 2026-02-27.
**Type:** Recommendations (docs-only).
**Scope:** Defining/building tasks and projects; project plans; PMA responsibility to build plans and clarify before execution.

Draft recommendations for updating specs in `docs/requirements/` and `docs/tech_specs/` so that:

- All tasks are associated to a project.
- Non-user-default projects have well-built **project plans** that outline the tasks required and the **order** in which they must be executed.
  There MAY be only **one plan per project**.
- A user's **default project** is a catch-all for unrelated tasks; the system SHOULD **prefer to associate to another project whenever possible** (e.g. when the user or PMA can resolve a named project or the work clearly belongs to an existing non-default project).
- Each (orchestrator) task may have **multiple jobs** that execute parts of the task (already true in schema).
- The **Project Manager Agent (PMA)** is responsible for **building** those project plans from user input **before** handing work off for execution.
- The PMA **prefers to ask clarifying questions** when user intent is unclear, rather than inferring and doling out tasks immediately.
- **Users MUST be able to edit and update project plans and associated tasks via client tools** (e.g. Web Console, CLI, or API).
- The PMA SHOULD **refine project plans as needed** based on updated information from the user (e.g. after clarification or change requests).
- **Project plan and task description text, updates, and related editable content MUST be stored as Markdown** to support editing in client tools.
- **Users MAY lock a project plan** so that only the plan's document (plan name, description/body) is not editable by the agent or by client interfaces until unlocked; enforcement via API checks.
  When locked: **users** (via clients/API) MAY still add/remove/reorder tasks and edit task fields; **agents** MUST NOT change the plan or its tasks but MAY update completion status and comments on plans and tasks.
  Unlock by the user (or by a principal with unlock permission).
  **RBAC** MUST allow assigning lock/unlock permissions for shared (group) project plans.

No code or spec edits are proposed in this document; only recommendations for future spec changes.

## 2. Current State Summary

Brief summary of how projects, tasks, jobs, and PMA are specified today.

### 2.1 Projects and Tasks Today

- **Projects** ([postgres_schema.md](../tech_specs/postgres_schema.md), [projects_and_scopes.md](../tech_specs/projects_and_scopes.md)): `projects` table has `id`, `slug`, `display_name`, `description`, `is_active`.
  No concept of a "project plan" or ordered task list.
  - Each user has a default project (catch-all); current specs do not state that the system should prefer associating to a non-default project when possible.
- **Tasks** ([postgres_schema.md](../tech_specs/postgres_schema.md)): `tasks` table has `project_id` (nullable); when null, default project is used per [REQ-PROJCT-0104](../requirements/projct.md#req-projct-0104).
  Tasks are not ordered within a project; there is no `project_plan` or task-ordering entity.
- **Jobs** ([postgres_schema.md](../tech_specs/postgres_schema.md)): `jobs` table has `task_id` (FK to `tasks.id`).
  One task can have many jobs; this already matches the desired "task has multiple jobs" model.

### 2.2 PMA Today

- **project_manager_agent.md**: PMA does "task intake and triage," "create and update tasks, subtasks, and acceptance criteria," "break work into executable steps," "planning and dispatch."
  There is no explicit responsibility to "build a project plan" or to "ask clarifying questions before creating/doling out tasks."
- **E2E gap analysis** ([2026-02-27_analysis_e2e_tests_gap_and_remediation.md](2026-02-27_analysis_e2e_tests_gap_and_remediation.md) section 4.3): REQ-USRGWY-0130 requires **chat threads and chat messages tracked separately from task lifecycle state**; it does not require or forbid "one task per message."
  **Positioning (for spec updates):** Building up a task properly may take **multiple messages** for clarification and to properly lay out the task.
  Specs (e.g. `chat_threads_and_messages`, `openai_compatible_chat_api`, or related orchestration/PM docs) should be updated to state this explicitly: **multi-message conversation is the intended way to clarify and lay out the task before or as it is executed.**
  This clarification belongs in the normative docs; for now it is recorded in dev_docs.

### 2.3 Agile Work Model (Draft)

- **draft_specs/agile_pm_rough_spec.md**: Introduces Epic -> Feature -> Story -> Task -> Sub-task; agile Sub-task maps to one CyNodeAI Job; agile Task/Story are parent work items.
  This is in `draft_specs/`, not yet normative.
  It does not define "project plan" or execution order of tasks.

## 3. Gaps

- **Area:** Projects
  - gap: No first-class "project plan" that lists tasks and their execution order.
  - gap: No constraint that there is at most one plan per project.
  - gap: No explicit positioning that user default project is a catch-all and that the system should prefer to associate to another project whenever possible.
  - gap: No requirement that project plan and task description text (and related editable content) are stored as Markdown for editing.
  - gap: No plan lock: users cannot lock the plan document (name, body) to prevent agent/client edits; no RBAC for lock/unlock on shared (group) plans.
- **Area:** Tasks
  - gap: Task-to-project association exists; "all tasks associated to a project" is effectively true via default project.
  - gap: No ordering of tasks within a project.
- **Area:** PMA
  - gap: No explicit requirement that PMA **builds** a project plan from user input **before** handing off for execution.
- **Area:** PMA
  - gap: No explicit requirement that PMA **prefers to ask clarifying questions** when unclear, rather than creating tasks immediately.
  - gap: No explicit requirement that PMA refines project plans as needed based on updated info from the user.
- **Area:** Clients / user tools
  - gap: No requirement that users can edit and update project plans and associated tasks via client tools (Web Console, CLI, API).
- **Area:** Multi-message
  - gap: Building up a task properly may take multiple messages for clarification and to properly lay out the task; multi-message conversation is the intended way to clarify and lay out the task before or as it is executed.
  - gap: This clarification belongs in the normative docs but is not yet there (recorded in dev_docs for now).

## 4. Recommendations for Spec Updates

Concrete suggestions for requirements and tech spec documents.

### 4.1 Requirements Layer

Requirements docs get **specific, short, traceable** REQ-* entries only: atomic, testable, one obligation per `REQ-<DOMAIN>-<NNNN>`, each with Traces To spec anchor(s).
No full spec text or implementation detail in requirements; "how" lives in tech specs (see [spec_authoring_writing_and_validation.md](../docs_standards/spec_authoring_writing_and_validation.md) Documentation Layers).

#### 4.1.1 PROJCT (`docs/requirements/projct.md`)

Add discrete, short REQ-PROJCT-* entries (one obligation per REQ; each with Traces To spec anchor).
Suggested scope:

- Project MAY have an associated project plan (task set and optionally order); at most one plan per project; implementation (first-class vs derived) in tech specs.
- Tasks created under a project MAY be associable with a plan when the project has a plan.
- User's default project is catch-all for unrelated tasks; system (gateway, PMA) SHOULD prefer to associate to another project whenever possible.
- Users MUST be able to edit and update project plans and associated tasks via client tools (Web Console, CLI, or API); implementation in tech specs (gateway API, client parity).
- Project plan and task description text, updates, and related editable content MUST be stored as Markdown for editing.
- Users MAY lock a project plan (plan document only: name, description/body); when locked, plan document not editable by agent or clients (API-enforced); only user or principal with unlock permission may unlock.
  When locked: users MAY still add/remove/reorder tasks and edit task fields via clients/API; agents MUST NOT change plan or tasks but MAY update completion status and comments on plans and tasks.
  RBAC MUST allow assigning lock/unlock permissions for shared (group) project plans.

#### 4.1.2 PMAGNT (`docs/requirements/pmagnt.md`)

Add discrete, short REQ-PMAGNT-* entries (one obligation per REQ; each with Traces To spec anchor).
Suggested scope:

- PMA MUST (or SHOULD) build a project plan (tasks + execution order) from user input before creating orchestrator tasks and handing work off, when the request implies multi-step or project-scoped effort.
- PMA SHOULD ask the user clarifying questions when scope, acceptance criteria, or execution order is ambiguous, and prefer clarification over inferring and creating tasks immediately.
- PMA SHOULD refine project plans as needed based on updated information from the user (e.g. after clarification or change requests).
- When a project plan is locked, PMA MUST NOT change the plan or its tasks; PMA MAY update completion status and comments on plans and tasks.

#### 4.1.3 AGENTS (`docs/requirements/agents.md`)

Add a discrete, short REQ-AGENTS-* entry (one obligation; Traces To spec anchor).
Suggested scope:

- Project Manager Agent SHOULD use multi-turn conversation to clarify and lay out a task or project plan when user intent is unclear; multi-message conversation is the intended way to clarify and lay out the task before or as it is executed.
  REQ links to `chat_threads_and_messages`, `openai_compatible_chat_api`, and REQ-USRGWY-0130.

#### 4.1.4 ORCHES / USRGWY (If Needed)

- If workflow start semantics depend on "plan approved" or "plan ready," add requirements that the orchestrator (or gateway) only starts workflow for a task when the task is part of an approved plan or when the PMA has explicitly handed off the task for execution (current behavior can remain the default).

### 4.2 Tech Specs Layer

Recommendations for tech spec documents.

#### 4.2.1 Projects and Scopes (`projects_and_scopes.md`)

- Define **project plan** (or equivalent): a set of tasks belonging to a project, with an optional **execution order** (e.g. ordered list of task ids or step numbers).
  There is at most **one plan per project**.
  Clarify whether this is a first-class stored entity (e.g. `project_plans` table with rows like `project_id`, `task_id`, `ordinal`) or a view over `tasks WHERE project_id = X` with an `ordinal`/`sequence` column on `tasks`.
- State that tasks in a project MAY be part of a plan and, when so, have a defined order for execution.
- State that the user's **default project** is a catch-all for unrelated tasks and that the system SHOULD **prefer to associate to another project whenever possible** (e.g. when the user or PMA can resolve a named project or the work clearly belongs to an existing non-default project).
- State that users MUST be able to edit and update project plans and associated tasks via client tools; specify or reference gateway API and client parity (Web Console, CLI) for plan and task CRUD.
- State that project plan and task description text, updates, and related editable content are stored as Markdown (e.g. plan body, task description, acceptance criteria text) so clients can edit and render consistently.
- Define **plan lock**: only the plan document (name, description/body) is locked; enforcement via API checks so plan document is not editable by agent or clients until unlocked.
  When locked: users MAY still add/remove/reorder tasks and edit task fields via clients/API; agents MUST NOT change plan or tasks but MAY update completion status and comments on plans and tasks.
  Unlock by user or principal with unlock permission.
  RBAC: allow assigning lock/unlock permissions for shared (group) project plans (reference `rbac_and_groups.md`, `access_control.md`).

#### 4.2.2 Project Manager Agent (`project_manager_agent.md`)

- Add a subsection **Project plan building**: PMA is responsible for building the project plan (tasks + order) from user input before creating orchestrator tasks and dispatching jobs.
  When the user describes a goal that implies multiple steps or a project, PMA MUST (or SHOULD) first produce a plan (e.g. list of tasks with order and acceptance criteria), persist it or associate tasks with the plan, and only then create tasks and hand off for execution.
  PMA SHOULD refine project plans as needed based on updated information from the user (e.g. after clarification or change requests).
- Add a subsection **Clarification before execution**: PMA SHOULD ask clarifying questions when scope, acceptance criteria, priorities, or execution order are ambiguous.
  PMA SHOULD prefer multi-turn clarification over inferring and creating tasks immediately.
  Reference the E2E gap doc positioning: multi-message conversation is the intended way to clarify and lay out the task before or as it is executed; specs such as `chat_threads_and_messages`, `openai_compatible_chat_api`, or related orchestration/PM docs should state this explicitly (see [2026-02-27_analysis_e2e_tests_gap_and_remediation.md](2026-02-27_analysis_e2e_tests_gap_and_remediation.md) section 4.3).
- In **Agent Responsibilities**, explicitly list "Build project plans from user input," "Refine project plans as needed based on updated info from the user," "Clarify with user before doling out tasks when unclear," and "Prefer to associate tasks to a non-default project when the user or context implies a named project or existing non-default project (default project is catch-all for unrelated work)."
- Add **When plan is locked**: PMA MUST NOT change the plan or its tasks; PMA MAY update completion status and comments on plans and tasks.
  API/gateway enforces lock so PMA tool calls that would edit plan document or tasks are rejected when plan is locked.

#### 4.2.3 Postgres Schema (`postgres_schema.md`)

- If project plan is first-class: add a **Project plans** (or equivalent) section and table(s), e.g. `project_plans` with `id`, `project_id`, `version`/`updated_at`, and a `project_plan_tasks` (or similar) with `plan_id`, `task_id`, `ordinal`, so that tasks can be ordered within a plan.
  Enforce at most one plan per project (e.g. unique on `project_plans.project_id` or equivalent).
  Include plan lock state (e.g. `is_locked`, optionally `locked_at`/`locked_by`) so API can enforce: when locked, plan document (name, body) is read-only; task list and task fields remain editable by users; agents may only update completion status and comments.
  Plan and task description/body text MUST be stored as Markdown (e.g. `plan.body` or `tasks` description/acceptance_criteria as Markdown).
- If project plan is task-centric: add an optional `ordinal` or `sequence` (and optionally `plan_id`) to the `tasks` table so that tasks within a project can be ordered; document that this ordering defines execution order when present.
  Task description and related editable text MUST be stored as Markdown.

#### 4.2.4 Agile PM Rough Spec (`agile_pm_rough_spec.md`) (If Promoted to Tech Specs)

- Align with the above: either (a) map "project plan" to an Epic/Feature/Story level that contains ordered Tasks/Sub-tasks, or (b) define how a project plan (task list + order) relates to Epic/Feature/Story hierarchy.
  Clarify that execution order is defined at the level that maps to orchestrator tasks (e.g. Story or Task level), and that Jobs remain the unit of sandbox execution per task.

#### 4.2.5 LangGraph MVP (`langgraph_mvp.md`) (If Needed)

- If workflow start or step selection depends on "plan" or "order," document that the workflow (or PMA) consumes the project plan / task order when deciding which task to run next or when to start a workflow for a given task.

### 4.3 Chat and Multi-Message Clarification

Per the E2E gap doc ([2026-02-27_analysis_e2e_tests_gap_and_remediation.md](2026-02-27_analysis_e2e_tests_gap_and_remediation.md) section 4.3): REQ-USRGWY-0130 requires that the system store chat history as **chat threads and chat messages tracked separately from task lifecycle state**; it does not require or forbid "one task per message."

**Positioning (for spec updates):** Building up a task properly may take **multiple messages** for clarification and to properly lay out the task.
Specs (e.g. `chat_threads_and_messages`, `openai_compatible_chat_api`, or related orchestration/PM docs) should be updated to state this explicitly: **multi-message conversation is the intended way to clarify and lay out the task before or as it is executed.**
This clarification belongs in the normative docs; for now it is recorded in dev_docs.

## 5. Summary Table

Per [spec_authoring_writing_and_validation.md](../docs_standards/spec_authoring_writing_and_validation.md): **requirements** (`docs/requirements/`) get atomic, testable, short REQ-* entries (one obligation per REQ; each with Traces To spec anchor); **tech specs** (`docs/tech_specs/`) get prescriptive "how" and link back to requirements.

- **Requirements - projct.md**
  - Add discrete REQ-PROJCT-* entries (atomic, traceable):
    - project MAY have associated project plan; at most one plan per project;
    - tasks associable with plan when project has plan;
    - default project is catch-all; system SHOULD prefer to associate to another project whenever possible
    - users MUST be able to edit and update project plans and associated tasks via client tools
    - plan and task description text MUST be stored as Markdown.
    - Lock: users MAY lock plan document (name, body); API enforces lock; when locked users may still add/remove/reorder tasks and edit task fields and agents may only update completion status and comments; RBAC allows assigning lock/unlock permissions for shared (group) project plans.
      Each REQ links to spec anchor(s).
- **Requirements - pmagnt.md**
  - Add discrete REQ-PMAGNT-* entries: PMA builds project plan from user input before creating tasks/handoff when request implies multi-step or project-scoped effort; PMA SHOULD ask clarifying questions when scope/criteria/order ambiguous and prefer clarification over inferring; PMA SHOULD refine project plans as needed based on updated info from the user.
  - When plan is locked: PMA MUST NOT change plan or tasks and MAY update completion status and comments only.
    Each REQ links to spec anchor(s).
- **Requirements - agents.md**
  - Add discrete REQ-AGENTS-* entry: Project Manager Agent SHOULD use multi-turn conversation to clarify and lay out task or plan when intent unclear; reference chat/thread specs and REQ-USRGWY-0130.
    REQ links to spec anchor(s).
- **Tech spec - projects_and_scopes.md**
  - Define project plan (task set + execution order), at most one plan per project, ordinal/plan storage, default project as catch-all, prefer association when possible; users able to edit/update plans and tasks via client tools (gateway API and client parity); plan and task text as Markdown; plan lock (plan document only, API-enforced), when locked users may edit tasks and agents may update status/comments only, RBAC for lock/unlock on shared (group) plans.
    Add Spec ID anchors; link back to new PROJCT requirements.
- **Tech spec - project_manager_agent.md**
  - Add subsections (project plan building, including refining plans from updated user info; clarification before execution; behavior when plan is locked: no plan/task edits, status and comments only).
    Update responsibilities list (include refine plans, client-editable plans, respect plan lock).
    Add Spec ID anchors; link back to new PMAGNT/AGENTS requirements.
- **Tech spec - postgres_schema.md**
  - Add project plan and/or task ordinal/plan_id per chosen model; at most one plan per project (e.g. unique on project_id in project_plans); plan lock state (e.g. is_locked, locked_at, locked_by); store plan and task description/editable text as Markdown (e.g. plan body, task description, acceptance_criteria text).
    Link to requirements and projects_and_scopes.
- **Tech spec - rbac_and_groups.md / access_control.md**
  - Define lock/unlock permissions for project plans; allow assigning these permissions for shared (group) project plans so group members can be granted lock or unlock.
    Link to PROJCT requirements and projects_and_scopes.
- **Tech spec - agile_pm_rough_spec** (if promoted)
  - Align project plan and execution order with Epic/Feature/Story/Task.
    Link to new requirements.
- **Tech spec - langgraph_mvp.md**
  - If workflow depends on plan/order, document consumption of project plan / task order.
    Link to requirements.
- **Tech spec - chat_threads_and_messages / openai_compatible_chat_api / related PM docs**
  - State explicitly: multi-message conversation is the intended way to clarify and lay out the task before or as it is executed; building up a task may take multiple messages; this clarification belongs in normative docs (per E2E gap doc section 4.3).
    Link to REQ-USRGWY-0130 and any new AGENTS REQ.

## 6. Traceability

- Requirements changes should follow [spec_authoring_writing_and_validation.md](../docs_standards/spec_authoring_writing_and_validation.md): new REQ-IDs, Traces To spec anchors.
- Tech spec changes should add Spec ID anchors where new behavior is prescribed and link back to the new requirements.

## 7. Next Steps

1. Decide whether "project plan" is a first-class entity (new table(s)) or task-centric (ordinal/plan_id on tasks).
2. Add PROJCT/PMAGNT/AGENTS requirements as above (or adjusted after review).
3. Update projects_and_scopes, project_manager_agent, postgres_schema per recommendations.
4. Add requirement and spec coverage for user-editable project plans and tasks via client tools (gateway API, Web Console, CLI parity).
5. Add plan lock (plan document only), API enforcement, and RBAC for lock/unlock on shared (group) plans; PMA behavior when locked (status and comments only).
6. Add multi-message clarification to chat/PM specs.
7. If promoting agile_pm_rough_spec, align project plan and execution order with that hierarchy.
