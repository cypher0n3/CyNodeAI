@suite_agents
Feature: SBA Job Lifecycle

  As an orchestrator
  I want the SBA to report in-progress and completion via callback when status URL is set
  So that job state can be updated without relying only on node-mediated delivery

@req_sbagnt_0001
@req_sbagnt_0110
@req_worker_0149
@req_worker_0157
@spec_cynai_sbagnt_joblifecycle
@spec_cynai_sbagnt_resultandartifactdelivery
@spec_cynai_worker_joblifecycleresultpersistence
Scenario: When status URL is set, SBA sends in_progress then completion
  Given I have a lifecycle callback server
  And I have a valid job file with one run_command step "echo ok"
  When I run the SBA runner with lifecycle callback
  Then the lifecycle server received "in_progress"
  And the lifecycle server received "completed"
  And the result status is "success"
