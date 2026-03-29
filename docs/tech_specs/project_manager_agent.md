# Project Manager Agent

- [Agent Purpose](#agent-purpose)
- [LLM and Tool Execution (Implementation)](#llm-and-tool-execution-implementation)
- [No Simulated or Guessed Output](#no-simulated-or-guessed-output)
  - [Traces to Requirements](#traces-to-requirements)
- [Agent Responsibilities](#agent-responsibilities)
  - [MVP inference assignment responsibility](#mvp-inference-assignment-responsibility)
  - [Project Manager model capability requirements (MVP)](#project-manager-model-capability-requirements-mvp)
  - [Thread Titling](#thread-titling)
  - [Default Model Line (MVP)](#default-model-line-mvp)
- [External Provider Usage](#external-provider-usage)
  - [External Provider Usage Applicable Requirements](#external-provider-usage-applicable-requirements)
- [External Provider Configuration](#external-provider-configuration)
  - [Standalone Orchestrator Scenario](#standalone-orchestrator-scenario)
  - [Required Configuration Steps](#required-configuration-steps)
  - [Bootstrap and Runtime Configuration](#bootstrap-and-runtime-configuration)
- [Tool Access and Database Access](#tool-access-and-database-access)
  - [Tool Access and Database Access Applicable Requirements](#tool-access-and-database-access-applicable-requirements)
- [Inputs and Outputs](#inputs-and-outputs)
  - [Agent Inputs](#agent-inputs)
  - [Agent Outputs](#agent-outputs)
- [LLM Context (Baseline and User-Configurable)](#llm-context-baseline-and-user-configurable)
  - [LLM Context (Baseline and User-Configurable) Requirements Traces](#llm-context-baseline-and-user-configurable-requirements-traces)
  - [Baseline Context](#baseline-context)
  - [Project-Level Context](#project-level-context)
  - [Task-Level Context](#task-level-context)
  - [User-Configurable Additional Context](#user-configurable-additional-context)
  - [Persona Assignment for SBA Jobs](#persona-assignment-for-sba-jobs)
- [Task Naming](#task-naming)
  - [Project Context From Chat Prompt](#project-context-from-chat-prompt)
  - [Task Naming Applicable Requirements](#task-naming-applicable-requirements)
- [Project Plan Building](#project-plan-building)
  - [Project Plan Building Requirements Traces](#project-plan-building-requirements-traces)
- [Clarification Before Execution](#clarification-before-execution)
  - [Clarification Before Execution Requirements Traces](#clarification-before-execution-requirements-traces)
- [When Plan is Locked](#when-plan-is-locked)
  - [When Plan is Locked Requirements Traces](#when-plan-is-locked-requirements-traces)
- [Plan Approval: Seek Explicit User Approval](#plan-approval-seek-explicit-user-approval)
  - [Plan Approval Seek Explicit User Approval Requirements Traces](#plan-approval-seek-explicit-user-approval-requirements-traces)
- [Plan Approved: PMA Tasked to Add or Update Tasks](#plan-approved-pma-tasked-to-add-or-update-tasks)
  - [Plan Approved PMA Tasked to Add or Update Tasks Requirements Traces](#plan-approved-pma-tasked-to-add-or-update-tasks-requirements-traces)
- [Preference Usage](#preference-usage)
  - [Preference Usage Applicable Requirements](#preference-usage-applicable-requirements)
- [Sub-Agent Model](#sub-agent-model)
  - [Sub-Agent Model Applicable Requirements](#sub-agent-model-applicable-requirements)
  - [Project Analyst Agent](#project-analyst-agent)

## Agent Purpose

The Project Manager Agent is a long-lived orchestrator-side agent responsible for driving tasks to completion.
It coordinates multi-step and multi-agent flows, enforces standards, and verifies outcomes against stored user task-execution preferences.
It is the control-plane agent responsible for setting up task execution, handing out work to task-scoped sub-agents, and orchestrating sandbox jobs on worker nodes.
It is also responsible for storing and retrieving task execution state and evidence in PostgreSQL through orchestrator-mediated **MCP tools** (catalog names such as `preference.*` and `task.*`, not `db.*` prefixes; see [MCP Tooling](mcp/mcp_tooling.md)).

Implementation artifact

- The concrete agent runtime is `cynode-pma` running in `project_manager` mode.
- See [`docs/tech_specs/cynode_pma.md`](cynode_pma.md).

User-facing chat is a conversation surface.
The PM and PA create and manage tasks via MCP tools during that conversation.
Chat messages are tracked separately from tasks.
See [`docs/tech_specs/openai_compatible_chat_api.md`](openai_compatible_chat_api.md) and [`docs/tech_specs/chat_threads_and_messages.md`](chat_threads_and_messages.md).

## LLM and Tool Execution (Implementation)

- Spec ID: `CYNAI.AGENTS.PMLlmToolImplementation` <a id="spec-cynai-agents-pmllmtoolimplementation"></a>

PMA uses **langchaingo** (Go) for LLM calls and tool execution, including **multiple simultaneous tool calls** where supported by the model and the orchestrator gateway.
The [LangGraph MVP workflow](langgraph_mvp.md) remains the graph runner and checkpoint owner.
Langchaingo implements the agentic steps within nodes (e.g. Plan Steps, Verify Step Result).
MCP tool calls from PMA go to the **orchestrator MCP gateway** (HTTP on the **control-plane** in default deployments; see [MCP Tool Access](cynode_pma.md#spec-cynai-pmagnt-mcptoolaccess)), typically via **`mcpclient`** from the PMA binary through the **worker internal proxy**.
Langchaingo tools wrap those MCP calls inside the PMA process.

## No Simulated or Guessed Output

- Spec ID: `CYNAI.AGENTS.NoSimulatedOutput` <a id="spec-cynai-agents-nosimulatedoutput"></a>

### Traces to Requirements

- [REQ-AGENTS-0137](../requirements/agents.md#req-agents-0137)

Agents MUST NOT guess or simulate output from tasks, database calls, tool calls, or external services.
They MUST use actual results from MCP tools, job callbacks, and system calls.
When data or results are unavailable (e.g. tool error, timeout, or missing resource), the agent MUST report that to the user or caller and MUST NOT invent, fabricate, or assume values.

## Agent Responsibilities

- Build project plans from user input; refine project plans as needed based on updated info from the user.
- Prefer to associate tasks to a non-default project when the user or context implies a named project or existing non-default project (default project is catch-all for unrelated work).
- Clarify with the user before doling out tasks when scope, acceptance criteria, or execution order are unclear.
- Task intake and triage
  - Create and update tasks, subtasks, and acceptance criteria in PostgreSQL.
  - Break work into executable steps suitable for worker nodes and sandbox containers.
- Project context from chat
  - When the user includes a project name or project id in a chat message, the PM agent SHOULD attempt to resolve it (e.g. by slug or id via MCP or gateway) and associate any tasks or related work created from that turn with that project, provided the user has access to the project.
  - Resolution and access MUST be performed via MCP/gateway; the PM MUST NOT assume access without verification.
  - If the user does not mention a project, the request context (e.g. thread project or default project) applies.
    See [Project context from chat prompt](#project-context-from-chat-prompt).
- Sub-agent management
  - Spin up sub-agents to monitor specific tasks and provide focused verification and feedback loops.
  - Ensure sub-agent findings are recorded in PostgreSQL and applied to task remediation.
  - Eagerly delegate tasks to Project Analyst sub-agents whenever possible.
- Planning and dispatch
  - Select worker nodes based on capabilities, health, data locality, and model availability.
  - Prefer nodes that already have the required model version loaded to reduce startup latency and network traffic.
  - Dispatch jobs with explicit sandbox requirements (network policy, mounts, resources, timeouts).
- Verification and remediation
  - Verify outputs against acceptance criteria and user preferences (style, security, completeness).
  - Request fixes or reruns when checks fail, and record rationale.
- Thread titling: automatically title the thread after the first user message when the user has not already set a title (see [Thread titling](project_manager_agent.md#spec-cynai-agents-threadtitling)).
- Model selection
  - Select models based on required capabilities and user task-execution preferences and constraints using the orchestrator model registry.
  - Request nodes to load required model versions when not already available.
  - Use external model routing when policy allows and local execution cannot satisfy requirements.
  - When selecting the Project Manager model itself (startup selection), prefer models with reliable structured tool calling and stable long-horizon planning over coder-only checkpoints.
  - The selection and warmup algorithm is defined in [Project Manager Model (Startup Selection and Warmup)](orchestrator.md#spec-cynai-orches-projectmanagermodelstartup).

### MVP Inference Assignment Responsibility

- For the MVP, the Project Manager model is the single decision-maker for inference task assignments.
  It selects local vs external inference targets, selects the model/version, and requests model loads when required.
- Continuous state maintenance
  - Keep task/job status, artifacts, logs, and summaries up to date in PostgreSQL.
  - Ensure workflows are resumable after restarts (checkpointed state).

### Project Manager Model Capability Requirements (MVP)

- Consistent tool/function calling and structured JSON output.
- Stable multi-step planning, task decomposition, and state tracking.
- Strong performance on common operator tooling prompts (git workflows, patch planning, CI, container operations).
- The Project Manager model SHOULD prioritize tool-use reliability and predictable latency over maximum coding intelligence.

### Thread Titling

- Spec ID: `CYNAI.AGENTS.ThreadTitling` <a id="spec-cynai-agents-threadtitling"></a>
- [REQ-PMAGNT-0119](../requirements/pmagnt.md#req-pmagnt-0119)
- The PMA MUST title the thread automatically after the first user message in the thread if the user has not already titled the thread.
- Auto-titling MUST NOT overwrite an existing title set by the user.
- The title SHOULD be derived from the first user message (e.g. truncated preview); persistence is via the gateway `PATCH /v1/chat/threads/{thread_id}` or an equivalent orchestrator-mediated update.

### Default Model Line (MVP)

- See [Project Manager Model (Startup Selection and Warmup)](orchestrator.md#spec-cynai-orches-projectmanagermodelstartup) for tier order and model baselines (Qwen3.5, Qwen2.5, `qwen3.5:0.8b`).

## External Provider Usage

The Project Manager Agent MAY use external AI providers for planning and verification.
External provider usage MUST be policy-controlled and audited.

### External Provider Usage Applicable Requirements

- Spec ID: `CYNAI.AGENTS.PMExternalProvider` <a id="spec-cynai-agents-pmexternalprovider"></a>

#### External Provider Usage Applicable Requirements Requirements Traces

- [REQ-AGENTS-0106](../requirements/agents.md#req-agents-0106)
- [REQ-AGENTS-0107](../requirements/agents.md#req-agents-0107)
- [REQ-AGENTS-0108](../requirements/agents.md#req-agents-0108)
- [REQ-AGENTS-0119](../requirements/agents.md#req-agents-0119)

See [`docs/tech_specs/external_model_routing.md`](external_model_routing.md) and [`docs/tech_specs/api_egress_server.md`](api_egress_server.md).

## External Provider Configuration

This section defines how to enable external AI APIs for the Project Manager Agent through orchestrator configuration.
External providers MUST be accessed through API Egress so provider credentials are never exposed to agents.

### Standalone Orchestrator Scenario

- The orchestrator may be running with zero local inference-capable nodes.
- If an external provider credential exists and policy allows it, the orchestrator MUST still be able to run the Project Manager Agent using an external provider.
- In this mode, the Project Manager Agent can plan, coordinate, and verify using external inference.
- Sandbox execution depends on whether any nodes exist that can run sandboxes.
- If there are no sandbox-capable nodes, the orchestrator SHOULD restrict or defer sandbox-required steps.

For tasks that require dependency downloads inside sandboxes, outbound HTTP(S) is mediated by the Web Egress Proxy.
Sandbox egress is only via worker proxies (inference, web egress, API Egress); sandboxes are not airgapped but have strict egress controls.
See [`docs/tech_specs/cynode_sba.md`](cynode_sba.md#spec-cynai-sbagnt-sandboxboundary), [`docs/tech_specs/sandbox_container.md`](sandbox_container.md#spec-cynai-sandbx-networkexpect), [`docs/tech_specs/web_egress_proxy.md`](web_egress_proxy.md), and [REQ-WEBPRX-0104](../requirements/webprx.md#req-webprx-0104).
If policy allows it, the Project Manager Agent may request task-scoped, temporary allowlist entries for the Web Egress Proxy.

### Required Configuration Steps

- Add provider credentials
  - Store the provider credential in PostgreSQL for API Egress (user-scoped).
  - See [`docs/tech_specs/api_egress_server.md`](api_egress_server.md).
- Groups MAY also use group-scoped credentials for shared integrations, subject to policy.
- Add access control rules
  - Allow the relevant subjects to call `api.call` for the chosen provider and operations.
  - Recommended stance is default-deny with narrow allow rules.
  - See [`docs/tech_specs/access_control.md`](access_control.md).
- Set preferences for agent routing
  - Configure which providers are allowed for this agent and the preferred order.
  - Suggested keys are:
    - `agents.project_manager.model_routing.prefer_local`
    - `agents.project_manager.model_routing.allowed_external_providers`
    - `agents.project_manager.model_routing.fallback_provider_order`
  - See [`docs/tech_specs/user_preferences.md`](user_preferences.md) and [`docs/tech_specs/external_model_routing.md`](external_model_routing.md).

### Bootstrap and Runtime Configuration

- These preferences and ACL rules MAY be seeded via orchestrator bootstrap YAML.
- After bootstrap, PostgreSQL remains the source of truth for changes.
- See [`docs/tech_specs/orchestrator_bootstrap.md`](orchestrator_bootstrap.md).

## Tool Access and Database Access

The Project Manager Agent is an orchestrator-side agent.
It MUST use MCP tools for privileged operations.

### Tool Access and Database Access Applicable Requirements

- Spec ID: `CYNAI.AGENTS.ProjectManagerToolAccess` <a id="spec-cynai-agents-pmtoolaccess"></a>

#### Tool Access and Database Access Applicable Requirements Requirements Traces

- [REQ-AGENTS-0109](../requirements/agents.md#req-agents-0109)
- [REQ-AGENTS-0110](../requirements/agents.md#req-agents-0110)

## Inputs and Outputs

This section defines the information the agent consumes and produces.

### Agent Inputs

- Inputs
  - Tasks, task metadata, and acceptance criteria.
  - User preferences (standards, policies, communication defaults).
  - System settings (operational configuration and policy parameters).
  - Worker inventory (capabilities, current load, health).

Startup context

On orchestrator startup, the Project Manager Agent MUST be provided sufficient context to begin dispatching work without relying on implicit defaults.
At minimum, the startup context MUST include:

- The Project Manager agent identity and role (for tool allowlists and policy evaluation).
- Access to MCP tools required for task execution:
  - task and job read/write tools
  - preference resolution tools
  - system settings get/list tools
  - artifact tools
  - node and sandbox tools (when dispatching work)
  - sandbox allowed-images tools: `sandbox.allowed_images.list` (always); `sandbox.allowed_images.add` only when the orchestrator system setting `agents.project_manager.sandbox.allow_add_to_allowed_images` is enabled (default disabled)
- The current set of pending and runnable tasks (or the ability to query them deterministically).
- The effective preference resolution model for each task context.
- The current system settings required for safe dispatch and routing (for example model selection settings and cache/download policy).

### Agent Outputs

- Outputs
  - Job dispatch requests to worker APIs.
  - Task state transitions and verification records in PostgreSQL.
  - Final task summaries and artifacts (including links to uploaded files).
  - **Artifact download links for users:** The PMA MAY provide users with download links for artifacts (e.g. in chat or task results) that point to the unified artifacts endpoint (`GET /v1/artifacts/{artifact_id}`); access MUST be gated by RBAC so only authorized users can retrieve the file (see [Orchestrator Artifacts Storage](orchestrator_artifacts_storage.md#spec-cynai-orches-artifactsrbac), [Chat Threads and Messages - Download References](chat_threads_and_messages.md#spec-cynai-usrgwy-chatthreadsmessages-downloadrefs)).

## LLM Context (Baseline and User-Configurable)

- Spec ID: `CYNAI.AGENTS.LLMContext` <a id="spec-cynai-agents-llmcontext"></a>

### LLM Context (Baseline and User-Configurable) Requirements Traces

- [REQ-AGENTS-0132](../requirements/agents.md#req-agents-0132)
- [REQ-AGENTS-0133](../requirements/agents.md#req-agents-0133)
- [REQ-AGENTS-0134](../requirements/agents.md#req-agents-0134)

This section applies to all agents that leverage LLMs (Project Manager, Project Analyst, Sandbox Agent, and any cloud-run agents).
Each such agent MUST supply baseline context and user-configurable additional context to every LLM prompt or system message it uses.
When a project or task is in scope, the agent MUST also include project-level and task-level context respectively.

### Baseline Context

- Baseline context is fixed per agent (or per role when one binary has multiple roles).
- It MUST describe: agent identity, role, responsibilities, and non-goals.
- It MUST be included in every LLM prompt or system message used by that agent.
- It is typically sourced from the agent's instructions bundle or a dedicated baseline document and MUST NOT be overridden by user preferences.

### Project-Level Context

- When the request or job has an associated `project_id` (and the agent has access to that project), the agent MUST include project-level context in the LLM prompt.
- Project-level context describes the current project: identity (id, name, slug), scope, and any project metadata relevant to the LLM (e.g. project kind, description).
- It is sourced from the orchestrator (e.g. via MCP project tools or handoff) and MUST be included after role instructions and before task-level context in the composition order.

### Task-Level Context

- When the request or job has an associated `task_id`, the agent MUST include task-level context in the LLM prompt.
- Task-level context describes the current task: identity (id, name), acceptance criteria summary, status, and any task metadata relevant to the LLM.
- It is sourced from the orchestrator (e.g. via MCP task tools or job payload) and MUST be included after project-level context (if present) and before user-configurable additional context in the composition order.

### User-Configurable Additional Context

- Additional context is resolved from user preferences using the same scope precedence as other preferences (task > project > user > group > system).
- Preference keys for agent additional context use the reserved namespace `agents.<agent_id>.additional_context` or role-based keys (e.g. `agents.project_manager.additional_context`, `agents.project_analyst.additional_context`, `agents.sandbox_agent.additional_context`).
- See [User preferences - Agent additional context](user_preferences.md#spec-cynai-stands-agentadditionalcontext).
- The effective additional context (after resolution) MUST be merged into the context supplied to the LLM in a defined order: baseline context, then role instructions (instructions bundle), then project-level context (when applicable), then task-level context (when applicable), then user-configurable additional context, then request-specific messages.

Implementation notes

- For `cynode-pma`, baseline context is part of the role-specific instructions bundle; the runtime MUST also resolve and append project-level context (when `project_id` in scope), task-level context (when `task_id` in scope), and preferences-based additional context when building system or prompt content.
- For `cynode-sba`, baseline context describes the sandbox agent identity and role and is supplied in the job; job context MUST include project-level context and task-level context (when the job is scoped to a project and task); preferences-based additional context for the SBA uses the same keys and is included in the job context or a dedicated slot.
- Cloud workers that run agent code and call LLMs MUST receive baseline context, project-level and task-level context (when applicable), and preferences-based additional context from the orchestrator (e.g. in the job payload or handoff) and MUST include all of them in LLM prompts.

### Persona Assignment for SBA Jobs

- Spec ID: `CYNAI.AGENTS.PersonaAssignment` <a id="spec-cynai-agents-personaassignment"></a>

When the PMA or PAA (or orchestrator job builder) creates or assigns a job that runs the SBA, it MUST be able to set the job's Agent persona.
The builder queries Agent personas via the User API Gateway or MCP (persona list, persona get by id), fetches the full persona (title, description), and **embeds it inline** in the job spec JSON.
The job payload delivered to the node contains only the inline `persona` object; the SBA never resolves a persona_id.

**Resolution precedence:** When multiple Agent personas match (e.g. same title in different scopes), the builder MUST apply the **most specific** that matches: user scope over project over group over system (global).
Example: if both a global Agent persona and a user-scoped Agent persona exist with the same title, the user-scoped one MUST be used.
The MCP contract for sandbox job creation (e.g. `sandbox.create`) MUST allow passing a persona by id or inline so the final job spec contains the inline persona.

**PMA and PAA built-in Agent personas:** PMA and PAA MUST always use their own dedicated system-scoped (global) Agent personas for their identity/role when running; these are part of the global default Agent personas (see [cynode_sba.md - Persona on the Job](cynode_sba.md#spec-cynai-sbagnt-jobpersona)).
See [cynode_sba.md - Persona on the Job](cynode_sba.md#spec-cynai-sbagnt-jobpersona) and [Sandbox tools](mcp_tools/sandbox_tools.md).

**Task-level persona and skills:** When creating or updating tasks (e.g. via MCP task create/update), the PMA SHOULD set `persona_id` and optional `recommended_skill_ids` (array of skill stable identifiers) so the orchestrator job builder can resolve the persona and merge skills (persona default_skill_ids union task recommended_skill_ids) when building the job.
Execution-ready tasks SHOULD have a `persona_id`; one persona per task.
**Task bundles:** The PMA MAY hand off a bundle of 1-3 tasks (same persona, in dependency order) for SBA execution in series; the orchestrator job builder builds a single job with `task_ids` (map keyed 10, 20, 30) and embedded per-task context (see [Job builder (task-to-job)](orchestrator.md#spec-cynai-orches-jobbuilder)).

## Task Naming

The Project Manager MUST assign each task a human-readable name in addition to its UUID so users can refer to tasks by name in the CLI and API (e.g. for `task get`, `task result`, `task cancel`).

Task name format

- All lowercase.
- Words separated by single dashes (e.g. `deploy-docs`, `run-tests`).
- Trailing numbers MAY be used for uniqueness when the same logical name would otherwise repeat (e.g. `deploy-docs-2`, `run-tests-3`).
- Task names MUST be unique within the scope where they are resolved (e.g. per user or per project as defined by the gateway).

User-supplied name on create

- The task create request (e.g. POST from the User API Gateway or CLI) MAY include a suggested name (e.g. `name` or `task_name`).
- The orchestrator MUST accept the suggested name when present, MUST normalize it to the task name format above (lowercase, single dashes), and MUST ensure uniqueness within scope (e.g. by appending `-2`, `-3`, etc.) when the normalized name already exists.

APIs and the CLI MUST accept either the task UUID or the task name as the task identifier when a task is referenced.

### Project Context From Chat Prompt

- Spec ID: `CYNAI.AGENTS.PMProjectFromPrompt` <a id="spec-cynai-agents-pmprojectfromprompt"></a>

#### Project Context From Chat Prompt Requirements Traces

- [REQ-AGENTS-0131](../requirements/agents.md#req-agents-0131)

When the user provides a project name or project id in a chat message (e.g. "create a task for project X" or "add this to the backend project"), the Project Manager Agent SHOULD attempt to resolve the project (by slug or id) and associate any tasks or related work created from that conversation turn with that project.
The PM MUST verify that the user has access to the project (e.g. via MCP or gateway-provided context) before associating work with it; if the user does not have access, the PM MUST NOT associate with that project and SHOULD use the thread project or the user's default project instead.
Resolution and access checks MUST be performed through MCP tools or gateway contracts; the PM MUST NOT assume access without verification.
If the user does not mention a project in the prompt, the existing request context (thread `project_id` or default project) applies.

### Task Naming Applicable Requirements

- Spec ID: `CYNAI.AGENTS.ProjectManagerTaskNaming` <a id="spec-cynai-agents-pmtasknaming"></a>

#### Task Naming Applicable Requirements Requirements Traces

- [REQ-AGENTS-0129](../requirements/agents.md#req-agents-0129)

## Project Plan Building

- Spec ID: `CYNAI.AGENTS.ProjectPlanBuilding` <a id="spec-cynai-agents-projectplanbuilding"></a>

### Project Plan Building Requirements Traces

- [REQ-PMAGNT-0111](../requirements/pmagnt.md#req-pmagnt-0111)
- [REQ-PMAGNT-0113](../requirements/pmagnt.md#req-pmagnt-0113)

The PMA is responsible for **building** the project plan (tasks and execution order) from user input before creating orchestrator tasks and dispatching jobs.
When the user describes a goal that implies multiple steps or a project, the PMA SHOULD first produce a plan (e.g. list of tasks with order and acceptance criteria), persist it or associate tasks with the plan, and only then create tasks and hand off for execution.
The PMA SHOULD refine project plans as needed based on updated information from the user (e.g. after clarification or change requests).

### Plan Creation Procedure (Skill)

- Spec ID: `CYNAI.AGENTS.PlanCreationProcedure` <a id="spec-cynai-agents-plancreationprocedure"></a>

When the deployment supplies the **PMA plan creation skill** ([`default_skills/pma_plan_creation_skill.md`](../../default_skills/pma_plan_creation_skill.md)), the PMA SHOULD follow that procedure when building or refining project plans so structure, task naming, dependencies, and MCP persistence align with host schemas and lock/approval rules.
Plan and task reads MUST remain via MCP tools (or gateway equivalents) per [Tool Access and Database Access](#tool-access-and-database-access); the skill content is procedural guidance, not a second source of truth for schemas.
Future catalog additions (for example `plan.help`, `task.help`, and project-plan CRUD tools) MUST be documented in [mcp_tools/](mcp_tools/README.md) and allowlists in [access_allowlists_and_scope.md](mcp_tools/access_allowlists_and_scope.md) when implemented.

#### Plan Creation Procedure Traces

- [REQ-PMAGNT-0111](../requirements/pmagnt.md#req-pmagnt-0111)

## Task Review and Ready Transition

- Spec ID: `CYNAI.AGENTS.TaskReviewAndReadyTransition` <a id="spec-cynai-agents-taskreviewandreadytransition"></a>

When a task is created via the User API Gateway (or CLI), it is persisted with `planning_state=draft` and routed to the PMA for review before workflow execution.
The PMA receives the task with full context and MAY enrich it (e.g. confirm or set `project_id`, normalize task name, add or refine description and acceptance criteria, attach artifacts, create dependency edges when the task is part of a plan).
The PMA **MUST attempt to clarify ambiguous tasks with the user** before marking the task as ready (part of its task creation/management skill; see [REQ-PMAGNT-0128](../requirements/pmagnt.md#req-pmagnt-0128)).
Clarification MUST occur **in the thread where the user directed task creation**, or **via notification to the user** (Notification spec TBD).
When the PMA determines the task is sufficiently specified for execution (after clarification when needed), the PMA transitions the task to `planning_state=ready`; that transition is the path that enables workflow execution (see [REQ-ORCHES-0179](../requirements/orches.md#req-orches-0179)).
The PMA MAY use `persona.list` and `persona.get` (MCP) to resolve and set `persona_id` and `recommended_skill_ids` during review or when creating tasks.

**PMA review request contract:** The orchestrator MUST supply the PMA with at least: `task_id` for the newly created task; `project_id` when known (or omit when unknown and allow PMA to resolve or request clarification); `user_id` for audit attribution and preference resolution; and `messages` containing the task prompt and a task-review instruction wrapper.
The request body type and endpoint (e.g. `POST /internal/chat/completion` with `InternalChatCompletionRequest`) are implementation-defined; the PMA response MUST be parseable so the orchestrator can extract enrichment outputs and the ready transition deterministically.

### Task Review and Ready Transition Traces To

- [REQ-ORCHES-0177](../requirements/orches.md#req-orches-0177)
- [REQ-ORCHES-0179](../requirements/orches.md#req-orches-0179)
- [REQ-PMAGNT-0128](../requirements/pmagnt.md#req-pmagnt-0128)

#### Ready to Draft Transition

- A task in `planning_state=ready` MAY be transitioned back to `draft` as long as the task was **not executed** (see [REQ-ORCHES-0185](../requirements/orches.md#req-orches-0185)).
- **Aborted executions do not count:** User-initiated cancel, job killed, or timeout before completion do NOT count as executed; only a job that reached `completed` or `failed` (non-abort) counts.
- The PMA and gateway MUST allow ready->draft when no job for the task has ever reached such a terminal state.

## User-Directed Task Cancel and Job Kill

- Spec ID: `CYNAI.AGENTS.UserDirectedTaskCancelAndJobKill` <a id="spec-cynai-agents-userdirectedtaskcancelandjobkill"></a>

The PMA MUST be able to **cancel a currently running task at user direction** and ensure **kill signals** are sent to the worker so that long-running or stuck work can be stopped without waiting for timeout.

- When the user tells the PMA in natural language to **kill a job**, **cancel a task**, or **stop a running job**, the PMA MUST interpret the intent, resolve the task (by task_id, task name, or conversation context), and invoke the same path as task cancel (gateway task-cancel API or MCP equivalent).
- The orchestrator, on receiving the cancel request, MUST mark the task as canceled and MUST send a **stop job** request to the worker node when the task has an active job (see [REQ-ORCHES-0184](../requirements/orches.md#req-orches-0184)); the node then stops the SBA gracefully (e.g. SIGTERM) with a container-kill fallback if the agent does not exit in time.
- Slash commands (e.g. `/task cancel <task_id>` or `/kill job <task_id>`) delivered via messaging connectors are handled by PMA the same way: PMA parses the command and invokes task cancel.
- See [worker_api.md - Stop Job](worker_api.md#spec-cynai-worker-stopjob) for the Stop Job contract; user-directed job kill is proposed in a draft spec (not yet canonical).

### User-Directed Task Cancel and Job Kill Traces To

- [REQ-ORCHES-0184](../requirements/orches.md#req-orches-0184)

## Clarification Before Execution

- Spec ID: `CYNAI.AGENTS.ClarificationBeforeExecution` <a id="spec-cynai-agents-clarificationbeforeexecution"></a>

### Clarification Before Execution Requirements Traces

- [REQ-PMAGNT-0112](../requirements/pmagnt.md#req-pmagnt-0112)
- [REQ-PMAGNT-0128](../requirements/pmagnt.md#req-pmagnt-0128)
- [REQ-AGENTS-0135](../requirements/agents.md#req-agents-0135)

The PMA MUST attempt to clarify ambiguous tasks with the user **before marking a task as ready** (see [REQ-PMAGNT-0128](../requirements/pmagnt.md#req-pmagnt-0128)); this is part of its task creation/management skill.
Clarification MUST occur **in the thread where the user directed task creation**, or **via notification to the user** (Notification spec TBD).
The PMA SHOULD ask clarifying questions when scope, acceptance criteria, priorities, or execution order are ambiguous, and SHOULD prefer multi-turn clarification over inferring and creating tasks immediately.
Multi-message conversation is the intended way to clarify and lay out the task before or as it is executed; building up a task properly may take multiple messages.
See [`chat_threads_and_messages.md`](chat_threads_and_messages.md) and [`openai_compatible_chat_api.md`](openai_compatible_chat_api.md).

## When Plan is Locked

- Spec ID: `CYNAI.AGENTS.WhenPlanLocked` <a id="spec-cynai-agents-whenplanlocked"></a>

### When Plan is Locked Requirements Traces

- [REQ-PMAGNT-0114](../requirements/pmagnt.md#req-pmagnt-0114)

When a project plan is locked, the PMA MUST NOT change the plan or its tasks; the PMA MAY update completion status and comments on plans and tasks only.
The API or gateway enforces the lock so that PMA tool calls that would edit the plan document or tasks are rejected when the plan is locked.

## Plan Approval: Seek Explicit User Approval

- Spec ID: `CYNAI.AGENTS.PlanApprovalSeekExplicitApproval` <a id="spec-cynai-agents-planapprovalseekexplicitapproval"></a>

### Plan Approval Seek Explicit User Approval Requirements Traces

- [REQ-AGENTS-0136](../requirements/agents.md#req-agents-0136)

The PMA MAY mark a plan as approved (set `plan_approved_at`, `plan_approved_by`) only after **seeking and obtaining explicit approval from the user**.
The MCP tool that performs plan approve MUST be described (in the tool catalog or tool description) so that the LLM understands it must seek explicit user approval before calling it; for example, the tool description MUST state that the agent must obtain explicit user approval before marking the plan as approved for execution.
Agent instructions (e.g. PMA instructions bundle) MUST state that the agent must not mark a plan as approved until the user has explicitly approved it (e.g. in chat or via explicit confirmation).
The PMA may build and refine plans and may request workflow start for a task (subject to the workflow start gate); when the PMA invokes plan approve, it must have obtained explicit user approval first.

## Plan Approved: PMA Tasked to Add or Update Tasks

- Spec ID: `CYNAI.AGENTS.PlanApprovedPmaTasked` <a id="spec-cynai-agents-planapprovedpmatasked"></a>

### Plan Approved PMA Tasked to Add or Update Tasks Requirements Traces

- [REQ-PROJCT-0122](../requirements/projct.md#req-projct-0122)

When a plan is approved (set to **ready**) by the user, the **first action** the system MUST take is to task the PMA to add or update tasks on that plan so that it is ready for execution.
The orchestrator (or component that processes plan approval) MUST enqueue or invoke a PMA job for the approved plan with the objective of ensuring the plan has at least one task and that the task list is suitable for execution (add or update tasks as needed).
The plan remains in state **ready** until it is activated (ready -> active); workflow for tasks in the plan MAY run only when the plan is **active**.
The PMA is responsible for populating or refining the task list as the first step after approval.

## Preference Usage

The following requirements apply.

### Preference Usage Applicable Requirements

- Spec ID: `CYNAI.AGENTS.ProjectManagerPreferenceUsage` <a id="spec-cynai-agents-pmpreferenceusage"></a>

#### Preference Usage Applicable Requirements Requirements Traces

- [REQ-AGENTS-0111](../requirements/agents.md#req-agents-0111)
- [REQ-AGENTS-0112](../requirements/agents.md#req-agents-0112)
- [REQ-AGENTS-0113](../requirements/agents.md#req-agents-0113)

See [`docs/tech_specs/user_preferences.md`](user_preferences.md).

## Sub-Agent Model

The Project Manager Agent MAY spin up sub-agents to monitor individual tasks.
Sub-agents run as long-lived, focused workers that watch task state and validate outputs against requirements.

### Sub-Agent Model Applicable Requirements

- Spec ID: `CYNAI.AGENTS.ProjectManagerSubAgent` <a id="spec-cynai-agents-pmsubagent"></a>

#### Sub-Agent Model Applicable Requirements Requirements Traces

- [REQ-AGENTS-0120](../requirements/agents.md#req-agents-0120)
- [REQ-AGENTS-0121](../requirements/agents.md#req-agents-0121)
- [REQ-AGENTS-0122](../requirements/agents.md#req-agents-0122)

### Project Analyst Agent

The Project Analyst Agent is a monitoring sub-agent that focuses on a specific task.
It validates that task outputs satisfy acceptance criteria and user preferences.

Implementation artifact

- The concrete analyst runtime is `cynode-pma` running in `project_analyst` mode with a separate instructions bundle.
- See [`docs/tech_specs/project_analyst_agent.md`](project_analyst_agent.md) and [`docs/tech_specs/cynode_pma.md`](cynode_pma.md).
