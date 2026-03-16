@suite_orchestrator
Feature: API Egress call endpoint

  As a workflow runner
  I want to call external providers via POST /v1/call
  So that tasks can trigger allowed provider operations with audit

## Background

  Given the API egress stub is configured with bearer token "egress-bearer" and allowlist "openai,github"

@req_apiegr_0110
@req_apiegr_0119
@spec_cynai_apiegr_call
Scenario: Allowed provider returns 501 Not Implemented
  When I call POST "/v1/call" with bearer "egress-bearer" and body provider "openai" operation "chat" task_id "t1"
  Then the response status is 501
  And the response JSON has "title" equal to "Not Implemented"

@req_apiegr_0110
@spec_cynai_apiegr_call
Scenario: Disallowed provider returns 403
  When I call POST "/v1/call" with bearer "egress-bearer" and body provider "unknown" operation "op" task_id "t2"
  Then the response status is 403
  And the response JSON has "detail" containing "not allowed"

@req_apiegr_0110
@spec_cynai_apiegr_call
Scenario: Missing bearer when required returns 401
  Given the API egress stub is configured with bearer token "required"
  When I call POST "/v1/call" without bearer with body provider "openai" operation "op" task_id "t3"
  Then the response status is 401
