# Identified Bugs

## Overview

This doc outlines any discovered bugs that need to be addressed.

## Bug 1: ROCM Ollama on Nvidia GPU

On a system with an NVIDIA GPU, the ROCM Ollama container was launched, meaning either the logic for determining what container to run is wrong, or it's not actually being set dynamically properly.
Could also be that it's detecting both GPUs on the system (laptop with discrete GPU), and it isn't set up to handle that.

### Bug 1 Evidence

```text
~  podman ps
CONTAINER ID  IMAGE                                      COMMAND               CREATED        STATUS                  PORTS                               NAMES
ce28a546db63  docker.io/pgvector/pgvector:pg16           postgres              6 minutes ago  Up 6 minutes (healthy)  0.0.0.0:5432->5432/tcp              cynodeai-postgres
d07722d1bd4d  localhost/orchestrator_mcp-gateway:latest                        6 minutes ago  Up 6 minutes (healthy)  0.0.0.0:12083->12083/tcp, 8083/tcp  cynodeai-mcp-gateway
a52ecd7f9334  localhost/orchestrator_api-egress:latest                         6 minutes ago  Up 6 minutes (healthy)  0.0.0.0:12084->12084/tcp, 8084/tcp  cynodeai-api-egress
dafe1e39ed0d  localhost/cynodeai-control-plane:dev                             6 minutes ago  Up 6 minutes (healthy)  0.0.0.0:12082->12082/tcp, 8082/tcp  cynodeai-control-plane
c86178471826  localhost/cynodeai-user-gateway:dev                              6 minutes ago  Up 6 minutes (healthy)  0.0.0.0:12080->12080/tcp, 8080/tcp  cynodeai-user-gateway
6ffbd30f245b  docker.io/ollama/ollama:rocm               serve                 6 minutes ago  Up 6 minutes            0.0.0.0:11434->11434/tcp            cynodeai-ollama
ed62630c55f3  localhost/cynodeai-cynode-pma:dev          --role=project_ma...  4 minutes ago  Up 4 minutes (healthy)  8090/tcp                            cynodeai-managed-pma-main
b70e21293710  docker.io/library/alpine:latest            sh -c sleep 300       2 minutes ago  Up 2 minutes                                                stupefied_hofstadter
~  lspci | grep VGA
01:00.0 VGA compatible controller: NVIDIA Corporation GA104M [GeForce RTX 3080 Mobile / Max-Q 8GB/16GB] (rev a1)
07:00.0 VGA compatible controller: Advanced Micro Devices, Inc. [AMD/ATI] Cezanne [Radeon Vega Series / Radeon Vega Mobile Series] (rev c4)
```

### Bug 1 Likely Causes (From Implementation Inspection)

Likely causes of the bug.

#### Worker GPU Detection and Reporting

- **Fixed:** In `worker_node/internal/nodeagent/gpu.go`, `detectGPU` merges devices from both `detectNVIDIAGPU` and `detectROCmGPU` into a single `GPUInfo`.
  Reports all GPUs from all supported vendors (REQ-WORKER-0265).

#### Multiple GPUs and Total VRAM per Vendor

GPU preference MUST be driven by **model and/or VRAM**, not vendor alone.
When multiple GPUs of different types exist, the system MUST prefer the **vendor whose total VRAM (sum of all devices of that vendor) is greatest**.

##### Edge Cases

- **1 AMD GPU 20 GB vs 3 NVIDIA GPUs 12 GB each:** Total NVIDIA = 36 GB > 20 GB AMD -> select **cuda** (NVIDIA).
- **2 NVIDIA GPUs 12 GB + 6 GB vs 1 AMD GPU 16 GB:** Total NVIDIA = 18 GB > 16 GB AMD -> select **cuda** (NVIDIA).
- **1 NVIDIA GPU 12 GB vs 3 AMD GPUs 8 GB each:** Total AMD = 24 GB > 12 GB NVIDIA -> select **rocm** (AMD).
- **2 AMD GPUs 16 GB + 12 GB vs 1 NVIDIA GPU 20 GB:** Total AMD = 28 GB > 20 GB NVIDIA -> select **rocm** (AMD).
- **1 NVIDIA GPU 8 GB vs 1 AMD GPU 16 GB:** Total AMD = 16 GB > 8 GB NVIDIA -> select **rocm** (AMD).

So variant selection MUST use **total VRAM per vendor** (and optionally discrete vs integrated by model when reported), not "first device" or "vendor order."

#### Orchestrator Stack Default Image

The dev stack is started via **`just start`** (from `scripts/justfile`; invoked as `just setup-dev start` or `just scripts/start` from repo root).
There is no `node-up` recipe.

##### How `just start` Works

1. Optionally loads `$root/.env.dev`.
2. Sets `ollama_in_stack` if any arg is `--ollama-in-stack` or env `SETUP_DEV_OLLAMA_IN_STACK=1`.
3. Builds dev binaries and cynode-pma image.
4. Detects runtime (podman or docker) and host alias.
5. Exports Postgres, JWT, ports, NODE_*, etc., then runs **compose down** and **compose up** with optional `--profile ollama` when `ollama_in_stack` is true.
6. **After** compose up, exports `OLLAMA_IMAGE="${OLLAMA_IMAGE:-ollama/ollama:rocm}"` and other node-manager env vars, then starts the node-manager binary in the background.
7. Waits for worker-api healthz and user-gateway readyz.

Because `OLLAMA_IMAGE` is set only after compose runs, the **compose** Ollama service (when using `--ollama-in-stack`) uses whatever `OLLAMA_IMAGE` was in the environment at compose time (often unset, so the compose file default `ollama/ollama`).
The **rocm** default is used by the **node-manager** when it starts the Ollama container (e.g. when Ollama is not in the stack, or when the node-manager brings up inference and no existing `cynodeai-ollama` container is found).
On an NVIDIA-only or NVIDIA-primary machine, that default is wrong.

#### Spec / Requirement Violations

The intended behavior is: the **orchestrator** directs whether and how to start the local inference backend; the **node-manager** MUST NOT start Ollama until it has received that direction and MUST use the orchestrator-supplied variant/image.

- **REQ-WORKER-0253** ([worker.md](../requirements/worker.md#req-worker-0253)): The node MUST NOT start the OLLAMA container until the orchestrator has acknowledged registration and returned node configuration that **instructs** the node to start the local inference backend (**including backend variant**, e.g. ROCm for AMD or CUDA for Nvidia).
  The node-manager does wait for config and only starts when `inference_backend.enabled` is true.
  When `inference_backend.image` is absent, it falls back to `OLLAMA_IMAGE` env (e.g. `ollama/ollama:rocm` from the justfile), which can **override** the orchestrator-derived **variant**, violating the requirement that the orchestrator's instruction (variant) be honored.
- **REQ-ORCHES-0149** ([orches.md](../requirements/orches.md#req-orches-0149)): The orchestrator MUST return a node configuration payload that instructs the node **whether and how** to start the local inference backend.
  The orchestrator does set `inference_backend.variant` (and optionally `image`) from capability; the failure is either (1) wrong variant due to worker GPU detection order, or (2) the node ignoring variant when image is absent by using a single env default.
- **worker_node_payloads** ([worker_node_payloads.md](../tech_specs/worker_node_payloads.md) `inference_backend`): "The node MUST use this [variant] to select or configure the correct image or runtime (ROCM for AMD GPUs, CUDA for Nvidia GPUs when reported in capabilities)."
  When `inference_backend.image` is absent, the node MUST derive the image from **variant** (for Ollama: rocm -> `ollama/ollama:rocm`; cuda or cpu -> `ollama/ollama` or `ollama/ollama:latest`, since Ollama has no cuda tag), not from a single `OLLAMA_IMAGE` env that ignores variant.
- **CYNAI.ORCHES.InferenceContainerDecision** ([orchestrator_inference_container_decision.md](../tech_specs/orchestrator_inference_container_decision.md)): The orchestrator owns the deterministic decision; the node must not substitute an env default that contradicts the orchestrator-derived variant.

#### Re-Evaluation of Related Code Paths

- **Node-manager startup** (`worker_node/internal/nodeagent/nodemanager.go`): Registration and `FetchConfig` happen first; `maybeStartOllama` is called only after config is fetched and only when `nodeConfig.InferenceBackend != nil` and `nodeConfig.InferenceBackend.Enabled`.
  **Fixed:** When `image` is empty and `variant` is set, the node now derives image from variant (`rocm` -> `ollama/ollama:rocm`; `cuda`/`cpu` -> `ollama/ollama`).
    Falls back to `OLLAMA_IMAGE` only when both are empty.
- **Justfile** (`scripts/justfile`): Setting `OLLAMA_IMAGE="${OLLAMA_IMAGE:-ollama/ollama:rocm}"` before starting the node-manager makes the node-manager's fallback a single-variant default that overrides the orchestrator's variant when the orchestrator omits `image`.
  The justfile should not set a default that contradicts orchestrator-directed behavior; ideally the node would never need this env when the orchestrator supplies variant (and the node derives image from variant when image is absent).
- **Orchestrator** (`orchestrator/internal/handlers/nodes.go`): `variantAndVRAM` sums VRAM per vendor and selects variant for vendor with greatest total.
  Worker reports all devices; orchestrator computes correctly.

- **Variant selection by total VRAM:** Orchestrator `variantAndVRAM` sums `vram_mb` per vendor and selects variant for vendor with greatest total.
  Tie-break: prefer cuda over rocm.
  Uses total VRAM of chosen vendor for inference env (orchestrator_inference_container_decision.md).

### Bug 1 Desired Behavior (Design - Docs Only)

- **GPU preference by model and/or VRAM.**
  Variant selection (rocm vs cuda) MUST be driven by **model and/or VRAM detection**, not vendor alone.
  When multiple GPUs exist (same or different vendors), the system MUST prefer the **vendor whose total VRAM (sum of all devices of that vendor) is greatest**; optionally, within that, prefer discrete over integrated by model when reported.
  If only one vendor is present, use that variant; if no GPU, use cpu.
- **Multiple GPUs: total VRAM per vendor.**
  The worker MUST report **all** GPUs (all vendors) with per-device `vram_mb` so the orchestrator can sum total VRAM per vendor and select the variant for the vendor with the greatest total.
  Edge cases (NVIDIA dominant): 1 AMD 20 GB vs 3*NVIDIA 12 GB (36) -> cuda; 2*NVIDIA 12+6 GB vs 1 AMD 16 GB -> cuda.
  Edge cases (AMD dominant): 1 NVIDIA 12 GB vs 3*AMD 8 GB (24) -> rocm; 2*AMD 16+12 GB vs 1 NVIDIA 20 GB -> rocm; 1 NVIDIA 8 GB vs 1 AMD 16 GB -> rocm.
- **Ollama gets all GPUs of the chosen variant.**
  The Ollama container MUST be configured with access to **all** GPUs of the selected variant (e.g. all NVIDIA devices when variant is cuda), not only the first device.
  This allows multi-GPU inference and avoids underusing hardware.

Implementation details (worker reporting all vendors, orchestrator summing VRAM per vendor, discrete vs integrated by model) belong in a tech spec or implementation plan when code changes are made.

### Bug 1 Updated References (Requirements and Specs)

Authoritative req/spec sources that were updated to clarify correct behavior (orchestrator directs inference backend; node derives image from variant when image absent; node must not use env default that overrides variant):

- **Type:** Requirement
  - id / doc: REQ-WORKER-0253
  - location: [worker.md](../requirements/worker.md#req-worker-0253)
- **Type:** Requirement
  - id / doc: REQ-ORCHES-0149
  - location: [orches.md](../requirements/orches.md#req-orches-0149)
- **Type:** Requirement
  - id / doc: REQ-WORKER-0265 (report all GPUs with vram_mb when multiple vendors)
  - location: [worker.md](../requirements/worker.md#req-worker-0265)
- **Type:** Requirement
  - id / doc: REQ-ORCHES-0175 (Intel support deferred until post-MVP)
  - location: [orches.md](../requirements/orches.md#req-orches-0175)
- **Type:** Requirement
  - id / doc: REQ-WORKER-0266 (Intel support deferred until post-MVP)
  - location: [worker.md](../requirements/worker.md#req-worker-0266)
- **Type:** Spec (payload)
  - id / doc: node_configuration_payload_v1 `inference_backend`
  - location: [worker_node_payloads.md](../tech_specs/worker_node_payloads.md)
- **Type:** Spec (orchestrator decision)
  - id / doc: CYNAI.ORCHES.InferenceContainerDecision
  - location: [orchestrator_inference_container_decision.md](../tech_specs/orchestrator_inference_container_decision.md)
- **Type:** Spec (node startup)
  - id / doc: CYNAI.WORKER.NodeStartupProcedure
  - location: [worker_node.md](../tech_specs/worker_node.md#spec-cynai-worker-nodestartupprocedure)
- **Type:** Spec (payload config)
  - id / doc: CYNAI.WORKER.Payload.ConfigurationV1
  - location: [worker_node_payloads.md](../tech_specs/worker_node_payloads.md#spec-cynai-worker-payload-configuration-v1)

Feature files with scenarios added for variant/image behavior:

- [features/worker_node/node_manager_config_startup.feature](../../features/worker_node/node_manager_config_startup.feature): "Node manager starts inference backend with orchestrator-supplied variant when image is absent"
- [features/orchestrator/node_registration_and_config.feature](../../features/orchestrator/node_registration_and_config.feature): "GET config returns inference_backend with variant from vendor with most total VRAM (single vendor)" and "GET config returns inference_backend with variant from vendor with most total VRAM (mixed GPUs)"

#### Intel Discrete GPUs (Deferred Until Post-MVP)

Intel discrete GPUs (e.g. Arc) are accounted for in capability and variant selection in the specs (e.g. `gpu.devices.vendor` MAY include `Intel`; total VRAM per vendor may include Intel post-MVP).
**Implementation of Intel support is deferred until post-MVP** (no standard Ollama Intel image yet; ecosystem still evolving).
See [REQ-ORCHES-0175](../requirements/orches.md#req-orches-0175) and [orchestrator_inference_container_decision.md - Vendor Support](../tech_specs/orchestrator_inference_container_decision.md#spec-cynai-orches-inferencevendorsupportmvp).

### Bug 1 Suggested Tests (To Catch This and Prevent Regressions)

- **Unit tests (worker_node)**
  - **`worker_node/internal/nodeagent/gpu_test.go`**: Add tests that when **both** AMD and NVIDIA are present, `detectGPU` (or the merged capability) returns **all** devices from both vendors with per-device `vram_mb`.
  - Add a test that documents required behavior: capability report includes all GPUs so orchestrator can compute total VRAM per vendor.

- **Unit tests (orchestrator)**
  - **`orchestrator/internal/handlers/nodes_test.go`**: Add `variantAndVRAM` (or equivalent) cases for **total VRAM per vendor**:
    - Multiple devices, one vendor: e.g. 3*NVIDIA 12 GB each -> variant cuda, total VRAM 36 GB.
    - Mixed vendors (NVIDIA dominant): 1 AMD 20 GB vs 3*NVIDIA 12 GB (36) -> variant cuda; 2*NVIDIA 12+6 GB vs 1 AMD 16 GB -> cuda.
    - Mixed vendors (AMD dominant): 1 NVIDIA 12 GB vs 3*AMD 8 GB (24) -> variant rocm; 2*AMD 16+12 GB vs 1 NVIDIA 20 GB -> rocm.
    - Tie-break: when total VRAM is equal, use deterministic tie-break (e.g. policy or vendor order).

- **Feature file / BDD**
  - **Worker node**: Scenario that when the node reports GPU capability with **all** devices (multiple vendors, each with `vram_mb`), the node receives config with `inference_backend.variant` derived from the vendor with most total VRAM; node starts Ollama with that variant.
  - **Orchestrator**: Scenarios for mixed GPUs: (1) 1 AMD 20 GB + 3 NVIDIA 12 GB each -> variant `cuda`; (2) 1 NVIDIA 12 GB + 3 AMD 8 GB each -> variant `rocm`; (3) 2 AMD 16+12 GB + 1 NVIDIA 20 GB -> variant `rocm`.

- **E2E** (implemented)
  - **`e2e_0800_gpu_variant_ollama.py`**: Asserts the running Ollama container image tag matches the expected variant for the host GPU.
  - The Python script **independently** detects GPU via `nvidia-smi` and `rocm-smi`, sums VRAM per vendor, and selects the expected variant (cuda vs rocm) by the same logic as the worker (vendor with greater total VRAM).
  - Run with `E2E_GPU_VARIANT_CHECK=1 just e2e --tags gpu_variant`; skips when env unset, no GPU, or Ollama container not running.
  - **Stack must NOT use** `SETUP_DEV_OLLAMA_IN_STACK` or `--ollama-in-stack` (node-manager must start Ollama, not compose).
  - Ollama image mapping: variant `rocm` -> `ollama/ollama:rocm`; variant `cuda` -> `ollama/ollama` (no separate cuda tag).
  - Traces: REQ-WORKER-0253, REQ-ORCHES-0149.

- **Justfile / compose**
  - Document or add a check: when `OLLAMA_IMAGE` is unset and the stack is brought up, either detect GPU and set `OLLAMA_IMAGE` (e.g. `ollama/ollama` or `ollama/ollama:latest` for NVIDIA; Ollama has no cuda tag) or document that users on NVIDIA use the default image.
  - An E2E or script that validates "Ollama container image matches expected variant for this host" would catch the wrong default.

### Bug 1 Implementation Status (Fully Spec-Compliant)

- **Worker GPU detection** (`gpu.go`): Reports **all** GPUs from all supported vendors (AMD and NVIDIA) in a single capability report.
  Each device includes `vendor`, `vram_mb`, and `features`.
  Compliant with REQ-WORKER-0265 and orchestrator_inference_container_decision.md line 195.
- **Orchestrator variantAndVRAM** (`nodes.go`): Sums `vram_mb` per vendor and selects variant for the vendor with greatest total VRAM.
  Tie-break: prefer cuda over rocm.
  Uses total VRAM of chosen vendor for inference env (OLLAMA_NUM_CTX).
  Handles multiple GPUs of same vendor.
  Compliant with orchestrator_inference_container_decision.md lines 164-165.
- **Node-manager image derivation** (`nodemanager.go`): When `inference_backend.image` is absent and `variant` is set, derives image: `rocm` -> `ollama/ollama:rocm`; `cuda`/`cpu` -> `ollama/ollama` (Ollama has no cuda tag).
  Compliant with worker_node_payloads and REQ-WORKER-0253.
- **Ollama container GPU access** (`main.go`): Uses `--gpus all` for CUDA.
  Compliant with "Ollama gets all GPUs of the chosen variant."
- **E2E validation** (`e2e_0800_gpu_variant_ollama.py`): Independently detects GPU, sums VRAM per vendor, asserts Ollama image tag.
  Stack must NOT use `SETUP_DEV_OLLAMA_IN_STACK`.
- **Unit tests:** Worker: `TestDetectGPU_ReportsAllDevicesWhenBothVendorsPresent`, `TestDetectGPU_SingleVendorReturnsAllDevices`.
  Orchestrator: `TestVariantAndVRAM_SumVRAMPerVendorMixedGPUs`, `TestVariantAndVRAM_MultiGPUSameVendorSumsTotal`, `TestVariantAndVRAM_MixedVendorsNVIDIADominant`, `TestVariantAndVRAM_TieBreakPrefersCuda`.
- **BDD:** Worker node scenario "Node manager starts inference backend with orchestrator-supplied variant when image is absent" passes.
