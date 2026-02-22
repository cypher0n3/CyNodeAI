@suite_cynork
Feature: cynork chat

  As a user of the cynork CLI
  I want to chat with the PM/PA via the OpenAI-compatible gateway
  So that I can have a conversation and use slash commands

  Background:
    Given a mock gateway is running
    And cynork is built

  @req_client_0161
  @spec_cynai_client_clichat
  Scenario: Chat without token fails with auth error
    When I run cynork chat
    Then cynork exits with code 3

  @req_client_0161
  @spec_cynai_client_clichat
  Scenario: Chat uses OpenAI-compatible chat surface and accepts exit
    Given I am logged in with username "alice" and password "secret"
    When I run cynork chat
    And I send "/exit" to cynork stdin
    Then cynork exits with code 0
