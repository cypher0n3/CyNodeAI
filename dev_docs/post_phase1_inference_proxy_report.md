# Post-Phase 1: Inference Proxy and Pod Path Implementation Report

- [1 Summary](#1-summary)
- [2 Delivered](#2-delivered)
- [3 Usage](#3-usage)
- [4 References](#4-references)

## 1 Summary

Implementation of **inference in sandbox** per `dev_docs/post_phase1_mvp_plan.md` Section 3 and implementation order step 3: inference proxy sidecar and Worker API path that runs jobs in a Podman pod with sandbox + proxy and sets `OLLAMA_BASE_URL=http://localhost:11434` in the sandbox.

`just ci` passes (2026-02-20).
All Go packages meet the 90% coverage threshold.

## 2 Delivered

| Item | Location / behavior |
|------|---------------------|
| **Contract** | `go_shared_libs/contracts/workerapi`: `SandboxSpec.UseInference` (bool, JSON `use_inference`). Orchestrator `ParseSandboxSpec` parses it. |
| **Inference proxy** | `worker_node/cmd/inference-proxy`: HTTP reverse proxy to Ollama. Listens on `:11434`; forwards to `OLLAMA_UPSTREAM_URL` (default `http://host.containers.internal:11434`). Enforces 10 MiB request body and 120s per-request timeout (node.md). No credentials. |
| **Proxy library** | `worker_node/internal/inferenceproxy`: `NewProxy(upstream *url.URL)`, constants `MaxRequestBodyBytes`, `PerRequestTimeout`. Unit tests (forward, 413 on oversized body, 500 on read error). |
| **Executor pod path** | `worker_node/cmd/worker-api/executor`: When `req.Sandbox.UseInference` and `ollamaUpstreamURL` and `inferenceProxyImage` are set and runtime is `podman`, runs job in a pod: `pod create`, run proxy container (optional `inferenceProxyCommand` for tests), run sandbox with `OLLAMA_BASE_URL=http://localhost:11434`. Helpers: `buildProxyRunArgs`, `buildSandboxRunArgsForPod`, `setPodInferenceError`. |
| **Worker API** | `OLLAMA_UPSTREAM_URL`, `INFERENCE_PROXY_IMAGE` (optional). When both set, jobs with `use_inference: true` use the pod path. Optional proxy command not exposed via env in main (used in tests). |
| **Tests** | Executor: pod create fail, proxy start fail, full pod run (when podman + alpine available); `buildProxyRunArgs` / `buildSandboxRunArgsForPod`; `sanitizePodName`. Inference-proxy main: error paths (invalid URL, listen fail, closed listener), nil listener start/shutdown. |

## 3 Usage

- **Node**: Set `OLLAMA_UPSTREAM_URL` (e.g. `http://host.containers.internal:11434`) and `INFERENCE_PROXY_IMAGE` to a image that runs the inference-proxy binary (build from `worker_node/cmd/inference-proxy` and push to a registry, or use a pre-built image).
  Then jobs with `use_inference: true` in the sandbox payload run in a pod with the proxy sidecar.
- **Orchestrator**: Include `"use_inference": true` in the job payload when the task needs inference in the sandbox.
  Dispatcher passes it through to the Worker API.

## 4 References

- `docs/tech_specs/node.md` - Option A (inference proxy sidecar).
- `docs/tech_specs/sandbox_container.md` - Node-local inference access.
- `dev_docs/post_phase1_mvp_plan.md` - Implementation order and E2E/feature file next steps.

Report generated 2026-02-20.
