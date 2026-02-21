# AGENTS Requirements

- [1 Overview](#1-overview)
- [2 Requirements](#2-requirements)

## 1 Overview

This document consolidates requirements for the `AGENTS` domain.
It covers agent behaviors, responsibilities, and workflow integration.

## 2 Requirements

- **REQ-AGENTS-0001:** Cloud workers: no direct PostgreSQL; orchestrator-mediated tools only; no embedded API keys; retries/idempotency.
  [CYNAI.AGENTS.CloudWorkerContract](../tech_specs/cloud_agents.md#spec-cynai-agents-cwcontract)
  [CYNAI.AGENTS.CloudWorkerToolAccess](../tech_specs/cloud_agents.md#spec-cynai-agents-cwtoolaccess)
  <a id="req-agents-0001"></a>
- **REQ-AGENTS-0002:** Project Manager: DB via MCP only; no provider API keys; external calls via API Egress; preference precedence and recording.
  [CYNAI.AGENTS.ProjectManagerToolAccess](../tech_specs/project_manager_agent.md#spec-cynai-agents-pmtoolaccess)
  [CYNAI.AGENTS.ProjectManagerPreferenceUsage](../tech_specs/project_manager_agent.md#spec-cynai-agents-pmpreferenceusage)
  <a id="req-agents-0002"></a>
- **REQ-AGENTS-0003:** Project Analyst: PostgreSQL access only through MCP tools; effective preferences and recording.
  [CYNAI.AGENTS.ProjectAnalystToolAccess](../tech_specs/project_analyst_agent.md#spec-cynai-agents-patoolaccess)
  <a id="req-agents-0003"></a>
- **REQ-AGENTS-0004:** LangGraph checkpoint to PostgreSQL per transition; resumable; idempotent steps.
  [CYNAI.AGENTS.LanggraphCheckpointing](../tech_specs/langgraph_mvp.md#spec-cynai-agents-lgcheckpoint)
  <a id="req-agents-0004"></a>

- **REQ-AGENTS-0100:** A cloud worker MUST NOT access PostgreSQL directly.
  [CYNAI.AGENTS.CloudWorkerContract](../tech_specs/cloud_agents.md#spec-cynai-agents-cwcontract)
  <a id="req-agents-0100"></a>
- **REQ-AGENTS-0101:** A cloud worker MUST use orchestrator-mediated tool access and MUST NOT embed provider API keys.
  [CYNAI.AGENTS.CloudWorkerContract](../tech_specs/cloud_agents.md#spec-cynai-agents-cwcontract)
  <a id="req-agents-0101"></a>
- **REQ-AGENTS-0102:** A cloud worker MUST support job retries and idempotency semantics as defined by the orchestrator.
  [CYNAI.AGENTS.CloudWorkerContract](../tech_specs/cloud_agents.md#spec-cynai-agents-cwcontract)
  <a id="req-agents-0102"></a>
- **REQ-AGENTS-0103:** Cloud workers MUST call tools through the orchestrator MCP gateway.
  [CYNAI.AGENTS.CloudWorkerToolAccess](../tech_specs/cloud_agents.md#spec-cynai-agents-cwtoolaccess)
  <a id="req-agents-0103"></a>
- **REQ-AGENTS-0104:** Cloud workers MUST NOT call arbitrary outbound network endpoints directly unless explicitly allowed by policy.
  [CYNAI.AGENTS.CloudWorkerToolAccess](../tech_specs/cloud_agents.md#spec-cynai-agents-cwtoolaccess)
  <a id="req-agents-0104"></a>
- **REQ-AGENTS-0105:** Cloud workers MUST use API Egress for external API calls.
  [CYNAI.AGENTS.CloudWorkerToolAccess](../tech_specs/cloud_agents.md#spec-cynai-agents-cwtoolaccess)
  <a id="req-agents-0105"></a>
- **REQ-AGENTS-0106:** The Project Manager Agent MUST NOT store provider API keys.
  [CYNAI.AGENTS.PMExternalProvider](../tech_specs/project_manager_agent.md#spec-cynai-agents-pmexternalprovider)
  <a id="req-agents-0106"></a>
- **REQ-AGENTS-0107:** External model calls MUST be routed through the API Egress Server.
  [CYNAI.AGENTS.PMExternalProvider](../tech_specs/project_manager_agent.md#spec-cynai-agents-pmexternalprovider)
  <a id="req-agents-0107"></a>
- **REQ-AGENTS-0108:** The Project Manager Agent SHOULD prefer local execution when it satisfies capability and policy constraints.
  [CYNAI.AGENTS.PMExternalProvider](../tech_specs/project_manager_agent.md#spec-cynai-agents-pmexternalprovider)
  <a id="req-agents-0108"></a>
- **REQ-AGENTS-0109:** All PostgreSQL access MUST happen through MCP tools.
  [CYNAI.AGENTS.ProjectManagerToolAccess](../tech_specs/project_manager_agent.md#spec-cynai-agents-pmtoolaccess)
  <a id="req-agents-0109"></a>
- **REQ-AGENTS-0110:** The Project Manager Agent MUST NOT connect directly to PostgreSQL.
  [CYNAI.AGENTS.ProjectManagerToolAccess](../tech_specs/project_manager_agent.md#spec-cynai-agents-pmtoolaccess)
  <a id="req-agents-0110"></a>
- **REQ-AGENTS-0111:** The Project Manager Agent MUST load applicable preferences before planning and before final verification.
  Applicable preferences MUST be resolved deterministically using the scope precedence rules defined in [`docs/tech_specs/user_preferences.md`](../tech_specs/user_preferences.md).
  Invalid preference entries (for example mismatched `value_type`) MUST be skipped and MUST NOT override lower-precedence valid entries.
  [CYNAI.AGENTS.ProjectManagerPreferenceUsage](../tech_specs/project_manager_agent.md#spec-cynai-agents-pmpreferenceusage)
  <a id="req-agents-0111"></a>
- **REQ-AGENTS-0112:** Preference precedence SHOULD be: task-specific > project-specific > user-default > group-default > system-default.
  [CYNAI.AGENTS.ProjectManagerPreferenceUsage](../tech_specs/project_manager_agent.md#spec-cynai-agents-pmpreferenceusage)
  <a id="req-agents-0112"></a>
- **REQ-AGENTS-0113:** The Project Manager Agent MUST record which preference set was applied for verification.
  The recorded applied preference set MUST be sufficient to reproduce the effective preference result, including the source scope and entry versions (or equivalent stable identifiers) for keys that were applied.
  [CYNAI.AGENTS.ProjectManagerPreferenceUsage](../tech_specs/project_manager_agent.md#spec-cynai-agents-pmpreferenceusage)
  <a id="req-agents-0113"></a>
- **REQ-AGENTS-0114:** All PostgreSQL access MUST happen through MCP tools (Project Analyst).
  [CYNAI.AGENTS.ProjectAnalystToolAccess](../tech_specs/project_analyst_agent.md#spec-cynai-agents-patoolaccess)
  <a id="req-agents-0114"></a>
- **REQ-AGENTS-0115:** The Project Analyst Agent MUST NOT connect directly to PostgreSQL.
  [CYNAI.AGENTS.ProjectAnalystToolAccess](../tech_specs/project_analyst_agent.md#spec-cynai-agents-patoolaccess)
  <a id="req-agents-0115"></a>
- **REQ-AGENTS-0116:** The workflow MUST checkpoint progress to PostgreSQL after each node transition.
  [CYNAI.AGENTS.LanggraphCheckpointing](../tech_specs/langgraph_mvp.md#spec-cynai-agents-lgcheckpoint)
  <a id="req-agents-0116"></a>
- **REQ-AGENTS-0117:** The workflow MUST be resumable after orchestrator restarts.
  [CYNAI.AGENTS.LanggraphCheckpointing](../tech_specs/langgraph_mvp.md#spec-cynai-agents-lgcheckpoint)
  <a id="req-agents-0117"></a>
- **REQ-AGENTS-0118:** Each step attempt MUST be idempotent or have idempotency keys.
  [CYNAI.AGENTS.LanggraphCheckpointing](../tech_specs/langgraph_mvp.md#spec-cynai-agents-lgcheckpoint)
  <a id="req-agents-0118"></a>

- **REQ-AGENTS-0119:** The Project Manager Agent SHOULD be able to pair external inference with sandbox execution on a node when tool runs are required.
  [CYNAI.AGENTS.PMExternalProvider](../tech_specs/project_manager_agent.md#spec-cynai-agents-pmexternalprovider)
  <a id="req-agents-0119"></a>
- **REQ-AGENTS-0120:** Sub-agents MUST operate within the same standards and preference precedence rules as the Project Manager Agent.
  [CYNAI.AGENTS.ProjectManagerSubAgent](../tech_specs/project_manager_agent.md#spec-cynai-agents-pmsubagent)
  <a id="req-agents-0120"></a>
- **REQ-AGENTS-0121:** Sub-agents MUST write their findings and recommended remediation steps back to PostgreSQL.
  [CYNAI.AGENTS.ProjectManagerSubAgent](../tech_specs/project_manager_agent.md#spec-cynai-agents-pmsubagent)
  <a id="req-agents-0121"></a>
- **REQ-AGENTS-0122:** Sub-agents SHOULD be scoped to a single task to avoid cross-task state leakage.
  [CYNAI.AGENTS.ProjectManagerSubAgent](../tech_specs/project_manager_agent.md#spec-cynai-agents-pmsubagent)
  <a id="req-agents-0122"></a>
- **REQ-AGENTS-0130:** The Project Manager Agent SHOULD eagerly delegate eligible tasks to Project Analyst agents for task-scoped monitoring and verification.
  Delegation SHOULD occur as soon as a task is runnable and the system has capacity, subject to policy constraints and configured concurrency limits.
  [CYNAI.AGENTS.ProjectManagerSubAgent](../tech_specs/project_manager_agent.md#spec-cynai-agents-pmsubagent)
  [project_analyst_agent.md](../tech_specs/project_analyst_agent.md)
  <a id="req-agents-0130"></a>
- **REQ-AGENTS-0123:** The Project Analyst Agent MUST compute and use effective preferences for the task using the same precedence rules as the Project Manager Agent.
  The Project Analyst Agent MUST treat unknown keys as opaque and MUST pass them through to verification and reporting without interpretation.
  [CYNAI.AGENTS.ProjectAnalystPreferenceUsage](../tech_specs/project_analyst_agent.md#spec-cynai-agents-papreferenceusage)
  <a id="req-agents-0123"></a>
- **REQ-AGENTS-0124:** The Project Analyst Agent MUST record which preference set was applied for verification.
  The recorded applied preference set MUST be sufficient to reproduce the effective preference result, including the source scope and entry versions (or equivalent stable identifiers) for keys that were applied.
  [CYNAI.AGENTS.ProjectAnalystPreferenceUsage](../tech_specs/project_analyst_agent.md#spec-cynai-agents-papreferenceusage)
  <a id="req-agents-0124"></a>
- **REQ-AGENTS-0125:** The Project Analyst Agent MUST NOT store provider API keys.
  [project_analyst_agent.md](../tech_specs/project_analyst_agent.md)
  <a id="req-agents-0125"></a>
- **REQ-AGENTS-0126:** External model calls for Project Analyst verification MUST be routed through the API Egress Server.
  [project_analyst_agent.md](../tech_specs/project_analyst_agent.md)
  <a id="req-agents-0126"></a>
- **REQ-AGENTS-0127:** The Project Analyst Agent SHOULD prefer local execution when it satisfies capability and policy constraints.
  [project_analyst_agent.md](../tech_specs/project_analyst_agent.md)
  <a id="req-agents-0127"></a>
- **REQ-AGENTS-0128:** For the MVP, the Project Manager model MUST be the single decision-maker for all inference task assignments.
  This includes selecting local vs external inference targets, selecting the model/version, and requesting local model loads when required.
  [project_manager_agent.md](../tech_specs/project_manager_agent.md)
  [model_management.md](../tech_specs/model_management.md)
  [external_model_routing.md](../tech_specs/external_model_routing.md)
  <a id="req-agents-0128"></a>
- **REQ-AGENTS-0129:** The Project Manager model MUST assign each task a human-readable task name in addition to its UUID, so users can call or refer to the task by name or by UUID.
  Task names MUST be all lowercase with dashes for word separation and MAY use trailing numbers for uniqueness (e.g. `deploy-docs`, `deploy-docs-2`).
  [CYNAI.AGENTS.ProjectManagerTaskNaming](../tech_specs/project_manager_agent.md#spec-cynai-agents-pmtasknaming)
  <a id="req-agents-0129"></a>
