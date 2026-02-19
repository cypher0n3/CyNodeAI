@suite_worker_node
Feature: Worker Node Sandbox Execution

  As an orchestrator calling a worker node
  I want the worker API to authenticate requests and run sandboxed jobs
  So that job execution is isolated and results are returned reliably

  Background:
    Given the worker API is running

  @req_worker_0100
  @req_worker_0101
  @req_worker_0102
  @spec_cynai_worker_workerauth
  Scenario: Worker API requires a bearer token
    When I call the worker API without a bearer token
    Then the worker API rejects the request
    When I call the worker API with an invalid bearer token
    Then the worker API rejects the request

  @req_worker_0103
  @req_worker_0106
  @req_worker_0107
  @spec_cynai_worker_sandboxexec
  @spec_cynai_worker_loglimits
  Scenario: Worker API runs a sandbox job and returns stdout and exit code
    Given the worker API is configured with a valid bearer token
    When I submit a sandbox job that runs command "echo hello"
    Then the sandbox job result contains stdout "hello"
    And the sandbox job exit code is 0
