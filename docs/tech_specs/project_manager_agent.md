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
- [Preference Usage](#preference-usage)
  - [Preference Usage Applicable Requirements](#preference-usage-applicable-requirements)
- [Sub-Agent Model](#sub-agent-model)
  - [Sub-Agent Model Applicable Requirements](#sub-agent-model-applicable-requirements)
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
  - Worker inventory (capabilities, current load, health).

### Agent Outputs

- Outputs
  - Job dispatch requests to worker APIs.
  - Task state transitions and verification records in PostgreSQL.
  - Final task summaries and artifacts (including links to uploaded files).

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
