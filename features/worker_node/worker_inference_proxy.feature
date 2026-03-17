@suite_worker_node
Feature: Worker Node Inference Proxy

  As a worker node
  I want a local inference proxy that binds loopback and exposes healthz
  So that pod inference requests are routed without exposing credentials

@req_worker_0114
@req_worker_0115
@spec_cynai_worker_nodelocalinference
@spec_cynai_stands_inferenceollamaandproxy
Scenario: Inference proxy rejects request body exceeding size limit
  Given the inference proxy is configured with an upstream
  When I send a request to the inference proxy with body size exceeding 10 MiB
  Then the inference proxy responds with status 413

@req_worker_0270
@spec_cynai_worker_unifiedudspath
Scenario: Inference proxy listens on Unix domain socket when INFERENCE_PROXY_SOCKET is set
  Given the inference proxy is configured with an upstream
  And the inference proxy is started with INFERENCE_PROXY_SOCKET set to a temp path
  Then the inference proxy socket file exists at that path
  And a healthz request over the Unix domain socket returns 200

@req_sandbx_0131
@spec_cynai_worker_unifiedudspath
Scenario: SBA pod container receives INFERENCE_PROXY_URL not TCP OLLAMA_BASE_URL
  Given the executor is configured with a proxy image and an upstream URL
  When the executor builds SBA pod run args for agent_inference mode
  Then the SBA container args contain INFERENCE_PROXY_URL with an http+unix scheme
  And the SBA container args do not contain OLLAMA_BASE_URL with a TCP localhost address

@req_sandbx_0131
@req_worker_0174
@spec_cynai_worker_unifiedudspath
Scenario: SBA non-pod container receives INFERENCE_PROXY_URL via socket mount when SBA_INFERENCE_PROXY_SOCKET is set
  Given the executor is configured with an upstream URL and no proxy image
  And SBA_INFERENCE_PROXY_SOCKET is set to a temp socket path
  When the executor builds SBA direct run args for agent_inference mode
  Then the SBA direct container args contain INFERENCE_PROXY_URL with an http+unix scheme
  And the SBA direct container args include a volume mount for the inference proxy socket dir
  And the SBA direct container args keep --network=none

@req_sandbx_0131
@req_worker_0174
@spec_cynai_worker_unifiedudspath
Scenario: SBA non-pod container omits INFERENCE_PROXY_URL when SBA_INFERENCE_PROXY_SOCKET is not set
  Given the executor is configured with an upstream URL and no proxy image
  And SBA_INFERENCE_PROXY_SOCKET is not set
  When the executor builds SBA direct run args for agent_inference mode
  Then the SBA direct container args do not contain INFERENCE_PROXY_URL
