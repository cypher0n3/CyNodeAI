@suite_worker_node
Feature: Worker API as Managed Service

  As a node manager
  I want to start worker-api as a container (worker-managed service) when NODE_MANAGER_WORKER_API_IMAGE is set
  So that worker-api runs in a container managed by the node on the host (same pattern as PMA)

Background:
  Given a mock orchestrator that returns bootstrap with node_config_url
  And the mock returns node config with worker_api bearer token

Scenario: Node manager starts worker API during startup sequence
  When the node manager runs the startup sequence against the mock orchestrator
  Then the worker API was started

Scenario: Node manager starts worker API as container when configured with image
  Given the node is configured to start worker-api as a container with image "cynodeai-worker-api"
  When the node manager runs the startup sequence against the mock orchestrator
  Then the worker API was started
  And the worker API was started as a container
