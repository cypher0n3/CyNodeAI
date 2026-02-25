@suite_agents
Feature: SBA apply_unified_diff path validation

  As a sandbox
  I want apply_unified_diff to reject diffs that write outside /workspace
  So that path escape is prevented

  @req_sbagnt_0001
  @spec_cynai_sbagnt_enforcement
  Scenario: apply_unified_diff tool rejects diff that escapes workspace
    Given I have a job with one apply_unified_diff step that escapes workspace
    When I run the SBA runner
    Then the result status is "failure"
    And the result failure message contains "path escapes workspace"
