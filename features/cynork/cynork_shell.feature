@suite_cynork
Feature: cynork interactive shell

  As a user of the cynork CLI
  I want to run interactive shell mode with tab completion
  So that I can complete task names and run commands from one session

  Background:
    Given a mock gateway is running
    And cynork is built

  @req_client_0159
  @spec_cynai_client_cliinteractivemode
  Scenario: Interactive mode offers tab-completion of task names
    Given I am logged in with username "alice" and password "secret"
    And at least one task exists with a human-readable name
    When I run cynork shell in interactive mode
    And I request tab-completion for a task identifier position
    Then the completion candidates include task names
