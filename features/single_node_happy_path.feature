Feature: Single Node Happy Path

  As a CyNodeAI user running orchestrator and worker on the same host (single-node)
  I want to create tasks that execute on that local worker node
  So that I can automate sandboxed execution of commands without a separate machine

# Precondition: at least one inference-capable path must be available.
# Inference may be node-local (Ollama or similar) or external via a configured provider key.
# In the single-node case, startup should fail fast if neither is available.

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

  @req_orches_0112
  @spec_cynai_orches_doc_orchestrator
  Scenario: Node registration with PSK
    Given a node with slug "test-node-01" and valid PSK
    When the node registers with the orchestrator
    Then the node receives a JWT token
    And the node is recorded in the database

  @req_orches_0113
  @spec_cynai_orches_doc_orchestrator
  Scenario: Node capability reporting
    Given a registered node "test-node-01"
    When the node reports its capabilities
    Then the orchestrator stores the capability snapshot
    And the capability hash is updated

  @req_orches_0121
  @spec_cynai_orches_doc_orchestrator
  Scenario: Create task as authenticated user
    Given I am logged in as "admin"
    When I create a task with prompt "echo hello world"
    Then I receive a task ID
    And the task status is "pending"

  @req_orches_0124
  @spec_cynai_orches_doc_orchestrator
  Scenario: Get task status
    Given I am logged in as "admin"
    And I have created a task
    When I get the task status
    Then I receive the task details including status

  @req_orches_0122
  @spec_cynai_worker_nodesandboxcontrolplane
  Scenario: End-to-end task execution
    Given I am logged in as "admin"
    And a registered node "test-node-01" is active
    When I create a task with command "echo hello"
    And the orchestrator dispatches a job to the node
    And the node executes the sandbox job
    Then the job result contains stdout "hello"
    And the task status becomes "completed"

  @req_orches_0123
  @spec_cynai_orches_doc_orchestrator
  Scenario: Retrieve task result
    Given I am logged in as "admin"
    And I have a completed task
    When I get the task result
    Then I receive the job output including stdout and exit code

  @req_orches_0113
  @req_worker_0135
  @spec_cynai_orches_doc_orchestrator
  @spec_cynai_worker_payload_configurationv1
  Scenario: Node fetches config after registration
    Given a node with slug "test-node-01" and valid PSK
    When the node registers with the orchestrator
    And the node requests its configuration
    Then the orchestrator returns a configuration payload for "test-node-01"
    And the payload includes config_version and worker_api bearer token

  @req_worker_0135
  @spec_cynai_worker_payload_configackv1
  Scenario: Node sends config acknowledgement
    Given a registered node "test-node-01" that has received configuration
    When the node applies the configuration
    And the node sends a config acknowledgement with status "applied"
    Then the orchestrator records the config ack for "test-node-01"
    And the node config_version is stored

  @req_worker_0100
  @req_orches_0122
  @spec_cynai_worker_workerauth
  @spec_cynai_orches_doc_orchestrator
  Scenario: Dispatcher uses per-node worker URL and token
    Given I am logged in as "admin"
    And a registered node "test-node-01" has worker_api_target_url and bearer token in config
    When I create a task with command "echo hello"
    And the orchestrator selects the node for dispatch
    Then the orchestrator calls the node Worker API at its configured target URL
    And the request includes the bearer token from that node's config

  @req_bootst_0002
  @spec_cynai_bootst_bootstrapsource
  Scenario: Orchestrator fails fast when no inference path is available
    Given no local inference (Ollama) is running
    And no external provider key is configured
    When the orchestrator starts
    Then the orchestrator does not enter ready state
    And the orchestrator reports that no inference path is available
