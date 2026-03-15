# Spec Integration: PMA Plan Creation Skill

- [Purpose](#purpose)
- [Current State](#current-state)
- [Proposed Spec and Requirement Changes](#proposed-spec-and-requirement-changes)
  - [Requirements (`docs/requirements/`)](#requirements-docsrequirements)
  - [Technical Specifications (`docs/tech_specs/`)](#technical-specifications-docstech_specs)
- [MCP Tool Catalog Additions](#mcp-tool-catalog-additions)
  - [Help tools (plan.help, task.help, requirement.help)](#help-tools-planhelp-taskhelp-requirementhelp)
- [Instructions and Skills Integration](#instructions-and-skills-integration)
- [Acceptance and Promotion Path](#acceptance-and-promotion-path)
- [References](#references)

## Purpose

This draft describes how to add or update requirements and technical specifications so that the **PMA Plan Creation Skill** ([pma_plan_creation_skill.md](../../default_skills/pma_plan_creation_skill.md)) is formally part of the system.
The skill definition lives in [default_skills/](../../default_skills/README.md).
This document is the integration plan for moving the skill into canonical docs and agent behavior.

## Current State

- **Skill definition:** [pma_plan_creation_skill.md](../../default_skills/pma_plan_creation_skill.md) in [default_skills/](../../default_skills/README.md) (draft until promoted).
- **Plan/task behavior:** Already specified in [project_manager_agent.md](../tech_specs/project_manager_agent.md) (Project Plan Building, Clarification, When Plan is Locked, Plan Approval, Plan Approved PMA Tasked), [projects_and_scopes.md](../tech_specs/projects_and_scopes.md) (plan state, revisions), [postgres_schema.md](../tech_specs/postgres_schema.md) (project_plans, tasks, task_dependencies, project_plan_revisions).
- **MCP tools:** [mcp_tool_catalog.md](../tech_specs/mcp_tool_catalog.md) defines `db.task.get`, `db.task.update_status`, `db.project.get`, `db.project.list`.
  There are **no** `db.plan.*` tools and no full task CRUD for agents in the catalog today; task creation is described as User API Gateway surface.
- **PMA instructions:** Agent instructions live in `agents/instructions/project_manager/` and `agents/instructions/project_analyst/`; they reference tools and behavior but do not yet encode the full plan-creation workflow from the skill.
- **Skills system:** [skills_storage_and_inference.md](../tech_specs/skills_storage_and_inference.md) defines the default CyNodeAI interaction skill and MCP skills tools; there is no second system skill for plan creation yet.

## Proposed Spec and Requirement Changes

The following requirements and tech spec edits are proposed.

### Requirements (`docs/requirements/`)

Proposed requirement edits:

#### `pmagnt.md` or `projct.md`

Add (or tighten) a requirement that when the PMA builds or refines a project plan, it MUST follow a defined plan-creation procedure that ensures:

- Plans are aligned with requirements and tech specs, executable (concrete tasks with acceptance criteria), and dependency-only for execution order.
- All plan and task reads/writes are performed via MCP; no direct DB access.
- Plan lock and user approval are respected.

Trace to the tech spec that will define or reference the procedure (e.g. the plan creation skill promoted to tech spec or instructions).

#### `agents.md` or `pmagnt.md`

Optionally add that the plan-creation procedure MUST include per-task structure (Requirements and Specifications, Discovery, Red, Green, Refactor, Testing, Closeout) and validation gates so that tasks are independently executable and test-gated.
This can be a SHOULD if we want flexibility for simpler plans.

### Technical Specifications (`docs/tech_specs/`)

Proposed tech spec edits:

#### Updates to `project_manager_agent.md`

- Add a subsection (e.g. "Plan creation procedure" or "Plan creation skill") that states the PMA MUST (or SHOULD) follow the documented plan-creation procedure when building or refining plans.
- Link to the canonical source of that procedure (either a new tech spec subsection, the instructions bundle, or the skill store entry).
- Optionally summarize: gather inputs, define scope, build plan content and task list, persist via MCP, respect lock and approval; task refs by name; dependencies only in plan body.
- State that plan documents reference tasks by **task name** (human-readable, unique within scope); dependencies are expressed as "Depends on: [task names]" in the body and/or as `depends_on` in **YAML frontmatter** when the host supports it; when frontmatter is present, task list and dependencies are taken from it for creation and `task_dependencies` resolution.
  The **task list and dependencies in the DB are authoritative** when they diverge from the plan body (e.g. after an API edit).

#### Updates to `cynode_pma.md`

- In [Skills and Default Skill](../tech_specs/cynode_pma.md#spec-cynai-pmagnt-skillsanddefaultskill), state that when the inference backend supports skills, the system MAY supply an additional **plan creation skill** to PMA (project_manager mode) so that plan-building behavior is consistent and spec-aligned.
- Link to the skill definition (once promoted) or to the tech spec that defines the procedure.

#### Updates to `mcp_tool_catalog.md`

- Add MCP tools required for the plan creation skill to retrieve and update plans and tasks from PMA (see [MCP Tool Catalog Additions](#mcp-tool-catalog-additions)).
- Document **help tools** `plan.help`, `task.help`, and `requirement.help` (see [Help tools (plan.help, task.help, requirement.help)](#help-tools-planhelp-taskhelp-requirementhelp)) so the PMA can obtain schema guidance before building payloads.
- Document any new tools (e.g. `db.plan.get`, `db.plan.list`, `db.plan.create`, `db.plan.update`, and task create/update/dependency tools for PM scope) with required/optional args and notes.
- Ensure PM allowlist in [mcp_gateway_enforcement.md](../tech_specs/mcp_gateway_enforcement.md) is updated to include the new plan (and task) tools and the help tools for project_manager.

#### Updates to `skills_storage_and_inference.md` (Optional)

- If the plan creation skill is stored as a **system skill** (like the default CyNodeAI skill), add a short subsection: when the inference request is for PMA in a plan-building context, the system MAY include the plan creation skill in the set of skills supplied to the model.
- Define how "plan-building context" is determined (e.g. handoff type, or always for PMA).

## MCP Tool Catalog Additions

For the plan creation skill to be executable by PMA without direct DB access, the following MCP tools SHOULD be available to the PM agent (project_manager allowlist).

### Help Tools (`plan.help`, `task.help`, `requirement.help`)

The PMA MUST use these tools (when available) to obtain current schema and guidance before building plan, task, or requirement payloads, so that created plans, tasks, and requirements comply with the spec.

**Backend implementation:** The gateway or MCP server that implements each `*.help` tool MUST obtain the **real structures** from the Orchestrator (e.g. schema, field definitions, or constraints derived from the Orchestrator's store or API), and MUST combine that with **additional help info** (e.g. narrative guidance, examples, allowed values).
The response MUST reflect the actual schema the Orchestrator enforces so the agent builds valid payloads; it MUST NOT return only static or out-of-date documentation.

- **`plan.help`** (read-only, no side effects)
  - **Args:** None (or optional `format`: `text` | `json`).
  - **Returns:** Schema and guidance for building a plan payload: required and optional fields (e.g. `plan_name`, `plan_body`, `state`, `comments`), plan state machine (draft, ready, active, suspended, completed, canceled), lock and approval rules, and any host-specific constraints.
    MAY include example JSON or YAML.
  - **Allowlist:** project_manager (PMA).
  - **Purpose:** Agent calls this before creating or updating plans so payloads conform to the project plans schema (e.g. [Project Plans Table](../tech_specs/postgres_schema.md#spec-cynai-schema-projectplanstable)) and related specs.

- **`task.help`** (read-only, no side effects)
  - **Args:** None (or optional `format`: `text` | `json`).
  - **Returns:** Schema and guidance for building a task payload: required and optional fields, task naming rules, `steps` map format (numeric keys, step object with `complete` and `description`), `requirements` array and requirement object shape, and any host-specific constraints.
    MAY include example JSON or YAML.
  - **Allowlist:** project_manager (PMA).
  - **Purpose:** Agent calls this before creating or updating tasks so payloads conform to [Tasks Table](../tech_specs/postgres_schema.md#spec-cynai-schema-taskstable) and related specs.

- **`requirement.help`** (read-only, no side effects)
  - **Args:** None (or optional `format`: `text` | `json`).
  - **Returns:** Schema and guidance for building a requirement object and the `requirements` array: required field `description` (Markdown), optional fields `ref`, `source`, `type`, `priority`, allowed values or semantics, and examples.
    MAY include example JSON or YAML.
  - **Allowlist:** project_manager (PMA).
  - **Purpose:** Agent calls this before populating a task's `requirements` array so each element conforms to [Requirement object structure](../tech_specs/postgres_schema.md#spec-cynai-schema-requirementobject).

If the host does not implement these tools, the agent MUST rely on the canonical schema in tech specs (e.g. postgres_schema.md) or instructions for plan, task, and requirement payloads.

- **Plan read**
  - `db.plan.get` - required args: `plan_id` (uuid); returns plan row (id, project_id, plan_name, plan_body, state, is_plan_locked, plan_approved_at, comments, etc.) if the authenticated context is authorized.
  - `db.plan.list` - required args: `project_id`; optional: `state`, `archived`, `limit`, `cursor`; returns plans for the project that the context is authorized to access.

- **Plan write (when plan not locked)**
  - `db.plan.create` - required args: `project_id`; optional: `plan_name`, `plan_body`; creates plan in `draft` state.
    The agent MUST NOT supply plan `id`; the orchestrator/database assigns it and returns it in the create response.
  - `db.plan.update` - required args: `plan_id`; optional: `plan_name`, `plan_body`, `state`, `comments` (JSON; same structure as task comments per [Comments structure](../tech_specs/postgres_schema.md#spec-cynai-schema-commentsstructure)); MUST fail if `is_plan_locked` is true and the update would change plan_name or plan_body (comments may still be updated when locked).
  - Plan approve MUST be a separate operation or a constrained update (e.g. only state transition to `ready` when explicit user approval has been recorded); tool description MUST state that the agent must obtain explicit user approval before marking the plan approved.

- **Task read**
  - Already: `db.task.get` by `task_id`.
  - Add: `db.task.list` - required args: `plan_id` (or optional `project_id`); optional: `limit`, `cursor`; returns tasks for the plan (or project) that the context is authorized to access.

- **Task write**
  - Task create/update for PMA: the agent MUST NOT supply task `id` (orchestrator/database assigns it).
    Either extend the gateway so that PMA can create/update tasks via MCP (e.g. `db.task.create`, `db.task.update` with required field `steps`: non-empty JSON object/map, keys = numeric step IDs (e.g. 10, 20, 30), values = step objects with `complete` and `description`; when read, sort by numeric key ascending).
    Optional: `requirements` = JSON array per [Requirement object structure](../tech_specs/postgres_schema.md#spec-cynai-schema-requirementobject); `comments` (JSON); other fields per [Tasks Table](../tech_specs/postgres_schema.md#spec-cynai-schema-taskstable).
    Or document that task creation is mediated by the orchestrator when PMA requests it (e.g. via a dedicated "add tasks to plan" tool that accepts task list and dependency graph).
  - Dependency write: `db.task_dependency.create` / `db.task_dependency.delete` or a single `db.plan.set_task_dependencies` that accepts plan_id and a list of (task_id, depends_on_task_id) or (task_name, depends_on_task_name) resolved within the plan.

If full CRUD for plans and tasks via MCP is out of scope for MVP, the integration draft SHOULD document the **minimum** tool set (e.g. plan get/list, plan update body, and a single "sync plan tasks" tool that creates/updates tasks and dependencies from a structured payload) and reference it from the plan creation skill so the skill text can say "use the MCP tools defined in the catalog for plan and task operations."

## Instructions and Skills Integration

- **Instructions bundle (project_manager):**
  Add a dedicated file or section in `agents/instructions/project_manager/` that encodes the plan creation workflow (or includes the skill content by reference).
  The PMA baseline and tools instructions should tell the agent to use this procedure when the user asks for a plan or to refine a plan, and to use only MCP for plan/task data.

- **Skill store (optional):**
  If the system supports multiple system skills, register the plan creation skill (e.g. name `pma-plan-creation` or `cynodeai-plan-creation`) and supply it to PMA when the request is plan-building or always for project_manager mode.
  The skill content can be the markdown from [pma_plan_creation_skill.md](../../default_skills/pma_plan_creation_skill.md) (after promotion and any edits for consistency with tech specs).
- **Project Analyst:**
  The plan creation skill is for PMA only.
    PAA does not create plans; it guides and validates the execution of plans.
    No change to PAA instructions is required for plan creation; PAA instructions should reflect its execution guidance and validation role.

## Acceptance and Promotion Path

1. **Review:** Review [pma_plan_creation_skill.md](../../default_skills/pma_plan_creation_skill.md) and this integration draft with stakeholders; align on MCP tool set (minimal vs full CRUD) and whether the skill is requirements-mandatory (MUST) or advisory (SHOULD).
2. **Requirements:** Add or update requirements in `docs/requirements/` as above; add traceability from requirements to tech specs.
3. **Tech specs:** Update `project_manager_agent.md`, `cynode_pma.md`, `mcp_tool_catalog.md`, and optionally `skills_storage_and_inference.md` and `mcp_gateway_enforcement.md` per sections above.
4. **Skill content:** Move the skill content to its canonical location: either (a) a new tech spec section or addendum that is the "plan creation procedure," or (b) the agent instructions bundle plus (if used) a system skill file in the skill store.
   Remove or archive the draft from `docs/draft_specs/` once the single source of truth is in place.
5. **Validation:** Ensure `just docs-check` and any link/spec validators pass; add or update feature files or E2E coverage for "PMA creates plan via MCP" and "PMA refines plan respecting lock" if not already covered.

## References

- Skill draft: [pma_plan_creation_skill.md](../../default_skills/pma_plan_creation_skill.md).
- Requirements: [pmagnt.md](../requirements/pmagnt.md), [projct.md](../requirements/projct.md), [agents.md](../requirements/agents.md).
- Tech specs: [project_manager_agent.md](../tech_specs/project_manager_agent.md), [cynode_pma.md](../tech_specs/cynode_pma.md), [mcp_tool_catalog.md](../tech_specs/mcp_tool_catalog.md), [mcp_gateway_enforcement.md](../tech_specs/mcp_gateway_enforcement.md), [skills_storage_and_inference.md](../tech_specs/skills_storage_and_inference.md), [projects_and_scopes.md](../tech_specs/projects_and_scopes.md), [postgres_schema.md](../tech_specs/postgres_schema.md).
- Drafts: [2026-03-13_pma_plan_drafting_improvements_proposal.md](../dev_docs/2026-03-13_pma_plan_drafting_improvements_proposal.md) (dependency-only execution, task refs by name, optional YAML frontmatter, runnability/parallelism), [task_routing_pma_first_task_state.md](task_routing_pma_first_task_state.md).
- Spec authoring: [spec_authoring_writing_and_validation.md](../docs_standards/spec_authoring_writing_and_validation.md).
