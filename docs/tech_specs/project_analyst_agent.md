# Project Analyst Agent

- [Document Overview](#document-overview)
- [Agent Purpose](#agent-purpose)
- [Agent Responsibilities](#agent-responsibilities)
- [Handoff Model](#handoff-model)
- [External Provider Usage](#external-provider-usage)
- [External Provider Configuration](#external-provider-configuration)
  - [Standalone Orchestrator Scenario](#standalone-orchestrator-scenario)
  - [Required Configuration Steps](#required-configuration-steps)
  - [Bootstrap and Runtime Configuration](#bootstrap-and-runtime-configuration)
- [Tool Access and Database Access](#tool-access-and-database-access)
- [Verification Outputs](#verification-outputs)
- [LLM Context (Baseline and User-Configurable)](#llm-context-baseline-and-user-configurable)
- [Preference Usage](#preference-usage)

## Document Overview

This document defines the Project Analyst Agent, a sub-agent spawned by the Project Manager Agent to monitor a single task.
It provides focused verification that task outputs satisfy acceptance criteria and user preferences.

Implementation artifact

- The concrete analyst runtime is `cynode-pma` running in `project_analyst` mode.
- It MUST use a separate instructions bundle from the Project Manager role.
- See [`docs/tech_specs/cynode_pma.md`](cynode_pma.md).

## Agent Purpose

The Project Analyst Agent monitors a specific task and produces verification findings.
It is intended to tighten feedback loops and keep task quality aligned with defined requirements.
The Project Analyst Agent is task-scoped.
It is responsible for managing and tracking a single task that has been handed off by the Project Manager Agent.

## Agent Responsibilities

- Monitor task state
  - Watch the task, subtasks, and job results in PostgreSQL.
  - Track revisions and identify when new outputs require re-verification.
- Verify against requirements
  - Evaluate outputs against task acceptance criteria.
  - Evaluate outputs against effective user preferences and standards.
  - Flag missing artifacts, incomplete steps, or policy violations.
- Recommend remediation
  - Propose concrete fix steps when outputs do not meet requirements.
  - Request re-runs or additional evidence when verification is inconclusive.
- Record findings
  - Write structured verification notes to PostgreSQL with timestamps and applied preference context.

## Handoff Model

Project Analyst agents are spawned and managed by the Project Manager Agent.
The Project Manager Agent SHOULD eagerly hand off tasks to Project Analyst agents whenever possible.

Handoff payload

When the Project Manager Agent spawns a Project Analyst agent for a task, it SHOULD provide a task-scoped handoff payload that includes:

- `task_id`
- the current acceptance criteria and task metadata
- a pointer to the effective preference computation for the task (or an effective preference snapshot with sources)
- relevant artifacts and evidence pointers for verification (for example artifact paths and run or job identifiers)
- any policy constraints relevant to verification (for example external provider allowance)

Task scope constraint

- A Project Analyst agent MUST treat its assigned `task_id` as its only authority scope for state, evidence, and outputs.
- A Project Analyst agent MUST NOT read or write data for unrelated tasks except through explicitly allowlisted, non-task-specific tools.

LangGraph MVP

- **Orchestrator kicks off to PMA.**
  In the **Verify Step Result** node of the LangGraph workflow, **PMA tasks the Project Analyst (or another sandbox agent)** to perform verification; findings are written back into the workflow state.
  See [langgraph_mvp.md](langgraph_mvp.md) Sub-Agent Invocation section.

## External Provider Usage

The Project Analyst Agent MAY use external AI providers for verification when allowed.
External provider usage MUST be policy-controlled and audited.

### Applicable Requirements

Traces To:

- [REQ-AGENTS-0125](../requirements/agents.md#req-agents-0125)
- [REQ-AGENTS-0126](../requirements/agents.md#req-agents-0126)
- [REQ-AGENTS-0127](../requirements/agents.md#req-agents-0127)

See [`docs/tech_specs/external_model_routing.md`](external_model_routing.md) and [`docs/tech_specs/api_egress_server.md`](api_egress_server.md).

## External Provider Configuration

This section defines how to enable external AI APIs for Project Analyst agents through orchestrator configuration.
Project Analyst agents are spawned by the Project Manager Agent and SHOULD follow the same policy and preference model.

### Standalone Orchestrator Scenario

- The orchestrator may be running with zero local inference-capable nodes.
- If an external provider credential exists and policy allows it, the orchestrator MUST still be able to run Project Analyst agents using an external provider.
- In this mode, Project Analyst agents can perform verification using external inference.

### Required Configuration Steps

- Add provider credentials
  - Store the provider credential in PostgreSQL for API Egress (user-scoped).
  - See [`docs/tech_specs/api_egress_server.md`](api_egress_server.md).
- Add access control rules
  - Allow the relevant subjects to call `api.call` for the chosen provider and operations.
  - Recommended stance is default-deny with narrow allow rules.
  - See [`docs/tech_specs/access_control.md`](access_control.md).
- Set preferences for agent routing
  - Configure which providers are allowed for this agent and the preferred order.
  - Suggested keys are:
    - `agents.project_analyst.model_routing.prefer_local`
    - `agents.project_analyst.model_routing.allowed_external_providers`
    - `agents.project_analyst.model_routing.fallback_provider_order`
  - See [`docs/tech_specs/user_preferences.md`](user_preferences.md) and [`docs/tech_specs/external_model_routing.md`](external_model_routing.md).

### Bootstrap and Runtime Configuration

- These preferences and ACL rules MAY be seeded via orchestrator bootstrap YAML.
- After bootstrap, PostgreSQL remains the source of truth for changes.
- See [`docs/tech_specs/orchestrator_bootstrap.md`](orchestrator_bootstrap.md).

## Tool Access and Database Access

The Project Analyst Agent is an orchestrator-side agent.
It MUST use MCP tools for privileged operations.

### Tool Access and Database Access Applicable Requirements

- Spec ID: `CYNAI.AGENTS.ProjectAnalystToolAccess` <a id="spec-cynai-agents-patoolaccess"></a>

Traces To:

- [REQ-AGENTS-0114](../requirements/agents.md#req-agents-0114)
- [REQ-AGENTS-0115](../requirements/agents.md#req-agents-0115)

## Verification Outputs

Minimum recommended fields for recorded findings

- `task_id`
- `status`: pass|fail|needs_clarification
- `findings`: array of structured issues and evidence
- `recommended_actions`: array of remediation steps
- `preferences_applied`: preference scope identifiers and versions used for verification

## LLM Context (Baseline and User-Configurable)

The Project Analyst Agent MUST pass baseline context (identity, role, responsibilities, non-goals) to every LLM it uses and MUST include project-level and task-level context when in scope, and user-configurable additional context resolved from preferences.
See [REQ-AGENTS-0132](../requirements/agents.md#req-agents-0132), [REQ-AGENTS-0133](../requirements/agents.md#req-agents-0133), [REQ-AGENTS-0134](../requirements/agents.md#req-agents-0134), and [LLM Context (Baseline and User-Configurable)](project_manager_agent.md#spec-cynai-agents-llmcontext).
The preference key `agents.project_analyst.additional_context` supplies the user-configurable additional context for this agent.

## Preference Usage

The following requirements apply.

### Preference Usage Applicable Requirements

- Spec ID: `CYNAI.AGENTS.ProjectAnalystPreferenceUsage` <a id="spec-cynai-agents-papreferenceusage"></a>

Traces To:

- [REQ-AGENTS-0123](../requirements/agents.md#req-agents-0123)
- [REQ-AGENTS-0124](../requirements/agents.md#req-agents-0124)

See [`docs/tech_specs/user_preferences.md`](user_preferences.md).
