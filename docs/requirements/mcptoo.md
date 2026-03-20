# MCPTOO Requirements

- [1 Overview](#1-overview)
- [2 Requirements](#2-requirements)

## 1 Overview

This document consolidates requirements for the `MCPTOO` domain.
It covers MCP tooling conventions, tool catalog expectations, and SDK installation and integration.

## 2 Requirements

- **REQ-MCPTOO-0001:** Agents use MCP tools for capabilities; no direct DB; user access via gateway only.
  [CYNAI.MCPTOO.McpRole](../tech_specs/mcp/mcp_tooling.md#spec-cynai-mcptoo-mcprole)
  [CYNAI.MCPTOO.DatabaseAccessRules](../tech_specs/mcp/mcp_tooling.md#spec-cynai-mcptoo-dbaccess)
  <a id="req-mcptoo-0001"></a>
- **REQ-MCPTOO-0002:** Tool names `namespace.operation`, stable ids; schema-validated, size-limited responses; no secrets in errors.
  [CYNAI.MCPTOO.ToolCatalogNaming](../tech_specs/mcp/mcp_tooling.md#spec-cynai-mcptoo-toolnaming)
  [CYNAI.MCPTOO.ToolCatalogResponseError](../tech_specs/mcp/mcp_tooling.md#spec-cynai-mcptoo-toolresponse)
  <a id="req-mcptoo-0002"></a>
- **REQ-MCPTOO-0003:** MCP SDK installation and integration constraints.
  [CYNAI.MCPTOO.Doc.McpSdkInstallation](../tech_specs/mcp/mcp_sdk_installation.md#spec-cynai-mcptoo-doc-mcpsdkinstallation)
  <a id="req-mcptoo-0003"></a>
- **REQ-MCPTOO-0100:** Sandboxed worker agents MUST use MCP tools for controlled capabilities.
  [CYNAI.MCPTOO.McpRole](../tech_specs/mcp/mcp_tooling.md#spec-cynai-mcptoo-mcprole)
  <a id="req-mcptoo-0100"></a>
- **REQ-MCPTOO-0101:** Orchestrator-side agents (Project Manager and Project Analyst) MUST use MCP tools for privileged operations.
  [CYNAI.MCPTOO.McpRole](../tech_specs/mcp/mcp_tooling.md#spec-cynai-mcptoo-mcprole)
  <a id="req-mcptoo-0101"></a>
- **REQ-MCPTOO-0102:** Direct access to internal services and databases SHOULD be avoided and MUST be restricted by policy.
  [CYNAI.MCPTOO.McpRole](../tech_specs/mcp/mcp_tooling.md#spec-cynai-mcptoo-mcprole)
  <a id="req-mcptoo-0102"></a>
- **REQ-MCPTOO-0103:** Sandboxed agents MUST NOT connect directly to PostgreSQL.
  [CYNAI.MCPTOO.DatabaseAccessRules](../tech_specs/mcp/mcp_tooling.md#spec-cynai-mcptoo-dbaccess)
  <a id="req-mcptoo-0103"></a>
- **REQ-MCPTOO-0104:** Orchestrator-side agents SHOULD NOT connect directly to PostgreSQL.
  [CYNAI.MCPTOO.DatabaseAccessRules](../tech_specs/mcp/mcp_tooling.md#spec-cynai-mcptoo-dbaccess)
  <a id="req-mcptoo-0104"></a>
- **REQ-MCPTOO-0105:** User-facing access MUST be mediated by the User API Gateway and MUST NOT expose raw SQL execution.
  [CYNAI.MCPTOO.DatabaseAccessRules](../tech_specs/mcp/mcp_tooling.md#spec-cynai-mcptoo-dbaccess)
  <a id="req-mcptoo-0105"></a>
- **REQ-MCPTOO-0106:** Tool names MUST use `namespace.operation` format.
  [CYNAI.MCPTOO.ToolCatalogNaming](../tech_specs/mcp/mcp_tooling.md#spec-cynai-mcptoo-toolnaming)
  <a id="req-mcptoo-0106"></a>
- **REQ-MCPTOO-0107:** Tool names MUST be stable identifiers.
  [CYNAI.MCPTOO.ToolCatalogNaming](../tech_specs/mcp/mcp_tooling.md#spec-cynai-mcptoo-toolnaming)
  <a id="req-mcptoo-0107"></a>
- **REQ-MCPTOO-0108:** Namespaces MUST be short and descriptive.
  [CYNAI.MCPTOO.ToolCatalogNaming](../tech_specs/mcp/mcp_tooling.md#spec-cynai-mcptoo-toolnaming)
  <a id="req-mcptoo-0108"></a>
- **REQ-MCPTOO-0109:** Tool responses MUST be schema-validated and size-limited.
  [CYNAI.MCPTOO.ToolCatalogResponseError](../tech_specs/mcp/mcp_tooling.md#spec-cynai-mcptoo-toolresponse)
  <a id="req-mcptoo-0109"></a>
- **REQ-MCPTOO-0110:** Errors MUST be structured and MUST NOT leak secrets.
  [CYNAI.MCPTOO.ToolCatalogResponseError](../tech_specs/mcp/mcp_tooling.md#spec-cynai-mcptoo-toolresponse)
  <a id="req-mcptoo-0110"></a>

- **REQ-MCPTOO-0111:** MCP SDK versions MUST be pinned in each service `go.mod`.
  [CYNAI.MCPTOO.GoSdkVersionPinning](../tech_specs/mcp/mcp_sdk_installation.md#spec-cynai-mcptoo-gosdkversionpinning)
  <a id="req-mcptoo-0111"></a>
- **REQ-MCPTOO-0112:** Each Go service SHOULD have its own `go.mod` at the service root.
  [CYNAI.MCPTOO.GoServiceModuleLayout](../tech_specs/mcp/mcp_sdk_installation.md#spec-cynai-mcptoo-goservicemodlayout)
  <a id="req-mcptoo-0112"></a>
- **REQ-MCPTOO-0113:** If a multi-module layout is used, a `go.work` MAY be used for local development.
  [CYNAI.MCPTOO.GoServiceModuleLayout](../tech_specs/mcp/mcp_sdk_installation.md#spec-cynai-mcptoo-goservicemodlayout)
  <a id="req-mcptoo-0113"></a>
- **REQ-MCPTOO-0114:** If a workflow runtime needs MCP protocol support directly, it SHOULD use the official Python MCP SDK.
  [CYNAI.MCPTOO.PythonSdkUsage](../tech_specs/mcp/mcp_sdk_installation.md#spec-cynai-mcptoo-pythonsdkusage)
  <a id="req-mcptoo-0114"></a>
- **REQ-MCPTOO-0115:** The workflow runtime MUST NOT connect directly to PostgreSQL and MUST use MCP database tools (or an internal service enforcing the same policy).
  [CYNAI.MCPTOO.WorkflowRuntimeNoDirectDb](../tech_specs/mcp/mcp_sdk_installation.md#spec-cynai-mcptoo-workflowruntimenodb)
  <a id="req-mcptoo-0115"></a>
- **REQ-MCPTOO-0116:** The system SHOULD expose a help MCP server (or help tools) that provide on-demand documentation for how to interact with CyNodeAI (e.g. tool usage, conventions, gateway).
  Help content SHOULD be aligned with and updated on the same cadence as the default CyNodeAI interaction skill.
  The help MCP MUST return tool and invocation help text from tool definitions.
  [CYNAI.MCPTOO.HelpMcpServer](../tech_specs/mcp/mcp_tooling.md#spec-cynai-mcptoo-helpmcpserver)
  [CYNAI.MCPTOO.HelpMcpContract](../tech_specs/mcp/mcp_tool_definitions.md#spec-cynai-mcptoo-helpmcpcontract)
  <a id="req-mcptoo-0116"></a>
- **REQ-MCPTOO-0117:** The MCP tool specifications MUST include typed preference read tools that allow agents to get, list, and resolve effective preferences.
  At minimum, the specifications MUST include `preference.get`, `preference.list`, and `preference.effective`.
  [mcp_tools/README.md](../tech_specs/mcp_tools/README.md)
  [user_preferences.md](../tech_specs/user_preferences.md)
  <a id="req-mcptoo-0117"></a>
- **REQ-MCPTOO-0118:** MCP tool families SHOULD support full CRUD by default.
  If full CRUD is not appropriate, the MCP tool specifications MUST document an intentional exception and the allowed operations.
  [mcp_tools/README.md](../tech_specs/mcp_tools/README.md)
  <a id="req-mcptoo-0118"></a>
- **REQ-MCPTOO-0119:** The system MUST provide a secure web search MCP tool (`web.search`) that is policy-controlled and does not expose raw internet access; search MUST be routed through a secure path (e.g. Secure Browser Service or a dedicated search proxy) so only sanitized or allowlisted search results are returned to agents.
  [CYNAI.MCPTOO.SecureWebSearch](../tech_specs/mcp_tools/secure_web_search.md#spec-cynai-mcptoo-securewebsearch)
  <a id="req-mcptoo-0119"></a>
- **REQ-MCPTOO-0120:** The PMA and PAA MUST be able to use MCP tools to list and get personas (persona.list, persona.get) for selection when assigning or creating tasks; the SBA MUST be able to get a persona for the correct scope via MCP (persona.get) when needed; when agents run on worker nodes, persona access MUST go through the worker proxy to the orchestrator MCP gateway.
  [CYNAI.MCPTOO.PersonaTools](../tech_specs/mcp_tools/persona_tools.md#spec-cynai-mcptoo-personatools)
  <a id="req-mcptoo-0120"></a>
- **REQ-MCPTOO-0121:** Tool definitions (internal and external) MUST use the same format: MCPTool (Server, Name, Help, Scope, Tools) and ToolInvocation (Name+Args or Ref, optional Scope override, Help).
  The database MUST be the source of truth for tool registration.
  [CYNAI.MCPTOO.MCPTool](../tech_specs/mcp/mcp_tool_definitions.md#spec-cynai-mcptoo-mcptool)
  [CYNAI.MCPTOO.ToolInvocation](../tech_specs/mcp/mcp_tool_definitions.md#spec-cynai-mcptoo-toolinvocation)
  [CYNAI.MCPTOO.GormToolDef](../tech_specs/mcp/mcp_tool_definitions.md#spec-cynai-mcptoo-gormtooldef)
  <a id="req-mcptoo-0121"></a>
- **REQ-MCPTOO-0122:** Scope MUST be enforced at the invocation level: each step in a composite tool's Tools list has an effective scope (the invocation's Scope if set, otherwise the parent MCPTool's Scope).
  The gateway MUST check the caller's agent type against each invocation's effective scope before executing that step.
  [CYNAI.MCPTOO.ExternalToolAgentScope](../tech_specs/mcp/mcp_tool_definitions.md#spec-cynai-mcptoo-externaltoolagentscope)
  <a id="req-mcptoo-0122"></a>
- **REQ-MCPTOO-0123:** The help MCP MUST return tool and invocation help text from tool definitions.
  When called without a tool-specific topic, it MUST return an overview and list of available tools (scoped to caller's identity).
  When called with a tool name, it MUST return the tool's Help plus each invocation's Help with effective scope.
  [CYNAI.MCPTOO.HelpMcpContract](../tech_specs/mcp/mcp_tool_definitions.md#spec-cynai-mcptoo-helpmcpcontract)
  <a id="req-mcptoo-0123"></a>
- **REQ-MCPTOO-0124:** The system MUST support external MCP server endpoint registration via the User API Gateway.
  Endpoints MUST be stored with base URL, credential reference, owner, and scope (user-scoped or shared).
  Tool definitions reference endpoints by a stable key (slug) in the Server field.
  [CYNAI.MCPTOO.EndpointRegistry](../tech_specs/mcp/mcp_endpoint_registry.md#spec-cynai-mcptoo-endpointregistry)
  [CYNAI.MCPTOO.EndpointRecord](../tech_specs/mcp/mcp_endpoint_registry.md#spec-cynai-mcptoo-endpointrecord)
  <a id="req-mcptoo-0124"></a>
- **REQ-MCPTOO-0125:** The gateway MUST resolve Server (when not "default") to (base_url, credentials) in request context.
  For user-scoped endpoints, resolution MUST restrict to the current user's endpoints plus shared endpoints the user is allowed to use.
  [CYNAI.MCPTOO.EndpointResolution](../tech_specs/mcp/mcp_endpoint_registry.md#spec-cynai-mcptoo-endpointresolution)
  <a id="req-mcptoo-0125"></a>
- **REQ-MCPTOO-0126:** Agents MUST NOT receive or observe API credentials for registered endpoints.
  When an agent invokes a tool targeting a registered endpoint, the MCP gateway MUST perform credential resolution and injection server-side.
  The agent's request and response MUST contain only tool identity and business payload.
  [CYNAI.MCPTOO.CredentialInjection](../tech_specs/mcp/mcp_endpoint_registry.md#spec-cynai-mcptoo-credentialinjection)
  <a id="req-mcptoo-0126"></a>
- **REQ-MCPTOO-0127:** Endpoint records MUST store a reference to credentials (credential_ref), not the secret itself.
  Credentials MUST be stored encrypted at rest and MUST follow the same patterns as Connector Framework or API Egress credential handling.
  [CYNAI.MCPTOO.EndpointCredentialStorage](../tech_specs/mcp/mcp_endpoint_registry.md#spec-cynai-mcptoo-endpointcredentialstorage)
  <a id="req-mcptoo-0127"></a>
