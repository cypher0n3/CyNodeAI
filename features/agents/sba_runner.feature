@suite_agents
Feature: SBA Runner Execution

  As a worker node
  I want the SBA runner to read a job file, execute steps, and write a result file
  So that SBA jobs can be run in sandboxes and results collected


  @req_sbagnt_0001
  @req_sbagnt_0103
  @spec_cynai_sbagnt_resultcontract
  Scenario: Runner executes run_command step and writes success result
    Given I have a valid job file with one run_command step "echo ok"
    When I run the SBA runner
    Then the result status is "success"
    And the result file contains "protocol_version"
    And the result file contains "job_id"
    And the result file contains "status"
    And the result file contains "steps"

  @req_sbagnt_0102
  @spec_cynai_sbagnt_enforcement
  Scenario: Runner produces result with step output
    Given I have a valid job file with one run_command step "printf hello"
    When I run the SBA runner
    Then the result status is "success"
    And the result file contains "steps"

  @req_sbagnt_0001
  @spec_cynai_sbagnt_resultcontract
  Scenario: Runner reads job from stdin and writes result to stdout
    Given I have a valid job JSON for stdin with one run_command step "echo ok"
    When I run the SBA runner with stdin and stdout
    Then the result status is "success"
    And the result output contains "protocol_version"
    And the result output contains "status"
