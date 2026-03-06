@suite_worker_node
Feature: Worker Node Inference Proxy

  As a worker node
  I want a local inference proxy that binds loopback and exposes healthz
  So that pod inference requests are routed without exposing credentials (Phase 1)

@req_worker_0114
@req_worker_0115
@spec_cynai_worker_nodelocalinference
@spec_cynai_stands_inferenceollamaandproxy
Scenario: Inference proxy rejects request body exceeding size limit
  Given the inference proxy is configured with an upstream
  When I send a request to the inference proxy with body size exceeding 10 MiB
  Then the inference proxy responds with status 413
