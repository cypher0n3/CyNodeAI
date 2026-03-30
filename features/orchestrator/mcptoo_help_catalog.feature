@suite_orchestrator
Feature: MCP tools catalog (help.list)

  As a tool author
  I want the embedded help.list surface to return structured topic metadata
  So that MCP clients can discover help topics consistent with the gateway implementation

@req_mcptoo_0001 @spec_cynai_mcptoo_doc_mcpsdkinstallation
Scenario: PM agent help.list returns topic catalog JSON
  Given a running PostgreSQL database
  And the orchestrator API is running
  When the MCP agent "pm" calls help.list with empty arguments
  Then the response status is 200
  And the last HTTP response body contains "topics"
