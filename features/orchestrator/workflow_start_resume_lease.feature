@suite_orchestrator
Feature: Workflow Start Resume and Lease

  As a workflow runner (e.g. LangGraph process)
  I want to start, resume, checkpoint, and release task workflows
  So that at most one active workflow runs per task and state is durable

Background:

  Given a running PostgreSQL database
  And the orchestrator API is running
  And an admin user exists with handle "admin"
  And I am logged in as "admin"

@req_orches_0144
@req_orches_0145
@spec_cynai_orches_workflowstartresumeapi
Scenario: Start workflow for task returns run id
  When I create a task with prompt "workflow test" and start workflow for task with holder "runner-1"
  Then workflow start response status is 200
  And workflow start response includes run_id

@req_orches_0145
@spec_cynai_orches_workflowstartresumeapi
Scenario: Duplicate start with different holder returns 409
  When I create a task with prompt "lease test" and start workflow for task with holder "runner-1" and start workflow for task with holder "runner-2"
  Then workflow start response status is 409

@req_orches_0145
@spec_cynai_orches_workflowstartresumeapi
Scenario: Same holder start again returns 200 already_running
  When I create a task with prompt "idempotent start" and start workflow for task with holder "runner-1" and start workflow for task again with holder "runner-1"
  Then workflow start response status is 200
  And workflow start response has status "already_running"

@req_orches_0144
@spec_cynai_orches_workflowstartresumeapi
Scenario: Resume workflow returns checkpoint when saved
  When I create a task with prompt "resume test" and start workflow for task with holder "runner-1" and save checkpoint for task with last_node_id "plan_steps" and resume workflow for task
  Then workflow resume response status is 200
  And workflow resume response includes last_node_id "plan_steps"

@req_orches_0146
@spec_cynai_orches_taskworkflowleaselifecycle
Scenario: Release lease then another holder can start
  When I create a task with prompt "release test" and start workflow for task with holder "runner-1" and store the lease_id from workflow start response and release workflow for task with stored lease_id and start workflow for task with holder "runner-2"
  Then workflow start response status is 200
