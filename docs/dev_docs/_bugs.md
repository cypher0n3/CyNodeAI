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

#### Worker Gpu Detection Order

   In `worker_node/internal/nodeagent/gpu.go`, `detectGPU` uses a fixed order: **AMD ROCm (rocm-smi) first, then NVIDIA (nvidia-smi)**.
   On a dual-GPU machine (e.g. laptop with integrated AMD Cezanne + discrete NVIDIA RTX 3080), if both `rocm-smi` and `nvidia-smi` are present, the first non-nil result wins.
   So ROCm is chosen whenever the AMD GPU is detectable, and the node reports `inference_backend.variant = "rocm"`.
   The orchestrator's `variantAndVRAM` (in `orchestrator/internal/handlers/nodes.go`) correctly maps vendor -> variant from the report; the wrong variant comes from the worker's detection order.

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
  When `inference_backend.image` is absent, the node MUST derive the image from **variant** (e.g. base image + `:variant`), not from a single `OLLAMA_IMAGE` env that ignores variant.
- **CYNAI.ORCHES.InferenceContainerDecision** ([orchestrator_inference_container_decision.md](../tech_specs/orchestrator_inference_container_decision.md)): The orchestrator owns the deterministic decision; the node must not substitute an env default that contradicts the orchestrator-derived variant.

#### Re-Evaluation of Related Code Paths

- **Node-manager startup** (`worker_node/internal/nodeagent/nodemanager.go`): Registration and `FetchConfig` happen first; `maybeStartOllama` is called only after config is fetched and only when `nodeConfig.InferenceBackend != nil` and `nodeConfig.InferenceBackend.Enabled`.
  So the node **does** wait for orchestrator direction before starting Ollama.
  The bug is in **how** it starts: in `maybeStartOllama`, when `nodeConfig.InferenceBackend.Image` is empty, the code uses `getEnv("OLLAMA_IMAGE", "ollama/ollama")` and does **not** derive the image from `nodeConfig.InferenceBackend.Variant`.
  So if the orchestrator sends `variant: "cuda"` and omits `image`, the node should start `ollama/ollama:cuda` (from variant); instead it starts whatever `OLLAMA_IMAGE` is (e.g. `ollama/ollama:rocm` from the justfile), violating the payload spec.
- **Justfile** (`scripts/justfile`): Setting `OLLAMA_IMAGE="${OLLAMA_IMAGE:-ollama/ollama:rocm}"` before starting the node-manager makes the node-manager's fallback a single-variant default that overrides the orchestrator's variant when the orchestrator omits `image`.
  The justfile should not set a default that contradicts orchestrator-directed behavior; ideally the node would never need this env when the orchestrator supplies variant (and the node derives image from variant when image is absent).
- **Orchestrator** (`orchestrator/internal/handlers/nodes.go`): `deriveInferenceBackend` and `variantAndVRAM` use the capability report; variant is correct **given** the report.
  The wrong variant in the report comes from the worker's GPU detection order (ROCM first), so fixing detection order (or preferring discrete GPUs) remains required so the orchestrator receives the right capability and sends the right variant.

- **No "primary" or "prefer discrete" policy** (worker GPU detection): Even if detection order were changed, there is no logic to prefer the discrete GPU (e.g. by VRAM or PCI slot) when both AMD and NVIDIA are present; `variantAndVRAM` uses `report.GPU.Devices[0]` only.

### Bug 1 Desired Behavior (Design - Docs Only)

- **Prefer discrete GPUs.**
  When multiple GPUs are present (e.g. integrated + discrete), the system MUST prefer discrete GPUs for inference.
  If there are no discrete GPUs, use whatever is available (e.g. integrated).
  Variant selection (rocm vs cuda) and primary device for VRAM sizing should be derived from this preferred set.
- **Ollama gets all discrete GPUs.**
  The Ollama container MUST be configured with access to **all** available discrete GPUs (e.g. `--gpus all` for the discrete set, or the equivalent device list for the runtime), not only the primary one.
  This allows multi-GPU inference and avoids underusing hardware.

Implementation details (detection of discrete vs integrated, payload shape, and container run args) belong in a tech spec or implementation plan when code changes are made.

### Bug 1 Updated References (Requirements and Specs)

Authoritative req/spec sources that were updated to clarify correct behavior (orchestrator directs inference backend; node derives image from variant when image absent; node must not use env default that overrides variant):

- **Type:** Requirement
  - id / doc: REQ-WORKER-0253
  - location: [worker.md](../requirements/worker.md#req-worker-0253)
- **Type:** Requirement
  - id / doc: REQ-ORCHES-0149
  - location: [orches.md](../requirements/orches.md#req-orches-0149)
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
- [features/orchestrator/node_registration_and_config.feature](../../features/orchestrator/node_registration_and_config.feature): "GET config returns inference_backend with variant consistent with node GPU capability"

### Bug 1 Suggested Tests (To Catch This and Prevent Regressions)

- **Unit tests (worker_node)**
  - **`worker_node/internal/nodeagent/gpu_test.go`**: Add a test that, when **both** ROCm and NVIDIA are "present" (e.g. mock or fake `exec.CommandContext` so `detectROCmGPU` and `detectNVIDIAGPU` return non-nil), `detectGPU` returns the **preferred** GPU (e.g. NVIDIA when policy is "prefer discrete" or "prefer NVIDIA when both present").
  - Until policy exists: add a test that documents current behavior (e.g. "when both ROCm and NVIDIA detect, first in detection order wins") so any future change to order or policy is caught if it regresses.

- **Unit tests (orchestrator)**
  - **`orchestrator/internal/handlers/nodes_test.go`**: Add a `variantAndVRAM` case where the capability report has **multiple GPU devices** (one AMD, one NVIDIA).
  - Assert which variant is chosen (today it is `Devices[0]`; if we add "prefer NVIDIA when both" or "prefer by VRAM," the test should encode that).

- **Feature file / BDD**
  - **Worker node**: In `features/worker_node/` (e.g. extend `node_manager_config_startup.feature` or add `worker_gpu_ollama_variant.feature`), add a scenario such as:
    - Given a node that reports GPU capability with **only** NVIDIA devices (or a single device with `cuda_capability`), when the node receives config with `inference_backend`, then the started Ollama image (or variant passed to StartOllama) is the **cuda** variant (not rocm).
  - **Orchestrator**: In `features/orchestrator/node_registration_and_config.feature`, add a scenario:
    - Given a capability snapshot that contains GPU with NVIDIA only (or primary NVIDIA), when GET config is called for that node, then the payload includes `inference_backend` with `variant` equal to `cuda` (not `rocm`).
  - Optional: scenario for "dual GPU (AMD + NVIDIA), variant reflects preferred GPU (e.g. cuda when policy is prefer-NVIDIA)."

- **E2E**
  - Add or extend an E2E that, when run on a host **with** NVIDIA and **without** AMD/rocm (or with a mock that reports only NVIDIA):
    - After stack bring-up (or node-up), assert the running Ollama container image tag is **cuda** (or the intended variant), not **rocm**.
  - Could be a tagged E2E (e.g. `@nvidia-gpu` or `@gpu-variant`) that is skipped in CI unless a flag or env (e.g. `E2E_GPU_VARIANT_CHECK=1` and nvidia-smi present) is set, to avoid flakiness on heterogeneous CI runners.

- **Justfile / compose**
  - Document or add a check: when `OLLAMA_IMAGE` is unset and the stack is brought up, either detect GPU and set `OLLAMA_IMAGE` (e.g. `ollama/ollama:cuda` when nvidia-smi succeeds) or document that users on NVIDIA must set `OLLAMA_IMAGE=ollama/ollama:cuda`.
  - An E2E or script that validates "Ollama container image matches expected variant for this host" would catch the wrong default.
