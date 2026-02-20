# Single-Node End-to-End Testing Plan

- [Goals](#goals)
- [Preconditions (From Specs)](#preconditions-from-specs)
- [Current vs Target Behavior](#current-vs-target-behavior)
- [Rationale for Host-Side Inference Smoke](#rationale-for-host-side-inference-smoke)
- [E2E Flow (Updated)](#e2e-flow-updated)
- [Implementation Notes](#implementation-notes)
- [Optional: Feature File Scenario](#optional-feature-file-scenario)
- [References](#references)

## Goals

This document describes the single-node (orchestrator and worker-node on the same host) E2E testing plan.
It includes loading an inference model into Ollama and executing a basic inference step.

1. **Single-node happy path**: Login, node registration, config fetch/ack, create task, dispatch, execute job, task completed (existing).
2. **Inference readiness**: Ensure Ollama is running with at least one model loaded and that a basic inference run succeeds.
3. **Traceability**: E2E flow aligns with `features/e2e/single_node_happy_path.feature` and tech spec preconditions (e.g. `docs/tech_specs/node.md`, Phase 1 single-node fail-fast when inference is unavailable).

## Preconditions (From Specs)

- At least one inference-capable path must be available (node-local Ollama or external provider key).
- In the single-node case, startup must fail fast if neither is available.
- Node manager starts Ollama after Worker API; no model is pulled by default.

## Current vs Target Behavior

- **Ollama**: Current: started by node-manager; no model loaded.
  Target: same; E2E pulls a small model and verifies inference.
- **Task in E2E**: Current: single task `echo Hello from sandbox` (no inference).
  Target: same task (sandbox job) plus a separate inference smoke step.
- **Inference execution**: Current: not exercised.
  Target: basic inference run via Ollama (host-side smoke after model load).

## Rationale for Host-Side Inference Smoke

Phase 1 sandbox runs with `--network=none`, so the job container cannot reach Ollama.
The intended design (inference proxy sidecar and sandbox calling `http://localhost:11434`) is not yet implemented.
Therefore the plan is:

1. **Now**: After the node (and thus Ollama) is up, the E2E script waits for the Ollama container, pulls a small model, and runs a basic generate (e.g. `ollama run <model> "<prompt>"`) inside the Ollama container via the container runtime.
   This proves "model loaded and inference works" on the node.
2. **Later**: When inference proxy and sandbox networking exist, add a scenario where a task's job command performs inference (e.g. calling the proxy at `localhost:11434` from inside the sandbox).

## E2E Flow (Updated)

1. Start PostgreSQL, build binaries, start control-plane, user-gateway, node-manager (node-manager starts Worker API and Ollama).
2. **Inference readiness** (new):
   - Wait for Ollama container `cynodeai-ollama` to be running.
   - Pull a small model (e.g. `tinyllama`; overridable via `OLLAMA_E2E_MODEL`).
   - Run a short inference (e.g. `ollama run <model> "Reply with one word: hello"`) and assert the response is non-empty or contains expected content.
3. Login as admin (user-gateway).
4. Create task with prompt `echo Hello from sandbox` (unchanged).
5. Node registration (control-plane) and capability report (unchanged).
6. Poll task/result until job completes; assert stdout and task status (unchanged).
7. Remaining steps: token refresh, logout (unchanged).

## Implementation Notes

- **Script**: `scripts/setup-dev.sh`; `full-demo` and `test-e2e` both run the E2E test.
  Inference steps run only when the node (and thus Ollama) is present; for `test-e2e` without a prior `full-demo`, inference steps are skipped if the Ollama container is not found.
- **Container name**: `cynodeai-ollama` (from `worker_node/cmd/node-manager/main.go`).
- **Runtime**: Same as rest of E2E (`detect_runtime`: podman or docker).
- **Model**: Default `tinyllama`; set `OLLAMA_E2E_MODEL` to use another model (e.g. `qwen2:0.5b`).
  First run may be slow due to pull.
- **Feature file**: `features/e2e/single_node_happy_path.feature` already states the precondition that at least one inference-capable path is available; the new script steps implement that precondition for the script-driven E2E.
  No change to the scenario text is required unless we add a dedicated "inference model loaded" scenario (optional).

## Optional: Feature File Scenario

A separate scenario can be added later, e.g. "Single-node with inference model loaded", with steps such as: Ollama container is running, model is loaded, and a basic inference request succeeds.
The script-driven E2E already enforces this; the feature would make it explicit for BDD when step definitions support it.

## References

- [`docs/tech_specs/node.md`](../docs/tech_specs/node.md) (Node Local Inference, Phase 1 single-node fail-fast).
- [`docs/tech_specs/_main.md`](../docs/tech_specs/_main.md) (Phase 1 Single Node Happy Path).
- [`features/e2e/single_node_happy_path.feature`](../features/e2e/single_node_happy_path.feature).
- [`scripts/setup-dev.sh`](../scripts/setup-dev.sh) (full-demo, test-e2e).
- [`justfile`](../justfile) (`just e2e` -> `./scripts/setup-dev.sh full-demo`).
