# Personas and Task Scoping (Draft Proposal)

- [Overview](#overview)
- [Goals](#goals)
- [Existing Behavior (Summary)](#existing-behavior-summary)
- [Task vs Job (Terminology)](#task-vs-job-terminology)
- [Proposed Changes](#proposed-changes)
  - [Persona as Skills Abstraction](#persona-as-skills-abstraction)
  - [Agent Persona Catalog](#agent-persona-catalog)
  - [User-Defined Personas and Copy-On-Edit](#user-defined-personas-and-copy-on-edit)
  - [Single Persona per Task and Task Granularity](#single-persona-per-task-and-task-granularity)
  - [Task-Scoped Persona](#task-scoped-persona)
  - [Orchestrator Model and Node Selection](#orchestrator-model-and-node-selection)
  - [Task Bundle and SBA Execution in Series](#task-bundle-and-sba-execution-in-series)
  - [Recommended Skills for a Task](#recommended-skills-for-a-task)
  - [PMA Use When Creating Tasks](#pma-use-when-creating-tasks)
  - [MCP Persona Tools and Gateways](#mcp-persona-tools-and-gateways)
- [Doc and Schema Impact](#doc-and-schema-impact)
- [Related Draft Specs](#related-draft-specs)
- [Traceability Placeholders](#traceability-placeholders)

## Overview

**Incorporated into canonical specs as of 2026-03-18.**
Single source of truth: [tech_specs/](../tech_specs/) and [requirements/](../requirements/).
See [_draft_specs_incorporation_and_conflicts_report.md](../dev_docs/_draft_specs_incorporation_and_conflicts_report.md) Section 4.3 and 8.

This draft proposes formalizing the **Agent persona catalog** (including role-based personas such as `developer-go` and `test-engineer` alongside the existing PMA and PAA identities), treating **personas as a skills abstraction** (identity plus optional default skills and optional recommended models to load), and supporting **user-defined and user-edited personas** via **copy-on-edit** (editing a system default creates a scoped copy; the system uses that copy instead of the default).
Tasks are scoped to a **single** persona only, with optional recommended skills; work may need to be broken into smaller task chunks so each task has one persona.
**Allowed models** at system, project, and user scope restrict which models may be used; the job is only sent with a model that is in the effective allowed set (intersection of applicable allowlists).
The orchestrator **selects one model** per job from the persona's recommended **cloud** models (by provider, available API keys, best option) or **local** models (available on workers), within the allowed set; when multiple worker nodes exist, the orchestrator **selects the best node** based on workload (assigned jobs, available resources, etc.).
The PMA can hand off a **bundle of 1-3 tasks** to the SBA for execution in series when dependency chains allow, so the same SBA run can execute multiple tasks in sequence.

Status: **Draft** (not accepted; not normative).

## Goals

- Define a **system default Agent persona catalog** (PMA, PAA, developer-go, test-engineer, and similar role personas) so tasks and jobs can be consistently scoped to a role.
- Treat **personas as a skills abstraction**: a persona is a lead-in of the form "You are &lt;persona&gt; and must load these skills: &lt;list&gt;"; identity (title, description) plus an optional default list of skills to load.
- Support **user-defined and user-edited personas**: system defaults are stored in a **non-immutable** way (readable, copyable); if a user edits a persona, a **copy** is created as a **user-, group-, or project-scoped** persona (never system-scoped) and the system uses that copy instead of the default.
- Allow the **orchestrator to update default personas on release**: when a new version of the orchestrator is deployed, the system MUST be able to update system-scoped default personas (e.g. description, default_skill_ids) so that the latest definitions are available for new deployments and users who have not copied.
- Allow **admins to create system-scoped personas**: admins MUST be able to create **system-scoped** personas in addition to the orchestrator-seeded defaults; these do **not** overwrite the defaults and are used when there is no more specifically defined scope (same resolution: user &gt; project &gt; group &gt; system).
- **MCP persona access:** PMA and PAA MUST be able to use MCP tools to **list** personas for selection when assigning or creating tasks; the SBA MUST be able to **get** a persona for the correct scope via MCP when loading per-task context; all such access MUST go through the MCP gateway (and worker proxies when running on worker nodes).
- Enforce **one persona per task**: each task is scoped to at most one persona; break work into smaller defined chunks so that each chunk is one task with one persona (when work spans multiple roles, create multiple tasks).
- Allow **tasks** to reference a **persona** and **recommended skills** so that the PMA can assign the right "who" and "what guidance" at task creation time and the job builder can resolve and pass them to the SBA.
- Support **task bundles**: the same SBA run MAY execute **1-3 tasks in series** when the PMA hands off a bundle and dependency chains allow; the job is **self-contained** (task_ids keyed by numeric order plus embedded full task context) so the SBA runs them in order without calling back to the orchestrator.
- **Persona recommended models:** Store and maintain **two** recommended-model lists per persona: **cloud models** (grouped by provider for selection by available API keys and best option) and **local models** (for worker-node inference); the orchestrator uses these when selecting a model for the job.
- **Allowed models (system / project / user):** The system MUST support **allowed model** allowlists at **system**, **project**, and **user** scope.
  Only models in the **effective allowed set** for the job may be used when the job is sent; the orchestrator MUST NOT dispatch a job with a model outside that set.
  The allowed set is determined **only** by these allowlists; what models are on worker nodes does **not** affect what is allowed (worker model inventory is used for workload placement and availability only).
- **Single model per job:** The orchestrator MUST select **exactly one model** for each job from persona **recommended cloud models** (by provider; only providers with available API keys; best option) or **recommended local models** (available on worker nodes), within the effective allowed set; the SBA receives that one model for the task(s).
- **User preference and API quota:** Model selection (cloud vs local, and within cloud which provider) MUST take into account **user preference** (e.g. prefer cloud vs local, or a provider); for cloud, the orchestrator MUST consider **available API quota** (rate limits, usage caps, remaining quota) and MUST NOT select a cloud option with insufficient quota.
- **Node selection when multiple workers:** When more than one worker node exists, the orchestrator MUST choose the target node based on **workload** (assigned jobs, available resources, allocated jobs, **model size** vs node capacity, etc.) and which nodes can run the chosen allowed model; worker model inventory is used for **placement**, not for defining what is allowed.
- **Review and revise model selection:** For worker-based execution, the orchestrator MUST check worker availability after selecting a model; if capable workers are overloaded, it SHOULD try a smaller model (up to 2 iterations) and re-place; if still no suitable worker, it MUST fall back to the originally selected model and place on the capable worker with the least assigned jobs.
- Keep a single source of truth for persona semantics (this proposal, then promoted to tech_specs/requirements as appropriate); avoid duplicating persona definitions across docs.

## Existing Behavior (Summary)

- **Agent personas** are already defined in [cynode_sba.md - Persona on the Job](../tech_specs/cynode_sba.md#spec-cynai-sbagnt-jobpersona): title, description, stored in DB; embedded inline in the **job** spec at job-build time; SBA uses persona as first context block.
- **Jobs** table has `persona_id` (FK to personas); job payload carries inline `persona: { title, description }` for SBA ([postgres_schema.md - Jobs Table](../tech_specs/postgres_schema.md#spec-cynai-schema-taskstable)).
- **Tasks** table has no `persona_id` and no recommended-skills field; persona is chosen at **job build** time, not at task creation time.
- **PMA and PAA** have dedicated system-scoped Agent personas for their own identity when running ([project_manager_agent.md - Persona Assignment](../tech_specs/project_manager_agent.md#spec-cynai-agents-personaassignment)).
- **Skills** are stored and registered per [skills_storage_and_inference.md](../tech_specs/skills_storage_and_inference.md); job context can include `skill_ids` or `skills` for the SBA ([cynode_sba.md - Context Supplied to SBA](../tech_specs/cynode_sba.md#spec-cynai-sbagnt-jobcontext)).
- **PMA task creation** is guided by [pma_task_creation_skill](../../default_skills/pma_task_creation_skill.md); it does not today specify setting a persona or recommended skills on the task.

## Task vs Job (Terminology)

This section makes the distinction between **task** and **job** explicit so that persona scoping, bundles, and job-building language in this draft are unambiguous.
Existing specs may not state it in one place; when this proposal is promoted, the following definitions should be reflected or reinforced where relevant.

- **Task:** A **durable work item** owned by the orchestrator and stored in the `tasks` table.
  A task is the unit of work that users and the PMA create, assign to a plan, give a persona and optional recommended skills, and order with dependencies.
  It describes *what* to do and *who* does it (one persona per task).
  A task outlives any single execution; it can be run, retried, or reassigned.
  Schema: tasks table (id, description, acceptance_criteria, steps, status, plan_id, persona_id, recommended_skill_ids, etc.).

- **Job:** A **single execution unit** dispatched to a worker, stored in the `jobs` table.
  A job is the runtime instance: "run this work on this node (and in this sandbox)."
  The job payload is what the worker and SBA actually execute (e.g. job spec with persona embedded inline, context, and for bundles task_ids plus **embedded full task context** so the job is self-contained and no SBA-to-orchestrator fetch is needed).
  A job is created when work is dispatched and completes (or fails); the task record is the durable authority.
  Schema: jobs table (task_ids, node_id, status, payload, result, lease, etc.).

- **Relationship:** The **job builder** (orchestrator or PMA) turns one or more **tasks** into a **job spec** (resolve persona, merge skills, supply per-task context).
  One **task** may result in zero or more **jobs** over time (e.g. retries, reassignment).
  One **job** may reference one task (single-task job) or 1-3 tasks in order (bundle job); the SBA runs the job and executes each referenced task in sequence when it is a bundle.
  So: **task** = durable, persona-scoped work definition; **job** = one dispatched run of that work on a worker, carrying the resolved persona and context.

## Proposed Changes

The following sections define the persona model (including skills abstraction), the default catalog, user-defined and copy-on-edit semantics, task-level persona and recommended skills, and PMA behavior.

### Persona as Skills Abstraction

- **Spec ID (draft):** `CYNAI.DRAFT.PersonaSkillsAbstraction`

A **persona** is a reusable combination of identity and default skills: conceptually "You are &lt;persona&gt; and must load these skills: &lt;list&gt;".
It is a **skills abstraction** in the sense that it bundles a role/identity (who the agent is) with an optional list of skills to load when that persona is used.

- **Identity:** `title` (required) and `description` (required), as today: short human-readable label and prose in the form "You are a [role] with [background] and [supporting details]."
- **Default skills:** Optional **`default_skill_ids`** (jsonb, array of skill stable identifiers, nullable).
  When present, the job builder (or SBA context builder) MUST resolve these skill IDs and include them in the context supplied to the SBA when this persona is used, unless overridden or extended by task-level `recommended_skill_ids`.
  Order in the array is significant (first = highest priority).
- **Semantics at job build:** When resolving skills for a job, the system merges the persona's `default_skill_ids` with the task's optional `recommended_skill_ids` (e.g. task skills take precedence or are appended; exact merge rule TBD, e.g. task skills first, then persona defaults, or union with task overriding duplicates).
- **Recommended cloud models:** Optional **`recommended_cloud_models`** (jsonb, nullable): a **map keyed by provider** (e.g. provider stable identifier such as `openai`, `anthropic`).
  Each value is an array of model stable identifiers for that provider.
  The orchestrator uses this to select a **cloud** model for the job: it MUST consider only **providers that have an available API key** (or equivalent credential) configured, then choose the **best option** (e.g. by provider preference order, cost, latency, or deployment policy).
  The system MUST use a **deterministic** "best option" algorithm (e.g. provider preference order, cost, latency, or deployment policy) so that the same persona and available credentials always yield the same chosen provider and model; the exact algorithm is **TBD** and MUST be specified in a tech spec before implementation.
- **Recommended local models:** Optional **`recommended_local_model_ids`** (jsonb, array of model stable identifiers, nullable) for inference on **worker nodes** (local).
  When present, the orchestrator uses this list together with models available on worker nodes to select a local model; order in the array MAY indicate preference (first = highest preference).
  For local selection, the orchestrator MUST choose only a model that **at least one available node** can run **with a good context window size**; it MUST NOT select a model that is too large to run on any node or that would allow only a **very small context window** (e.g. model fits but leaves insufficient headroom for task context).

Schema: add **`default_skill_ids`** (jsonb, nullable), **`recommended_cloud_models`** (jsonb, nullable; map keyed by provider, value = array of model ids), and **`recommended_local_model_ids`** (jsonb, nullable; array of model ids) to the **personas** table; existing persona rows have null for these (no default skills, no recommended models).

### Agent Persona Catalog

- **Spec ID (draft):** `CYNAI.DRAFT.PersonasCatalog`

Define a **system default set** of Agent personas, seeded at bootstrap, that the system and PMA can rely on for task and job assignment.

- **Orchestrator/control-plane personas (identity for the agent itself):**
  - **PMA (Project Manager Agent):** Used by the Project Manager Agent for its own identity when running; system-scoped; required.
  - **PAA (Project Analyst Agent):** Used by the Project Analyst Agent for its own identity when running; system-scoped; required.

- **Execution-role personas (assigned to tasks/jobs for SBA work):**
  - **developer-go:** Go backend developer; emphasis on idiomatic Go, testing, and APIs.
  - **test-engineer:** Test and quality engineer; emphasis on coverage, automation, and acceptance criteria.
  - **code-reviewer:** Code reviewer; emphasis on clarity, security, and maintainability.
  - **security-reviewer:** Security-focused reviewer (already mentioned in cynode_sba as an example).
  - **Backend Developer (generic):** Generic backend developer (already in cynode_sba example).

Personas are stored in the `personas` table with `scope_type` = system and `scope_id` = null for these defaults.
Each has a stable **title** (e.g. `developer-go`, `test-engineer`, `PMA`, `PAA`), a short **description**, and optionally **default_skill_ids**, **recommended_cloud_models** (by provider), and **recommended_local_model_ids**; see Persona as Skills Abstraction and Orchestrator Model and Node Selection.
**System defaults are stored in a user non-immutable way:** they are normal rows in the DB (system scope); they are readable and copyable by users.
They are **not** locked or immutable: the system does not prevent users from creating a **copy** in their own scope and using that instead (see copy-on-edit below).

- **Updating default personas on release:** The system MUST be able to **update** system default (orchestrator-seeded) personas when a new version of the orchestrator is released (e.g. updated description, default_skill_ids, recommended_cloud_models, or recommended_local_model_ids).
  On release or upgrade, the orchestrator MUST apply the latest default persona definitions to the deployment so that new users and contexts receive the current defaults.
  Only **orchestrator-seeded** system persona rows MAY be updated by this process; **admin-created** system personas and all user-, project-, and group-scoped personas MUST NOT be modified.
  The implementation MUST distinguish orchestrator-seeded system personas from admin-created system personas (e.g. by a flag, by a well-known set of titles or ids, or by created_by) so that release updates target only the seeded defaults and never overwrite admin-created system personas.
  The update mechanism (e.g. bootstrap migration, startup seed when orchestrator version changes, or admin-triggered sync) is implementation-defined; updates MUST match seeded system personas by a stable key (e.g. title) and MUST update existing rows (description, default_skill_ids, recommended_cloud_models, recommended_local_model_ids) or insert new defaults that were added in the release.
  Users who have already created a copy of a default retain their copy; resolution continues to prefer user/project/group scope over system, so only contexts that still resolve to the system default see the updated content.

- **Admin-created system-scoped personas:** Admins MUST be able to **create** system-scoped personas (scope_type = system, scope_id = null) in addition to the orchestrator-seeded defaults.
  Admin-created system personas do **not** overwrite the defaults; they are **additive** (new rows in the personas table).
  They participate in the same resolution as defaults: when there is no user-, project-, or group-scoped match, **system** scope is used, and the resolver MAY return either an orchestrator-seeded default or an admin-created system persona (e.g. by title or id).
  The implementation MUST ensure that release updates (see above) never modify or delete admin-created system personas; only orchestrator-seeded system personas are updated on release.

### User-Defined Personas and Copy-On-Edit

- **Spec ID (draft):** `CYNAI.DRAFT.PersonaCopyOnEdit`

- **User-defined personas:** Users (and projects/groups, per RBAC) MAY create new personas in their scope (user, project, or group) with title, description, and optional default_skill_ids.
  Full CRUD for personas in allowed scopes is already required (data_rest_api, CLI personas management); this proposal extends the persona model with default_skill_ids and clarifies edit semantics.

- **Editing a persona:** If the user wants to **edit** a persona (including a system default), the system MUST **not** modify the original in place when the original is system-scoped (or when the user is not the owner of the persona).
  Instead, the system MUST create a **copy** of the persona that is **user-, group-, or project-scoped** (e.g. scope_type = user with scope_id = user_id, or project/group as appropriate; never system-scoped) with the user's edits applied (title, description, default_skill_ids).
  The original persona (e.g. the system default) remains unchanged.

- **Resolution and use:** When resolving which persona to use (e.g. by persona_id, or by title for a given context), the existing **scope precedence** applies: user over project over group over system (per [project_manager_agent.md - Persona assignment](../tech_specs/project_manager_agent.md#spec-cynai-agents-personaassignment)).
  So when a task or job references a persona by id, that id already points to a specific row (which may be the user's copy).
  When resolving by title (e.g. "developer-go"), the resolver returns the **most specific** matching persona: if the user has a user-scoped persona with title "developer-go", that one is used instead of the system-scoped "developer-go".
  Thus, once a user has created a copy (by "editing" a default), the system uses that edited persona instead of the default for that user (or project) going forward.

- **UI/API:** "Edit persona" on a system (or non-owned) persona MUST be implemented as "Create a copy in my scope with the same title (or user-chosen title) and the edits I provide."
  The API MAY expose this as a single "edit" action that returns the new persona_id when a copy was created, so clients can update references (e.g. task persona_id) to the new id if desired.

### Single Persona per Task and Task Granularity

- **Spec ID (draft):** `CYNAI.DRAFT.SinglePersonaPerTask`

- Each **task** is scoped to **at most one** persona: the task has a single optional **`persona_id`** (uuid, FK to `personas.id`).
  The system MUST NOT associate a task with more than one persona; schema and API MUST enforce one persona per task.

- **Task granularity:** Work SHOULD be broken into **smaller defined chunks** so that each chunk is one task with one persona.
  When a unit of work spans multiple roles or would benefit from different personas (e.g. implement then test then review), the PMA SHOULD create **multiple tasks**, each with its own persona_id, and use task dependencies and (when applicable) task bundles so the SBA can run compatible tasks in series (see [Task Bundle and SBA Execution in Series](#task-bundle-and-sba-execution-in-series)).

- **Execution-ready tasks:** When a task is selected for execution (solo or as part of a bundle), it SHOULD have a non-null persona_id so the job has a well-defined persona; the job builder MAY fall back to a default persona when persona_id is null for backward compatibility.
  The job builder MUST only build or dispatch jobs for tasks that satisfy **execution gating** (e.g. only tasks eligible to run; when [task_routing_pma_first_task_state.md](task_routing_pma_first_task_state.md) is adopted, only tasks with `planning_state=ready` are eligible).

### Task-Scoped Persona

- **Spec ID (draft):** `CYNAI.DRAFT.TaskScopedPersona`

- Add optional **`persona_id`** (uuid, FK to `personas.id`) to the **tasks** table.
- When present, the **job builder** (orchestrator or PMA when constructing a job for this task or task bundle) MUST resolve the persona by id (or by task's persona_id), embed `title` and `description` inline in the job spec per [cynode_sba.md - Persona on the Job](../tech_specs/cynode_sba.md#spec-cynai-sbagnt-jobpersona), and set the job's `persona_id` from the task's (or bundle's) persona.
- When `persona_id` is null on the task, the job builder MAY choose a default persona (e.g. generic Backend Developer) or leave the job without a persona; existing behavior (persona on job only) remains valid for jobs created without a task-level persona.

This makes the **task** the place where "which persona runs this work" is decided; the job carries that decision through to the SBA.

### Orchestrator Model and Node Selection

- **Spec ID (draft):** `CYNAI.DRAFT.OrchestratorModelAndNodeSelection`

- **Allowed models (system, project, user):** The system MUST maintain **allowed model** allowlists at three scopes: **system** (deployment-wide), **project** (per project, when the task or job is scoped to a project), and **user** (per user, e.g. the task owner or the identity requesting the job).
  Each scope may define a list of model stable identifiers (or equivalent) that are **allowed** for inference; null or absent list at a scope means **no restriction** at that scope.
  The **effective allowed set** for a job is the **intersection** of the applicable lists: system allowed, project allowed (if the task has a project_id), and user allowed (for the relevant user).
  **Worker node model inventory is not part of the allowed set.**
  What models are on which workers is used only for workload placement optimization and for determining which allowed model is available on a given node; it does not define or restrict what is allowed.
  The orchestrator MUST only select a model that is in the effective allowed set; the job MUST NOT be sent with a model outside that set.
  Storage: system allowed models may be config or a system-level table; project allowed models may be a column on the projects table (e.g. `allowed_model_ids` jsonb, nullable); user allowed models may be user preferences or a user-scoped table (e.g. `allowed_model_ids`).

- **Single model per job:** When building a job, the **orchestrator** MUST resolve the job's persona (from the task or bundle) and select **exactly one model** for that job.
  Selection is from the intersection of (1) **effective allowed models** (system/project/user above), (2) persona recommended models (**cloud** or **local**, see below), and (3) models **available** (on target worker for local, or via external APIs with valid credentials for cloud).
  **Cloud path:** When using the persona's **recommended_cloud_models**, the orchestrator MUST consider only **providers that have an available API key** (or equivalent) configured; it MUST also consider **available API quota** (e.g. rate limits, usage caps, remaining quota per provider or per key) and MUST NOT select a cloud model/provider that has insufficient quota for the job.
  Among providers with keys and sufficient quota, it MUST select the **best option** (e.g. by provider preference order, cost, latency, or deployment policy; implementation-defined).
  Cloud models are grouped by provider in the persona so the orchestrator can match provider to configured keys and then choose one model from the best provider's list (subject to allowed set and model size).
  **Local path:** When using the persona's **recommended_local_model_ids**, the orchestrator selects from models available on worker node(s), subject to allowed set and model size vs node capacity.
  For local, the orchestrator MUST select only a model that **at least one available node** can run **with a good context window size**; it MUST NOT select a model that is too large to run on any node or that would allow only a **very small context window** (e.g. model fits but leaves insufficient headroom for task context).
  **User preference and cloud vs local:** Model selection (cloud vs local, and within cloud which provider/model) MUST take into account **user preference** (e.g. user-scoped preference for cloud vs local, or for a provider); where stored (user preferences, project default) is implementation-defined.
  The orchestrator MAY prefer cloud over local or vice versa (or try one then fall back) in line with user preference and constraints; the selected model MUST be from allowed set and from either persona cloud recommendations (with available API key and sufficient quota) or persona local recommendations (with node availability and sufficient context window).
  **Model size** MUST be a factor in selection when multiple models satisfy the above; for local, the model must fit target node capacity and yield an acceptable context window.
  When no model satisfies (allowed and (cloud with key and sufficient quota or local-available)), the job MUST NOT be dispatched until configuration or availability changes; the orchestrator MAY report a defined error.
  The **job payload** (consumed by the SBA) MUST include that **one resolved model** (e.g. a single model identifier or endpoint) so the SBA uses only that model for the task(s) for that persona.

- **Node selection when multiple workers:** When the deployment has **more than one** worker node, the orchestrator MUST select the **target node** for the job based on **workload and resources** (and, for optimization, which nodes have the selected model available).
  The orchestrator MUST consider at least: workload currently assigned to each node (e.g. number or aggregate size of jobs already running or queued), available resources (e.g. capacity, memory, GPU), **model size** (e.g. memory or GPU footprint of the chosen model so the node can fit it), and allocated jobs per node.
  **Model size** MUST be a factor in placement: the target node MUST have sufficient capacity for the selected model's size (e.g. GPU memory, RAM); when choosing among nodes, the orchestrator MUST use model size together with node capacity and current workload to place the job.
  Which models are on which nodes is used for **workload placement optimization** (e.g. route to a node that can run the chosen allowed model), not for defining the allowed set.
  The exact algorithm (e.g. least-loaded, round-robin, capability match) is implementation-defined; the requirement is that the orchestrator chooses the best node for the job given available information rather than dispatching arbitrarily.

- **Review and revise model selection based on worker workload:** After selecting an initial model, the orchestrator MUST **check worker availability** for that model.
  If the selected model would need to run on workers that are **overloaded** (e.g. already have multiple jobs assigned), the orchestrator SHOULD **revise** the selection: choose a **smaller model** (if possible) from the allowed or preferred list and **re-place** the job (attempt to find a suitable worker for the smaller model).
  The orchestrator MAY perform up to **2 iterations** of this reselection (select smaller model, attempt to place; if no suitable worker, try again with another smaller model).
  If after 2 iterations of reselecting the orchestrator **cannot find a suitable worker**, it MUST **fall back** to the **originally selected model** and place the job on a **capable worker that has the least assigned jobs** (best effort among overloaded workers).
  This provision applies when the job is to run on workers (e.g. local models); cloud model selection is not revised by worker workload (quota and user preference already apply).

- **Order of operations:** The orchestrator (or job builder) resolves persona, resolves the **effective allowed model set** (system, project, user), applies **user preference** (cloud vs local, provider) and checks **available API quota** for cloud.
  It then resolves one model from persona **recommended_cloud_models** (by provider, keys, quota, best option) or **recommended_local_model_ids** (by node availability and model size).
  For worker-based (local) execution, it **reviews** worker availability; if capable workers are overloaded, it may **revise** model selection (up to 2 iterations: smaller model, re-place) and then **fall back** to the original model on the least-loaded capable worker if still no suitable worker.
  It selects the target node, then builds and dispatches the job with the single model and embedded context.
  The job is only sent if the selected model is in the effective allowed set.

### Task Bundle and SBA Execution in Series

- **Spec ID (draft):** `CYNAI.DRAFT.TaskBundleSeries`

- **Same SBA, multiple tasks in sequence:** When dependency chains allow, the **same SBA** (same sandbox run / job) MAY execute **multiple tasks in series**.
  The PMA MAY hand off a **bundle of 1-3 tasks** to the SBA for execution in a single job; the SBA runs the tasks in **explicit order** and reports completion (and optionally per-task results) per job lifecycle.

- **Explicit task order in the bundle:** A task bundle is an **ordered sequence**, not a set.
  Execution order MUST be deterministic.
  JSON arrays and Go slices preserve element order, but to align with the **task steps** pattern (see [postgres_schema.md - Tasks table steps](../tech_specs/postgres_schema.md)) and to avoid reliance on map/object key iteration order in implementations, the job spec MUST represent task order using a **map keyed by numeric order** (not a raw array).
  When read, consumers MUST sort by numeric key ascending to obtain execution order.

- **Task reference shape (same pattern as task steps):** The job spec (and jobs table or payload) MUST carry **`task_ids`** as a **map/object** keyed by **numeric order** (integer, stored as JSON number or string that parses to integer).
  Keys define order: when read, **task_ids MUST be sorted by numeric key ascending** to obtain deterministic execution order.
  Use increments of 10 (e.g. 10, 20, 30) so order is explicit and insertable.
  For 1-3 tasks, keys 10, 20, 30.
  Each value = task uuid (string).
  Example: `"task_ids": { "10": "uuid-first", "20": "uuid-second", "30": "uuid-third" }`.
  Single-task job = one key (e.g. "10"); bundle = two or three keys.
  The SBA MUST execute tasks in the order given by sorting keys ascending and MUST NOT reorder or skip based on task_id alone.

- **Bundle constraints:**
  - **Size:** A bundle contains **1 to 3** task entries in the task_ids map.
  - **Single persona:** All tasks in a bundle MUST share the **same** persona_id so the job has one persona for the entire run.
  - **Dependencies:** Tasks in a bundle SHOULD respect dependency order (e.g. task B can be in the same bundle as task A only if A appears before B in the sorted task_ids order and A's dependencies are satisfied); the PMA is responsible for forming valid bundles.

- **Job bundle is self-contained:** The job payload for a bundle MUST **embed all task information** needed to run each task in the bundle.
  The SBA MUST NOT call back to the orchestrator (e.g. via MCP) to fetch task details; the job is **self-contained** so it can be sent once and executed without further round-trips.
  The payload MUST remain **relatively small** (essential task fields only; project-level or baseline context MAY be shared once at top level) so it is easy to send.

- **SBA per-task context when running a bundle:** For each task in the bundle, the SBA **loads and applies that task's context** from the job payload before executing that task.
  Persona is the same for all tasks in the bundle; task-level context and per-task recommended skills MAY differ.
  The job spec MUST supply **full per-task context** in the bundle (see Job spec definition below); there is no option for the SBA to obtain task context via MCP when advancing to the next task.
  The SBA MUST NOT use the previous task's description, acceptance criteria, requirements, steps, or task-specific skills when executing the next task; it MUST switch to the next task's context from the embedded payload.

- **Job shape:** The job spec (or job record) MUST carry **`task_ids`** (map keyed by numeric order) and for **bundles** MUST also carry **embedded task context** (context.task_contexts or equivalent) so the job is self-contained.
  Single-task = one entry in task_ids (e.g. key 10); bundle = two or three entries with full per-task context embedded.
  The SBA sorts keys ascending, then executes each task in that order: for each key, load context from the embedded payload, perform work, record result, then advance to the next key.

- **Schema/API impact:** Jobs table (or job payload) uses **`task_ids`** (jsonb, map/object keyed by numeric order, value = task uuid string).
  Single-task job = one key; bundle = two or three keys.
  Consumers MUST sort by numeric key ascending to obtain execution order (same pattern as task steps in [postgres_schema.md](../tech_specs/postgres_schema.md)).

- **Job spec definition for task bundles:** The job payload (JSON) consumed by the SBA MUST support the following when the job is a task bundle.
  Existing job spec (e.g. [cynode_sba.md - Job Specification](../tech_specs/cynode_sba.md#spec-cynai-sbagnt-schemavalidation)) defines minimum fields: `protocol_version`, `job_id`, `task_id`, `constraints`.
    The canonical job spec (cynode_sba.md) MUST be updated to use **`task_ids`** only, as follows.

  - **Task reference:** **`task_ids`** (required): **map/object** keyed by numeric order (e.g. 10, 20, 30); each value = task uuid (string).
    Execution order = sort keys ascending (10, then 20, then 30).
    Single-task job = one key (e.g. "10"); bundle = two or three keys.
    Same pattern as task **steps** ([postgres_schema.md](../tech_specs/postgres_schema.md)): keys define order; when read, sort by numeric key ascending.

  - **Persona:** Unchanged for bundles: one top-level **`persona`** object (title, description; optional persona_id for auditing).
    All tasks in the bundle share this persona; the job builder sets it from the (common) task persona_id.
  - **Model:** The job spec MUST include **exactly one** resolved model (e.g. `model_id` or `inference.model`) chosen by the orchestrator from the persona's recommended_cloud_models (by provider, available API keys, best option) or recommended_local_model_ids (available on target node).
    The SBA uses this single model for all task(s) in the job; the orchestrator MUST set it at job build time per [Orchestrator Model and Node Selection](#orchestrator-model-and-node-selection).

  - **Per-task context for bundles (required, self-contained):** When the job has **`task_ids`** with more than one key (bundle), the job spec MUST **embed full task information** for every task in the bundle.
    Top-level **`context.task_contexts`** (or equivalent): a **map keyed by the same numeric keys** as task_ids (10, 20, 30).
    Each value = **full context object** for that task: at least task identity (id, name), description, acceptance_criteria, requirements, steps, and skill_ids (or resolved skills).
    Project-level and baseline context MAY be shared (single copy at top level) to keep the payload relatively small.
    The SBA sorts task_ids keys ascending and uses `context.task_contexts[key]` when executing the task at `task_ids[key]`.
    The bundle MUST NOT require the SBA to fetch task details from the orchestrator; all information needed to run each task is in the job payload.

  - **Single-task job:** When **`task_ids`** has a single key (one entry), the existing **`context`** shape applies: single `task_context`, `requirements`, `acceptance_criteria`, `skill_ids`/`skills`, etc. No `task_contexts` map is required.

  - **Result reporting:** The job result (e.g. result contract in cynode_sba.md / worker_api.md) SHOULD allow per-task results when the job is a bundle (e.g. an array of { task_id, status, result_snippet } or similar) so the orchestrator can update each task's status and store task-level outcomes.
    Exact shape is for the canonical result spec; this draft notes that bundle jobs require a way to report which task(s) completed and with what outcome.
  - **User-directed job kill:** When a user (or PMA) directs that a job be killed, the **entire job** is stopped; for a bundle job, all tasks in that job are stopped together.
    See [user_directed_job_kill_proposal.md](user_directed_job_kill_proposal.md) for stop semantics and orchestrator/worker behavior.

### Recommended Skills for a Task

- **Spec ID (draft):** `CYNAI.DRAFT.TaskRecommendedSkills`

- Add a way to associate **recommended skill identifiers** with a task so the job builder (or SBA context) can include those skills when running the task.

**Option A (preferred for MVP):** Add **`recommended_skill_ids`** (jsonb, nullable) to the **tasks** table: array of skill stable identifiers (strings).
Order is significant (first = highest priority).
Empty array or null means no recommended skills.

**Option B:** New junction table **`task_recommended_skills`** (`task_id`, `skill_id`, `sort_order`) for normalized many-to-many with explicit ordering.

- When building a job from a task, the job builder MUST resolve skills from (1) the task's `recommended_skill_ids` (if any) and (2) the resolved persona's `default_skill_ids` (if any), merge them (e.g. task skills first then persona defaults, or union; exact rule TBD), resolve by stable id, and include them in the job context per [cynode_sba.md - Context Supplied to SBA](../tech_specs/cynode_sba.md#spec-cynai-sbagnt-jobcontext).
- Skills that are not found or not visible to the caller are skipped; the job is still built with the subset that resolved.

### PMA Use When Creating Tasks

- **Spec ID (draft):** `CYNAI.DRAFT.PmaTaskPersonaAndSkills`

- When the PMA creates or updates a task (via MCP task create/update), it SHOULD set **`persona_id`** when the work is best performed by a specific execution-role persona (e.g. developer-go for Go implementation tasks, test-engineer for test or QA tasks).
  Each task has at most one persona; when work spans multiple roles, the PMA SHOULD create multiple tasks (smaller chunks), each with its own persona.
- The PMA SHOULD set **`recommended_skill_ids`** when specific skills (e.g. project skill for Go standards, or a testing skill) would help the SBA perform the task.
- The PMA MUST use MCP or gateway to resolve persona by id (or list/get personas) and skills by id (skills.list, skills.get) so that only valid, visible identifiers are written.
- **Task bundle handoff:** When dispatching work to the SBA, the PMA MAY hand off a **bundle of 1-3 tasks** (same persona, dependency order) so the SBA runs them in series in one job; the PMA is responsible for forming valid bundles and for setting the job's task_ids (map keyed by numeric order).
- Task creation and update payloads (and MCP task tools) MUST accept optional `persona_id` and `recommended_skill_ids` once schema and API are extended.

Promotion of this behavior into the canonical **pma_task_creation_skill** and **project_manager_agent** specs would follow acceptance of this draft.

### MCP Persona Tools and Gateways

- **Spec ID (draft):** `CYNAI.DRAFT.McpPersonaToolsGateways`

- **PMA and PAA:** The PMA and PAA MUST be able to use MCP tools to **list** personas for selection when assigning or creating tasks (e.g. `persona.list` with optional scope filter) and to **get** a persona by id (e.g. `persona.get`) so they can resolve and embed the chosen persona when building tasks and job specs.
  When running on a worker node, they MUST go through the **worker proxy** to reach the orchestrator MCP gateway.
  The gateway MUST allow persona.list and persona.get on the PM and PA agent allowlists (per [mcp_tool_catalog.md - Persona Tools](../tech_specs/mcp_tool_catalog.md#spec-cynai-mcptoo-personatools) and [mcp_gateway_enforcement.md](../tech_specs/mcp_gateway_enforcement.md)).

- **SBA:** The SBA MUST be able to **get** a persona for the correct scope via MCP (e.g. `persona.get` by persona_id) when needed (e.g. when validating or resolving job context).
  For bundles, per-task context is embedded in the job payload; the SBA does not fetch task details from the orchestrator when advancing to the next task.
  SBA runs on workers, so SBA persona access MUST go through the **worker proxy** (worker proxy to orchestrator MCP gateway); the gateway MUST allow persona.get (and optionally persona.list) on the **Worker Agent (sandbox)** allowlist so the SBA can fetch persona for the task's scope.

- **All agents on workers use worker proxy:** All agents running on worker nodes (PMA, PAA, SBA, and any other worker-hosted agent) MUST use the **worker proxy** to reach the orchestrator MCP gateway; they MUST NOT call the orchestrator gateway directly.

- **Summary:** Persona MCP access (list, get) is mediated by the MCP gateway; when agents run on workers they use the worker proxy to reach it.
  The implementation MUST ensure `persona.list` and `persona.get` are on the appropriate allowlists and that the SBA receives task-scoped or job-scoped context so it can request the persona for the correct scope (e.g. by persona_id from the job or task context).

## Doc and Schema Impact

When this proposal is accepted, the following updates are recommended:

- **Area:** Task vs job terminology (existing specs)
  - document: orchestrator.md, cynode_sba.md, postgres_schema.md, langgraph_mvp.md, worker_api.md, mcp_tool_catalog.md (sandbox/job tools)
  - change: Ensure the **task** (durable work item, tasks table, one persona, plan/dependencies) vs **job** (execution unit dispatched to a worker, jobs table, job payload consumed by SBA) distinction is explicit where these concepts are introduced or used.
    Add a short "Task vs job" subsection or reference in the spec index (_main.md) if no single canonical definition exists.
    In orchestrator.md: clarify that tasks are created and managed; jobs are created when dispatching to a node and carry the resolved task context.
    In cynode_sba.md: clarify that the SBA receives a **job** spec (built from one or more **tasks**).
    In postgres_schema.md: in the Tasks and Jobs table sections, state that tasks are durable work items and jobs are execution instances.
    Existing specs need not be rewritten; add definitions or cross-references where the distinction is critical (e.g. job builder, bundle, persona resolution).

- **Area:** Schema
  - document: postgres_schema.md
  - change: Personas table: add `default_skill_ids` (jsonb, nullable); add `recommended_cloud_models` (jsonb, nullable), map keyed by provider id, value = array of model stable ids; add `recommended_local_model_ids` (jsonb, nullable), array of model stable identifiers.
    Tasks table: add `persona_id` (uuid, FK personas.id, nullable); add `recommended_skill_ids` (jsonb, nullable) or add task_recommended_skills table; enforce one persona per task.
    Jobs table (or job payload): use **task_ids** (jsonb, map keyed by numeric order 10/20/30, value = task uuid); one key = single-task job, two or three keys = bundle (tasks run in series; sort keys ascending for order).
  - change: **Allowed models:** Define storage for system-, project-, and user-scoped allowed model allowlists (e.g. system config or table; projects table add `allowed_model_ids` (jsonb, nullable); user preferences or users table add `allowed_model_ids` (jsonb, nullable)); null/absent means no restriction at that scope; effective allowed set = intersection of applicable lists only (worker node model inventory does not define allowed set; used for placement and availability only).
- **Area:** API
  - document: data_rest_api.md
  - change: Persona resource: accept and return optional `default_skill_ids`, `recommended_cloud_models` (map by provider), and `recommended_local_model_ids`.
    Define "edit" semantics: when editing a system-scoped or non-owned persona, create a copy that is user-, group-, or project-scoped (never system-scoped) and return the new persona_id.
    Task resource: accept and return optional `persona_id`, `recommended_skill_ids` in create/update/get.
  - change: Project resource: accept and return optional `allowed_model_ids` (array of model stable IDs; null = no restriction).
    User preferences (or user resource): accept and return optional `allowed_model_ids` for user-scoped allowed models; system allowed models via admin or config API as defined.
- **Area:** Job build and bundles
  - document: cynode_sba.md / project_manager_agent.md / orchestrator.md
  - change: State that job builder uses task.persona_id (resolve persona, embed title/description and persona.default_skill_ids) and task.recommended_skill_ids; merge persona default skills with task recommended skills when building job context.
  - change: **Allowed models:** Orchestrator MUST resolve effective allowed model set (intersection of system, project, user allowlists); job MUST NOT be sent with a model outside that set.
  - change: **Model selection:** Orchestrator MUST select exactly one model per job: use persona **recommended_cloud_models** (by provider; only providers with available API keys and **sufficient API quota**; select best option) or **recommended_local_model_ids** (models available on worker nodes); **user preference** (cloud vs local, provider) and **API quota** MUST be taken into account.
  - change: For **local**, only select a model that at least one node can run **with a good context window size**; selection must be in effective allowed set; **model size** MUST be a factor; job spec MUST carry that single model; do not dispatch if no allowed model is available (cloud with key and quota or local on node with sufficient context window).
  - change: **Node selection:** When multiple worker nodes exist, orchestrator MUST select target node based on workload (assigned jobs, available resources, allocated jobs, **model size** vs node capacity, etc.); target node MUST have capacity for the selected model's size; worker model inventory used for placement optimization only, not for defining allowed models; algorithm implementation-defined.
  - change: **Review and revise model selection (worker workload):** For worker-based (local) execution, orchestrator MUST check worker availability after initial model selection; if capable workers are overloaded, SHOULD revise selection (smaller model from allowed/preferred list, re-place) up to 2 iterations; if after 2 iterations no suitable worker, MUST fall back to originally selected model and place on capable worker with least assigned jobs.
  - change: Support job as bundle of 1-3 task_ids in **explicit order** (map keyed by numeric order 10, 20, 30; sort keys ascending = execution order); SBA runs tasks in series in that order; all tasks in bundle share same persona.
  - change: Job bundle MUST be **self-contained**: job spec MUST embed full per-task context (map keyed by same numeric keys as task_ids); SBA MUST NOT call back to orchestrator for task details.
  - change: Payload kept relatively small (essential task fields; project/baseline context MAY be shared once).
  - change: Jobs table or payload: use **task_ids** (map keyed by numeric order, value = task uuid); one key = single-task, two or three keys = bundle.
- **Area:** Job spec for task bundles (cynode_sba.md Job Specification)
  - document: cynode_sba.md (Job Specification section)
  - change: **Job spec model:** Job payload MUST include exactly one resolved model set by orchestrator (from persona recommended_cloud_models with available API key or recommended_local_model_ids on node); model MUST be in effective allowed set; SBA uses that single model for the task(s).
  - change: **Define the job spec for task bundles:** Use **task_ids** (required, map keyed by numeric order 10, 20, 30; value = task uuid); execution order = sort keys ascending; one key = single-task, two or three keys = bundle.
    Same pattern as task steps ([postgres_schema.md](../tech_specs/postgres_schema.md)).
    Update minimum required fields to **task_ids** (replace task_id).
  - change: Define **context.task_contexts** (or equivalent) for bundles: **required** map keyed by same numeric keys as task_ids, value = **full** per-task context object (id, name, description, acceptance_criteria, requirements, steps, skill_ids); bundle is self-contained, no SBA-to-orchestrator fetch for task info.
  - change: Retain single **context** shape for single-task jobs (task_ids with one key).
    Define or extend the **result contract** for bundle jobs so the SBA can report per-task outcomes (e.g. array of task_id + status + result) for orchestrator to update each task.
  - change: When user-directed job kill is adopted (see [user_directed_job_kill_proposal.md](user_directed_job_kill_proposal.md)), bundle jobs: stop request applies to the whole job (all tasks in the bundle).
- **Area:** PMA skill
  - document: default_skills/pma_task_creation_skill.md
  - change: Add steps: one persona per task; break work into smaller chunks when multiple roles; set persona_id when appropriate; set recommended_skill_ids; call persona/skills MCP or gateway to resolve.
  - change: When dispatching, may form bundles of 1-3 tasks (same persona, dependency order) for SBA execution in series.
- **Area:** Persona catalog and CRUD
  - document: New or existing tech spec
  - change: Promote persona catalog (PMA, PAA, developer-go, test-engineer, etc.) and bootstrap seeding; document that system defaults are non-immutable (copy-on-edit); document user-defined personas and copy-on-edit semantics.
  - change: Document that the orchestrator MUST be able to update system default personas on release (only orchestrator-seeded system rows; never admin-created system personas; match by stable key; update or insert); mechanism (bootstrap/startup/version-triggered) implementation-defined.
  - change: Document that admins MUST be able to create system-scoped personas (additive; do not overwrite defaults); admin-created system personas are used when there is no more specifically defined scope; implementation must distinguish seeded vs admin-created so release updates do not touch admin-created.
- **Area:** MCP persona tools and gateways
  - document: mcp_tool_catalog.md / mcp_gateway_enforcement.md
  - change: State that PMA and PAA MUST be able to use persona.list and persona.get for listing personas for selection when assigning/creating tasks and for resolving persona by id; when they run on a worker they MUST go through the worker proxy.
  - change: State that all agents running on worker nodes (including PMA/PAA when co-located with a worker, SBA) MUST use the worker proxy to reach the orchestrator MCP gateway.
  - change: State that SBA MUST be able to get a persona for the correct scope via MCP through the worker gateway; add persona.get (and optionally persona.list) to the Worker Agent (sandbox) allowlist so SBA can fetch persona when loading per-task context.
- **Area:** Requirements
  - document: docs/requirements (e.g. AGENTS, PROJCT, SCHEMA, DATAPI, ORCHES, MCPTOO, MCPGAT)
  - change: Add REQ-* for: task-scoped persona (one per task); task recommended skills; persona default_skill_ids, recommended_cloud_models (by provider), and recommended_local_model_ids; persona copy-on-edit; orchestrator update of system default personas on release (orchestrator-seeded only); admin create system-scoped personas (additive).
  - change: Add REQ-* for: system/project/user allowed model allowlists; job only sent with model in effective allowed set; orchestrator select one model per job from persona cloud (by provider, available API keys, **available API quota**, best option) or local recommendations (local: at least one node able to run with good context window size); **user preference** (cloud vs local, provider) and **API quota** MUST be taken into account; in allowed set, considering model size.
  - change: Add REQ-* for: orchestrator select target node by workload and model size vs node capacity when multiple workers; orchestrator review and revise model selection based on worker workload (check availability; if overloaded, up to 2 iterations smaller model and re-place; then fall back to original model on least-loaded capable worker).
  - change: Add REQ-* for: PMA/PAA MCP persona list/get; SBA persona get via worker gateway; task bundle (1-3 tasks in series, same persona); SBA execution of task bundle in one job.

## Related Draft Specs

The following draft specs in `docs/draft_specs/` are related; this proposal aligns with or defers to them as follows.

- **[task_routing_pma_first_task_state.md](task_routing_pma_first_task_state.md):** Introduces task `planning_state` (draft/ready) and PMA-first review; only tasks in `ready` are eligible for execution.
  This proposal assumes the job builder dispatches only for tasks that meet execution gates; when that draft is adopted, only `planning_state=ready` tasks are eligible for job build/dispatch.
- **[pma_plan_creation_skill_spec_integration.md](pma_plan_creation_skill_spec_integration.md):** Plan and task creation via MCP (plan.help, task.help, db.plan.*, task steps as map keyed by numeric IDs).
  This proposal's task steps and task_ids map pattern are consistent; persona_id and recommended_skill_ids on tasks are complementary to the plan-creation skill; persona.list/persona.get for PMA support task assignment.
- **[user_directed_job_kill_proposal.md](user_directed_job_kill_proposal.md):** User- or PMA-directed job kill; orchestrator sends stop to worker; SBA graceful stop then container kill.
  For bundle jobs, killing the job stops the entire job (all tasks in the bundle); one job_id, one stop request.
- **[worker_node_agent_draft_spec.md](worker_node_agent_draft_spec.md):** WNA (host-level agent) uses the same job spec and result contract as container SBA.
  This proposal's job shape (task_ids map, embedded task context for bundles) applies to both WNA and container SBA.
- **[orchestrator_specifications_table.md](orchestrator_specifications_table.md):** Specifications table and plan/task reference join tables.
  Tasks may reference specifications; persona and recommended_skill_ids on tasks are independent of specification references; no conflict.

## Traceability Placeholders

- Requirements: To be added in docs/requirements (AGENTS, PROJCT, and/or SCHEMA) after acceptance.
- Existing specs: [cynode_sba.md](../tech_specs/cynode_sba.md), [project_manager_agent.md](../tech_specs/project_manager_agent.md), [postgres_schema.md](../tech_specs/postgres_schema.md), [skills_storage_and_inference.md](../tech_specs/skills_storage_and_inference.md), [mcp_tool_catalog.md](../tech_specs/mcp_tool_catalog.md), [mcp_gateway_enforcement.md](../tech_specs/mcp_gateway_enforcement.md), [pma_task_creation_skill](../../default_skills/pma_task_creation_skill.md).
