Feature: Node Registration and Configuration

  As an operator
  I want worker nodes to register with the orchestrator and receive configuration
  So that the orchestrator can dispatch work to nodes using the correct URL and credentials

  Background:
    Given a running PostgreSQL database
    And the orchestrator API is running

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
