Feature: Single Node Happy Path
  As a CyNodeAI user
  I want to create tasks that execute on worker nodes
  So that I can automate sandboxed execution of commands

  Background:
    Given a running PostgreSQL database
    And the orchestrator API is running
    And an admin user exists with handle "admin"

  Scenario: User login with valid credentials
    When I login as "admin" with password "admin123"
    Then I receive an access token
    And I receive a refresh token

  Scenario: Token refresh returns new tokens
    Given I am logged in as "admin"
    When I refresh my token
    Then I receive a new access token
    And I receive a new refresh token
    And the old refresh token is invalidated

  Scenario: User logout revokes session
    Given I am logged in as "admin"
    When I logout
    Then my refresh token is invalidated
    And I cannot use the old access token

  Scenario: Get current user info
    Given I am logged in as "admin"
    When I request my user info
    Then I receive my user details including handle "admin"

  Scenario: Node registration with PSK
    Given a node with slug "test-node-01" and valid PSK
    When the node registers with the orchestrator
    Then the node receives a JWT token
    And the node is recorded in the database

  Scenario: Node capability reporting
    Given a registered node "test-node-01"
    When the node reports its capabilities
    Then the orchestrator stores the capability snapshot
    And the capability hash is updated

  Scenario: Create task as authenticated user
    Given I am logged in as "admin"
    When I create a task with prompt "echo hello world"
    Then I receive a task ID
    And the task status is "pending"

  Scenario: Get task status
    Given I am logged in as "admin"
    And I have created a task
    When I get the task status
    Then I receive the task details including status

  Scenario: End-to-end task execution
    Given I am logged in as "admin"
    And a registered node "test-node-01" is active
    When I create a task with command "echo hello"
    And the orchestrator dispatches a job to the node
    And the node executes the sandbox job
    Then the job result contains stdout "hello"
    And the task status becomes "completed"

  Scenario: Retrieve task result
    Given I am logged in as "admin"
    And I have a completed task
    When I get the task result
    Then I receive the job output including stdout and exit code
