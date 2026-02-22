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

  @req_usrgwy_0130
  @spec_cynai_usrgwy_openaichatapi
  Scenario: Chat does not imply one task per message
    When I call POST "/v1/chat/completions" with OpenAI-format messages
    Then the response is a chat completion
    And no assertion is made that a user-visible task was created
