# Spec Updates From 2026-02-27 Recommendations

## Summary

- **Date:** 2026-03-02
- **Source:** [2026-02-27_recommendations_tasks_projects_pma_spec_updates.md](2026-02-27_recommendations_tasks_projects_pma_spec_updates.md)

Tech specs and requirements were updated to align with the recommendations for project plans, default-project behavior, PMA plan building and clarification, plan lock, and multi-message clarification.

## Requirements Added

- **PROJCT** (`projct.md`): REQ-PROJCT-0110 (project plan, one per project), 0111 (tasks associable with plan), 0112 (default project catch-all; prefer other project when possible), 0113 (users edit plans/tasks via clients), 0114 (Markdown storage), 0115 (plan lock), 0116 (RBAC for lock/unlock on shared plans).
- **PMAGNT** (`pmagnt.md`): REQ-PMAGNT-0111 (build plan before handoff), 0112 (clarify when ambiguous), 0113 (refine plans from user updates), 0114 (when locked: no plan/task edits, status/comments only).
- **AGENTS** (`agents.md`): REQ-AGENTS-0135 (multi-turn conversation to clarify and lay out task or plan; links to chat specs and REQ-USRGWY-0130).

## Tech Specs Updated

- **projects_and_scopes.md**: Project plan (definition, one per project, task order), default project as catch-all and prefer association, client edit, Markdown, plan lock (document only; when locked users may edit tasks and agents may update status/comments), link to RBAC.
  New Spec IDs: CYNAI.ACCESS.ProjectPlan, ProjectPlanClientEdit, ProjectPlanMarkdown, ProjectPlanLock.
- **project_manager_agent.md**: Agent responsibilities updated (build/refine plans, prefer non-default project, clarify before doling out tasks).
  New sections: Project plan building, Clarification before execution, When plan is locked.
  New Spec IDs: CYNAI.AGENTS.ProjectPlanBuilding, ClarificationBeforeExecution, WhenPlanLocked.
- **postgres_schema.md**: Projects table: `plan_name`, `plan_body` (Markdown), `is_plan_locked`, `plan_locked_at`, `plan_locked_by`.
  Tasks table: `ordinal` (execution order), `description` (Markdown); acceptance_criteria text as Markdown.
  Index (`project_id`, `ordinal`) for plan ordering.
- **rbac_and_groups.md**: New subsection "Project plan lock RBAC" (Spec ID CYNAI.ACCESS.ProjectPlanLockRbac); RBAC allows assigning lock/unlock permissions for project plans, including shared (group) plans.
- **chat_threads_and_messages.md**: Goal added that multi-message conversation is the intended way to clarify and lay out a task (or project plan); links to REQ-AGENTS-0135 and ClarificationBeforeExecution spec.
- **openai_compatible_chat_api.md**: In Conversation Model, added sentence that multi-message conversation is the intended way to clarify and lay out a task (or project plan); link to REQ-AGENTS-0135.
- **langgraph_mvp.md**: New subsection "Project plan and task order" (Spec ID CYNAI.ORCHES.WorkflowPlanOrder): when a task belongs to a project that has a plan, the workflow engine and orchestrator MUST respect the plan's task execution order when deciding which task to run next.

## Design Choice

Project plan is implemented in a **task-centric** way for MVP: one plan per project represented by optional plan fields on the project (`plan_name`, `plan_body`, lock state) and task execution order via `ordinal` on tasks.
A first-class `project_plans` table remains an allowed future alternative (see recommendations 4.2.3 and 7.1).

## Not Done in This Pass

- agile_pm_rough_spec alignment (recommendation 4.2.4): out of scope for open-core; moved to an enterprise feature.

## Later Additions (Post 2026-03-02)

- **Plan approved state and workflow gate:** REQ-ORCHES-0152, REQ-PROJCT-0117/0118/0119/0120; plan approved state, auto un-approve on edit, plan revisions table, workflow start gate (langgraph_mvp), user review/approve via clients (REQ-CLIENT-0179).
  See projects_and_scopes, postgres_schema, langgraph_mvp, user_api_gateway, access_control, rbac_and_groups, cynork_cli, web_console.
- **Access control formal action names:** access_control.md now prescribes `project_plan.read`, `project_plan.update`, `project_plan.lock`, `project_plan.unlock`, `project_plan.approve` (Spec ID CYNAI.ACCESS.ProjectPlanActions).

## Validation

- `just docs-check` (markdownlint + doc link validation + feature file validation) passes.
