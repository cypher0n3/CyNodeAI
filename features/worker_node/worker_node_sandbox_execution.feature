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

  @req_worker_0103
  @req_worker_0104
  @spec_cynai_worker_sandboxexec
  @phase1_sandbox
  Scenario: Sandbox runs with network_policy "none"
    Given the worker API is configured with a valid bearer token
    When I submit a sandbox job with network_policy "none" that runs command "echo ok"
    Then the sandbox job completes successfully

  @req_worker_0103
  @req_worker_0104
  @spec_cynai_worker_sandboxexec
  @phase1_sandbox
  Scenario: Sandbox runs with network_policy "restricted" (deny-all)
    Given the worker API is configured with a valid bearer token
    When I submit a sandbox job with network_policy "restricted" that runs command "echo ok"
    Then the sandbox job completes successfully

  @req_worker_0103
  @req_worker_0104
  @spec_cynai_worker_sandboxexec
  @phase1_sandbox
  Scenario: Sandbox has working directory and task context environment
    Given the worker API is configured with a valid bearer token
    When I submit a sandbox job that runs command "echo CYNODE_TASK_ID=$CYNODE_TASK_ID CYNODE_JOB_ID=$CYNODE_JOB_ID CYNODE_WORKSPACE_DIR=$CYNODE_WORKSPACE_DIR"
    Then the sandbox job result stdout contains "CYNODE_TASK_ID=bdd-task"
    And the sandbox job result stdout contains "CYNODE_JOB_ID=bdd-job"
    And the sandbox job result stdout contains "CYNODE_WORKSPACE_DIR="

  @req_worker_0104
  @spec_cynai_worker_sandboxexec
  @phase1_sandbox
  Scenario: Request env cannot override CYNODE_ task context
    Given the worker API is configured with a valid bearer token
    When I submit a sandbox job with env "CYNODE_TASK_ID=forged" that runs command "echo $CYNODE_TASK_ID"
    Then the sandbox job result stdout contains "bdd-task"

  @req_worker_0103
  @req_worker_0104
  @spec_cynai_worker_sandboxexec
  @spec_cynai_stands_portsandendpoints
  @inference_in_sandbox
  Scenario: Sandbox receives OLLAMA_BASE_URL when job requests inference
    Given the worker API is configured with a valid bearer token
    When I submit a sandbox job with use_inference that runs command "echo $OLLAMA_BASE_URL"
    Then the sandbox job completes successfully
    And the sandbox job result stdout contains "http://localhost:11434"

  @req_worker_0140
  @req_worker_0142
  @spec_cynai_worker_workerapihealthchecks
  Scenario: Worker API GET readyz returns ready when runtime is available
    When I call GET /readyz on the worker API
    Then the worker API returns status 200
    And the response body is "ready"

  @req_worker_0145
  @spec_cynai_worker_workerapirequestsizelimits
  Scenario: Worker API returns 413 when request body is too large
    Given the worker API is configured with a valid bearer token
    When I submit a sandbox job request with body size exceeding the limit
    Then the worker API returns status 413
