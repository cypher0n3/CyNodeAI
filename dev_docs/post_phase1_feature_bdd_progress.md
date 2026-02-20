# Post-Phase 1 Feature Files and BDD Progress

- [1 Completed](#1-completed)
- [2 Not done this session](#2-not-done-this-session)

## 1 Completed

Date: 2026-02-20.
Plan ref: [post_phase1_mvp_plan.md](post_phase1_mvp_plan.md) Section 4 (Feature files), Section 6 (BDD), Section 8 (Implementation order item 4).

The following was implemented so that `just ci` passes.

### 1.1 Orchestrator Fail-Fast Scenario (Clarified)

   In `features/orchestrator/orchestrator_startup.feature`, added a comment that "inference path" means at least one dispatchable node; when none are available, readyz returns 503.
   BDD runs the scenario with a mock where no nodes are registered.

### 1.2 E2E Inference-in-Sandbox Scenario

   In `features/e2e/single_node_happy_path.feature`, added scenario "Single-node task execution with inference in sandbox" with tag `@inference_in_sandbox`.
   Steps assume an inference-capable node (proxy + model).
   No BDD runner currently loads `features/e2e/`; scenario is for traceability and future E2E BDD.

### 1.3 Worker Node: OLLAMA_BASE_URL in Direct Mode

   In `worker_node/cmd/worker-api/executor/executor.go`, when `runDirect` is used with `UseInference` true and `ollamaUpstreamURL` set, the sandbox env now includes `OLLAMA_BASE_URL=http://localhost:11434` so BDD can assert without a real pod.

### 1.4 Worker Node: BDD Scenario and Steps

   New step in `worker_node/_bdd/steps.go`: "I submit a sandbox job with use_inference that runs command \"...\"".
   New scenario in `features/worker_node/worker_node_sandbox_execution.feature`: "Sandbox receives OLLAMA_BASE_URL when use_inference is true" (tag `@inference_in_sandbox`).
   BDD executor created with `OLLAMA_UPSTREAM_URL` default `http://localhost:11434` so the inference scenario passes in direct runtime.

### 1.5 CI and Coverage

   `just ci` passes.
   Cynork coverage was below 90%; added tests in `cynork/cmd` and `cynork/internal/config` (including config `userHomeDir` injection for `ConfigDir`/`ConfigPath` error paths) so all packages meet the 90% threshold.

## 2 Not Done This Session

- Optional worker_node scenarios (413, truncation) per plan.
- E2E script extension (`just e2e` / setup-dev.sh) for inference-in-sandbox; plan item 5.
- No new e2e _bdd runner; `features/e2e/` is still not executed by `just test-bdd`.
