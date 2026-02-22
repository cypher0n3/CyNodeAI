# MCP Tooling

- [Document Overview](#document-overview)
- [MCP Role in CyNodeAI](#mcp-role-in-cynodeai)
- [Tool Categories](#tool-categories)
- [Help MCP Server](#help-mcp-server)
- [Role-Based Tool Access](#role-based-tool-access)
  - [Worker Agent Tool Access](#worker-agent-tool-access)
  - [Project Manager Agent Tool Access](#project-manager-agent-tool-access)
  - [Project Analyst Agent Tool Access](#project-analyst-agent-tool-access)
- [Database Access Rules](#database-access-rules)
- [Access Control and Auditing](#access-control-and-auditing)
- [Sandbox Connectivity Model](#sandbox-connectivity-model)

## Document Overview

This document defines how CyNodeAI uses MCP as the standard interface for agent tools.
It covers tool categories, role-based access, and database access restrictions.
Gateway enforcement, role allowlists, and **per-tool scope (sandbox vs PM)** are defined in [`docs/tech_specs/mcp_gateway_enforcement.md`](mcp_gateway_enforcement.md).
The orchestrator tracks for each tool whether it is available to sandbox agents, PM agents, or both; users can install custom MCP tools and set this per tool.
The tool catalog is defined in [`docs/tech_specs/mcp_tool_catalog.md`](mcp_tool_catalog.md).
Tool call audit storage is defined in [`docs/tech_specs/mcp_tool_call_auditing.md`](mcp_tool_call_auditing.md).
Practical SDK installation guidance is in [`docs/tech_specs/mcp_sdk_installation.md`](mcp_sdk_installation.md).

## MCP Role in CyNodeAI

MCP provides a consistent protocol for agents to request tool operations.
The orchestrator is the policy and routing point for tools.

### MCP Role Applicable Requirements

- Spec ID: `CYNAI.MCPTOO.McpRole` <a id="spec-cynai-mcptoo-mcprole"></a>
- Sandboxed worker agents must use MCP tools for controlled capabilities.
- Orchestrator-side agents (Project Manager and Project Analyst) must use MCP tools for privileged operations.
- The User API Gateway is intended for external user clients and integrations, not for internal agent operations.
- Direct access to internal services and databases should be avoided and restricted by policy.

Traces To:

- [REQ-MCPTOO-0100](../requirements/mcptoo.md#req-mcptoo-0100)
- [REQ-MCPTOO-0101](../requirements/mcptoo.md#req-mcptoo-0101)
- [REQ-MCPTOO-0102](../requirements/mcptoo.md#req-mcptoo-0102)

## Tool Categories

Tools exposed to agents SHOULD be grouped into categories.

- Web and information tools
  - sanitized web fetch
  - document retrieval
- External API tools
  - outbound provider calls through API Egress Server
  - git operations through Git Egress MCP
- Connector tools
  - manage connector instances and credentials
  - invoke connector operations (read, send) subject to policy
- Artifact tools
  - upload, download, and list task artifacts
- Node and sandbox tools
  - request sandbox execution
  - request node configuration refresh
- Model tools
  - request model load on a node
  - list available models and capabilities
- Skills tools
  - See [Skills Storage and Inference Exposure](skills_storage_and_inference.md) for tool contract and controls; catalog lists tool names only.
- Help tools
  - on-demand documentation for how to interact with CyNodeAI; see [Help MCP Server](#help-mcp-server).
- Database query tools
  - read task state, preferences, and audit records
  - write task updates, verification results, and summaries
  - preference retrieval is performed via `db.preference.get`, `db.preference.list`, and `db.preference.effective` as defined in [`docs/tech_specs/mcp_tool_catalog.md`](mcp_tool_catalog.md)

## Help MCP Server

- Spec ID: `CYNAI.MCPTOO.HelpMcpServer` <a id="spec-cynai-mcptoo-helpmcpserver"></a>

Traces To:

- [REQ-MCPTOO-0116](../requirements/mcptoo.md#req-mcptoo-0116)

The system MAY expose a help MCP server (or a set of help tools) so that agents can request documentation on demand during a run.
This complements the [default CyNodeAI interaction skill](skills_storage_and_inference.md#spec-cynai-skills-defaultcynodeaiskill): the skill is always included in inference context for baseline guidance; the help server provides deeper or targeted documentation when the agent explicitly calls a help tool (e.g. to look up a specific tool schema or convention).

Scope

- **Purpose**: Help tools return documentation content about CyNodeAI: how to use MCP tools, gateway usage, task and project context, conventions, and references to authoritative docs.
  Content SHOULD be aligned with (and MAY be derived from) the default CyNodeAI interaction skill and updated on the same cadence.
- **Read-only**: Help tools MUST NOT modify state; they only return documentation or not-found.
- **Allowlists**: Help tools are allowlisted for orchestrator-side agents (Project Manager, Project Analyst) and MAY be allowlisted for Worker agents so sandboxed agents can look up usage when needed.
- **Catalog**: Tool names and argument schemas are defined in the [MCP tool catalog](mcp_tool_catalog.md#spec-cynai-mcptoo-helptools).

## Role-Based Tool Access

Tool access MUST be role-based and policy-controlled.

### Worker Agent Tool Access

Worker agents run in sandbox containers and SHOULD have the minimal tool surface needed to complete dispatched work.

Recommended tool access

- Artifact read and write for the current task
- Sanitized web fetch when allowed
- External API calls when explicitly allowed for the task
- No direct database query tools

### Project Manager Agent Tool Access

The Project Manager Agent is orchestrator-side and requires additional tools for planning, scheduling, and verification.

Recommended tool access

- Database read and write tools for tasks, preferences, and routing metadata
- Project search and resolve tools (e.g. `db.project.get`, `db.project.list`) scoped to the authenticated user for resolving project context from chat
- Model registry and node capability tools
- Node configuration and sandbox orchestration tools
- Connector management and invocation tools, subject to policy
- External API and web tools, subject to policy

### Project Analyst Agent Tool Access

The Project Analyst Agent is orchestrator-side and focuses on verification.

Recommended tool access

- Database read tools for tasks, preferences, and evidence
- Database write tools for verification findings and recommendations
- Artifact read tools for produced outputs
- Sanitized web fetch and external API tools, when allowed for verification

## Database Access Rules

All agent interactions with PostgreSQL MUST happen through MCP database tools.
User clients MAY access database-backed information through the User API Gateway Data REST API.

### Database Access Rules Applicable Requirements

- Spec ID: `CYNAI.MCPTOO.DatabaseAccessRules` <a id="spec-cynai-mcptoo-dbaccess"></a>
- Sandboxed agents must not connect directly to PostgreSQL.
- Orchestrator-side agents should not connect directly to PostgreSQL.
- The orchestrator owns database credentials and exposes only scoped MCP database tools.
- User-facing access must be mediated by the User API Gateway and must not expose raw SQL execution.

Traces To:

- [REQ-MCPTOO-0103](../requirements/mcptoo.md#req-mcptoo-0103)
- [REQ-MCPTOO-0104](../requirements/mcptoo.md#req-mcptoo-0104)
- [REQ-MCPTOO-0105](../requirements/mcptoo.md#req-mcptoo-0105)

## Access Control and Auditing

All MCP tool calls MUST be audited with task context.
Access control SHOULD be defined via [`docs/tech_specs/access_control.md`](access_control.md).

Recommended auditing fields

- subject identity
- tool name and operation
- task_id and project_id
- decision allow or deny
- timing and error status

## Sandbox Connectivity Model

Agents do not connect to sandboxes over the network.
Instead, sandbox lifecycle and command execution are mediated through MCP tools and the orchestrator.

Key principles

- Sandboxes SHOULD not expose inbound network services for control (no SSH requirement).
- Orchestrator-side agents request sandbox execution through MCP.
- The orchestrator routes sandbox operations to the target node's worker API.
- The node uses a container runtime (Docker or Podman; Podman preferred for rootless) to create containers and execute commands.

Node-hosted sandbox MCP

- Nodes SHOULD expose sandbox operations via a node-local MCP server.
- The orchestrator SHOULD register each node MCP server with an allowlist and route sandbox tool calls to the correct node.
- Remote agents MUST call sandbox tools through the orchestrator and MUST NOT call node MCP servers directly.
  Node-local agent runtimes MAY call node-hosted sandbox tools directly only when the node enforces orchestrator-issued capability leases and produces auditable tool call records.
  See `docs/tech_specs/worker_node.md#node-local-agent-sandbox-control-low-latency-path`.

Recommended sandbox tool operations

- `sandbox.create`
  - Creates a sandbox container on a selected node for a given task.
- `sandbox.exec`
  - Executes a command inside an existing sandbox container.
- `sandbox.put_file`
  - Uploads a file into the sandbox workspace.
- `sandbox.get_file`
  - Downloads a file from the sandbox workspace.
- `sandbox.stream_logs`
  - Streams stdout and stderr from sandbox execution.
- `sandbox.destroy`
  - Stops and removes the sandbox container.

Worker agent note

- Worker agents that run inside a sandbox container execute commands locally inside that sandbox.
- They do not need an attachment mechanism to the sandbox because they are already running inside it.
