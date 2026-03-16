@suite_worker_node
Feature: Worker Telemetry API

  As an operator
  I want to query node info and stats from the Worker API
  So that I can pull operational telemetry for debugging and operations

## Background

  Given a Worker API is running with bearer token "test-bearer-token"

@req_worker_0200
@req_worker_0230
@req_worker_0231
@spec_cynai_worker_telemetrysurface_v1
Scenario: Get node info with bearer returns build and platform
  When I call GET "/v1/worker/telemetry/node:info" with bearer token "test-bearer-token"
  Then the response status is 200
  And the response JSON has "version" equal to 1
  And the response JSON has "node_slug"

@req_worker_0232
@spec_cynai_worker_telemetrysurface_v1
Scenario: Get node stats with bearer returns snapshot
  When I call GET "/v1/worker/telemetry/node:stats" with bearer token "test-bearer-token"
  Then the response status is 200
  And the response JSON has "version" equal to 1
  And the response JSON has "captured_at"

@req_worker_0200
@req_worker_0201
@spec_cynai_worker_telemetrysurface_v1
Scenario: Telemetry without bearer returns 401
  When I call GET "/v1/worker/telemetry/node:info" without authorization
  Then the response status is 401

@req_worker_0240
@req_worker_0241
@spec_cynai_worker_telemetrysurface_v1
Scenario: Get containers list with bearer returns array
  When I call GET "/v1/worker/telemetry/containers" with bearer token "test-bearer-token"
  Then the response status is 200
  And the response JSON has "containers"

@req_worker_0242
@req_worker_0243
@spec_cynai_worker_telemetrysurface_v1
Scenario: Get logs with bearer returns entries
  When I call GET "/v1/worker/telemetry/logs?source_kind=service&source_name=node_manager" with bearer token "test-bearer-token"
  Then the response status is 200
  And the response JSON has "events"

@req_worker_0240
@req_worker_0241
@spec_cynai_worker_telemetrysurface_v1
Scenario: Get containers list returns recorded container when store has data
  Given a sandbox container is recorded for task "task-bdd-1" job "job-bdd-1"
  When I call GET "/v1/worker/telemetry/containers" with bearer token "test-bearer-token"
  Then the response status is 200
  And the response contains a container with task_id "task-bdd-1" and job_id "job-bdd-1"

@req_worker_0240
@req_worker_0241
@req_worker_0242
@req_worker_0243
@spec_cynai_worker_telemetrysurface_v1
Scenario: Get logs returns recorded service log when store has data
  Given a service log event is recorded for source "worker_api" with message "BDD test log line"
  When I call GET "/v1/worker/telemetry/logs?source_kind=service&source_name=worker_api" with bearer token "test-bearer-token"
  Then the response status is 200
  And the response contains a log event with message "BDD test log line"
