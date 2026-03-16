@suite_worker_node
Feature: Node Manager Telemetry Ownership

  As a node operator
  I want the node manager to own the telemetry DB lifecycle (node_boot, retention, vacuum, shutdown)
  So that shutdown outcomes are recorded and worker-api does not duplicate node_boot when started by node manager

  Per worker_telemetry_api.md and worker_node.md: node-manager records node_boot, runs retention/vacuum,
  and records a service log event on shutdown (source_name=node_manager).
    Worker-api skips node_boot
  when NODE_SKIP_NODE_BOOT_RECORD is set (e.g. when started by node-manager).

@req_worker_0258
@spec_cynai_worker_telemetry_lifecycle
Scenario: Node manager shutdown event is visible in telemetry logs
  Given a Worker API is running with bearer token "test-bearer-token"
  And a service log event is recorded for source "node_manager" with message "node manager shutdown"
  When I call GET "/v1/worker/telemetry/logs?source_kind=service&source_name=node_manager" with bearer token "test-bearer-token"
  Then the response status is 200
  And the response contains a log event with message "node manager shutdown"
