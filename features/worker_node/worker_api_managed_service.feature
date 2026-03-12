@suite_worker_node
Feature: Worker API Embedded (Single-Process)

  As a node manager
  I want to start the Worker API in-process (single binary)
  So that the worker node runs as one process (cynodeai-wnm) with Node Manager and Worker API

@req_worker_0262
@req_worker_0263
@spec_cynai_worker_payload_configurationv1
Scenario: Node manager starts worker API during startup sequence
  Given a mock orchestrator that returns bootstrap with node_config_url
  And the mock returns node config with worker_api bearer token
  When the node manager runs the startup sequence against the mock orchestrator
  Then the worker API was started
