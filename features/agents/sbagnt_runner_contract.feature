@suite_agents
Feature: SBA runner contract (REQ-SBAGNT-0001)

  As an integrator
  I want job specs that match the cynode-sba contract to validate cleanly
  So that only well-formed jobs are eligible for the sandbox runner

@req_sbagnt_0001 @spec_cynai_sbagnt_doc_cynodesba
Scenario: Supported protocol job spec validates for cynode-sba
  Given I have a SBA job spec with protocol_version "1.0" and required fields
  When I validate the SBA job spec
  Then the SBA job spec validation succeeds
