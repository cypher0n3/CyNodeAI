# MCP Gateway Enforcement and Tool Allowlists

- [Spec IDs](#spec-ids)
- [Document Overview](#document-overview)
- [Goals](#goals)
- [Standard MCP Usage](#standard-mcp-usage)
- [Gateway Enforcement Responsibilities](#gateway-enforcement-responsibilities)
- [Role-Based Tool Allowlists](#role-based-tool-allowlists)
  - [Worker Agent Allowlist](#worker-agent-allowlist)
  - [Project Manager Agent Allowlist](#project-manager-agent-allowlist)
  - [Project Analyst Agent Allowlist](#project-analyst-agent-allowlist)
- [Tool Argument Schema Requirements](#tool-argument-schema-requirements)
- [Access Control Mapping](#access-control-mapping)
- [Auditing Requirements](#auditing-requirements)
- [Compatibility and Versioning](#compatibility-and-versioning)

## Spec IDs

- Spec ID: `CYNAI.MCPGAT.Doc.GatewayEnforcement` <a id="spec-cynai-mcpgat-doc-gatewayenforcement"></a>

This section defines stable Spec ID anchors for referencing this document.

## Document Overview

This document defines how CyNodeAI enforces policy and auditing for MCP tool calls.
CyNodeAI uses the standard MCP protocol on the wire.
The orchestrator MCP gateway is the enforcement and audit point.

Related documents

- MCP concepts: [`docs/tech_specs/mcp_tooling.md`](mcp_tooling.md)
- MCP tool catalog: [`docs/tech_specs/mcp_tool_catalog.md`](mcp_tool_catalog.md)
- Access control and auditing: [`docs/tech_specs/access_control.md`](access_control.md)
- Tool call audit storage: [`docs/tech_specs/mcp_tool_call_auditing.md`](mcp_tool_call_auditing.md)
- Git egress tool patterns: [`docs/tech_specs/git_egress_mcp.md`](git_egress_mcp.md)

## Goals

- Maximize compatibility with existing MCP clients and configurations.
- Enforce allowlists, access control, and auditing centrally at the orchestrator gateway.
- Make task scoping deterministic using strict tool argument schemas for task-scoped tools.

## Standard MCP Usage

This section describes how CyNodeAI uses MCP without extending MCP wire messages.

### Applicable Requirements (Standard MCP Usage)

- Spec ID: `CYNAI.MCPGAT.StandardMcpUsage` <a id="spec-cynai-mcpgat-stdmcpusage"></a>

Traces To:

- [REQ-MCPGAT-0100](../requirements/mcpgat.md#req-mcpgat-0100)
- [REQ-MCPGAT-0101](../requirements/mcpgat.md#req-mcpgat-0101)
- [REQ-MCPGAT-0102](../requirements/mcpgat.md#req-mcpgat-0102)

## Gateway Enforcement Responsibilities

The orchestrator MCP gateway MUST perform the following for every tool call it routes.

- Resolve identity and context from orchestrator state.
  This includes user identity, RBAC context, task context, and run or job context when available.
- Enforce role-based tool allowlists (coarse gating).
- Enforce access control policy (`access_control_rules`) using `mcp.tool.invoke`.
- Enforce request and response limits.
  This includes size limits, timeouts, and schema validation for tool responses when defined.
- Emit audit records for the decision and outcome.

The orchestrator MUST fail closed.
If required context is missing for a tool call, the call MUST be rejected.

## Edge Enforcement Mode (Node-Local Agent Runtimes)

- Spec ID: `CYNAI.MCPGAT.EdgeEnforcementMode` <a id="spec-cynai-mcpgat-edgeenforcement"></a>

Traces To:

- [REQ-MCPGAT-0112](../requirements/mcpgat.md#req-mcpgat-0112)

This section defines an edge enforcement mode for tool calls that are not routed by the orchestrator MCP gateway.
The primary use case is a node-local agent runtime interacting directly with a node-local MCP server on the same host for low-latency sandbox operations.

Requirements

- Edge enforcement MUST NOT extend MCP wire messages.
  Task scoping MUST still be expressed in tool arguments, consistent with `Tool Argument Schema Requirements`.
- Edge enforcement MUST be authorized using orchestrator-issued capability leases.
  Leases MUST be short-lived, least-privilege, and MUST scope allowed tool identities and required context (for example `task_id`).
- The node-local MCP server (or an edge enforcement proxy colocated with it) MUST validate leases, enforce tool allowlists, and MUST fail closed.
- The node-local MCP server MUST emit audit records for edge-routed tool calls with the same minimum audit fields used by the orchestrator gateway.
  See `docs/tech_specs/mcp_tool_call_auditing.md`.

## Role-Based Tool Allowlists

Allowlists are a coarse safety control.
They define which tool namespaces are eligible for routing for a given agent role.

Allowlists are not sufficient for authorization.
Fine-grained authorization MUST still be enforced by access control policy rules.

### Worker Agent Allowlist

Worker agents run in sandbox containers and SHOULD have the minimal tool surface needed to complete dispatched work.

Recommended allowlist

- `artifact.*` (scoped to current task)
- `web.fetch` (sanitized, when allowed by policy)
- `api.call` (through API Egress, when explicitly allowed for the task)

Explicitly disallowed

- `db.*`
- `node.*`
- `sandbox.*` (worker is already inside a sandbox)

### Project Manager Agent Allowlist

Recommended allowlist

- `db.*` (tasks, jobs, preferences, routing metadata)
- `node.*` (capabilities, status, config refresh)
- `sandbox.*` (create, exec, file transfer, logs, destroy)
- `artifact.*`
- `model.*` (registry and availability)
- `connector.*` (management and invocation, subject to policy)
- `web.fetch` (sanitized, subject to policy)
- `api.call` (through API Egress, subject to policy)
- `git.*` (through Git egress, subject to policy)

### Project Analyst Agent Allowlist

Recommended allowlist

- `db.read` and limited `db.write` (verification findings only)
- `artifact.*` (read for produced outputs)
- `web.fetch` (sanitized, when allowed for verification)
- `api.call` (through API Egress, when allowed for verification)

## Tool Argument Schema Requirements

Because CyNodeAI does not extend MCP wire messages, task scoping MUST be expressed in tool arguments.

### Applicable Requirements (Tool Argument Schemas)

- Spec ID: `CYNAI.MCPGAT.ToolArgumentSchema` <a id="spec-cynai-mcpgat-toolargschema"></a>

Traces To:

- [REQ-MCPGAT-0103](../requirements/mcpgat.md#req-mcpgat-0103)
- [REQ-MCPGAT-0104](../requirements/mcpgat.md#req-mcpgat-0104)
- [REQ-MCPGAT-0105](../requirements/mcpgat.md#req-mcpgat-0105)
- [REQ-MCPGAT-0106](../requirements/mcpgat.md#req-mcpgat-0106)

Recommended guidance

- Prefer stable, explicit ids (`task_id`, `run_id`, `job_id`) over implicit scoping.
- Prefer allowlisted tool patterns over dynamic tool discovery.

## Access Control Mapping

Access control SHOULD be defined via [`docs/tech_specs/access_control.md`](access_control.md).

Recommended mapping for tool calls

- `action`: `mcp.tool.invoke`
- `resource_type`: `mcp.tool`
- `resource_pattern`: tool identity pattern
  - examples: `sandbox.*`, `git.pr.create`, `api.call`

## Auditing Requirements

All MCP tool calls routed by the orchestrator MUST be audited with task context when applicable.

Minimum audit fields (recommended)

- subject identity (user_id, group_ids, role_names) when applicable
- tool identity (server and tool name)
- `task_id` and `project_id` when applicable
- decision allow or deny
- status success or error
- timing and error status

## Compatibility and Versioning

- Changes to allowlists and policy MUST be applied centrally at the gateway.
- Tool argument schemas MAY evolve by adding optional fields.
- Required fields MUST NOT change meaning without a versioned tool name or tool schema versioning mechanism.
