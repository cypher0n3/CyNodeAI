# MCP Tooling

- [Document Overview](#document-overview)
- [MCP Role in CyNodeAI](#mcp-role-in-cynodeai)
- [Tool Categories](#tool-categories)
- [Role-Based Tool Access](#role-based-tool-access)
  - [Worker Agent Tool Access](#worker-agent-tool-access)
  - [Project Manager Agent Tool Access](#project-manager-agent-tool-access)
  - [Project Analyst Agent Tool Access](#project-analyst-agent-tool-access)
- [Database Access Rules](#database-access-rules)
- [External MCP Servers](#external-mcp-servers)
- [Access Control and Auditing](#access-control-and-auditing)
- [Sandbox Connectivity Model](#sandbox-connectivity-model)

## Document Overview

This document defines how CyNodeAI uses MCP as the standard interface for agent tools.
It covers tool categories, role-based access, database access restrictions, and external MCP server integration.

## MCP Role in CyNodeAI

MCP provides a consistent protocol for agents to request tool operations.
The orchestrator is the policy and routing point for tools.

Normative requirements

- Sandboxed worker agents MUST use MCP tools for controlled capabilities.
- Orchestrator-side agents (Project Manager and Project Analyst) MUST use MCP tools for privileged operations.
- The User API Gateway is intended for external user clients and integrations, not for internal agent operations.
- Direct access to internal services and databases SHOULD be avoided and MUST be restricted by policy.

## Tool Categories

Tools exposed to agents SHOULD be grouped into categories.

- Web and information tools
  - sanitized web fetch
  - document retrieval
- External API tools
  - outbound provider calls through API Egress Server
  - git operations through Git Egress MCP
- Artifact tools
  - upload, download, and list task artifacts
- Node and sandbox tools
  - request sandbox execution
  - request node configuration refresh
- Model tools
  - request model load on a node
  - list available models and capabilities
- Database query tools
  - read task state, preferences, and audit records
  - write task updates, verification results, and summaries

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
- Model registry and node capability tools
- Node configuration and sandbox orchestration tools
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

Normative requirements

- Sandboxed agents MUST NOT connect directly to PostgreSQL.
- Orchestrator-side agents SHOULD NOT connect directly to PostgreSQL.
- The orchestrator owns database credentials and exposes only scoped MCP database tools.
- User-facing access MUST be mediated by the User API Gateway and MUST NOT expose raw SQL execution.

## External MCP Servers

The orchestrator MAY connect to external MCP servers to expose additional tools.
External MCP servers MUST be subject to policy controls, audit logging, and allowlisting.

Recommended behavior

- External MCP servers SHOULD be registered with:
  - a logical name
  - a tool allowlist
  - per-user and per-project enablement
- External MCP server tool responses MUST be treated as untrusted data.

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
- The node uses the container runtime (e.g. Podman) to create containers and execute commands.

Node-hosted sandbox MCP

- Nodes SHOULD expose sandbox operations via a node-local MCP server.
- The orchestrator SHOULD register node MCP servers as external MCP servers and route sandbox tool calls to the correct node.
- Agents MUST call sandbox tools through the orchestrator and MUST NOT call node MCP servers directly.

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
