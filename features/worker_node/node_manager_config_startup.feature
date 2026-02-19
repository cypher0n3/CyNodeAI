@suite_worker_node
Feature: Node Manager Config Fetch and Startup

  As a node manager
  I want to fetch configuration from the orchestrator and apply it before starting services
  So that Worker API bearer token and startup order follow the spec (Phase 1)

  Background:
    Given a mock orchestrator that returns bootstrap with node_config_url
    And the mock returns node config with worker_api bearer token

  @req_worker_0135
  @spec_cynai_worker_payload_configurationv1
  Scenario: Node manager fetches config via bootstrap node_config_url
    When the node manager runs the startup sequence against the mock orchestrator
    Then the node manager requested config using the bootstrap node_config_url
    And the received config contains worker_api orchestrator_bearer_token

  @req_worker_0135
  @spec_cynai_worker_payload_configackv1
  Scenario: Node manager sends config acknowledgement after applying config
    When the node manager runs the startup sequence against the mock orchestrator
    Then the node manager sent a config acknowledgement with status "applied"

  @req_worker_0002
  @spec_cynai_worker_failfast
  Scenario: Node manager fail-fast when inference startup fails
    Given the node manager is configured to fail inference startup
    When the node manager runs the startup sequence against the mock orchestrator
    Then the node manager exits with an error
    And the error indicates inference startup failed
