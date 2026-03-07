# Startup Sequence Remediation Plan (2026-02-28)

## Purpose

This document drafts a remediation plan to align the current implementation with the prescribed startup sequence and requirements.
Remediation MUST follow the project's **Red-Green-Refactor** workflow and use the **Python E2E test suite** for user- and API-facing behavior; see [Workflow: Red-Green-Refactor](#workflow-red-green-refactor) and [Remediation Tasks](#remediation-tasks-checklist).
The specs and requirements have been updated so that:

1. Orchestrator services (control-plane, user-gateway, etc.) start **independently** of any OLLAMA container.
2. The Worker API starts on the node and the node contacts the orchestrator **first** with its capabilities bundle.
3. The orchestrator acknowledges and **instructs** the worker (via node configuration) to start the OLLAMA container with the correct config (e.g. ROCm for AMD, CUDA for Nvidia).

References (requirements): [REQ-BOOTST-0105](../requirements/bootst.md#req-bootst-0105), [REQ-ORCHES-0149](../requirements/orches.md#req-orches-0149), [REQ-WORKER-0253](../requirements/worker.md#req-worker-0253), [REQ-WORKER-0254](../requirements/worker.md#req-worker-0254), [REQ-WORKER-0255](../requirements/worker.md#req-worker-0255), [REQ-WORKER-0256](../requirements/worker.md#req-worker-0256).

References (tech specs): [Node Startup Procedure](../tech_specs/worker_node.md#spec-cynai-worker-nodestartupprocedure), [Orchestrator Independent Startup](../tech_specs/orchestrator_bootstrap.md#spec-cynai-bootst-orchestratorindependentstartup), [Configuration Delivery](../tech_specs/worker_node.md#spec-cynai-worker-configurationdelivery), [Existing Inference Service on Host](../tech_specs/worker_node.md#spec-cynai-worker-existinginferenceservice).  
Payloads: [worker_node_payloads.md](../tech_specs/worker_node_payloads.md) (`inference_backend`, `inference.existing_service`).

## Prescribed Startup Sequence (Target)

Target order (aligned with [Node Startup Procedure](../tech_specs/worker_node.md#spec-cynai-worker-nodestartupprocedure) and [Orchestrator Independent Startup](../tech_specs/orchestrator_bootstrap.md#spec-cynai-bootst-orchestratorindependentstartup)):

1. **Orchestrator:** Control-plane, user-gateway, and other core services start **without** an OLLAMA container.
2. **Node:** Node Manager starts, loads YAML, runs startup checks, collects capabilities.
3. **Node:** Registers with orchestrator and sends capability report (capabilities bundle); when inference is supported, report includes `inference.existing_service` and `inference.running` per [worker_node_payloads.md](../tech_specs/worker_node_payloads.md) so the orchestrator can avoid instructing start when the node already has host inference ([REQ-WORKER-0256](../requirements/worker.md#req-worker-0256)).
4. **Orchestrator:** Acks registration and returns node configuration payload.
   When the node is inference-capable and inference is enabled and the node is **not** already using an existing host inference service (`inference.existing_service` not true), config includes `inference_backend` (enabled, image, variant e.g. ROCm/CUDA) derived from capability report ([REQ-ORCHES-0149](../requirements/orches.md#req-orches-0149), [Configuration Delivery](../tech_specs/worker_node.md#spec-cynai-worker-configurationdelivery)).
5. **Node:** Fetches config, then starts Worker API (so node is reachable for dispatch).
6. **Node (local inference):** Applies [Existing Inference Service on Host](../tech_specs/worker_node.md#spec-cynai-worker-existinginferenceservice) ([REQ-WORKER-0255](../requirements/worker.md#req-worker-0255)): if an inference service (OLLAMA or equivalent) is already running on the host and reachable, the node MUST use it and MUST NOT start another.
   Only when **no** existing service is detected **and** config instructs (`inference_backend.enabled` and image or variant present), the node starts the single OLLAMA container with the specified image and variant.
   Otherwise the node does not start an inference container.
7. **Node:** Sends config ack and continues capability reporting; when the node has become ready (Worker API and, when instructed, inference container or existing host inference), reports so the orchestrator can treat the node as an inference path ([REQ-WORKER-0254](../requirements/worker.md#req-worker-0254)).

## Current Implementation Gaps

The following gaps exist between the prescribed sequence and the current implementation.

### 1. Orchestrator Compose Includes OLLAMA

- **Location:** `orchestrator/docker-compose.yml`
- **Gap:** The full stack includes an `ollama` service; user-gateway and cynode-pma `depends_on: ollama`.
- **Effect:** Orchestrator does not start independently of OLLAMA; single-host dev assumes OLLAMA in the same compose.
- **Remediation:** Make OLLAMA optional in the orchestrator compose (e.g. profile or separate compose file for "with inference").
  Core stack (postgres, control-plane, user-gateway, cynode-pma) must start and remain operational when OLLAMA is not present.
  PMA and user-gateway may use external model routing or wait for a registered node that provides inference.
  Document that production/multi-node deployments do not run OLLAMA in the orchestrator stack.

### 2. Node Configuration Payload Lacks `inference_backend`

- **Location:** Orchestrator handlers that build and return `node_configuration_payload_v1`; `go_shared_libs` contracts.
- **Gap:** The node config payload does not yet include `inference_backend` (enabled, image, variant).
- **Effect:** Orchestrator cannot "instruct" the node to start OLLAMA with a specific image/variant (ROCM/CUDA).
- **Remediation:** Add `inference_backend` to the node configuration payload in shared contracts and orchestrator response.
  Orchestrator derives variant (and optionally image) from node capability report (`gpu.present`, `gpu.devices[].features` or platform) and policy.
  When the node reports `inference.existing_service` true in the capability report, the orchestrator MUST NOT instruct the node to start a container (node already has inference); see [Existing Inference Service on Host](../tech_specs/worker_node.md#spec-cynai-worker-existinginferenceservice) and [REQ-WORKER-0256](../requirements/worker.md#req-worker-0256).

### 3. Node Manager Starts OLLAMA Without Orchestrator Instruction

- **Location:** `worker_node/internal/nodeagent/nodemanager.go`, `worker_node/cmd/node-manager/main.go`
- **Gap:** Node starts OLLAMA via `StartOllama()` using only local env (`OLLAMA_IMAGE`); no check for orchestrator-provided `inference_backend` in config; no [Existing Inference Service on Host](../tech_specs/worker_node.md#spec-cynai-worker-existinginferenceservice) check ([REQ-WORKER-0255](../requirements/worker.md#req-worker-0255)); capability report may omit `inference.existing_service` and `inference.running` ([REQ-WORKER-0256](../requirements/worker.md#req-worker-0256)).
- **Effect:** Node may start OLLAMA even when orchestrator did not instruct it, or when an existing host inference service is already present; or with wrong image/variant (e.g. generic image instead of ROCm/CUDA).
- **Remediation:** In Node Manager, apply Existing Inference Service on Host first: if an inference service is already running on the host and reachable, use it and do not start another; report `inference.existing_service` (and `inference.running`) in the capability report.
  Start the inference container only when **no** existing service is detected **and** `nodeConfig.InferenceBackend != nil` and `InferenceBackend.Enabled` (or equivalent).
  Use `InferenceBackend.Image` and `InferenceBackend.Variant` from config; fall back to env or node YAML only when config omits them.
  Pass variant to the container (e.g. env or image selection) so ROCm vs CUDA is applied.

### 4. Startup Order: Worker API vs Registration

- **Location:** `worker_node/internal/nodeagent/nodemanager.go` (RunWithOptions order).
- **Current order:** Register -> fetch config -> start Worker API -> start Ollama -> config ack -> capability loop.
- **Spec:** Worker API must be started **before** starting OLLAMA; registration and capability report must occur **before** starting OLLAMA.
  Current order is already: register and fetch config first, then start Worker API, then start Ollama.
  The critical constraint (do not start Ollama until after config is received) is partially met.
  The missing part is that config must **instruct** (`inference_backend`) and node must obey that instruction.
  No change to order required if `inference_backend` is added and node only starts Ollama when instructed.

### 5. Dev / Single-Host Scripts Assume OLLAMA in Stack

- **Location:** `scripts/setup-dev.sh`, `scripts/setup_dev_impl.py`, E2E and smoke tests.
- **Gap:** Scripts and tests assume orchestrator compose brings up OLLAMA and that node may skip starting OLLAMA if container `cynodeai-ollama` already exists.
- **Remediation:** Keep dev convenience path (OLLAMA in compose) as an optional profile or documented single-host mode.
  Add a mode or profile where orchestrator stack runs without OLLAMA and node starts OLLAMA after registration (with `inference_backend` from config).
  E2E tests should be runnable in both modes; at least one path must exercise the prescribed sequence (orchestrator without OLLAMA, node starts OLLAMA when instructed).

## Workflow: Red-Green-Refactor

All remediation MUST follow the BDD/TDD cycle defined in [ai_files/ai_coding_instructions.md](../../ai_files/ai_coding_instructions.md): write the user story (Gherkin) first, then Red (add or adjust tests so they fail for the right reason), then Green (implement until tests pass), then Refactor.

- **BDD:** Create or update `.feature` files under `features/` for the prescribed startup behavior.
  Relevant existing features: [features/orchestrator/node_registration_and_config.feature](../../features/orchestrator/node_registration_and_config.feature), [features/worker_node/node_manager_config_startup.feature](../../features/worker_node/node_manager_config_startup.feature).
- **Red phase:** Add or extend tests so new scenarios fail before implementation.
  - **Go:** Unit tests and/or Godog BDD step definitions in orchestrator and worker_node.
  - **Python E2E:** Add or extend modules in `scripts/test_scripts/` for gateway, control-plane, or cynork-visible behavior.
    See [scripts/test_scripts/README.md](../../scripts/test_scripts/README.md) for layout, numbering, state, and how to add tests.
    Modules are named `e2e_NNN_descriptive_name.py` (e.g. `e2e_175_*` between 170 and 180); run order is alphabetical by module name.
    Use `run_e2e.py` (or `just e2e`), `config`, `helpers`, and `e2e_state`; support `--skip-ollama` / `E2E_SKIP_INFERENCE_SMOKE` where appropriate.
- **Green phase:** Implement the smallest change that makes the new tests pass.
- **Refactor:** Improve structure while keeping tests green.

Do not implement a remediation item until its Red phase (failing test and, when applicable, scenario) is in place.

## Remediation Tasks (Checklist)

Execute each task in Red-Green-Refactor order: scenario/BDD and failing test(s) first, then implementation.

- [ ] **Orchestrator compose:** Make OLLAMA an optional service (profile or separate file).
  Ensure control-plane, user-gateway, cynode-pma can start and run when OLLAMA is not present (readyz may stay 503 until inference path exists; document behavior).
  Red: scenario(s) and/or E2E that expect stack up without OLLAMA; Green: compose change.
- [ ] **Shared payloads:** Add `inference_backend` (enabled, image, variant, port) to `node_configuration_payload_v1` in `go_shared_libs`; spec already in [worker_node_payloads.md](../tech_specs/worker_node_payloads.md).
  Red: Go tests / BDD that expect payload to include inference_backend; Green: struct and JSON.
- [ ] **Orchestrator config builder:** When building node configuration for a registered node, derive `inference_backend` from node capability report (GPU features, platform) and policy; set image/variant (e.g. ROCm vs CUDA).
  Include in payload when the node is inference-capable and inference is enabled **and** the node does not report `inference.existing_service` true (do not instruct start when node already has host inference).
  Red: unit or BDD that GET /v1/nodes/config returns inference_backend when node is inference-capable and not existing_service; Green: builder logic.
- [ ] **Node Manager:** Implement [Existing Inference Service on Host](../tech_specs/worker_node.md#spec-cynai-worker-existinginferenceservice): check for existing inference service first; if present, use it and do not start a container; report `inference.existing_service` and `inference.running` in capability report.
  Read `inference_backend` from fetched config; start OLLAMA only when **no** existing service **and** instructed (`inference_backend.enabled` and image or variant present).
  Use config image and variant; pass variant to container (env or image tag).
  Red: unit/BDD that node does not start OLLAMA when config omits or disables inference_backend, and that node uses existing host inference when detected; Green: conditional in applyConfigAndStartServices and capability reporting.
- [ ] **Node Manager startOllama:** Accept parameters (image, variant) from config instead of only env; implement variant-specific image or env (e.g. ROCm vs CUDA image or runtime flags).
  Red: tests that startOllama is called with config-derived params when instructed; Green: signature and implementation.
- [ ] **Dev docs / justfile:** Document "orchestrator-only" vs "orchestrator + ollama" vs "full node" startup.
  Add target or env to run prescribed sequence (orchestrator without OLLAMA, node with OLLAMA after config).
- [ ] **E2E (prescribed sequence):** Add or adjust one E2E path to verify the prescribed sequence: orchestrator stack without OLLAMA, node registers, receives `inference_backend` in config, starts Worker API then OLLAMA per config.
  **Red:** Add or extend a Gherkin scenario under `features/` for this flow; add or extend a Python E2E module in `scripts/test_scripts/` (e.g. `e2e_175_prescribed_startup_sequence.py` or similar) so the scenario is exercised and fails before implementation.
  Follow [scripts/test_scripts/README.md](../../scripts/test_scripts/README.md): numbering (gaps of 10), `config`/`helpers`/`e2e_state`, `just e2e` / `run_e2e.py`; support `--skip-ollama` or inference-skip env where the test runs without OLLAMA in compose.
  **Green:** Implement remediation items above until this E2E path passes.

## Out of Scope for This Plan

- Implementation of ROCm/CUDA-specific images or image selection logic.
  Orchestrator may return a variant string.
  Node chooses image or env from it.
- Changes to Makefile/justfile beyond documenting new targets or env (per docs-only constraint).
  Implementation phase may add just targets.
- Linter or lint-rule changes.
