@suite_orchestrator
Feature: Chat completion reliability

  As a CyNodeAI user
  I want chat completions to respect bounded wait and retry transient failures
  So that the API is predictable and resilient (REQ-ORCHES-0131, REQ-ORCHES-0132)

@req_orches_0131
@req_orches_0132
@spec_cynai_usrgwy_openaichatapi
Scenario: Chat completion returns 200 or acceptable error status
  Given a running PostgreSQL database
  And the orchestrator API is running
  And an admin user exists with handle "admin"
  And I am logged in as "admin"
  When I send a chat message "Hello"
  Then the response status is one of 200, 502, 504
