@suite_orchestrator
Feature: Orchestrator Job Timeout Tracking

  As an operator or system
  I want the orchestrator to track job effective deadlines and mark overdue jobs as failed (timeout)
  So that jobs are not left in an ambiguous state when a node does not report back

Background:

  Given a running PostgreSQL database
  And the orchestrator API is running
  And an admin user exists with handle "admin"
  And I am logged in as "admin"
  And a registered node "test-node-01" is active
  And the node "test-node-01" has worker_api_target_url and bearer token in config

@req_orches_0173
@spec_cynai_orches_rule_jobtimeouttracking
Scenario: Job that exceeds effective deadline without completion is marked failed (timeout)
  Given a job is dispatched with sandbox timeout seconds 2
  And the node does not report completion before the effective deadline
  When the orchestrator runs its job timeout check
  Then the job is marked as failed with reason timeout
  And the task or job state is observable as timeout for retry or audit

@req_orches_0174
@spec_cynai_orches_rule_jobtimeouttracking
Scenario: Job within extended deadline is not marked timed out
  Given a job is dispatched with sandbox timeout seconds 2
  And the orchestrator is informed of a timeout extension with a new effective deadline 10 seconds from now
  And the original effective deadline has passed
  When the orchestrator runs its job timeout check
  Then the job is not marked as failed (timeout)
  And the job remains in progress until the extended deadline or completion
