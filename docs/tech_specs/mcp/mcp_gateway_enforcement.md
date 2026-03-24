# MCP Gateway Enforcement

- [Spec IDs](#spec-ids)
- [Document Overview](#document-overview)
- [Goals](#goals)
- [Standard MCP Usage](#standard-mcp-usage)
  - [Applicable Requirements (Standard MCP Usage)](#applicable-requirements-standard-mcp-usage)
- [Gateway Enforcement Responsibilities](#gateway-enforcement-responsibilities)
- [Agent-Scoped Tokens or API Keys](#agent-scoped-tokens-or-api-keys)
  - [Token Handling (Normative)](#token-handling-normative)
  - [PMA Per-User Session Tokens and Rotation](#pma-per-user-session-tokens-and-rotation)
  - [PMA Invocation Class (Interactive vs Orchestrator-Initiated)](#pma-invocation-class-interactive-vs-orchestrator-initiated)
- [Edge Enforcement Mode (Node-Local Agent Runtimes)](#edge-enforcement-mode-node-local-agent-runtimes)
  - [Edge Enforcement Mode (Node-Local Agent Runtimes) Requirements Traces](#edge-enforcement-mode-node-local-agent-runtimes-requirements-traces)
- [Role Allowlists, Per-Tool Scope, and Admin Enable or Disable](#role-allowlists-per-tool-scope-and-admin-enable-or-disable)
- [Tool Argument Schema Requirements](#spec-cynai-mcpgat-toolargschema)
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

**Deployment:** Gateway enforcement is implemented as HTTP routes on the **orchestrator control-plane** process.
A **standalone** gateway listener on dedicated port **12083** (historical `cmd/mcp-gateway`) is **deprecated**; **do not use** it for new work or new stacks.
Remove it from compose when that file is next edited; PMA and SBA rely on the control-plane route.
See [Ports and Endpoints](../ports_and_endpoints.md) and CYNAI.PMAGNT.McpToolAccess in [`cynode_pma.md`](../cynode_pma.md).

Related documents

- MCP concepts: [`docs/tech_specs/mcp/mcp_tooling.md`](mcp_tooling.md)
- Canonical tool specs, allowlists, and per-tool scope: [`docs/tech_specs/mcp_tools/`](../mcp_tools/README.md) (see [Access, allowlists, and per-tool scope](../mcp_tools/access_allowlists_and_scope.md))
- Access control and auditing: [`docs/tech_specs/access_control.md`](../access_control.md)
- Tool call audit storage: [`docs/tech_specs/mcp/mcp_tool_call_auditing.md`](mcp_tool_call_auditing.md)
- Git egress tool patterns: [`docs/tech_specs/mcp_tools/git_egress.md`](../mcp_tools/git_egress.md)

## Goals

- Maximize compatibility with existing MCP clients and configurations.
- Enforce allowlists, access control, and auditing centrally at the orchestrator gateway.
- Make task scoping deterministic using strict tool argument schemas for task-scoped tools.

## Standard MCP Usage

This section describes how CyNodeAI uses MCP without extending MCP wire messages.

### Applicable Requirements (Standard MCP Usage)

- Spec ID: `CYNAI.MCPGAT.StandardMcpUsage` <a id="spec-cynai-mcpgat-stdmcpusage"></a>

#### Traces to Requirements

- [REQ-MCPGAT-0100](../../requirements/mcpgat.md#req-mcpgat-0100)
- [REQ-MCPGAT-0101](../../requirements/mcpgat.md#req-mcpgat-0101)
- [REQ-MCPGAT-0102](../../requirements/mcpgat.md#req-mcpgat-0102)

## Gateway Enforcement Responsibilities

The orchestrator MCP gateway MUST perform the following for every tool call it routes.

- Resolve identity and context.
  Identity and agent type MAY be resolved from an **agent-scoped token or API key** presented with the MCP request (see [Agent-Scoped Tokens or API Keys](#agent-scoped-tokens-or-api-keys)); otherwise from orchestrator state.
  Resolved context MUST include agent type (PM, PA, or sandbox), and when available user identity, task context, and run or job context.
  For **PMA**, resolved context MUST include the credential's **invocation class** (see [PMA Invocation Class (Interactive vs Orchestrator-Initiated)](#pma-invocation-class-interactive-vs-orchestrator-initiated)).
- Enforce role-based tool allowlists (coarse gating) and per-tool scope using the resolved agent type.
- Enforce access control policy (`access_control_rules`) using `mcp.tool.invoke` when applicable.
- Enforce request and response limits.
  This includes size limits, timeouts, and schema validation for tool responses when defined.
- Emit audit records for the decision and outcome.
- Enforce per-tool specifications that forbid **ADMIN**-level invocation through MCP (for example [Skills MCP Tools](../mcp_tools/skills_tools.md)); the gateway MUST reject such tool calls when resolved authorization would execute them with ADMIN-level privileges for that tool surface.

The orchestrator MUST fail closed.
If required context is missing for a tool call, the call MUST be rejected.

## Agent-Scoped Tokens or API Keys

- Spec ID: `CYNAI.MCPGAT.AgentScopedTokens` <a id="spec-cynai-mcpgat-agentscopedtokens"></a>

Tool access for PM/PA and sandbox agents MAY be controlled by **agent-scoped tokens or API keys** instead of (or in addition to) resolving identity from orchestrator state.
This avoids the need for a separate RBAC spec for agents when the goal is only to restrict which tools an agent can call.

### Token Handling (Normative)

- Spec ID: `CYNAI.MCPGAT.AgentTokensWorkerProxyOnly` <a id="spec-cynai-mcpgat-agenttokensworkerproxyonly"></a>

Agents MUST NOT be given tokens or secrets directly.
The orchestrator delivers agent tokens to the **worker** (e.g. in node configuration); the **worker proxy** holds them and attaches the appropriate token when forwarding agent-originated requests to the MCP gateway.
The agent never sees or presents the token; the gateway receives requests from the worker proxy with the token already attached.

Traces To: [REQ-WORKER-0164](../../requirements/worker.md#req-worker-0164).

#### Token Issuance

- The orchestrator MUST be able to **issue tokens or API keys** for use when the **worker proxy** forwards MCP requests on behalf of PM/PA or sandbox agents.
  Tokens are delivered to the worker (e.g. in managed service desired state or job payload); the worker proxy MUST hold and use them; the worker MUST NOT pass tokens or secrets to agent containers or to agents.
- A **PMA MCP credential** (Project Manager Agent) is issued by the orchestrator for an **authenticated user** and a **session** (for example a chat thread or orchestrator-defined session identifier) so that concurrent user work on the same node does not share the same gateway authorization context.
  The credential MUST be associated with agent type (PM), with **user_id**, with an **invocation class** (see [PMA Invocation Class (Interactive vs Orchestrator-Initiated)](#pma-invocation-class-interactive-vs-orchestrator-initiated)), and with session binding metadata the orchestrator defines.
  The gateway MUST resolve **user identity** from a valid PMA MCP credential and MUST reject MCP tool calls where resolved tool or resource scope implies a **different user** than the credential (fail closed).
  Credential **expiry** and **rotation** (including an overlap window where both previous and successor credentials validate) are defined in [PMA Per-User Session Tokens and Rotation](#pma-per-user-session-tokens-and-rotation).
- A **PA agent token** (Project Analyst Agent, PAA) is issued when the orchestrator hands off to the PA agent for a user- or task-scoped interaction.
  The token MUST be associated with agent type (PA) and with **the user on whose behalf the agent is acting**.
  The orchestrator MUST track this association so the gateway can resolve user context for preferences, access control to user- and project-scoped data, and audit attribution.
  The token MAY also carry or be bound to task_id, project_id, or session scope.
- A **sandbox agent token** (SBA) is issued when a sandbox job or agent context is created (e.g. when the orchestrator dispatches work to a node or when cynode-sba runs in agent mode).
  The token MUST be associated with at least: agent type (sandbox), and task/job context (task_id, job_id, and **user** or project when available).
  The orchestrator MUST associate the token with the user (e.g. task creator) so the gateway can resolve user context for preferences, access control, and audit attribution.
  The token MUST also be bound to task_id, project_id, and session scope.
  The orchestrator MUST invalidate the token when the job is stopped or canceled; see [Task Cancel and Stop Job](../orchestrator.md#spec-cynai-orches-taskcancelandstopjob).

#### Token Use at the Gateway

- Requests that carry an agent-scoped token or API key arrive from the **worker proxy**, which attaches the token when forwarding on behalf of an agent; the agent does not present the token.
  The gateway MUST **authenticate** the request using that credential (e.g. validate signature or lookup in a credential store) and MUST resolve from it: **agent type** (PM, PA, or sandbox), **user/task context** as stored at issuance (if any), and for **PMA** the **invocation class**.
  For **PMA, PA, and sandbox** tokens, the gateway MUST use the resolved user context for preference resolution, access control to user- and project-scoped resources, and audit attribution when the token binds a user (PMA, PA, sandbox per [Token Issuance](#token-issuance)).
- The gateway MUST **reject** requests that present a token the orchestrator has **invalidated** (e.g. for a stopped or canceled job).
  See [Task Cancel and Stop Job](../orchestrator.md#spec-cynai-orches-taskcancelandstopjob).
- The gateway MUST then restrict tool access to the **allowlist and per-tool scope for that agent type**.
  For example: a token issued for a PM agent allows only tools on the Project Manager allowlist with scope PM or both; a token issued for a sandbox agent allows only tools on the Worker allowlist with scope sandbox or both.
  No separate RBAC evaluation is required for agent-type restriction; the token itself conveys agent type.
- Access control rules (`mcp.tool.invoke` with subject/resource) MAY still be applied using the resolved user/task context when the system uses RBAC for user-level policy; the token does not replace user-level policy when that is present, but it is sufficient for restricting tools by agent type.

Audit

- Audit records for tool calls made with an agent-scoped token MUST include the resolved agent type and, when the token is user-associated (PMA, PA, sandbox), the user/task context derived from the token so that tool use remains attributable.
  For **PMA**, audit records MUST also include the resolved **invocation class**.

##### Token Use at the Gateway Requirements Traces

- [REQ-MCPGAT-0116](../../requirements/mcpgat.md#req-mcpgat-0116)
- [REQ-MCPGAT-0117](../../requirements/mcpgat.md#req-mcpgat-0117)
- [REQ-MCPGAT-0118](../../requirements/mcpgat.md#req-mcpgat-0118)
- [REQ-WORKER-0164](../../requirements/worker.md#req-worker-0164)
- [REQ-WORKER-0175](../../requirements/worker.md#req-worker-0175)

#### PMA Per-User Session Tokens and Rotation

- Spec ID: `CYNAI.MCPGAT.PmaSessionTokens` <a id="spec-cynai-mcpgat-pmasessiontokens"></a>

This section normatively defines PMA MCP credentials as **per-user session** credentials (replacing any node-wide, non-user-bound PM token model).

#### PMA Invocation Class (Interactive vs Orchestrator-Initiated)

- Spec ID: `CYNAI.MCPGAT.PmaInvocationClass` <a id="spec-cynai-mcpgat-pmainvocationclass"></a>

Every PMA MCP credential MUST carry exactly one **invocation class** so the gateway can tell **interactive** user-driven work from **orchestrator-triggered** automation on the user's behalf.

Normative values

- **`user_gateway_session`:** The user is authenticated through the **User API Gateway** and is actively driving PMA (for example interactive chat or equivalent gateway-mediated PMA work).
  Issuance for this class is triggered per [REQ-USRGWY-0160](../../requirements/usrgwy.md#req-usrgwy-0160).
- **`orchestrator_initiated`:** PMA is invoked **without** a live interactive gateway session; the orchestrator triggers work on behalf of the user (for example **scheduled** interpretation, **task** routing or review handoff, workflow continuation, or other automation).
  Issuance for this class MUST follow [REQ-ORCHES-0187](../../requirements/orches.md#req-orches-0187).

Gateway and policy

- The gateway MUST resolve the invocation class from the credential and MUST treat it as authoritative for policy that differs between classes (for example stricter tool surfaces or access control rules for orchestrator-initiated runs when configured).
- The gateway MUST reject MCP tool calls when the credential's invocation class is inconsistent with orchestrator-attributed context for the request when such checks are defined by product policy (fail closed).

##### PMA Invocation Class Requirements Traces

- [REQ-MCPGAT-0118](../../requirements/mcpgat.md#req-mcpgat-0118)
- [REQ-ORCHES-0187](../../requirements/orches.md#req-orches-0187)
- [REQ-USRGWY-0160](../../requirements/usrgwy.md#req-usrgwy-0160)

Issuance and gateway triggers

- The orchestrator MUST issue PMA MCP credentials with invocation class **`user_gateway_session`** when a **User API Gateway** path establishes or continues authenticated PMA-backed work (for example interactive chat handoff to PMA); see [REQ-USRGWY-0160](../../requirements/usrgwy.md#req-usrgwy-0160).
- The orchestrator MUST issue PMA MCP credentials with invocation class **`orchestrator_initiated`** when it triggers PMA **without** an active User API Gateway session (for example scheduled runs and task-completion handoffs); see [REQ-ORCHES-0187](../../requirements/orches.md#req-orches-0187).
- The orchestrator MUST deliver credential material to the worker via node configuration (or an equivalent channel) so the **worker proxy** can attach credentials without exposing them to PMA; see [REQ-ORCHES-0186](../../requirements/orches.md#req-orches-0186).

Rotation and long-lived sessions

- Each PMA MCP credential MUST have a **not-after** (expiry) time known to the orchestrator and enforced at the gateway.
- The orchestrator MUST **rotate** credentials before expiry for **long-lived** sessions so that user activity does not stall solely because a credential expired.
- During rotation, the orchestrator MUST support an **overlap window** where **both** the previous and successor credentials authenticate successfully at the gateway; after that window, the orchestrator MUST invalidate the superseded credential so the gateway rejects it.

Worker proxy selection

- The worker proxy MUST store PMA MCP credentials keyed so it can attach the credential that matches the **active session binding** and **invocation class** for each forwarded MCP request; PMA MUST NOT receive or present the credential (see [Token Handling (Normative)](#token-handling-normative)).
- When the deployment uses **one PMA managed service instance per session binding** ([CYNAI.ORCHES.PmaInstancePerSessionBinding](../orchestrator_bootstrap.md#spec-cynai-orches-pmainstancepersessionbinding)), the worker MAY select the credential using the calling instance's **`service_id`** alone (from the UDS binding) together with the single credential row for that instance.
- The orchestrator MUST define any additional correlation when multiple credentials exist for the same instance (for example rotation overlap); see [CYNAI.WORKER.AgentTokenStorageAndLifecycle](../worker_node.md#spec-cynai-worker-agenttokenstorageandlifecycle).

Authorization

- The gateway MUST treat the resolved **user_id** from the PMA MCP credential as authoritative for user-scoped tool and resource access; tool arguments that imply a different user MUST cause rejection (fail closed), consistent with [REQ-MCPGAT-0117](../../requirements/mcpgat.md#req-mcpgat-0117).

##### PMA Per-User Session Tokens and Rotation Requirements Traces

- [REQ-MCPGAT-0117](../../requirements/mcpgat.md#req-mcpgat-0117)
- [REQ-MCPGAT-0118](../../requirements/mcpgat.md#req-mcpgat-0118)
- [REQ-ORCHES-0186](../../requirements/orches.md#req-orches-0186)
- [REQ-ORCHES-0187](../../requirements/orches.md#req-orches-0187)
- [REQ-USRGWY-0160](../../requirements/usrgwy.md#req-usrgwy-0160)
- [REQ-WORKER-0175](../../requirements/worker.md#req-worker-0175)

## Edge Enforcement Mode (Node-Local Agent Runtimes)

- Spec ID: `CYNAI.MCPGAT.EdgeEnforcementMode` <a id="spec-cynai-mcpgat-edgeenforcement"></a>

### Edge Enforcement Mode (Node-Local Agent Runtimes) Requirements Traces

- [REQ-MCPGAT-0112](../../requirements/mcpgat.md#req-mcpgat-0112)

This section defines an edge enforcement mode for tool calls that are not routed by the orchestrator MCP gateway.
The primary use case is a node-local agent runtime interacting directly with a node-local MCP server on the same host for low-latency sandbox operations.

Managed agent runtimes (normative integration)

- Node-local agent runtimes include worker-managed long-lived agent service containers (for example PMA) when co-located with a worker node.
- When a managed agent runtime uses edge enforcement mode, the same capability lease rules and auditing requirements apply.
- This mode is complementary to worker-managed agent proxying: agent-to-orchestrator communication is worker-proxied, while node-local sandbox tool calls may be edge-enforced under leases.

Requirements

- Edge enforcement MUST NOT extend MCP wire messages.
  Task scoping MUST still be expressed in tool arguments, consistent with `Tool Argument Schema Requirements`.
- Edge enforcement MUST be authorized using orchestrator-issued capability leases.
  Leases MUST be short-lived, least-privilege, and MUST scope allowed tool identities and required context (for example `task_id`).
- The node-local MCP server (or an edge enforcement proxy colocated with it) MUST validate leases, enforce tool allowlists per [Access, allowlists, and per-tool scope](../mcp_tools/access_allowlists_and_scope.md), and MUST fail closed.
- The node-local MCP server MUST emit audit records for edge-routed tool calls with the same minimum audit fields used by the orchestrator gateway.
  See `docs/tech_specs/mcp/mcp_tool_call_auditing.md`.

## Role Allowlists, Per-Tool Scope, and Admin Enable or Disable

Normative definitions for role-based tool allowlists (worker, PM, PA), per-tool scope (sandbox vs PM vs both), and admin-configurable per-tool enable or disable are canonical in [Access, allowlists, and per-tool scope](../mcp_tools/access_allowlists_and_scope.md).
The gateway MUST enforce those definitions together with access control and auditing as described in [Gateway Enforcement Responsibilities](#gateway-enforcement-responsibilities).

## Tool Argument Schema Requirements

Because CyNodeAI does not extend MCP wire messages, task scoping MUST be expressed in tool arguments.

### Applicable Requirements (Tool Argument Schemas)

- Spec ID: `CYNAI.MCPGAT.ToolArgumentSchema` <a id="spec-cynai-mcpgat-toolargschema"></a>

#### Applicable Requirements (Tool Argument Schemas) Requirements Traces

- [REQ-MCPGAT-0103](../../requirements/mcpgat.md#req-mcpgat-0103)
- [REQ-MCPGAT-0104](../../requirements/mcpgat.md#req-mcpgat-0104)
- [REQ-MCPGAT-0105](../../requirements/mcpgat.md#req-mcpgat-0105)
- [REQ-MCPGAT-0106](../../requirements/mcpgat.md#req-mcpgat-0106)

Recommended guidance

- Prefer stable, explicit ids (`task_id`, `run_id`, `job_id`) over implicit scoping.
- Prefer allowlisted tool patterns over dynamic tool discovery.

Normative

- The gateway MUST **ignore** argument keys that are **not** part of the invoked tool's defined input schema (after catalog lookup) and MUST **not** reject the call solely for that reason.
  This includes keys such as `task_id` on tools that do not use `task_id` (e.g. [Skills MCP Tools](../mcp_tools/skills_tools.md)).
  See [Extraneous arguments](../mcp/mcp_tooling.md#spec-cynai-mcptoo-extraneousarguments).

## Access Control Mapping

Access control SHOULD be defined via [`docs/tech_specs/access_control.md`](../access_control.md).

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
