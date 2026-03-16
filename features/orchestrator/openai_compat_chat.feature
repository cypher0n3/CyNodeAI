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
  When I send a chat message "What is the status?"
    with model cynodeai.pm
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
  When I send a responses request with input "What is the status?"
    and model cynodeai.pm
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

@req_usrgwy_0152
@spec_cynai_usrgwy_openaichatapi_streamingperendpointsseformat
Scenario: Gateway relays PMA NDJSON events as per-endpoint SSE for chat completions
  Given a running PostgreSQL database
  And the orchestrator API is running
  And an admin user exists with handle "admin"
  And I am logged in as "admin"
  And a mock PMA server is running that streams NDJSON events including delta, thinking, tool_call, and iteration_start
  And a node "bdd-pma" exists and has reported PMA ready at the mock PMA server
  When I send a streaming chat message "Hello" with model cynodeai.pm to POST "/v1/chat/completions"
  Then visible-text deltas arrive as unnamed SSE data lines with choices[0].delta.content
  And thinking events arrive as "event: cynodeai.thinking_delta" SSE events
  And tool-call events arrive as "event: cynodeai.tool_call" SSE events
  And iteration-start events arrive as "event: cynodeai.iteration_start" SSE events
  And the stream ends with "data: [DONE]"

@req_usrgwy_0153
@spec_cynai_usrgwy_openaichatapi_streamingredactionpipeline
Scenario: Gateway maintains three accumulators and scans all for secrets post-stream
  Given a running PostgreSQL database
  And the orchestrator API is running
  And an admin user exists with handle "admin"
  And I am logged in as "admin"
  And a mock PMA server is running that streams visible text, thinking content, and tool-call content containing a secret
  And a node "bdd-pma" exists and has reported PMA ready at the mock PMA server
  When I send a streaming chat message "Summarize keys" with model cynodeai.pm
  Then the gateway scans all three accumulators for secrets before emitting DONE
  And the gateway emits cynodeai.amendment events for each content type where secrets were detected
  And only redacted content is persisted

@req_usrgwy_0154
@spec_cynai_usrgwy_openaichatapi_streamingredactionpipeline
Scenario: Gateway persists redacted thinking and tool-call content as structured turn parts
  Given a running PostgreSQL database
  And the orchestrator API is running
  And an admin user exists with handle "admin"
  And I am logged in as "admin"
  And a mock PMA server is running that streams thinking content and tool-call content
  And a node "bdd-pma" exists and has reported PMA ready at the mock PMA server
  When I send a streaming chat message "Plan the task" with model cynodeai.pm
  Then the persisted assistant turn includes thinking content alongside visible text
  And the persisted assistant turn includes tool-call content alongside visible text
  And only redacted versions are persisted

@req_usrgwy_0155
@spec_cynai_usrgwy_openaichatapi_streamingredactionpipeline
Scenario: Gateway relays PMA overwrite events and updates its own accumulator
  Given a running PostgreSQL database
  And the orchestrator API is running
  And an admin user exists with handle "admin"
  And I am logged in as "admin"
  And a mock PMA server is running that streams visible text then emits a per-iteration overwrite event
  And a node "bdd-pma" exists and has reported PMA ready at the mock PMA server
  When I send a streaming chat message "Explain the API key" with model cynodeai.pm
  Then the gateway relays the overwrite as a cynodeai.amendment SSE event
  And the gateway accumulator reflects the overwritten content
  And the final persisted content matches the overwritten text

@req_usrgwy_0156
@spec_cynai_usrgwy_openaichatapi_streamingheartbeatfallback
Scenario: Gateway emits heartbeat SSE events when PMA cannot stream
  Given a running PostgreSQL database
  And the orchestrator API is running
  And an admin user exists with handle "admin"
  And I am logged in as "admin"
  And a mock PMA server is running that responds with a non-streaming JSON payload after a delay
  And a node "bdd-pma" exists and has reported PMA ready at the mock PMA server
  When I send a streaming chat message "Hello" with model cynodeai.pm
  Then the gateway emits periodic "event: cynodeai.heartbeat" SSE events with elapsed_s metadata
  And when the PMA response completes the full content arrives as a single visible-text delta
  And the stream ends with the terminal DONE event
