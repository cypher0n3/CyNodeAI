@suite_agents
Feature: PMA chat file context

  As an orchestrator
  I want cynode-pma to preserve accepted chat file context in model requests
  So that user-attached files remain available to planning and response generation

@wip
@req_pmagnt_0115
@spec_cynai_pmagnt_chatfilecontext
Scenario: PMA includes accepted chat file context in the LLM request
  Given I have an internal chat completion request with one user message and an accepted file reference
  And I have a mock inference server that captures the request payload
  When I send the request to the PMA internal chat completion endpoint
  Then the captured model request includes the accepted file context

@wip
@req_pmagnt_0115
@spec_cynai_pmagnt_chatfilecontext
Scenario: PMA returns a clear error when the selected model cannot support an accepted file type
  Given I have an internal chat completion request with an accepted unsupported binary file
  When I send the request to the PMA internal chat completion endpoint
  Then the response contains a user-visible unsupported-file-type error
