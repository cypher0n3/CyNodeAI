@suite_cynork
Feature: cynork CLI

  As a user of the cynork CLI
  I want to call the User API Gateway for status, auth, and tasks
  So that I can manage sessions and run tasks from the command line

  Background:
    Given a mock gateway is running
    And cynork is built

  @req_client_0101
  @spec_cynai_client_clicommandsurface
  Scenario: Check gateway status
    When I run cynork status
    Then cynork exits with code 0
    And cynork stdout contains "ok"

  @req_client_0105
  @spec_cynai_client_cliauth
  Scenario: Login and whoami
    When I run cynork auth login with username "alice" and password "secret"
    Then cynork exits with code 0
    When I run cynork auth whoami
    Then cynork exits with code 0
    And cynork stdout contains "handle=alice"

  @req_client_0104
  @spec_cynai_client_cliauth
  Scenario: Whoami without token fails
    When I run cynork auth whoami
    Then cynork exits with code 1

  @req_client_0101
  @spec_cynai_client_clicommandsurface
  Scenario: Create task and get result
    Given I am logged in with username "alice" and password "secret"
    When I run cynork task create with prompt "echo hello"
    Then cynork exits with code 0
    And I store the task id from cynork stdout
    When I run cynork task result with the stored task id
    Then cynork exits with code 0
    And cynork stdout contains "status=completed"
    And cynork stdout contains "hello"
