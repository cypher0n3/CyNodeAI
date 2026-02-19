Feature: Orchestrator Startup

  As an operator
  I want the orchestrator to fail fast when no inference path is available
  So that I know the system is not ready and can fix configuration before accepting work

  @req_bootst_0002
  @spec_cynai_bootst_bootstrapsource
  Scenario: Orchestrator fails fast when no inference path is available
    Given no local inference (Ollama) is running
    And no external provider key is configured
    When the orchestrator starts
    Then the orchestrator does not enter ready state
    And the orchestrator reports that no inference path is available
