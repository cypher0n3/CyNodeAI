@suite_agents
Feature: SBA Failure Codes

  As an orchestrator
  I want the SBA result to use canonical failure codes when applicable
  So that failures can be categorized (constraint_violation, ext_net_required, etc.)

  @req_sbagnt_0001
  @spec_cynai_sbagnt_resultcontract
  Scenario: Result shape with constraint_violation when output exceeds max_output_bytes
    Given I have a job with max_output_bytes 5 and one run_command that outputs 10 bytes
    When I run the SBA runner
    Then the result status is "failure"
    And the result failure_code is "constraint_violation"
