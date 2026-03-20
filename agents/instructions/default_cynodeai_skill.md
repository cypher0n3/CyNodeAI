# Default CyNodeAI Interaction Skill

## Overview

This is the system-owned default skill for how AIs interact with CyNodeAI.
It is included in every inference request that receives skills (PMA, SBA, and other agents).
Do not modify or delete it via user-facing skill tools.

## MCP Tools and Gateway

- All tool calls MUST go through the **orchestrator MCP gateway**.
  The gateway enforces allowlists, access control, and auditing.
  Use the agent-scoped token provided for your role.
- Task-scoped tools require a `task_id` (uuid) argument.
  Provide it for every call that accepts it.
  Job-scoped tools may require `job_id`; run-scoped may use `run_id`.
- The gateway returns structured errors (e.g. 403, 404, 422).
  Treat rejections as hard failures; do not retry without correcting the request or checking policy.
- Canonical tool names and argument schemas are in [MCP tool specifications](../../docs/tech_specs/mcp_tools/README.md); your role allowlist determines which tools you may call.

## Task and Project Context

- Associate work with the current **task** and **project** when provided in the request context.
  Use task_id and project_id in tool arguments when required.
- User preferences (e.g. additional context, style) are resolved by scope: task > project > user > group > system.
  Apply them when verifying or generating output.

## Sandbox and Nodes

- Sandbox jobs run on worker nodes.
  Use `sandbox.*` tools (when allowed by your role) to create, exec, transfer files, stream logs, and destroy.
  Use `node.*` to list nodes and check capabilities.
  Do not assume a node is available; check the registry and model availability.
- External provider calls (APIs, web fetch, search) go through API Egress or Secure Browser Service; do not open raw network connections from the agent.

## Conventions

- Do not guess or simulate output from tasks, database calls, tool calls, or external services.
  Use actual tool and system results only; report unavailability or errors; do not invent or assume values.
- Use `help.*` tools for on-demand documentation when available.
- Skills (skills.list, skills.get, etc.) are available when the allowlist and access control permit; use them to load user-defined guidance.
  The default skill (this content) is always included.
- Output and tool-use contracts are defined in your role instructions bundle; stay within those boundaries.
