# User-Installable MCP Tools

- [Document Overview](#document-overview)
- [Scope](#scope)
- [Registration and Persistence](#registration-and-persistence)
- [Per-Tool Scope (Sandbox vs PM)](#per-tool-scope-sandbox-vs-pm)
- [Discovery and Lifecycle](#discovery-and-lifecycle)
- [Integration with Gateway Enforcement](#integration-with-gateway-enforcement)
- [Related Documents](#related-documents)

## Document Overview

- Spec ID: `CYNAI.MCPTOO.UserInstallableTools` <a id="spec-cynai-mcptoo-userinstallabletools"></a>

This spec defines how users can install their own (custom) MCP tools with the orchestrator, configure per-tool scope (sandbox vs PM), and manage them via Web Console and CLI.
Gateway enforcement of that scope is defined in [`mcp_gateway_enforcement.md`](mcp_gateway_enforcement.md); this spec is the single place for registration, configuration, and lifecycle of user-installed tools.

Traces To:

- [REQ-MCPGAT-0115](../requirements/mcpgat.md#req-mcpgat-0115)

## Scope

- **In scope:** User-installable (custom) MCP tool registration with the orchestrator; per-tool configuration (including sandbox vs PM scope); persistence of tool metadata and scope; exposure of view and change via Web Console and CLI; use of that data by the MCP gateway when enforcing tool calls.
- **Out of scope:** Built-in tool definitions (see [MCP tool catalog](mcp_tool_catalog.md)); gateway enforcement rules and allowlists (see [MCP gateway enforcement](mcp_gateway_enforcement.md)); MCP wire protocol and SDK details (see [MCP tooling](mcp_tooling.md) and [MCP SDK installation](mcp_sdk_installation.md)).

## Registration and Persistence

- Users MUST be able to **install their own MCP tools** (register custom tools with the orchestrator).
  Registration MUST persist at least: a stable tool identity (e.g. server name and tool name or a canonical name), and the per-tool scope (sandbox only, PM only, or both).
  The orchestrator MAY persist additional metadata (e.g. owner, created_at, MCP server endpoint or package reference) as needed for discovery and lifecycle.
- Registration and updates MUST be subject to the same permissions as other user-configurable resources; admin or user permissions for viewing and changing tools are defined elsewhere (e.g. access control, RBAC).

## Per-Tool Scope (Sandbox vs PM)

- For each user-installed tool, the user MUST be able to **configure the per-tool scope**: sandbox only, PM only, or both.
  This determines which agent types (sandbox agents vs PM/PA agents) are allowed to invoke the tool at the gateway.
- The orchestrator MUST persist this setting and MUST expose it to the MCP gateway so that the gateway can enforce the rules in [Per-tool scope: Sandbox vs PM](mcp_gateway_enforcement.md#spec-cynai-mcpgat-pertoolscope).
  The gateway does not define how scope is stored; it consumes the resolved scope when evaluating each tool call.

## Discovery and Lifecycle

- Installed tools MUST be discoverable (e.g. listable by the owning user or by admins) so that users can view and change the per-tool scope and so that the gateway can resolve scope for each tool name.
  The system MAY support update and uninstall (lifecycle) for user-installed tools; details are implementation-defined.
  After uninstall, the tool MUST NOT be invokable and MUST NOT appear in the effective allowlist for any agent type.

## Integration With Gateway Enforcement

- The MCP gateway enforces tool access using the per-tool scope stored for both built-in and user-installed tools.
  When a tool is user-installed, its scope is the value configured by the user (sandbox only, PM only, or both).
  The gateway MUST allow or reject the call according to [Per-tool scope: Sandbox vs PM](mcp_gateway_enforcement.md#spec-cynai-mcpgat-pertoolscope).
  No separate enforcement logic is required in this spec; the contract is that the orchestrator persists scope and the gateway uses it.

## Related Documents

- MCP gateway enforcement (allowlists, per-tool scope enforcement, tokens): [`mcp_gateway_enforcement.md`](mcp_gateway_enforcement.md)
- MCP tool catalog (built-in tool names and schemas): [`mcp_tool_catalog.md`](mcp_tool_catalog.md)
- MCP tooling (concepts, role-based access): [`mcp_tooling.md`](mcp_tooling.md)
