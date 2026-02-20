# Post-Phase 1: What is Left and Full Inference E2E

- [What is left](#what-is-left)
- [Full inference-in-sandbox E2E](#full-inference-in-sandbox-e2e)
- [References](#references)

## What is Left

Per [post_phase1_mvp_plan.md](post_phase1_mvp_plan.md), after the use_inference API/CLI wiring:

- **Phase 1 gaps (optional):** config_version ULID in orchestrator; Worker API `GET /readyz`; clarify orchestrator fail-fast scenario scope.
- **Feature/BDD (optional):** 413/truncation worker_node scenarios; fail-fast wording; optional dedicated e2e feature file.
- **Full inference E2E:** Script-driven E2E (`just e2e` / `setup-dev.sh full-demo`) did not yet run a task with `use_inference` that executes inside the sandbox (proxy + model).
  It only ran host-side inference smoke (model pull + `ollama run` inside the Ollama container).

## Full Inference-in-Sandbox E2E

To run a full test that spins up Ollama, loads a model, and executes a task that uses inference from inside the sandbox:

1. **Node must be inference-ready:** Worker API needs `OLLAMA_UPSTREAM_URL` (e.g. `http://host.containers.internal:11434`) and `INFERENCE_PROXY_IMAGE` (image that runs the inference-proxy binary).
   Node-manager passes through env to worker-api; the script must export these before starting the node.
2. **Ollama reachable from the proxy:** The inference proxy runs in a pod and forwards to the host.
   Ollama must publish port 11434 to the host (e.g. `-p 11434:11434` when starting the Ollama container).
3. **Inference-proxy image:** Build a container image from `worker_node/cmd/inference-proxy` and set `INFERENCE_PROXY_IMAGE` to that image.
4. **E2E step:** After the existing happy-path task, create a second task with `use_inference: true` and a command that uses the proxy (e.g. `sh -c 'echo $OLLAMA_BASE_URL'` or a small curl to the proxy); poll for completion and assert the result.

Implementation adds: Dockerfile for inference-proxy, build of that image in full-demo, node-manager exposing Ollama port 11434, env in start_node for inference, and an E2E test step for the use_inference task.

## References

- [post_phase1_mvp_plan.md](post_phase1_mvp_plan.md) Section 2.1, 3.3, 8.
- [single_node_e2e_testing_plan.md](single_node_e2e_testing_plan.md).
- [post_phase1_inference_proxy_report.md](post_phase1_inference_proxy_report.md).
