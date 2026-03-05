@suite_agents
Feature: SBA Inference and User-Facing Reply

  As a user or test harness
  I want SBA tasks that use inference to complete with a user-facing reply when inference is available
  So that inference path and SBA agent behavior are validated and failures are not masked as skips

  Per REQ-SBAGNT-0109 and remediation plan: when inference is required, task failure is product failure.
  E2E tests must fail (not skip) when SBA inference task reaches status "failed".

@req_sbagnt_0001
@req_sbagnt_0109
@spec_cynai_sbagnt_workerproxies
@spec_cynai_sbagnt_jobinferencemodel
Scenario: SBA task with inference completes with sba_result and user-facing reply
  Given inference path is available and SBA runner is configured
  When I create a SBA task with inference and prompt "Reply back with the current time"
  And the task runs to terminal status
  Then the task status is "completed"
  And the job result contains "sba_result"
  And the job result has a user-facing reply (non-empty stdout or sba_result steps/reply)

@req_sbagnt_0109
@spec_cynai_sbagnt_jobinferencemodel
Scenario: SBA task with inference that fails is treated as product failure
  Given inference is required for the SBA task (not gated by skip-inference flag)
  When I create a SBA task with inference and the task reaches status "failed"
  Then the outcome is treated as product failure
  And the failure is not treated as environmental skip
  And the test or harness fails (does not skip)
