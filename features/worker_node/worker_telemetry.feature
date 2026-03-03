@suite_worker_node
Feature: Worker Telemetry API

  As an operator
  I want to query node info and stats from the Worker API
  So that I can pull operational telemetry for debugging and operations

Background:
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
