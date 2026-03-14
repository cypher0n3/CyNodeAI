@suite_e2e
Feature: OpenAI-compatible chat contract

  As a CyNodeAI user
  I want to chat through an OpenAI-compatible API surface
  So that Open WebUI and cynork can interoperate with the gateway

Background:
  Given a running PostgreSQL database
  And the orchestrator API is running
  And an admin user exists with handle "admin"

@req_usrgwy_0127
@spec_cynai_usrgwy_openaichatapi
Scenario: OpenAI-compatible model listing is available
  When I call GET "/v1/models"
  Then the response status is 200
  And the response is a list-models payload

@req_usrgwy_0127
@spec_cynai_usrgwy_openaichatapi
Scenario: OpenAI-compatible chat completion returns a completion response
  When I call POST "/v1/chat/completions" with OpenAI-format messages
  Then the response status is 200
  And the response contains a completion at "choices[0].message.content"

@req_usrgwy_0127
@spec_cynai_usrgwy_openaichatapi
Scenario: OpenAI-compatible responses request returns a responses payload
  When I call POST "/v1/responses" with supported OpenAI-format input
  Then the response status is 200
  And the response contains a stable response id
  And the response contains text output in the OpenAI responses shape

@req_usrgwy_0127
@spec_cynai_usrgwy_openaichatapi
Scenario: OpenAI-compatible responses request supports previous_response_id continuation
  Given I previously called POST "/v1/responses" successfully
  When I call POST "/v1/responses" with that previous_response_id and supported OpenAI-format input
  Then the response status is 200
  And the response contains a stable response id

@req_usrgwy_0149
@spec_cynai_usrgwy_openaichatapi_streaming
Scenario: OpenAI-compatible chat completions support stream=true
  When I call POST "/v1/chat/completions" with OpenAI-format messages and stream=true
  Then the response status is 200
  And the response is a streaming chat payload
  And the stream contains ordered incremental assistant events
  And the stream ends with a terminal completion or error event

@req_usrgwy_0149
@spec_cynai_usrgwy_openaichatapi_streaming
Scenario: OpenAI-compatible responses support stream=true
  When I call POST "/v1/responses" with supported OpenAI-format input and stream=true
  Then the response status is 200
  And the response is a streaming chat payload
  And the stream contains ordered incremental assistant events
  And the stream ends with a terminal completion or error event

@req_usrgwy_0130
@spec_cynai_usrgwy_openaichatapi
Scenario: Chat does not imply one task per message
  When I call POST "/v1/chat/completions" with OpenAI-format messages
  Then the response is a chat completion
  And no assertion is made that a user-visible task was created

@req_usrgwy_0135
@spec_cynai_usrgwy_chatthreadsmessages_apisurface
@spec_cynai_usrgwy_chatthreadsmessages_threadacquisition
Scenario: Explicit thread creation returns a distinct thread resource
  When I call POST "/v1/chat/threads" as an authenticated user
  Then the response status is 201
  And the response contains a created chat thread identifier

@req_usrgwy_0150
@spec_cynai_usrgwy_openaichatapi_streaming
Scenario: Client cancel or disconnect causes gateway to treat streaming request as canceled
  Given a streaming POST "/v1/chat/completions" or POST "/v1/responses" with stream=true is in progress
  When the client closes the connection or cancels the request before the stream completes
  Then the gateway treats that stream as canceled
  And the gateway stops or detaches upstream generation on a best-effort basis
  And request-scoped resources are released even when the final assistant turn is incomplete
