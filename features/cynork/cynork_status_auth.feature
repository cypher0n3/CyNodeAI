@suite_cynork
Feature: cynork status and auth

  As a user of the cynork CLI
  I want to check gateway status and authenticate
  So that I can call protected APIs and manage my session

  Background:
    Given a mock gateway is running
    And cynork is built

  @req_client_0101
  @spec_cynai_client_clicommandsurface
  Scenario: Check gateway status
    When I run cynork status
    Then cynork exits with code 0
    And cynork stdout contains "ok"

  @req_client_0158
  @spec_cynai_client_clicommandsurface
  Scenario: Shorthand flags are accepted
    When I run cynork status with output json using shorthand "-o"
    Then cynork exits with code 0

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
    Then cynork exits with code 3

  @req_client_0150
  @spec_cynai_client_clisessionpersistence
  Scenario: Consecutive invocations use stored session
    When I run cynork auth login with username "alice" and password "secret"
    Then cynork exits with code 0
    When I run cynork auth whoami using the stored config
    Then cynork exits with code 0
    And cynork stdout contains "handle=alice"
