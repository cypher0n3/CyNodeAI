@suite_e2e
Feature: Single Node Happy Path

  As a CyNodeAI user running orchestrator and worker on the same host (single-node)
  I want to create tasks that execute on that local worker node
  So that I can automate sandboxed execution of commands without a separate machine

# Precondition: at least one inference-capable path must be available.
# Inference may be node-local (Ollama or similar) or external via a configured provider key.
# In the single-node case, startup should fail fast if neither is available.
#
# Script-driven E2E (just e2e / setup-dev.sh full-demo): after the node starts, the script
# loads a small model into Ollama and runs a basic inference smoke to verify inference works.
# See dev_docs/single_node_e2e_testing_plan.md.

  Background:
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
    When I login as "admin" with password "admin123"
    And a node with slug "test-node-01" registers with the orchestrator using a valid PSK
    And the node requests its configuration
    And the node applies the configuration and sends a config acknowledgement with status "applied"
    And I create a task with command "echo hello"
    And the orchestrator dispatches a job to the node
    And the node executes the sandbox job
    Then the job result contains stdout "hello"
    And the task status becomes "completed"
