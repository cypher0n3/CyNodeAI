# Orchestrator Deterministic Inference Container Decision

- [Document Overview](#document-overview)
  - [Traces To](#traces-to)
  - [Related Specs](#related-specs)
- [Scope and Goals](#scope-and-goals)
  - [Vendor Support (MVP vs Post-MVP)](#vendor-support-mvp-vs-post-mvp)
- [Decision Contract](#decision-contract)
- [Inputs](#inputs)
  - [From Node Capability Report](#from-node-capability-report)
  - [From Orchestrator Policy](#from-orchestrator-policy)
- [Output](#output)
- [Deterministic Decision Algorithm](#deterministic-decision-algorithm)
  - [`InferenceDecision` Algorithm](#inferencedecision-algorithm)
- [Variant Selection Rules](#variant-selection-rules)
  - [When Multiple GPU Types (Mixed Vendors) Are Reported](#when-multiple-gpu-types-mixed-vendors-are-reported)
- [VRAM Considerations](#vram-considerations)
- [Compute Considerations](#compute-considerations)
- [Image Selection](#image-selection)
- [Backend Environment Derivation](#backend-environment-derivation)
- [Traceability](#traceability)

## Document Overview

This spec defines how the orchestrator deterministically decides what inference container instruction to include in the node configuration payload when a worker node registers (or re-registers at startup).
Same capability report and policy MUST yield the same `inference_backend` result so that node behavior is predictable and testable.

### Traces To

- [REQ-ORCHES-0149](../requirements/orches.md#req-orches-0149)

### Related Specs

- [worker_node.md](worker_node.md): Node Startup Procedure, Configuration Delivery, Sandbox-Only Nodes, Existing Inference Service
- [worker_node_payloads.md](worker_node_payloads.md): `node_capability_report_v1`, `node_configuration_payload_v1` `inference_backend`

## Scope and Goals

- **In scope:** The logic the orchestrator uses when building `node_configuration_payload_v1` for a given node to set or omit `inference_backend`.
- **Goals:** Determinism (same inputs => same output), alignment with [REQ-ORCHES-0149](../requirements/orches.md#req-orches-0149) and [Configuration Delivery](worker_node.md#spec-cynai-worker-configurationdelivery), and a single place to define the decision so implementers and tests can verify behavior.

### Vendor Support (MVP vs Post-MVP)

- Spec ID: `CYNAI.ORCHES.InferenceVendorSupportMvp` <a id="spec-cynai-orches-inferencevendorsupportmvp"></a>

- **MVP:** Supported GPU vendors for variant selection are **AMD** (variant `rocm`) and **NVIDIA** (variant `cuda`).
  The orchestrator sums total VRAM per vendor across these two only and selects the variant for the vendor with the greatest total VRAM.
- **Intel (deferred until post-MVP):** Intel discrete GPUs (e.g. Arc) are capable of LLM inference (e.g. via SYCL/oneAPI or OpenVINO), but **Intel GPU support is deferred until post-MVP**.
  When implemented post-MVP: the node MAY report Intel devices (vendor `Intel`, `vram_mb`, and appropriate features); the orchestrator will include Intel in the total-VRAM-per-vendor comparison and MAY emit variant `intel` (or `sycl`) when Intel has the greatest total VRAM; policy and image selection will support an Intel backend image.
  Until then, nodes MUST NOT report Intel as a selected variant for inference backend; if only Intel GPUs are present, the orchestrator SHALL treat the node as CPU for inference (or "do not start" per policy).
  See [REQ-ORCHES-0175](../requirements/orches.md#req-orches-0175) and [Vendor Support (MVP vs Post-MVP)](#spec-cynai-orches-inferencevendorsupportmvp).

## Decision Contract

- Spec ID: `CYNAI.ORCHES.InferenceContainerDecision` <a id="spec-cynai-orches-inferencecontainerdecision"></a>

When the orchestrator builds or updates the node configuration payload for a registered node (at registration or on config refresh), it MUST compute the `inference_backend` section (or its absence) using only:

- The node's capability report (current or last stored snapshot used for config build).
- Orchestrator-side policy (e.g. default image, allowed variants, feature flags).
- No randomness or environment-dependent data that would change the result for the same inputs.

The result MUST be one of:

- **Do not start inference container:** Omit `inference_backend`, or set `inference_backend.enabled` to `false`.
- **Start inference container:** Set `inference_backend` with `enabled: true` and at least one of `image` or `variant` (and optionally `port`) so the node can start the correct container per [worker_node_payloads.md](worker_node_payloads.md#spec-cynai-worker-payload-configuration-v1).

## Inputs

- Spec ID: `CYNAI.ORCHES.InferenceDecisionInputs` <a id="spec-cynai-orches-inferencedecisioninputs"></a>

### From Node Capability Report

The node capability report MUST contain only factual information (and an optional user-defined override) so that the orchestrator can make the decision.
The orchestrator MUST use the following fields from the node capability report (schema: [node_capability_report_v1](worker_node_payloads.md#spec-cynai-worker-payload-capabilityreport-v1)):

#### Factual (Node Observes and Reports)

- `inference.existing_service` (boolean, optional): When true, the node is already using a host-existing inference service; the node MUST NOT start another container.
- `inference.running` (boolean, optional): Whether inference is currently available on the node.
- `inference.supported` (boolean, optional): Factual: the node has the hardware and runtime capability to run inference (e.g. GPU or CPU, container runtime).
  Derived by the node from local detection, not from user preference.
- `gpu.present` (boolean, optional).
- `gpu.devices` (array, optional): When multiple GPU types (e.g. AMD and NVIDIA) exist, the node MUST report **all** devices from all supported vendors, each with `vendor`, `vram_mb`, and `features` (e.g. `cuda_capability`, `rocm_version`) so the orchestrator can compute total VRAM per vendor for variant selection.
  Each device MAY include `model`, `device_id`, `available_vram_mb` (when reported).
- `gpu.total_vram_mb` (int, optional): Total VRAM across all devices; tracked separately from per-device values.
- `gpu.total_available_vram_mb` (int, optional): Total available (free) VRAM; tracked separately from per-device available VRAM and from total capacity.
- `compute` (object): CPU and system memory; see [Compute considerations](#compute-considerations).
  - `cpu_model` (string, optional): CPU model identifier.
  - `cpu_count` (int, optional): Number of CPU sockets or physical CPUs; tracked separately from core count.
  - `cpu_cores` (int): Total logical or physical cores (existing in schema).
  - `cpu_clock_base_mhz` (number, optional): Base clock speed in MHz.
  - `cpu_clock_boost_mhz` (number, optional): Boost/max clock speed in MHz; tracked separately from base.
  - `ram_mb` (int): System memory (total) in MiB.
  - `ram_available_mb` (int, optional): Available system memory in MiB; tracked separately from total.
- `platform.arch` (string): e.g. `amd64`, `arm64`.
- `node.labels` (array of strings, optional): e.g. `sandbox_only`, `gpu`, `region_*`.

#### User-Defined Override (Optional)

- `inference.mode` (string, optional): User-defined override read by the node from local config (node startup YAML, environment variables, etc.) at startup.
  The node MUST report this when the operator has set it; when absent, the orchestrator treats it as `allow`.
  Allowed values: `allow` (no override; orchestrator decides from facts and policy), `disabled` (operator requires no inference on this node), `require` (operator requires inference when policy allows).
  The orchestrator MUST honor this override when making the decision.

Missing or omitted fields are treated as specified in the algorithm (e.g. missing `inference.supported` => treat as not supported unless other signals indicate otherwise; missing `inference.existing_service` => treat as false; missing `inference.mode` => treat as `allow`).

### From Orchestrator Policy

- Default or allowed inference backend image(s) per variant (e.g. default Ollama image for `cpu`, `cuda`, `rocm`).
- Optional allowlist of variants (e.g. only `cpu` and `cuda`).
- Optional feature flag or policy to disable node-local inference for specific nodes or globally.

Policy source (e.g. bootstrap YAML, database, env) is out of scope for this spec; the algorithm assumes policy values are already resolved when the decision runs.

## Output

- Spec ID: `CYNAI.ORCHES.InferenceDecisionOutput` <a id="spec-cynai-orches-inferencedecisionoutput"></a>

The output is the `inference_backend` object (or its absence) for `node_configuration_payload_v1`:

- When the decision is "do not start": omit `inference_backend` or set `inference_backend.enabled = false`.
- When the decision is "start": set `inference_backend` with:
  - `enabled: true`
  - `variant` (string): One of `cpu`, `cuda`, `rocm`, or another orchestrator-defined variant consistent with node capabilities.
  - `image` (string, optional): OCI image reference; when absent the node MUST derive the image from the supplied variant per [worker_node_payloads](worker_node_payloads.md) `inference_backend` (MUST NOT use a node-local default that ignores or overrides variant).
  - `port` (int, optional): e.g. 11434 for Ollama.
  - `env` (object, optional): deterministic orchestrator-derived environment values required for the selected backend behavior.

## Deterministic Decision Algorithm

- Spec ID: `CYNAI.ORCHES.InferenceDecisionAlgorithm` <a id="spec-cynai-orches-inferencedecisionalgorithm"></a>

The orchestrator MUST compute the result by applying the following steps in order.
The first condition that matches determines the outcome; no later steps are applied.

### `InferenceDecision` Algorithm

<a id="algo-cynai-orches-inferencedecisionalgorithm"></a>

1. **Existing service:** If `inference.existing_service === true`, output "do not start" (omit `inference_backend` or set `enabled: false`). <a id="algo-cynai-orches-inferencedecisionalgorithm-step-1"></a>
   Rationale: The node already has an inference service; see [Existing Inference Service on Host](worker_node.md#spec-cynai-worker-existinginferenceservice).

2. **User override disabled:** If the node reports a user-defined override `inference.mode === 'disabled'` (from local config/env), output "do not start". <a id="algo-cynai-orches-inferencedecisionalgorithm-step-2"></a>

3. **Inference not supported (factual):** If `inference.supported === false` (node lacks hardware or runtime capability), output "do not start". <a id="algo-cynai-orches-inferencedecisionalgorithm-step-3"></a>

4. **Sandbox-only node:** If the node's labels include `sandbox_only` (case-sensitive match in `node.labels`), or if the capability report indicates the node is configured as sandbox-only (e.g. no GPU and labels imply sandbox-only), output "do not start". <a id="algo-cynai-orches-inferencedecisionalgorithm-step-4"></a>
   See [Sandbox-Only Nodes](worker_node.md#spec-cynai-worker-sandboxonlynodes).

5. **Policy override:** If orchestrator policy explicitly disables node-local inference for this node (e.g. by node_slug or label), output "do not start". <a id="algo-cynai-orches-inferencedecisionalgorithm-step-5"></a>
   When the node reports `inference.mode === 'require'`, the orchestrator MUST NOT output "do not start" in this step solely due to optional policy (e.g. allowlist); mandatory policy (e.g. safety) still applies.

6. **Otherwise:** The node is inference-capable and should start a container. <a id="algo-cynai-orches-inferencedecisionalgorithm-step-6"></a>
   Compute `variant` per [Variant Selection Rules](#variant-selection-rules) and `image` per [Image Selection](#image-selection).
   Output `inference_backend` with `enabled: true`, `variant`, and optionally `image`, `port`, and `env`.

## Variant Selection Rules

- Spec ID: `CYNAI.ORCHES.InferenceVariantSelection` <a id="spec-cynai-orches-inferencevariantselection"></a>

Variant MUST be chosen deterministically from the capability report and policy.
GPU preference MUST be driven by **model and/or VRAM**, not vendor alone.

### When Multiple GPU Types (Mixed Vendors) Are Reported

1. **Total VRAM per vendor:** For each **MVP-supported** vendor (AMD, NVIDIA), sum `vram_mb` across all devices of that vendor in `gpu.devices` (use 0 when `vram_mb` is missing for a device).
  Intel is not included in MVP; see [Vendor Support (MVP vs Post-MVP)](#spec-cynai-orches-inferencevendorsupportmvp).
2. **Choose variant:** Select the variant (rocm or cuda) that corresponds to the **vendor with the greatest total VRAM** (among AMD and NVIDIA only for MVP).
   If only one vendor has devices, use that variant.
   If both vendors have the same total VRAM, use a deterministic tie-break (e.g. policy: prefer cuda over rocm, or prefer rocm over cuda; same report and policy MUST always yield the same result).
3. **CPU:** Use variant `cpu` when no GPU is present, no device matches a supported variant, or policy only allows `cpu`.

#### Edge Cases (Informative)

NVIDIA dominant:

- 1 AMD device 20 GB vs 3 NVIDIA devices 12 GB each -> NVIDIA total 36 GB > 20 GB -> variant `cuda`.
- 2 NVIDIA devices 12 GB + 6 GB vs 1 AMD device 16 GB -> NVIDIA total 18 GB > 16 GB -> variant `cuda`.

AMD dominant:

- 1 NVIDIA device 12 GB vs 3 AMD devices 8 GB each -> AMD total 24 GB > 12 GB -> variant `rocm`.
- 2 AMD devices 16 GB + 12 GB vs 1 NVIDIA device 20 GB -> AMD total 28 GB > 20 GB -> variant `rocm`.
- 1 NVIDIA device 8 GB vs 1 AMD device 16 GB -> AMD total 16 GB > 8 GB -> variant `rocm`.

**When only one vendor is reported:** Use that vendor's variant (rocm for AMD, cuda for NVIDIA); otherwise `cpu`.
If the node reports only Intel GPUs (post-MVP capability report), the orchestrator SHALL not select an Intel variant in MVP and SHALL fall back to `cpu` (or "do not start" per policy); see [Vendor Support (MVP vs Post-MVP)](#spec-cynai-orches-inferencevendorsupportmvp).

## VRAM Considerations

- Spec ID: `CYNAI.ORCHES.InferenceVramConsiderations` <a id="spec-cynai-orches-inferencevramconsiderations"></a>

Variant selection MUST use VRAM when the capability report provides it and multiple vendors are present.
Per-GPU and total values are tracked separately so the orchestrator can apply policy.

- **Total VRAM per vendor:** When `gpu.devices` contains devices from more than one vendor (e.g. AMD and NVIDIA), the orchestrator MUST sum `vram_mb` per vendor and select the variant for the vendor with the **greatest total VRAM** per [Variant Selection Rules](#variant-selection-rules).
  The node capability report MUST include **all** GPUs (all vendors) with per-device `vram_mb` so this sum is correct.
- **Per GPU:** Each entry in `gpu.devices` MAY include `vram_mb` (capacity) and `available_vram_mb` (free at report time).
  When present, the orchestrator MAY use per-device available VRAM for policy (e.g. exclude devices below a threshold) or for tie-breaking.
- **Total:** `gpu.total_vram_mb` and `gpu.total_available_vram_mb` are tracked separately from per-device values.
  When present, the orchestrator MAY use total available VRAM for policy (e.g. minimum total available VRAM to enable GPU inference, or to choose a lighter image when below a threshold).

Any use of VRAM in the decision MUST be deterministic (e.g. same report and policy yield the same result).
Policy MAY define minimum per-device or minimum total available VRAM for GPU variants; when below threshold, the orchestrator falls back deterministically (e.g. to `cpu` or to "do not start") per algorithm order.

## Compute Considerations

- Spec ID: `CYNAI.ORCHES.InferenceComputeConsiderations` <a id="spec-cynai-orches-inferencecomputeconsiderations"></a>

The orchestrator MUST track and MAY use CPU and system memory from the capability report when making the inference container decision.
These inputs support policy thresholds and deterministic fallbacks (e.g. CPU-only inference when GPU VRAM is insufficient, or "do not start" when system memory is below minimum).

- **CPUs:** `compute.cpu_model` and `compute.cpu_count` (sockets/physical CPUs) are tracked separately from core count.
- **Cores:** `compute.cpu_cores` (total logical or physical cores) is used when present.
- **Clock speed:** `compute.cpu_clock_base_mhz` (base) and `compute.cpu_clock_boost_mhz` (boost) are tracked separately so policy can use either or both (e.g. minimum base clock for CPU inference).
- **System memory:** `compute.ram_mb` (total) and `compute.ram_available_mb` (available) are tracked separately.
  When present, the orchestrator MAY use total or available RAM for policy (e.g. minimum system RAM to start inference, or to choose a lighter image when below a threshold).

Any use of compute fields in the decision MUST be deterministic.
Policy MAY define minimum cores, minimum base or boost clock, or minimum (total or available) system memory; when below threshold, the orchestrator falls back deterministically per algorithm order.

## Image Selection

- Spec ID: `CYNAI.ORCHES.InferenceImageSelection` <a id="spec-cynai-orches-inferenceimageselection"></a>

The orchestrator MUST select the container image from policy keyed by the chosen `variant` (e.g. default image for `cpu`, `cuda`, `rocm`).
When policy does not specify an image for the variant, the orchestrator MAY omit `inference_backend.image`; the node MUST then derive the image from the supplied variant per [worker_node_payloads](worker_node_payloads.md) `inference_backend` (and MUST NOT use a node-local default that ignores or overrides the variant).
For **Ollama**, when the node derives the image: variant `rocm` -> `ollama/ollama:rocm`; variant `cuda` or `cpu` -> `ollama/ollama` or `ollama/ollama:latest` (Ollama publishes no separate cuda tag).

Image selection MUST be deterministic for the same (variant, policy) pair.

## Backend Environment Derivation

- Spec ID: `CYNAI.ORCHES.InferenceBackendEnv` <a id="spec-cynai-orches-inferencebackendenv"></a>

The orchestrator MUST determine the effective runtime configuration for node-local inference in a way that maximizes the safe usable context window for the expected local model workload within the node's available resources.

- That determination MUST be deterministic for the same capability report and policy inputs.
- Relevant inputs MAY include available VRAM, total VRAM, available system memory, backend variant, and the expected loaded model set or model class for the node.
- When backend-specific environment values are required to realize that effective runtime configuration, the orchestrator MUST derive and emit them in `inference_backend.env`.
- When the orchestrator also directs a managed service whose `inference.mode` is `node_local`, the orchestrator MUST mirror the same effective backend environment values into that managed service's `inference.backend_env` when the service requires the same backend behavior.
- Example backend environment values include context-window sizing or runner-tuning inputs such as `OLLAMA_CONTEXT_LENGTH` or `OLLAMA_NUM_CTX`.

## Traceability

- **REQ-ORCHES-0149:** [orches.md](../requirements/orches.md#req-orches-0149).
  Orchestrator returns node config that instructs whether and how to start the local inference backend.
  Inference backend instructions are derived from the node capability report.
- **REQ-ORCHES-0175:** [orches.md](../requirements/orches.md#req-orches-0175).
  Intel GPU support (variant selection, inference backend image) is deferred until post-MVP; MVP supports AMD and NVIDIA only.
- **Configuration Delivery:** [worker_node.md#spec-cynai-worker-configurationdelivery](worker_node.md#spec-cynai-worker-configurationdelivery).
  Orchestrator derives inference backend instruction from capability report and policy.
  It includes it when the node is inference-capable and inference is enabled.
- **REQ-ORCHES-0169:** [orches.md](../requirements/orches.md#req-orches-0169).
  Orchestrator owns effective backend-derived environment values and delivers them through the canonical node-configuration contract.
- **Payload schema:** [worker_node_payloads.md#spec-cynai-worker-payload-configuration-v1](worker_node_payloads.md#spec-cynai-worker-payload-configuration-v1).
  `inference_backend` structure and semantics.
