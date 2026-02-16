# Project Manager Agent

- [Agent Purpose](#agent-purpose)
- [Agent Responsibilities](#agent-responsibilities)
- [External Provider Usage](#external-provider-usage)
- [External Provider Configuration](#external-provider-configuration)
  - [Standalone Orchestrator Scenario](#standalone-orchestrator-scenario)
  - [Required Configuration Steps](#required-configuration-steps)
  - [Bootstrap and Runtime Configuration](#bootstrap-and-runtime-configuration)
- [Tool Access and Database Access](#tool-access-and-database-access)
- [Inputs and Outputs](#inputs-and-outputs)
  - [Agent Inputs](#agent-inputs)
  - [Agent Outputs](#agent-outputs)
- [Preference Usage](#preference-usage)
- [Sub-Agent Model](#sub-agent-model)
  - [Project Analyst Agent](#project-analyst-agent)

## Agent Purpose

The Project Manager Agent is a long-lived orchestrator-side agent responsible for driving tasks to completion.
It coordinates multi-step and multi-agent flows, enforces standards, and verifies outcomes against stored preferences.

## Agent Responsibilities

- Task intake and triage
  - Create and update tasks, subtasks, and acceptance criteria in PostgreSQL.
  - Break work into executable steps suitable for worker nodes and sandbox containers.
- Sub-agent management
  - Spin up sub-agents to monitor specific tasks and provide focused verification and feedback loops.
  - Ensure sub-agent findings are recorded in PostgreSQL and applied to task remediation.
- Planning and dispatch
  - Select worker nodes based on capabilities, health, data locality, and model availability.
  - Prefer nodes that already have the required model version loaded to reduce startup latency and network traffic.
  - Dispatch jobs with explicit sandbox requirements (network policy, mounts, resources, timeouts).
- Verification and remediation
  - Verify outputs against acceptance criteria and user preferences (style, security, completeness).
  - Request fixes or reruns when checks fail, and record rationale.
- Model selection
  - Select models based on required capabilities and preferences using the orchestrator model registry.
  - Request nodes to load required model versions when not already available.
  - Use external model routing when policy allows and local execution cannot satisfy requirements.
- Continuous state maintenance
  - Keep task/job status, artifacts, logs, and summaries up to date in PostgreSQL.
  - Ensure workflows are resumable after restarts (checkpointed state).

## External Provider Usage

The Project Manager Agent MAY use external AI providers for planning and verification.
External provider usage MUST be policy-controlled and audited.

Normative requirements

- The agent MUST NOT store provider API keys.
- External model calls MUST be routed through the API Egress Server.
- The agent SHOULD prefer local execution when it satisfies capability and policy constraints.
- The agent SHOULD be able to pair external inference with sandbox execution on a node when tool runs are required.

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

### Required Configuration Steps

- Add provider credentials
  - Store the provider credential in PostgreSQL for API Egress (user-scoped).
  - See [`docs/tech_specs/api_egress_server.md`](api_egress_server.md).
  - Teams MAY also use team-scoped credentials for shared integrations, subject to policy.
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

Normative requirements

- All PostgreSQL access MUST happen through MCP tools.
- The agent MUST NOT connect directly to PostgreSQL.

## Inputs and Outputs

This section defines the information the agent consumes and produces.

### Agent Inputs

- Inputs
  - Tasks, task metadata, and acceptance criteria.
  - User preferences (standards, policies, communication defaults).
  - Worker inventory (capabilities, current load, health).

### Agent Outputs

- Outputs
  - Job dispatch requests to worker APIs.
  - Task state transitions and verification records in PostgreSQL.
  - Final task summaries and artifacts (including links to uploaded files).

## Preference Usage

Normative requirements for planning and verification.

- The agent MUST load applicable preferences before planning and before final verification.
- Preference precedence SHOULD be: task-specific > project-specific > user-default > system-default.
- The agent MUST record which preference set was applied for verification.

See [`docs/tech_specs/user_preferences.md`](user_preferences.md).

## Sub-Agent Model

The Project Manager Agent MAY spin up sub-agents to monitor individual tasks.
Sub-agents run as long-lived, focused workers that watch task state and validate outputs against requirements.

Normative requirements for sub-agents

- Sub-agents MUST operate within the same standards and preference precedence rules as the Project Manager Agent.
- Sub-agents MUST write their findings and recommended remediation steps back to PostgreSQL.
- Sub-agents SHOULD be scoped to a single task to avoid cross-task state leakage.

### Project Analyst Agent

The Project Analyst Agent is a monitoring sub-agent that focuses on a specific task.
It validates that task outputs satisfy acceptance criteria and user preferences.

See [`docs/tech_specs/project_analyst_agent.md`](project_analyst_agent.md).
