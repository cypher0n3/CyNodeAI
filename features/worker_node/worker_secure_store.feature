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
@req_worker_0171
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

@req_worker_0168
@spec_cynai_worker_nodelocalsecurestore
Scenario: Managed-service container run args do not mount secure store
  Given a managed service is configured for the node
  When the node manager builds run args for the managed-service container
  Then the run args do not contain any mount of the secure store or secrets path

@req_worker_0269
@spec_cynai_worker_payload_configurationv1
Scenario: Managed-service container run args include health-check when config has healthcheck and runtime is podman
  When the node manager builds run args for the managed-service container with healthcheck and runtime podman
  Then the run args include podman health-check options

@req_worker_0169
@spec_cynai_worker_nodelocalsecurestore
Scenario: Secure store is distinct from telemetry database
  Given the node state directory is set
  Then the secure store path is under state_dir and distinct from the telemetry database path

@req_worker_0170
@spec_cynai_worker_nodelocalsecurestore
Scenario: FIPS mode rejects env master key fallback
  Given FIPS mode is enabled or unknown on the host
  And the environment variable CYNODE_SECURE_STORE_MASTER_KEY_B64 is set
  When the node opens the secure store with env master key only
  Then opening the secure store fails with FIPS-related error

@req_worker_0172
@spec_cynai_worker_securestoreprocessboundary
Scenario: Process boundary for secure store is documented
  Given the worker node codebase
  Then the secure store process boundary document exists and states writer and reader components

@req_worker_0270
@req_worker_0174
@spec_cynai_worker_unifiedudspath
Scenario: Managed-service container run args inject UDS OLLAMA_BASE_URL not TCP
  Given a managed service is configured for the node
  When the node manager builds run args for the managed-service container with node_local inference
  Then the run args contain OLLAMA_BASE_URL with an http+unix scheme
  And the run args do not contain OLLAMA_BASE_URL with a TCP address

@req_worker_0174
@spec_cynai_worker_unifiedudspath
Scenario: Managed-service container run args include network=none
  Given a managed service is configured for the node
  When the node manager builds run args for the managed-service container
  Then the run args include --network=none

@req_worker_0270
@req_worker_0174
@spec_cynai_worker_unifiedudspath
Scenario: Managed-service container run args do not publish TCP port 8090
  Given a managed service is configured for the node
  When the node manager builds run args for the managed-service container
  Then the run args do not publish port 8090
