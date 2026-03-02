# Spec Validation: 2026-02-27 Recommendations

- [Summary](#summary)
- [Requirements Layer (4.1)](#requirements-layer-41)
- [Tech Specs Layer (4.2)](#tech-specs-layer-42)
- [Traceability Check](#traceability-check)
- [Design Choice](#design-choice)
- [Conclusion](#conclusion)

## Summary

- **Date:** 2026-03-02
- **Source:** [2026-02-27_recommendations_tasks_projects_pma_spec_updates.md](2026-02-27_recommendations_tasks_projects_pma_spec_updates.md)
- **Reference:** [2026-03-02_spec_updates_from_recommendations.md](2026-03-02_spec_updates_from_recommendations.md)

Validation confirms that the tech specs and requirements have been **properly updated** to reflect the 2026-02-27 recommendations.
All recommended requirement entries and tech spec changes are present, with correct traceability (REQ-* to Spec ID anchors).

## Requirements Layer (4.1)

- **Recommendation 4.1.1 PROJCT**
  - Document: `projct.md`
  - Status: Done
  - Notes: REQ-PROJCT-0110 through 0116 present.
  - Notes: Each has Traces To spec anchor(s).
- **Recommendation 4.1.2 PMAGNT**
  - Document: `pmagnt.md`
  - Status: Done
  - Notes: REQ-PMAGNT-0111 (build plan), 0112 (clarify), 0113 (refine), 0114 (when locked).
- **Recommendation 4.1.3 AGENTS**
  - Document: `agents.md`
  - Status: Done
  - Notes: REQ-AGENTS-0135 (multi-turn clarify/lay out).
  - Notes: Links to chat specs and REQ-USRGWY-0130.
- **Recommendation 4.1.4 ORCHES/USRGWY**
  - Document: (none)
  - Status: Deferred
  - Notes: Intentionally not done this pass (see 2026-03-02_spec_updates_from_recommendations.md).

## Tech Specs Layer (4.2)

- **Recommendation 4.2.1** (project plan, default project, client edit, Markdown, lock, RBAC)
  - Document: `projects_and_scopes.md`
  - Status: Done
  - Notes: Project plan (one per project, task order), default project catch-all and prefer association.
  - Notes: ProjectPlanClientEdit, ProjectPlanMarkdown, ProjectPlanLock; link to RBAC.
  - Notes: Spec IDs present.
- **Recommendation 4.2.2** (plan building, clarification, when locked; responsibilities)
  - Document: `project_manager_agent.md`
  - Status: Done
  - Notes: Subsections Project plan building, Clarification before execution, When plan is locked.
  - Notes: Responsibilities list includes build/refine plans, prefer non-default project, clarify before doling out.
- **Recommendation 4.2.3** (schema: plan fields, ordinal, Markdown)
  - Document: `postgres_schema.md`
  - Status: Done
  - Notes: Projects: `plan_name`, `plan_body`, `is_plan_locked`, `plan_locked_at`, `plan_locked_by`.
  - Notes: Tasks: `ordinal`, `description` (Markdown), `acceptance_criteria` (Markdown).
  - Notes: Index (`project_id`, `ordinal`).
- **Recommendation 4.2.4** (Agile PM rough spec)
  - Document: (none)
  - Status: Out of scope
  - Notes: Moved to an enterprise feature; not in open-core component.
- **Recommendation 4.2.5** (LangGraph MVP)
  - Document: `langgraph_mvp.md`
  - Status: Done
  - Notes: New subsection "Project plan and task order" (CYNAI.ORCHES.WorkflowPlanOrder).
  - Notes: Workflow and orchestrator MUST respect plan task order when a plan exists.
- **RBAC lock/unlock for shared plans**
  - Document: `rbac_and_groups.md`
  - Status: Done
  - Notes: Subsection "Project plan lock RBAC" (CYNAI.ACCESS.ProjectPlanLockRbac).
  - Notes: Traces To REQ-PROJCT-0116.
- **Recommendation 4.3** (multi-message clarification)
  - Document: `chat_threads_and_messages.md`
  - Status: Done
  - Notes: Goal states multi-message conversation is intended way to clarify and lay out task.
  - Notes: Links to REQ-AGENTS-0135 and ClarificationBeforeExecution.
- **Recommendation 4.3** (multi-message clarification)
  - Document: `openai_compatible_chat_api.md`
  - Status: Done
  - Notes: Conversation Model includes sentence and link to REQ-AGENTS-0135.

## Traceability Check

- All new REQ-PROJCT, REQ-PMAGNT, and REQ-AGENTS entries reference existing Spec ID anchors in the tech specs.
- Spec IDs referenced from requirements exist in `projects_and_scopes.md`, `project_manager_agent.md`, and `rbac_and_groups.md` with matching `id` attributes for fragment links.

## Design Choice

Per [2026-03-02_spec_updates_from_recommendations.md](2026-03-02_spec_updates_from_recommendations.md): project plan is implemented in a **task-centric** way.
Plan metadata and lock live on `projects`; execution order via `ordinal` on `tasks`.
This matches recommendation 4.2.3 (task-centric option).

## Conclusion

The specs have been properly updated to reflect the 2026-02-27 recommendations.
Items explicitly deferred (ORCHES/USRGWY workflow start, agile_pm_rough_spec, langgraph_mvp consumption, access_control action names) are documented in 2026-03-02_spec_updates_from_recommendations.md.
Those deferrals are consistent with the recommendations' "if needed" / "if promoted" wording.
