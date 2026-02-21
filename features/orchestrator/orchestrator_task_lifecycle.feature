@suite_orchestrator
Feature: Orchestrator Task Lifecycle

  As an authenticated CyNodeAI user
  I want to create tasks and retrieve their status and results
  So that I can use the orchestrator to schedule and observe work execution

  Background:
    Given a running PostgreSQL database
    And the orchestrator API is running
    And an admin user exists with handle "admin"
    And I am logged in as "admin"
    And a registered node "test-node-01" is active
    And the node "test-node-01" has worker_api_target_url and bearer token in config

  @req_orches_0121
  @spec_cynai_orches_doc_orchestrator
  Scenario: Create task as authenticated user
    When I create a task with prompt "echo hello world"
    Then I receive a task ID
    And the task status is "pending"

  @req_orches_0124
  @spec_cynai_orches_doc_orchestrator
  Scenario: Get task status
    Given I have created a task
    When I get the task status
    Then I receive the task details including status

  @req_orches_0123
  @spec_cynai_orches_doc_orchestrator
  Scenario: Retrieve task result
    Given I have a completed task
    When I get the task result
    Then I receive the job output including stdout and exit code

  @req_worker_0100
  @req_orches_0122
  @spec_cynai_worker_workerauth
  @spec_cynai_orches_doc_orchestrator
  Scenario: Dispatcher uses per-node worker URL and token
    When I create a task with command "echo hello"
    And the orchestrator selects the node for dispatch
    Then the orchestrator calls the node Worker API at its configured target URL
    And the request includes the bearer token from that node's config

  @req_orches_0125
  @req_orches_0127
  @spec_cynai_client_clitaskcreateprompt
  Scenario: Task with natural-language prompt (default) completes with model output
    When I create a task with prompt "What is 2+2?"
    And the task completes
    And I get the task result
    Then the task result contains model output

  @req_orches_0125
  @req_orches_0127
  @spec_cynai_client_clitaskcreateprompt
  Scenario: Task with input_mode commands runs literal shell
    When I create a task with input_mode "commands" and prompt "echo hello"
    And the orchestrator selects the node for dispatch
    Then the job sent to the worker has command containing "echo hello"
