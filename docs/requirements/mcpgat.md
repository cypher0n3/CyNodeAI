# MCPGAT Requirements

- [1 Overview](#1-overview)
- [2 Requirements](#2-requirements)

## 1 Overview

This document consolidates requirements for the `MCPGAT` domain.
It covers MCP gateway enforcement and auditing for tool invocation.

## 2 Requirements

- REQ-MCPGAT-0001: Standard MCP protocol on the wire; task/run/job-scoped tool argument schema and rejection of mismatched context.
  [CYNAI.MCPGAT.StandardMcpUsage](../tech_specs/mcp_gateway_enforcement.md#spec-cynai-mcpgat-stdmcpusage)
  [CYNAI.MCPGAT.ToolArgumentSchema](../tech_specs/mcp_gateway_enforcement.md#spec-cynai-mcpgat-toolargschema)
  <a id="req-mcpgat-0001"></a>
- REQ-MCPGAT-0002: Audit record for every tool call (allow/deny and success/failure).
  [CYNAI.MCPGAT.ToolCallAuditPoint](../tech_specs/mcp_tool_call_auditing.md#spec-cynai-mcpgat-toolaudit)
  <a id="req-mcpgat-0002"></a>

- REQ-MCPGAT-0100: The MCP gateway MUST use the standard MCP protocol messages on the wire to MCP servers.
  [CYNAI.MCPGAT.StandardMcpUsage](../tech_specs/mcp_gateway_enforcement.md#spec-cynai-mcpgat-stdmcpusage)
  <a id="req-mcpgat-0100"></a>
- REQ-MCPGAT-0101: The MCP gateway MUST NOT require MCP servers to accept CyNodeAI-specific wrapper fields.
  [CYNAI.MCPGAT.StandardMcpUsage](../tech_specs/mcp_gateway_enforcement.md#spec-cynai-mcpgat-stdmcpusage)
  <a id="req-mcpgat-0101"></a>
- REQ-MCPGAT-0102: The orchestrator MAY attach internal metadata to an invocation record, but it MUST NOT depend on non-standard wire fields.
  [CYNAI.MCPGAT.StandardMcpUsage](../tech_specs/mcp_gateway_enforcement.md#spec-cynai-mcpgat-stdmcpusage)
  <a id="req-mcpgat-0102"></a>
- REQ-MCPGAT-0103: Any tool that is task-scoped MUST include `task_id` in its arguments schema.
  [CYNAI.MCPGAT.ToolArgumentSchema](../tech_specs/mcp_gateway_enforcement.md#spec-cynai-mcpgat-toolargschema)
  <a id="req-mcpgat-0103"></a>
- REQ-MCPGAT-0104: Any tool that is run-scoped SHOULD include `run_id` in its arguments schema.
  [CYNAI.MCPGAT.ToolArgumentSchema](../tech_specs/mcp_gateway_enforcement.md#spec-cynai-mcpgat-toolargschema)
  <a id="req-mcpgat-0104"></a>
- REQ-MCPGAT-0105: Any tool that is job-scoped SHOULD include `job_id` in its arguments schema.
  [CYNAI.MCPGAT.ToolArgumentSchema](../tech_specs/mcp_gateway_enforcement.md#spec-cynai-mcpgat-toolargschema)
  <a id="req-mcpgat-0105"></a>
- REQ-MCPGAT-0106: Tools MUST reject calls where required scoped ids are missing or do not match orchestrator context.
  [CYNAI.MCPGAT.ToolArgumentSchema](../tech_specs/mcp_gateway_enforcement.md#spec-cynai-mcpgat-toolargschema)
  <a id="req-mcpgat-0106"></a>
- REQ-MCPGAT-0107: The orchestrator MCP gateway MUST emit an audit record for every tool call it routes.
  [CYNAI.MCPGAT.ToolCallAuditPoint](../tech_specs/mcp_tool_call_auditing.md#spec-cynai-mcpgat-toolaudit)
  <a id="req-mcpgat-0107"></a>
- REQ-MCPGAT-0108: Audit records MUST be written regardless of allow or deny decisions.
  [CYNAI.MCPGAT.ToolCallAuditPoint](../tech_specs/mcp_tool_call_auditing.md#spec-cynai-mcpgat-toolaudit)
  <a id="req-mcpgat-0108"></a>
- REQ-MCPGAT-0109: Audit records MUST be written regardless of tool call success or failure.
  [CYNAI.MCPGAT.ToolCallAuditPoint](../tech_specs/mcp_tool_call_auditing.md#spec-cynai-mcpgat-toolaudit)
  <a id="req-mcpgat-0109"></a>

- REQ-MCPGAT-0110: For MVP, tool argument payloads and tool result payloads MUST NOT be stored in PostgreSQL audit tables.
  [mcp_tool_call_auditing.md](../tech_specs/mcp_tool_call_auditing.md)
  <a id="req-mcpgat-0110"></a>
- REQ-MCPGAT-0111: Tool payloads MAY be stored in structured logs when needed for debugging, subject to redaction.
  [mcp_tool_call_auditing.md](../tech_specs/mcp_tool_call_auditing.md)
  <a id="req-mcpgat-0111"></a>
