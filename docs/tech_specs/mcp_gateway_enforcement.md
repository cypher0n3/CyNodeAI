# MCP Gateway Enforcement and Tool Allowlists

- [Spec IDs](#spec-ids)
- [Document Overview](#document-overview)
- [Goals](#goals)
- [Standard MCP Usage](#standard-mcp-usage)
  - [Applicable Requirements (Standard MCP Usage)](#applicable-requirements-standard-mcp-usage)
- [Gateway Enforcement Responsibilities](#gateway-enforcement-responsibilities)
- [Agent-Scoped Tokens or API Keys](#agent-scoped-tokens-or-api-keys)
- [Edge Enforcement Mode (Node-Local Agent Runtimes)](#edge-enforcement-mode-node-local-agent-runtimes)
- [Role-Based Tool Allowlists](#role-based-tool-allowlists)
  - [Worker Agent Allowlist](#worker-agent-allowlist)
  - [Project Manager Agent Allowlist](#project-manager-agent-allowlist)
  - [Project Analyst Agent Allowlist](#project-analyst-agent-allowlist)
- [Per-Tool Scope: Sandbox vs PM](#per-tool-scope-sandbox-vs-pm)
- [Admin-Configurable Per-Tool Enable/Disable](#admin-configurable-per-tool-enabledisable)
- [Tool Argument Schema Requirements](#tool-argument-schema-requirements)
  - [Applicable Requirements (Tool Argument Schemas)](#applicable-requirements-tool-argument-schemas)
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

- Resolve identity and context.
  Identity and agent type MAY be resolved from an **agent-scoped token or API key** presented with the MCP request (see [Agent-Scoped Tokens or API Keys](#agent-scoped-tokens-or-api-keys)); otherwise from orchestrator state.
  Resolved context MUST include agent type (PM, PA, or sandbox), and when available user identity, task context, and run or job context.
- Enforce role-based tool allowlists (coarse gating) and per-tool scope using the resolved agent type.
- Enforce access control policy (`access_control_rules`) using `mcp.tool.invoke` when applicable.
- Enforce request and response limits.
  This includes size limits, timeouts, and schema validation for tool responses when defined.
- Emit audit records for the decision and outcome.

The orchestrator MUST fail closed.
If required context is missing for a tool call, the call MUST be rejected.

## Agent-Scoped Tokens or API Keys

- Spec ID: `CYNAI.MCPGAT.AgentScopedTokens` <a id="spec-cynai-mcpgat-agentscopedtokens"></a>

Tool access for PM/PA and sandbox agents MAY be controlled by **agent-scoped tokens or API keys** instead of (or in addition to) resolving identity from orchestrator state.
This avoids the need for a separate RBAC spec for agents when the goal is only to restrict which tools an agent can call.

Issuance

- The orchestrator MUST be able to **issue tokens or API keys** for use by PM/PA agents and by sandbox agents when they make MCP requests.
- A **PM or PA agent token** (or key) is issued when the orchestrator starts or hands off to the agent (e.g. when handling a chat completion for `cynodeai.pm`).
  The token MUST be associated with at least: agent type (PM or PA), and user context (the user on whose behalf the agent is acting).
  The token MAY also carry or be bound to task_id, project_id, or session scope.
- A **sandbox agent token** (or key) is issued when a sandbox job or agent context is created (e.g. when the orchestrator dispatches work to a node or when cynode-sba runs in agent mode).
  The token MUST be associated with at least: agent type (sandbox), and task/job context (task_id, job_id, and user or project when available).

Use at the gateway

- When an MCP request includes an agent-scoped token or API key, the gateway MUST **authenticate** the request using that credential (e.g. validate signature or lookup in a credential store) and MUST resolve from it: **agent type** (PM, PA, or sandbox) and **user/task context** as stored at issuance.
- The gateway MUST then restrict tool access to the **allowlist and per-tool scope for that agent type**.
  For example: a token issued for a PM agent allows only tools on the Project Manager allowlist with scope PM or both; a token issued for a sandbox agent allows only tools on the Worker allowlist with scope sandbox or both.
  No separate RBAC evaluation is required for agent-type restriction; the token itself conveys agent type.
- Access control rules (`mcp.tool.invoke` with subject/resource) MAY still be applied using the resolved user/task context when the system uses RBAC for user-level policy; the token does not replace user-level policy when that is present, but it is sufficient for restricting tools by agent type.

Audit

- Audit records for tool calls made with an agent-scoped token MUST include the resolved agent type and the user/task context derived from the token so that tool use remains attributable.

Traces To:

- [REQ-MCPGAT-0116](../requirements/mcpgat.md#req-mcpgat-0116)

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

- Spec ID: `CYNAI.MCPGAT.WorkerAgentAllowlist` <a id="spec-cynai-mcpgat-workeragentallowlist"></a>

Worker agents run in sandbox containers and SHOULD have the minimal tool surface needed to complete dispatched work.

Recommended allowlist

- `artifact.*` (scoped to current task)
- `skills.list`, `skills.get` (read-only; when allowed by policy, so the SBA can fetch relevant skills via the gateway)
- `web.fetch` (sanitized, when allowed by policy)
- `web.search` (secure web search, when allowed by policy)
- `api.call` (through API Egress, when explicitly allowed for the task)
- `help.*` (on-demand docs; optional for worker)

Explicitly disallowed

- `db.*`
- `node.*`
- `sandbox.*` (worker is already inside a sandbox)

### Project Manager Agent Allowlist

- Spec ID: `CYNAI.MCPGAT.PmAgentAllowlist` <a id="spec-cynai-mcpgat-pmagentallowlist"></a>

Recommended allowlist

- `db.*` (tasks, jobs, preferences, routing metadata)
- `node.*` (capabilities, status, config refresh)
- `sandbox.*` (create, exec, file transfer, logs, destroy; and when enabled by system setting, `sandbox.allowed_images.list`, `sandbox.allowed_images.add`)
  - For `sandbox.allowed_images.add`, the gateway MUST allow the call only when the system setting `agents.project_manager.sandbox.allow_add_to_allowed_images` is `true`; when `false` (default), the gateway MUST reject the call.
  - `sandbox.allowed_images.list` is allowed for the PM agent regardless of that setting.
- `artifact.*`
- `model.*` (registry and availability)
- `connector.*` (management and invocation, subject to policy)
- `web.fetch` (sanitized, subject to policy)
- `web.search` (secure web search, subject to policy)
- `api.call` (through API Egress, subject to policy)
- `git.*` (through Git egress, subject to policy)
- `help.*` (on-demand docs)

### Project Analyst Agent Allowlist

- Spec ID: `CYNAI.MCPGAT.PaAgentAllowlist` <a id="spec-cynai-mcpgat-paagentallowlist"></a>

Recommended allowlist

- `db.read` and limited `db.write` (verification findings only)
- `artifact.*` (read for produced outputs)
- `web.fetch` (sanitized, when allowed for verification)
- `web.search` (secure web search, when allowed for verification)
- `api.call` (through API Egress, when allowed for verification)
- `help.*` (on-demand docs)

## Per-Tool Scope: Sandbox vs PM

- Spec ID: `CYNAI.MCPGAT.PerToolScope` <a id="spec-cynai-mcpgat-pertoolscope"></a>

The orchestrator MUST track for each MCP tool whether the tool is available to **sandbox agents**, **PM (orchestrator-side) agents**, or **both**.
This scope is used in addition to role-based allowlists so that sandbox agents never receive PM-only tools and PM/PA agents never receive sandbox-only tools unless the tool is explicitly marked for both.

Enforcement rules

- When the caller is a **sandbox agent** (worker/cynode-sba in agent mode), the gateway MUST allow the tool call only if the tool is enabled and the tool's scope includes **sandbox** (or **both**).
  The gateway MUST reject calls to tools that are PM-only.
- When the caller is a **PM or Project Analyst agent**, the gateway MUST allow the tool call only if the tool is enabled and the tool's scope includes **PM** (or **both**).
  The gateway MUST reject calls to tools that are sandbox-only.

Built-in tools

- Built-in tools in the [Worker Agent allowlist](#worker-agent-allowlist) MUST be registered with scope **sandbox** (or **both** if they are also on a PM allowlist).
- Built-in tools in the [Project Manager Agent allowlist](#project-manager-agent-allowlist) and [Project Analyst Agent allowlist](#project-analyst-agent-allowlist) MUST be registered with scope **PM**.
  Tools that appear on both worker and PM allowlists (e.g. `help.*`) MAY be registered as **both**.

User-installed (custom) MCP tools

- User-installable MCP tools (registration, per-tool scope configuration, persistence, Web Console and CLI exposure) are defined in a dedicated spec.
  Users MUST be able to install their own MCP tools and configure per-tool scope (sandbox only, PM only, or both); the orchestrator MUST persist that scope and the gateway MUST use it when enforcing the rules above.
  See [User-Installable MCP Tools](user_installable_mcp_tools.md).

Traces To:

- [REQ-MCPGAT-0114](../requirements/mcpgat.md#req-mcpgat-0114)
- [REQ-MCPGAT-0115](../requirements/mcpgat.md#req-mcpgat-0115)

## Admin-Configurable Per-Tool Enable/Disable

- Spec ID: `CYNAI.MCPGAT.AdminPerToolEnableDisable` <a id="spec-cynai-mcpgat-adminpertoolenabledisable"></a>

Admins MUST be able to turn individual MCP tools on or off.

Traces To:

- [REQ-MCPGAT-0113](../requirements/mcpgat.md#req-mcpgat-0113)

- The system MUST support admin-configurable enable/disable per tool (by canonical tool name, e.g. `web.fetch`, `sandbox.create`, `git.push`).
- When a tool is disabled, the MCP gateway MUST reject invocations of that tool regardless of role allowlist or access control allow rules.
- When a tool is enabled (or not listed as disabled), normal role allowlist and access control evaluation apply.
- Configuration MAY be stored in system settings (e.g. `mcp.tools.disabled` as an array of tool names, or per-tool keys such as `mcp.tool.<tool_name>.enabled`) and/or enforced via access control rules (e.g. deny rules for specific tools).
- The Web Console and CLI management app MUST expose the ability for admins to view and change per-tool enable/disable state, consistent with [client capability parity](cynork_cli.md) and [Web Console](web_console.md).

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
