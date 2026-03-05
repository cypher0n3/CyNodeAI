@suite_worker_node
Feature: Worker Node SBA Job Spec and Result Contract

  As a worker node or orchestrator
  I want SBA job specifications to be validated (protocol version, schema) and result contract to be well-defined
  So that cynode-sba jobs are executed only with valid specs and results are auditable

@req_sbagnt_0100
@req_sbagnt_0101
@spec_cynai_sbagnt_protocolversioning
@spec_cynai_sbagnt_schemavalidation
Scenario: Valid SBA job spec with supported protocol version passes validation
  Given I have a SBA job spec with protocol_version "1.0" and required fields
  When I validate the SBA job spec
  Then the SBA job spec validation succeeds

@req_sbagnt_0100
@req_sbagnt_0101
@spec_cynai_sbagnt_protocolversioning
@spec_cynai_sbagnt_schemavalidation
Scenario: SBA job spec with unknown major protocol version fails validation
  Given I have a SBA job spec with protocol_version "99.0" and required fields
  When I validate the SBA job spec
  Then the SBA job spec validation fails
  And the validation error is for field "protocol_version"

@req_sbagnt_0101
@spec_cynai_sbagnt_schemavalidation
Scenario: SBA job spec with unknown field fails validation
  Given I have a SBA job spec with an unknown field
  When I validate the SBA job spec
  Then the SBA job spec validation fails

@req_sbagnt_0101
@spec_cynai_sbagnt_schemavalidation
Scenario: SBA job spec with missing required job_id fails validation
  Given I have a SBA job spec with protocol_version "1.0" and empty job_id
  When I validate the SBA job spec
  Then the SBA job spec validation fails
  And the validation error is for field "job_id"

@req_sbagnt_0103
@spec_cynai_sbagnt_resultcontract
Scenario: SBA result contract has required shape for orchestrator storage
  Given I have a SBA result with status "success" and job_id "j1"
  When I marshal the SBA result to JSON
  Then the JSON contains "protocol_version"
  And the JSON contains "job_id"
  And the JSON contains "status"
  And the JSON contains "steps"
  And the JSON contains "artifacts"

@req_sbagnt_0103
@spec_cynai_sbagnt_resultcontract
@spec_cynai_worker_nodemediatedsbaresultsync
Scenario: SBA result contract from task result is validated only when task is completed
  Given I have a completed task that used the SBA runner
  When I get the task result and extract the first job result
  Then the task status is "completed"
  And the job result contains "sba_result"
  And the sba_result contains "protocol_version"
  And the sba_result contains "job_id"
  And the sba_result contains "status"
  And the sba_result contains "steps"
  And the sba_result contains "artifacts"
