# MCPTOO Requirements

- [1 Overview](#1-overview)
- [2 Requirements](#2-requirements)

## 1 Overview

This document consolidates requirements for the `MCPTOO` domain.
It covers MCP tooling conventions, tool catalog expectations, and SDK installation and integration.

## 2 Requirements

- **REQ-MCPTOO-0001:** Agents use MCP tools for capabilities; no direct DB; user access via gateway only.
  [CYNAI.MCPTOO.McpRole](../tech_specs/mcp_tooling.md#spec-cynai-mcptoo-mcprole)
  [CYNAI.MCPTOO.DatabaseAccessRules](../tech_specs/mcp_tooling.md#spec-cynai-mcptoo-dbaccess)
  <a id="req-mcptoo-0001"></a>
- **REQ-MCPTOO-0002:** Tool names `namespace.operation`, stable ids; schema-validated, size-limited responses; no secrets in errors.
  [CYNAI.MCPTOO.ToolCatalogNaming](../tech_specs/mcp_tool_catalog.md#spec-cynai-mcptoo-toolnaming)
  [CYNAI.MCPTOO.ToolCatalogResponseError](../tech_specs/mcp_tool_catalog.md#spec-cynai-mcptoo-toolresponse)
  <a id="req-mcptoo-0002"></a>
- **REQ-MCPTOO-0003:** MCP SDK installation and integration constraints.
  [CYNAI.MCPTOO.Doc.McpSdkInstallation](../tech_specs/mcp_sdk_installation.md#spec-cynai-mcptoo-doc-mcpsdkinstallation)
  <a id="req-mcptoo-0003"></a>
- **REQ-MCPTOO-0100:** Sandboxed worker agents MUST use MCP tools for controlled capabilities.
  [CYNAI.MCPTOO.McpRole](../tech_specs/mcp_tooling.md#spec-cynai-mcptoo-mcprole)
  <a id="req-mcptoo-0100"></a>
- **REQ-MCPTOO-0101:** Orchestrator-side agents (Project Manager and Project Analyst) MUST use MCP tools for privileged operations.
  [CYNAI.MCPTOO.McpRole](../tech_specs/mcp_tooling.md#spec-cynai-mcptoo-mcprole)
  <a id="req-mcptoo-0101"></a>
- **REQ-MCPTOO-0102:** Direct access to internal services and databases SHOULD be avoided and MUST be restricted by policy.
  [CYNAI.MCPTOO.McpRole](../tech_specs/mcp_tooling.md#spec-cynai-mcptoo-mcprole)
  <a id="req-mcptoo-0102"></a>
- **REQ-MCPTOO-0103:** Sandboxed agents MUST NOT connect directly to PostgreSQL.
  [CYNAI.MCPTOO.DatabaseAccessRules](../tech_specs/mcp_tooling.md#spec-cynai-mcptoo-dbaccess)
  <a id="req-mcptoo-0103"></a>
- **REQ-MCPTOO-0104:** Orchestrator-side agents SHOULD NOT connect directly to PostgreSQL.
  [CYNAI.MCPTOO.DatabaseAccessRules](../tech_specs/mcp_tooling.md#spec-cynai-mcptoo-dbaccess)
  <a id="req-mcptoo-0104"></a>
- **REQ-MCPTOO-0105:** User-facing access MUST be mediated by the User API Gateway and MUST NOT expose raw SQL execution.
  [CYNAI.MCPTOO.DatabaseAccessRules](../tech_specs/mcp_tooling.md#spec-cynai-mcptoo-dbaccess)
  <a id="req-mcptoo-0105"></a>
- **REQ-MCPTOO-0106:** Tool names MUST use `namespace.operation` format.
  [CYNAI.MCPTOO.ToolCatalogNaming](../tech_specs/mcp_tool_catalog.md#spec-cynai-mcptoo-toolnaming)
  <a id="req-mcptoo-0106"></a>
- **REQ-MCPTOO-0107:** Tool names MUST be stable identifiers.
  [CYNAI.MCPTOO.ToolCatalogNaming](../tech_specs/mcp_tool_catalog.md#spec-cynai-mcptoo-toolnaming)
  <a id="req-mcptoo-0107"></a>
- **REQ-MCPTOO-0108:** Namespaces MUST be short and descriptive.
  [CYNAI.MCPTOO.ToolCatalogNaming](../tech_specs/mcp_tool_catalog.md#spec-cynai-mcptoo-toolnaming)
  <a id="req-mcptoo-0108"></a>
- **REQ-MCPTOO-0109:** Tool responses MUST be schema-validated and size-limited.
  [CYNAI.MCPTOO.ToolCatalogResponseError](../tech_specs/mcp_tool_catalog.md#spec-cynai-mcptoo-toolresponse)
  <a id="req-mcptoo-0109"></a>
- **REQ-MCPTOO-0110:** Errors MUST be structured and MUST NOT leak secrets.
  [CYNAI.MCPTOO.ToolCatalogResponseError](../tech_specs/mcp_tool_catalog.md#spec-cynai-mcptoo-toolresponse)
  <a id="req-mcptoo-0110"></a>

- **REQ-MCPTOO-0111:** MCP SDK versions MUST be pinned in each service `go.mod`.
  [CYNAI.MCPTOO.GoSdkVersionPinning](../tech_specs/mcp_sdk_installation.md#spec-cynai-mcptoo-gosdkversionpinning)
  <a id="req-mcptoo-0111"></a>
- **REQ-MCPTOO-0112:** Each Go service SHOULD have its own `go.mod` at the service root.
  [CYNAI.MCPTOO.GoServiceModuleLayout](../tech_specs/mcp_sdk_installation.md#spec-cynai-mcptoo-goservicemodlayout)
  <a id="req-mcptoo-0112"></a>
- **REQ-MCPTOO-0113:** If a multi-module layout is used, a `go.work` MAY be used for local development.
  [CYNAI.MCPTOO.GoServiceModuleLayout](../tech_specs/mcp_sdk_installation.md#spec-cynai-mcptoo-goservicemodlayout)
  <a id="req-mcptoo-0113"></a>
- **REQ-MCPTOO-0114:** If a workflow runtime needs MCP protocol support directly, it SHOULD use the official Python MCP SDK.
  [CYNAI.MCPTOO.PythonSdkUsage](../tech_specs/mcp_sdk_installation.md#spec-cynai-mcptoo-pythonsdkusage)
  <a id="req-mcptoo-0114"></a>
- **REQ-MCPTOO-0115:** The workflow runtime MUST NOT connect directly to PostgreSQL and MUST use MCP database tools (or an internal service enforcing the same policy).
  [CYNAI.MCPTOO.WorkflowRuntimeNoDirectDb](../tech_specs/mcp_sdk_installation.md#spec-cynai-mcptoo-workflowruntimenodb)
  <a id="req-mcptoo-0115"></a>
- **REQ-MCPTOO-0116:** The system SHOULD expose a help MCP server (or help tools) that provide on-demand documentation for how to interact with CyNodeAI (e.g. tool usage, conventions, gateway).
  Help content SHOULD be aligned with and updated on the same cadence as the default CyNodeAI interaction skill.
  [CYNAI.MCPTOO.HelpMcpServer](../tech_specs/mcp_tooling.md#spec-cynai-mcptoo-helpmcpserver)
  <a id="req-mcptoo-0116"></a>
