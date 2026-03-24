# MCP Tool Specifications

## Overview

This directory is the **canonical reference** for MCP tool names, argument schemas, behavior, algorithms, role allowlists, and per-tool scope.
Each document provides full specs, argument contracts, behavior, and requirement traceability.
Naming conventions, common argument requirements (task/job scoping, size limits), and the response/error model are in [MCP Tooling](../mcp/mcp_tooling.md).

## Canonical Definition Format

All tool definitions in these specs comply with the project's canonical MCP tool definition format and endpoint resolution:

- **Definition format**: `MCPTool`, `ToolInvocation`, `ToolAgentScope`; `Server` (default or endpoint key), `Name`, `Help`, `Scope`, `Tools`.
  See [MCP Tool Definitions](../mcp/mcp_tool_definitions.md) for the complete specification.
- **Endpoint resolution**: When `Server` is not `default`, the gateway resolves the endpoint slug to base URL and credentials in request context.
  See [MCP Endpoint Registry](../mcp/mcp_endpoint_registry.md) for the complete specification.

Built-in catalog tools normally use `Server: default` and are implemented by the orchestrator MCP gateway, except **agent-local** tools documented as such (e.g. [Memory tools](../agent_local_tools/memory_tools.md) for the SBA: on-disk in the sandbox, no gateway hop).

## Access, Allowlists, and Per-Tool Scope

- [Access, allowlists, and per-tool scope](access_allowlists_and_scope.md): worker / PM / PA allowlists, sandbox vs PM scope, admin per-tool enable or disable (Spec IDs `CYNAI.MCPGAT.*`).
  Agent tool calls MUST NOT execute with **ADMIN**-level privileges where the relevant tool spec forbids it; see [Skills MCP Tools](skills_tools.md) and [MCP Gateway Enforcement](../mcp/mcp_gateway_enforcement.md).

## Index of Tool Specs

- Artifact (scoped user / group / project / global + unified API): [artifact_tools.md](artifact_tools.md)
- Memory (job-scoped, SBA agent-local on-disk): [memory_tools.md](../agent_local_tools/memory_tools.md)
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

- [MCP Tooling](../mcp/mcp_tooling.md): goals, naming conventions, common argument requirements, response/error model, MCP role, categories, help server.
- [MCP Gateway Enforcement](../mcp/mcp_gateway_enforcement.md): gateway enforcement mechanics, tokens, edge mode, auditing (allowlist and scope definitions are in this directory).
