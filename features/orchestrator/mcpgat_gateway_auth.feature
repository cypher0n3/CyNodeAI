@suite_orchestrator
Feature: MCP gateway authentication

  As a control-plane operator
  I want MCP tool calls to require agent credentials
  So that anonymous callers cannot invoke tools (gateway enforcement)

@req_mcpgat_0001 @spec_cynai_mcpgat_doc_gatewayenforcement
Scenario: MCP tool call without agent token is unauthorized
  Given a running PostgreSQL database
  And the orchestrator API is running
  When I call POST "/v1/mcp/tools/call" without Authorization with tool_name "help.list"
  Then the response status is 401
