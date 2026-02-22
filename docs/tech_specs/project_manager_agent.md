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
- [Task Naming](#task-naming)
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

User-facing chat is a conversation surface.
The PM and PA create and manage tasks via MCP tools during that conversation.
Chat messages are tracked separately from tasks.
See [`docs/tech_specs/openai_compatible_chat_api.md`](openai_compatible_chat_api.md) and [`docs/tech_specs/chat_threads_and_messages.md`](chat_threads_and_messages.md).

## Agent Responsibilities

- Task intake and triage
  - Create and update tasks, subtasks, and acceptance criteria in PostgreSQL.
  - Break work into executable steps suitable for worker nodes and sandbox containers.
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
  - The selection and warmup algorithm is defined in [Project Manager Model (Startup Selection and Warmup)](orchestrator.md#project-manager-model-startup-selection-and-warmup).

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

- See [Project Manager Model (Startup Selection and Warmup)](orchestrator.md#project-manager-model-startup-selection-and-warmup) for tier order and model baselines (Qwen2.5/Llama 3.3/`tinyllama`).

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
If policy allows it, the Project Manager Agent may request task-scoped, temporary allowlist entries for the Web Egress Proxy.
See [`docs/tech_specs/web_egress_proxy.md`](web_egress_proxy.md) and [REQ-WEBPRX-0104](../requirements/webprx.md#req-webprx-0104).

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
- The current set of pending and runnable tasks (or the ability to query them deterministically).
- The effective preference resolution model for each task context.
- The current system settings required for safe dispatch and routing (for example model selection settings and cache/download policy).

### Agent Outputs

- Outputs
  - Job dispatch requests to worker APIs.
  - Task state transitions and verification records in PostgreSQL.
  - Final task summaries and artifacts (including links to uploaded files).

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

See [`docs/tech_specs/project_analyst_agent.md`](project_analyst_agent.md).
