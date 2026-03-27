@suite_orchestrator
Feature: Orchestrator MCP skills tools

  As a PMA agent
  I want to call skills.create with user_id and content only
  So that user-scoped skills work without a task_id

@req_mcpgat_0106 @spec_cynai_mcpgat_doc_gatewayenforcement
Scenario: MCP PM agent skills.create succeeds with user_id and extraneous task_id
  Given a running PostgreSQL database
  And the orchestrator API is running
  And an admin user exists with handle "admin"
  And I am logged in as "admin" with password "admin123"
  When the MCP agent "pm" calls skills.create for user "admin" with content "# BDD skill body" including extraneous task_id
  Then the response status is 200
