@suite_worker_node
Feature: Worker Internal Agent-to-Orchestrator Proxy

  As a worker node
  I want internal proxy endpoints to be unreachable from the public API mux
  So that agent-to-orchestrator forwarding is only via identity-bound transport

@req_worker_0162
@req_worker_0163
@spec_cynai_worker_managedagentproxybidirectional
Scenario: Internal proxy routes are not exposed on public worker API
  Given the worker API is running
  When I POST to the worker API path "/v1/worker/internal/orchestrator/mcp:call" with body "{}"
  Then the worker API returns status 404
