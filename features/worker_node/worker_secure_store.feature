@suite_worker_node
Feature: Worker Node Secure Store and Agent Tokens

  As a worker node operator
  I want orchestrator-issued secrets stored securely on the node
  So that agent tokens are never exposed to containers and secrets are encrypted at rest

@req_worker_0165
@spec_cynai_worker_nodelocalsecurestore
Scenario: Worker stores orchestrator-issued secrets encrypted at rest
  Given the node manager receives configuration containing orchestrator-issued secrets
  When the node manager applies configuration
  Then the node stores secrets in the node-local secure store under storage.state_dir
  And the persisted secret values are encrypted at rest

@req_worker_0164
@spec_cynai_worker_agenttokenstorageandlifecycle
Scenario: Worker holds agent token and does not pass it to managed-service containers
  Given the orchestrator configures a managed agent service with an agent token
  When the node manager starts the managed-service container
  Then the managed-service container does not receive the agent token via env vars, files, or mounts
  And the worker proxy attaches the agent token when forwarding agent-originated requests

@req_worker_0167
@spec_cynai_worker_nodelocalsecurestore
Scenario: Worker warns when using env var master key fallback
  Given the node cannot access TPM, OS key store, or system service credentials
  And the environment variable CYNODE_SECURE_STORE_MASTER_KEY_B64 is set
  When the node starts
  Then the node uses the env_b64 master key backend
  And the node emits a startup warning indicating a less-secure master key backend

