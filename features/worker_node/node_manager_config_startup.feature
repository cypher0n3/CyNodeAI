@suite_worker_node
Feature: Node Manager Config Fetch and Startup

  As a node manager
  I want to fetch configuration from the orchestrator and apply it before starting services
  So that Worker API bearer token and startup order follow the spec

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

@req_worker_0162
@spec_cynai_worker_workerproxybidirectionalmanagedagents
Scenario: Node manager starts managed services from config desired state
  Given a mock orchestrator that returns bootstrap with node_config_url
  And the mock returns node config with worker_api bearer token
  And the mock returns node config with managed_services containing service "pma-main" of type "pma"
  When the node manager runs the startup sequence against the mock orchestrator
  Then the node manager started managed services from config
  And the node manager sent a config acknowledgement with status "applied"

@req_orches_0169
@req_worker_0264
@spec_cynai_worker_nodestartupprocedure
@spec_cynai_worker_payload_configurationv1
Scenario: Node manager passes through orchestrator-directed backend env values
  Given a mock orchestrator that returns bootstrap with node_config_url
  And the mock returns node capabilities indicating resources sufficient for a large local context window
  And the mock returns node config with inference_backend env key "OLLAMA_NUM_CTX" derived from those capabilities and policy
  And the mock returns node config with managed_services containing service "pma-main" using node_local inference with matching backend_env key "OLLAMA_NUM_CTX"
  When the node manager runs the startup sequence against the mock orchestrator
  Then the node manager launches the local inference backend with the derived "OLLAMA_NUM_CTX" value
  And the node manager launches the managed service with the same derived "OLLAMA_NUM_CTX" value

@req_worker_0002
@spec_cynai_worker_failfast
@wip
Scenario: Node manager fail-fast when inference startup fails
  Given the node manager is configured to fail inference startup
  When the node manager runs the startup sequence against the mock orchestrator
  Then the node manager exits with an error
  And the error indicates inference startup failed
