@suite_orchestrator
Feature: OpenAI-compatible interactive chat reliability

  As a CyNodeAI user
  I want interactive chat requests to respect bounded wait and retry transient failures
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

@req_orches_0131
@req_orches_0132
@spec_cynai_usrgwy_openaichatapi
Scenario: Responses request returns 200 or acceptable error status
  Given a running PostgreSQL database
  And the orchestrator API is running
  And an admin user exists with handle "admin"
  And I am logged in as "admin"
  When I send a responses request with input "Hello"
  Then the response status is one of 200, 502, 504

@req_orches_0162
@spec_cynai_usrgwy_openaichatapi
Scenario: Chat with model cynodeai.pm uses worker-reported PMA endpoint only
  Given a running PostgreSQL database
  And the orchestrator API is running
  And an admin user exists with handle "admin"
  And I am logged in as "admin"
  And a mock PMA server is running
  And a node "bdd-pma" exists and has reported PMA ready at the mock PMA server
  When I send a chat message "What is the status?" with model cynodeai.pm
  Then I receive a 200 response with non-empty response field

@req_orches_0162
@spec_cynai_usrgwy_openaichatapi
Scenario: Responses request with model cynodeai.pm uses worker-reported PMA endpoint only
  Given a running PostgreSQL database
  And the orchestrator API is running
  And an admin user exists with handle "admin"
  And I am logged in as "admin"
  And a mock PMA server is running
  And a node "bdd-pma" exists and has reported PMA ready at the mock PMA server
  When I send a responses request with input "What is the status?" and model cynodeai.pm
  Then I receive a 200 response with non-empty response field

@req_orches_0165
@req_orches_0166
@spec_cynai_orches_responsescontinuationstate
Scenario: Responses continuation accepts a retained same-scope previous_response_id
  Given a running PostgreSQL database
  And the orchestrator API is running
  And an admin user exists with handle "admin"
  And I am logged in as "admin"
  And I have a retained responses state for project "proj-1"
  When I send a responses request with previous_response_id for project "proj-1" and input "Continue"
  Then the response status is 200
  And the response contains a stable response id

@req_orches_0165
@spec_cynai_orches_responsescontinuationstate
Scenario: Responses continuation rejects a previous_response_id from another project
  Given a running PostgreSQL database
  And the orchestrator API is running
  And an admin user exists with handle "admin"
  And I am logged in as "admin"
  And I have a retained responses state for project "proj-a"
  When I send a responses request with that previous_response_id for project "proj-b" and input "Continue"
  Then the response status is 400
  And the response is an OpenAI-style error payload
