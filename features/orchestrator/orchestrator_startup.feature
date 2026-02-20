@suite_orchestrator
Feature: Orchestrator Startup

  As an operator
  I want the orchestrator to fail fast when no inference path is available
  So that I know the system is not ready and can fix configuration before accepting work

  # "Inference path" here means at least one dispatchable (registered, config-acked) node.
  # When no node is available, readyz returns 503 and a message indicating no inference path.
  # BDD runs this scenario with a mock where no nodes are registered.

  @req_bootst_0002
  @spec_cynai_bootst_bootstrapsource
  Scenario: Orchestrator fails fast when no inference path is available
    Given no local inference (Ollama) is running
    And no external provider key is configured
    When the orchestrator starts
    Then the orchestrator does not enter ready state
    And the orchestrator reports that no inference path is available
