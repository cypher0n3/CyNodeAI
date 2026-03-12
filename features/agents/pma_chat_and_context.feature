@suite_agents
Feature: PMA chat and context composition

  As an orchestrator
  I want cynode-pma to accept handoff requests and compose LLM context in a defined order
  So that chat completions use baseline, project, task, and user context correctly (REQ-PMAGNT-0108)

@req_pmagnt_0108
@spec_cynai_pmagnt_llmcontextcomposition
Scenario: Internal chat completion accepts handoff with messages only
  Given I have an internal chat completion request with messages only "Hello"
  And I have a mock inference server
  When I send the request to the PMA internal chat completion endpoint
  Then the response status is 200
  And the response content is non-empty

@req_pmagnt_0108
@spec_cynai_pmagnt_llmcontextcomposition
Scenario: Composed context order is baseline then project then task then additional
  Given I have an internal chat completion request with project_id "proj-1" and task_id "task-1" and additional_context "user extra"
  And I have a mock inference server that captures the prompt
  When I send the request to the PMA internal chat completion endpoint
  Then the captured prompt contains "## Project context"
  And the captured prompt contains "## Task context"
  And the captured prompt contains "## User additional context"
  And "## Project context" appears before "## Task context" in the captured prompt
  And "## Task context" appears before "## User additional context" in the captured prompt

@req_orches_0165
@spec_cynai_pmagnt_conversationhistory
@spec_cynai_pmagnt_chatsurfacemapping
Scenario: Responses-surface continuation preserves prior turns and keeps current user input distinct
  Given I have a normalized PMA handoff from POST "/v1/responses" with retained prior turns and current input "Continue the plan"
  And I have a mock inference server that captures the messages
  When I send the request to the PMA internal chat completion endpoint
  Then the captured messages include the retained prior user and assistant turns in order
  And the last captured user message is "Continue the plan"
  And the last captured user message is not folded into the system message
