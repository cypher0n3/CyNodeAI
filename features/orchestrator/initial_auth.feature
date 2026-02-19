@suite_orchestrator
Feature: Initial User Authentication

  As a CyNodeAI user
  I want to sign in with local credentials (login, refresh, logout, current user)
  So that I can call protected APIs and manage my session

  Background:
    Given a running PostgreSQL database
    And the orchestrator API is running
    And an admin user exists with handle "admin"

  @req_identy_0104
  @spec_cynai_identy_authenticationmodel
  Scenario: User login with valid credentials
    When I login as "admin" with password "admin123"
    Then I receive an access token
    And I receive a refresh token

  @req_identy_0105
  @spec_cynai_identy_authenticationmodel
  Scenario: Token refresh returns new tokens
    Given I am logged in as "admin"
    When I refresh my token
    Then I receive a new access token
    And I receive a new refresh token
    And the old refresh token is invalidated

  @req_identy_0106
  @spec_cynai_identy_authenticationmodel
  Scenario: User logout revokes session
    Given I am logged in as "admin"
    When I logout
    Then my refresh token is invalidated
    And I cannot use the old access token

  @req_identy_0119
  @spec_cynai_identy_userapigatewaysurface
  Scenario: Get current user info
    Given I am logged in as "admin"
    When I request my user info
    Then I receive my user details including handle "admin"
