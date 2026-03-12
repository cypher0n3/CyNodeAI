@suite_orchestrator
Feature: Node Registration and Configuration

  As an operator
  I want worker nodes to register with the orchestrator and receive configuration
  So that the orchestrator can dispatch work to nodes using the correct URL and credentials

Background:
  Given a running PostgreSQL database
  And the orchestrator API is running

@req_orches_0112
@req_worker_0139
@req_orches_0148
@spec_cynai_orches_doc_orchestrator
Scenario: Node registration with PSK
  Given a node with slug "test-node-01" and valid PSK
  When the node registers with the orchestrator
  Then the node receives a JWT token
  And the node is recorded in the database

@req_worker_0139
@req_orches_0148
@spec_cynai_worker_payload_capabilityreport_v1
Scenario: Node registration includes worker API base_url
  Given a node with slug "test-node-02" and valid PSK and worker API URL "http://worker-02.example.com:12090"
  When the node registers with the orchestrator
  Then the node is recorded in the database
  And the orchestrator stored worker_api_target_url from the node-reported base_url for "test-node-02"

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
  When the node registers with the orchestrator and requests its configuration
  Then the orchestrator returns a configuration payload for "test-node-01"
  And the payload includes config_version and worker_api bearer token

@req_worker_0135
@spec_cynai_worker_payload_configackv1
Scenario: Node sends config acknowledgement
  Given a registered node "test-node-01" that has received configuration
  When the node applies the configuration and sends a config acknowledgement with status "applied"
  Then the orchestrator records the config ack for "test-node-01"
  And the node config_version is stored

@req_worker_0135
@spec_cynai_worker_payload_configurationv1
Scenario: GET config without node JWT returns 401
  When an unauthenticated request requests node configuration
  Then the orchestrator responds with 401 Unauthorized

@req_worker_0135
@spec_cynai_worker_payload_configackv1
Scenario: Config ack with wrong node_slug is rejected
  Given a registered node "test-node-01" that has received configuration
  When the node sends a config acknowledgement with node_slug "wrong-slug" and status "applied"
  Then the orchestrator responds with 400 Bad Request

@req_orches_0149
@spec_cynai_worker_payload_configurationv1
Scenario: GET config returns inference_backend when node is inference-capable and not existing_service
  Given a node with slug "inference-node-01" and valid PSK
  And the node registers with capability inference supported and not existing_service
  When the node requests its configuration
  Then the orchestrator returns a configuration payload for "inference-node-01"
  And the payload includes inference_backend with enabled true
