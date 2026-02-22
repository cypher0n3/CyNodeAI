# Project Manager Agent

- [Agent Purpose](#agent-purpose)
- [Agent Responsibilities](#agent-responsibilities)
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
- [Task Naming](#task-naming)
  - [Project Context From Chat Prompt](#project-context-from-chat-prompt)
  - [Task Naming Applicable Requirements](#task-naming-applicable-requirements)
- [Preference Usage](#preference-usage)
  - [Preference Usage Applicable Requirements](#preference-usage-applicable-requirements)
- [Sub-Agent Model](#sub-agent-model)
  - [Sub-Agent Model Applicable Requirements](#sub-agent-model-applicable-requirements)
  - [Project Analyst Agent](#project-analyst-agent)

## Agent Purpose

The Project Manager Agent is a long-lived orchestrator-side agent responsible for driving tasks to completion.
It coordinates multi-step and multi-agent flows, enforces standards, and verifies outcomes against stored user task-execution preferences.
It is the control-plane agent responsible for setting up task execution, handing out work to task-scoped sub-agents, and orchestrating sandbox jobs on worker nodes.
It is also responsible for storing and retrieving task execution state and evidence in PostgreSQL through MCP database tools.

Implementation artifact

- The concrete agent runtime is `cynode-pma` running in `project_manager` mode.
- See [`docs/tech_specs/cynode_pma.md`](cynode_pma.md).

User-facing chat is a conversation surface.
The PM and PA create and manage tasks via MCP tools during that conversation.
Chat messages are tracked separately from tasks.
See [`docs/tech_specs/openai_compatible_chat_api.md`](openai_compatible_chat_api.md) and [`docs/tech_specs/chat_threads_and_messages.md`](chat_threads_and_messages.md).

## Agent Responsibilities

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
- Model selection
  - Select models based on required capabilities and user task-execution preferences and constraints using the orchestrator model registry.
  - Request nodes to load required model versions when not already available.
  - Use external model routing when policy allows and local execution cannot satisfy requirements.
  - When selecting the Project Manager model itself (startup selection), prefer models with reliable structured tool calling and stable long-horizon planning over coder-only checkpoints.
  - The selection and warmup algorithm is defined in [Project Manager Model (Startup Selection and Warmup)](orchestrator.md#spec-cynai-orches-projectmanagermodelstartup).

MVP inference assignment responsibility

- For the MVP, the Project Manager model is the single decision-maker for inference task assignments.
  It selects local vs external inference targets, selects the model/version, and requests model loads when required.
- Continuous state maintenance
  - Keep task/job status, artifacts, logs, and summaries up to date in PostgreSQL.
  - Ensure workflows are resumable after restarts (checkpointed state).

Project Manager model capability requirements (MVP)

- Consistent tool/function calling and structured JSON output.
- Stable multi-step planning, task decomposition, and state tracking.
- Strong performance on common operator tooling prompts (git workflows, patch planning, CI, container operations).
- The Project Manager model SHOULD prioritize tool-use reliability and predictable latency over maximum coding intelligence.

Recommended default model line (MVP)

- See [Project Manager Model (Startup Selection and Warmup)](orchestrator.md#spec-cynai-orches-projectmanagermodelstartup) for tier order and model baselines (Qwen2.5/Llama 3.3/`tinyllama`).

## External Provider Usage

The Project Manager Agent MAY use external AI providers for planning and verification.
External provider usage MUST be policy-controlled and audited.

### External Provider Usage Applicable Requirements

- Spec ID: `CYNAI.AGENTS.PMExternalProvider` <a id="spec-cynai-agents-pmexternalprovider"></a>

Traces To:

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

Traces To:

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

## LLM Context (Baseline and User-Configurable)

- Spec ID: `CYNAI.AGENTS.LLMContext` <a id="spec-cynai-agents-llmcontext"></a>

Traces To:

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

Traces To:

- [REQ-AGENTS-0131](../requirements/agents.md#req-agents-0131)

When the user provides a project name or project id in a chat message (e.g. "create a task for project X" or "add this to the backend project"), the Project Manager Agent SHOULD attempt to resolve the project (by slug or id) and associate any tasks or related work created from that conversation turn with that project.
The PM MUST verify that the user has access to the project (e.g. via MCP or gateway-provided context) before associating work with it; if the user does not have access, the PM MUST NOT associate with that project and SHOULD use the thread project or the user's default project instead.
Resolution and access checks MUST be performed through MCP tools or gateway contracts; the PM MUST NOT assume access without verification.
If the user does not mention a project in the prompt, the existing request context (thread `project_id` or default project) applies.

### Task Naming Applicable Requirements

- Spec ID: `CYNAI.AGENTS.ProjectManagerTaskNaming` <a id="spec-cynai-agents-pmtasknaming"></a>

Traces To:

- [REQ-AGENTS-0129](../requirements/agents.md#req-agents-0129)

## Preference Usage

The following requirements apply.

### Preference Usage Applicable Requirements

- Spec ID: `CYNAI.AGENTS.ProjectManagerPreferenceUsage` <a id="spec-cynai-agents-pmpreferenceusage"></a>

Traces To:

- [REQ-AGENTS-0111](../requirements/agents.md#req-agents-0111)
- [REQ-AGENTS-0112](../requirements/agents.md#req-agents-0112)
- [REQ-AGENTS-0113](../requirements/agents.md#req-agents-0113)

See [`docs/tech_specs/user_preferences.md`](user_preferences.md).

## Sub-Agent Model

The Project Manager Agent MAY spin up sub-agents to monitor individual tasks.
Sub-agents run as long-lived, focused workers that watch task state and validate outputs against requirements.

### Sub-Agent Model Applicable Requirements

- Spec ID: `CYNAI.AGENTS.ProjectManagerSubAgent` <a id="spec-cynai-agents-pmsubagent"></a>

Traces To:

- [REQ-AGENTS-0120](../requirements/agents.md#req-agents-0120)
- [REQ-AGENTS-0121](../requirements/agents.md#req-agents-0121)
- [REQ-AGENTS-0122](../requirements/agents.md#req-agents-0122)

### Project Analyst Agent

The Project Analyst Agent is a monitoring sub-agent that focuses on a specific task.
It validates that task outputs satisfy acceptance criteria and user preferences.

Implementation artifact

- The concrete analyst runtime is `cynode-pma` running in `project_analyst` mode with a separate instructions bundle.
- See [`docs/tech_specs/project_analyst_agent.md`](project_analyst_agent.md) and [`docs/tech_specs/cynode_pma.md`](cynode_pma.md).
