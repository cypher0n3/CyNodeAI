# MCP Tool Specifications

## Overview

This directory is the **canonical reference** for MCP tool names, argument schemas, behavior, algorithms, role allowlists, and per-tool scope.
Each document provides full specs, argument contracts, behavior, and requirement traceability.
Naming conventions, common argument requirements (task/job scoping, size limits), and the response/error model are in [MCP Tooling](../mcp_tooling.md).

## Canonical Definition Format

All tool definitions in these specs comply with the project's canonical MCP tool definition format and endpoint resolution:

- **Definition format**: `MCPTool`, `ToolInvocation`, `ToolAgentScope`; `Server` (default or endpoint key), `Name`, `Help`, `Scope`, `Tools`.
- **Endpoint resolution**: When `Server` is not `default`, the gateway resolves the endpoint slug to base URL and credentials in request context (per the project's endpoint registry spec once adopted).

Built-in catalog tools use `Server: default` and are implemented by the orchestrator MCP gateway.

## Access, Allowlists, and Per-Tool Scope

- [Access, allowlists, and per-tool scope](access_allowlists_and_scope.md): worker / PM / PA allowlists, sandbox vs PM scope, admin per-tool enable or disable (Spec IDs `CYNAI.MCPGAT.*`).

## Index of Tool Specs

- Artifact (task-scoped + unified): [artifact_tools.md](artifact_tools.md)
- Memory (job-scoped): [memory_tools.md](memory_tools.md)
- Sandbox: [sandbox_tools.md](sandbox_tools.md)
- Sandbox allowed images (PM): [sandbox_allowed_images.md](sandbox_allowed_images.md)
- Web fetch: [web_fetch.md](web_fetch.md)
- Secure web search: [secure_web_search.md](secure_web_search.md)
- API egress: [api_egress.md](api_egress.md)
- Git egress: [git_egress.md](git_egress.md)
- Node: [node_tools.md](node_tools.md)
- Model registry: [model_registry.md](model_registry.md)
- Persona: [persona_tools.md](persona_tools.md)
- Skills: [skills_tools.md](skills_tools.md)
- Help: [help_tools.md](help_tools.md)
- Preference: [preference_tools.md](preference_tools.md)
- System setting: [system_setting_tools.md](system_setting_tools.md)
- Task: [task_tools.md](task_tools.md)
- Job: [job_tools.md](job_tools.md)
- Project: [project_tools.md](project_tools.md)
- Specification: [specification_tools.md](specification_tools.md)

## Related Documents

- [MCP Tooling](../mcp_tooling.md): goals, naming conventions, common argument requirements, response/error model, MCP role, categories, help server.
- [MCP Gateway Enforcement](../mcp_gateway_enforcement.md): gateway enforcement mechanics, tokens, edge mode, auditing (allowlist and scope definitions are in this directory).
