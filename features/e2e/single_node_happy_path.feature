@suite_e2e
Feature: Single Node Happy Path

  As a CyNodeAI user running orchestrator and worker on the same host (single-node)
  I want to create tasks that execute on that local worker node
  So that I can automate sandboxed execution of commands without a separate machine

# Precondition: At Least One Inference-Capable Path Must Be available

# Inference May Be Node-Local (Ollama or Similar) or External via a Configured Provider key

# In the Single-Node Case, Startup Should Fail Fast If Neither is available

#

# Script-Driven E2E (Just e2e / `setup-dev.sh` Full-Demo): After the Node Starts, the Script

# Pulls the Effective Project Manager Model Into Ollama and Runs a Basic Inference Smoke to Verify Inference works

# Default PM Model is Chosen by VRAM Tier (8 GB: `Qwen3.5` 9B; 16 GB: `Qwen3.5` 9B / `Qwen2.5` 14B; `qwen3.5:0.8b` is Smallest fallback)

# See dev_docs/single_node_e2e_testing_plan.md

## Background

  Given a running PostgreSQL database
  And the orchestrator API is running
  And an admin user exists with handle "admin"
  And a worker node is running and reachable by the orchestrator

@req_identy_0104
@req_orches_0112
@req_orches_0122
@spec_cynai_identy_authenticationmodel
@spec_cynai_orches_doc_orchestrator
@spec_cynai_worker_workerauth
Scenario: End-to-end single-node task execution (happy path)
  When I login as "admin" with password "admin123" and a node with slug "test-node-01" registers with the orchestrator using a valid PSK and the node requests its configuration and the node applies the configuration and sends a config acknowledgement with status "applied" and I create a task with command "echo hello" and the orchestrator dispatches a job to the node and the node executes the sandbox job
  Then the job result contains stdout "hello"
  And the task status becomes "completed"

# Requires Inference-Capable Node (Proxy Sidecar + Model Loaded). Select With: --Tags=@inference_in_sandbox

@inference_in_sandbox
@req_orches_0112
@spec_cynai_worker_sandboxexec
@spec_cynai_stands_portsandendpoints
Scenario: Single-node task execution with inference in sandbox over UDS
  When I login as "admin" with password "admin123" and a node with slug "test-node-01" registers with the orchestrator using a valid PSK and the node requests its configuration and the node applies the configuration and sends a config acknowledgement with status "applied" and I create a task with command "sh -c 'echo $INFERENCE_PROXY_URL'" and the orchestrator dispatches a job to the node and the node executes the sandbox job in a pod with inference proxy
  Then the job result contains stdout "http+unix://"
  And the task status becomes "completed"

@inference_in_sandbox
@req_sbagnt_0109
@spec_cynai_sbagnt_workerproxies
@spec_cynai_sbagnt_resultcontract
Scenario: Single-node SBA task with inference completes with user-facing reply
  When I login as "admin" with password "admin123" and a node with slug "test-node-01" registers with the orchestrator using a valid PSK and the node requests its configuration and the node applies the configuration and sends a config acknowledgement with status "applied" and I create a SBA task with inference and prompt "Reply back with the current time" and the orchestrator dispatches the job to the node and the node runs the SBA and returns the result
  Then the task status becomes "completed"
  And the job result contains "sba_result"
  And the job result has a user-facing reply
