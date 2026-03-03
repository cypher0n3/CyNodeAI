# Orchestrator Deterministic Inference Container Decision (Draft Spec)

- [Document Overview](#document-overview)
- [Scope and Goals](#scope-and-goals)
- [Decision Contract](#decision-contract)
- [Inputs](#inputs)
- [Output](#output)
- [Deterministic Decision Algorithm](#deterministic-decision-algorithm)
- [Variant Selection Rules](#variant-selection-rules)
- [VRAM Considerations](#vram-considerations)
- [Compute Considerations](#compute-considerations)
- [NPU and Hardware Model Considerations](#npu-and-hardware-model-considerations)
- [Image Selection](#image-selection)
- [Traceability](#traceability)

## Document Overview

This draft spec defines how the orchestrator deterministically decides what inference container instruction to include in the node configuration payload when a worker node registers (or re-registers at startup).
Same capability report and policy MUST yield the same `inference_backend` result so that node behavior is predictable and testable.

Status: Draft in `dev_docs`; not yet promoted to `docs/tech_specs/`.

### Related Specs

- [worker_node.md](../tech_specs/worker_node.md): Node Startup Procedure, Configuration Delivery, Sandbox-Only Nodes, Existing Inference Service
- [worker_node_payloads.md](../tech_specs/worker_node_payloads.md): `node_capability_report_v1`, `node_configuration_payload_v1` `inference_backend`

## Scope and Goals

- **In scope:** The logic the orchestrator uses when building `node_configuration_payload_v1` for a given node to set or omit `inference_backend`.
- **Goals:** Determinism (same inputs => same output), alignment with [REQ-ORCHES-0149](../requirements/orches.md#req-orches-0149) and [Configuration Delivery](../tech_specs/worker_node.md#spec-cynai-worker-configurationdelivery), and a single place to define the decision so implementers and tests can verify behavior.

## Decision Contract

- Spec ID: `CYNAI.ORCHES.InferenceContainerDecision` <a id="spec-cynai-orches-inferencecontainerdecision"></a>

When the orchestrator builds or updates the node configuration payload for a registered node (at registration or on config refresh), it MUST compute the `inference_backend` section (or its absence) using only:

- The node's capability report (current or last stored snapshot used for config build).
- Orchestrator-side policy (e.g. default image, allowed variants, feature flags).
- No randomness or environment-dependent data that would change the result for the same inputs.

The result MUST be one of:

- **Do not start inference container:** Omit `inference_backend`, or set `inference_backend.enabled` to `false`.
- **Start inference container:** Set `inference_backend` with `enabled: true` and at least one of `image` or `variant` (and optionally `port`) so the node can start the correct container per [worker_node_payloads.md](../tech_specs/worker_node_payloads.md#spec-cynai-worker-payload-configuration-v1).

## Inputs

- Spec ID: `CYNAI.ORCHES.InferenceDecisionInputs` <a id="spec-cynai-orches-inferencedecisioninputs"></a>

### From Node Capability Report

The orchestrator MUST use the following fields from the node capability report (schema: [node_capability_report_v1](../tech_specs/worker_node_payloads.md#spec-cynai-worker-payload-capabilityreport-v1)):

- `inference.supported` (boolean, optional): Whether the node supports inference.
- `inference.mode` (string, optional): e.g. `allow`, `disabled`.
- `inference.existing_service` (boolean, optional): When true, the node is already using a host-existing inference service; the node MUST NOT start another container.
- `inference.running` (boolean, optional): Whether inference is currently available on the node.
- `gpu.present` (boolean, optional).
- `gpu.devices` (array, optional): Each device MAY include `vendor`, `model`, `device_id`, `vram_mb`, `available_vram_mb` (when reported), `features` (e.g. `cuda_capability`, `rocm_version`).
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
- `platform.os` (string, optional): e.g. `linux`, `darwin`; used with arch and hardware model for platform identity.
- `hardware` (object, optional): Hardware identity and model information; see [NPU and Hardware Model Considerations](#npu-and-hardware-model-considerations).
  - `product_name` (string, optional): Human-oriented product or machine name, e.g. `Mac mini`, `MacBook Pro`, `ThinkPad X1`.
  - `chip` (string, optional): SoC or chip family identifier, e.g. `M2`, `M2 Pro`, `M3`, `Apple M1`.
  - `machine_model` (string, optional): Stable machine/model identifier from the OS or firmware (e.g. `Mac14,12` for Mac mini M2).
  - When present, the orchestrator MAY use these to recognize known-good CPU inference platforms (e.g. Mac mini M2 or newer).
- `npu` (object, optional): Neural Processing Unit(s); see [NPU and Hardware Model Considerations](#npu-and-hardware-model-considerations).
  - `present` (boolean, optional): Whether any NPU was detected.
  - `devices` (array, optional): Each entry MAY include `vendor`, `model`, `device_id`, `features` (e.g. driver/runtime identifiers).
  - When present, the orchestrator MAY use NPU for future variant selection (e.g. an `npu` variant) or policy.
- `node.labels` (array of strings, optional): e.g. `sandbox_only`, `gpu`, `region_*`.

Missing or omitted fields are treated as specified in the algorithm (e.g. missing `inference.supported` => treat as not supported unless other signals indicate otherwise; missing `inference.existing_service` => treat as false).

### From Orchestrator Policy

- Default or allowed inference backend image(s) per variant (e.g. default Ollama image for `cpu`, `cuda`, `rocm`).
- Optional allowlist of variants (e.g. only `cpu` and `cuda`).
- Optional feature flag or tenant policy to disable node-local inference for specific nodes or globally.

Policy source (e.g. bootstrap YAML, database, env) is out of scope for this spec; the algorithm assumes policy values are already resolved when the decision runs.

## Output

- Spec ID: `CYNAI.ORCHES.InferenceDecisionOutput` <a id="spec-cynai-orches-inferencedecisionoutput"></a>

The output is the `inference_backend` object (or its absence) for `node_configuration_payload_v1`:

- When the decision is "do not start": omit `inference_backend` or set `inference_backend.enabled = false`.
- When the decision is "start": set `inference_backend` with:
  - `enabled: true`
  - `variant` (string): One of `cpu`, `cuda`, `rocm`, or another orchestrator-defined variant consistent with node capabilities.
  - `image` (string, optional): OCI image reference; when absent the node MAY use a node-local default.
  - `port` (int, optional): e.g. 11434 for Ollama.

## Deterministic Decision Algorithm

- Spec ID: `CYNAI.ORCHES.InferenceDecisionAlgorithm` <a id="spec-cynai-orches-inferencedecisionalgorithm"></a>

The orchestrator MUST compute the result by applying the following steps in order.
The first condition that matches determines the outcome; no later steps are applied.

1. **Existing service:** If `inference.existing_service === true`, output "do not start" (omit `inference_backend` or set `enabled: false`).
   Rationale: The node already has an inference service; see [Existing Inference Service on Host](../tech_specs/worker_node.md#spec-cynai-worker-existinginferenceservice).

2. **Inference not supported or disabled:** If `inference.supported === false` OR `inference.mode === 'disabled'`, output "do not start".

3. **Sandbox-only node:** If the node's labels include `sandbox_only` (case-sensitive match in `node.labels`), or if the capability report indicates the node is configured as sandbox-only (e.g. no GPU and labels imply sandbox-only), output "do not start".
   See [Sandbox-Only Nodes](../tech_specs/worker_node.md#spec-cynai-worker-sandboxonlynodes).

4. **Policy override:** If orchestrator policy explicitly disables node-local inference for this node (e.g. by node_slug or label), output "do not start".

5. **Otherwise:** The node is inference-capable and should start a container.
   Compute `variant` per [Variant Selection Rules](#variant-selection-rules) and `image` per [Image Selection](#image-selection).
   Output `inference_backend` with `enabled: true`, `variant`, and optionally `image` and `port`.

## Variant Selection Rules

- Spec ID: `CYNAI.ORCHES.InferenceVariantSelection` <a id="spec-cynai-orches-inferencevariantselection"></a>

Variant MUST be chosen deterministically from the capability report and policy:

1. **GPU present with ROCm:** If `gpu.present === true` and any entry in `gpu.devices` has `features.rocm_version` set (or vendor/model heuristics indicate AMD), use variant `rocm` if allowed by policy; otherwise fall back to next rule.
2. **GPU present with CUDA:** If `gpu.present === true` and any entry in `gpu.devices` has `features.cuda_capability` set (or vendor/model heuristics indicate Nvidia), use variant `cuda` if allowed by policy; otherwise fall back to next rule.
3. **NPU (future):** If policy defines an `npu` variant and `npu.present === true` with supported device(s) reported, use variant `npu` if allowed by policy; otherwise fall back to next rule.
4. **CPU:** Use variant `cpu` for all other cases (no GPU, or GPU not matching a supported variant, or policy only allows `cpu`).
   Policy MAY use `hardware.product_name`, `hardware.chip`, and `platform.arch` to allow or prefer CPU inference on known-capable platforms (e.g. Mac mini M2 or newer: `product_name` "Mac mini" and `chip` matching M2/M2 Pro/M3 or later).

When multiple GPUs of different types are reported, the orchestrator MAY define a deterministic tie-break (e.g. prefer first device, or prefer CUDA over ROCm over CPU by fixed priority).
This spec recommends: prefer ROCm if any AMD device is present and policy allows; else prefer CUDA if any Nvidia device is present and policy allows; else prefer NPU if `npu.present` and policy allows an `npu` variant; else `cpu`.

## VRAM Considerations

- Spec ID: `CYNAI.ORCHES.InferenceVramConsiderations` <a id="spec-cynai-orches-inferencevramconsiderations"></a>

Available VRAM MUST be a consideration when the capability report provides it.
Per-GPU and total values are tracked separately so the orchestrator can apply policy to each.

- **Per GPU:** Each entry in `gpu.devices` MAY include `vram_mb` (capacity) and `available_vram_mb` (free at report time).
  When present, the orchestrator MAY use per-device available VRAM for variant selection (e.g. prefer a device with sufficient available VRAM) or to exclude devices below a policy threshold.
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

## NPU and Hardware Model Considerations

- Spec ID: `CYNAI.ORCHES.InferenceNpuAndHardwareModel` <a id="spec-cynai-orches-inferencenpuandhardwaremodel"></a>

The orchestrator MUST track and MAY use NPU presence and hardware model information from the capability report when making the inference container decision.
Detection of NPUs, specific CPU architecture (beyond `platform.arch`), and machine identity (e.g. product name, chip family) allows policy to treat certain nodes as well-suited for AI workloads even when no discrete GPU is present.

- **NPU:** When `npu.present === true` and `npu.devices` is reported, the orchestrator MAY use this for future variant selection (e.g. a dedicated `npu` variant when runtime support exists) or to prefer NPU-backed inference over CPU when policy allows.
  Any use of NPU in the decision MUST be deterministic (same report and policy yield the same result).
- **Hardware model:** When `hardware.product_name`, `hardware.chip`, or `hardware.machine_model` are present, the orchestrator MAY use them to recognize known-good CPU inference platforms.
  Example: a node reporting `product_name` "Mac mini" and `chip` "M2" (or "M2 Pro", "M3", etc.) can be treated as suitable for CPU (or future NPU) inference with appropriate policy (e.g. minimum RAM, allow CPU variant).
  Policy MAY define allowlists or rules keyed by product name, chip family, or machine model; when matched, the orchestrator MAY enable or prefer inference on that node deterministically.
- **CPU architecture:** `platform.arch` (e.g. `arm64`, `amd64`) is already an input; together with `compute.cpu_model` and optional `hardware.chip`, the orchestrator can distinguish e.g. Apple Silicon (arm64 + M-series chip) from other arm64 or amd64 systems for policy.

Any use of NPU or hardware model fields in the decision MUST be deterministic.
The node is responsible for populating these fields via platform-specific detection (e.g. sysfs, SMBIOS, macOS system profiler, or vendor APIs); schema and detection methods belong in the payload spec and node capability-reporting logic.

## Image Selection

- Spec ID: `CYNAI.ORCHES.InferenceImageSelection` <a id="spec-cynai-orches-inferenceimageselection"></a>

The orchestrator MUST select the container image from policy keyed by the chosen `variant` (e.g. default image for `cpu`, `cuda`, `rocm`).
When policy does not specify an image for the variant, the orchestrator MAY omit `inference_backend.image` so the node uses its node-local default (e.g. from node startup YAML or environment).

Image selection MUST be deterministic for the same (variant, policy) pair.

## Traceability

- **REQ-ORCHES-0149:** [orches.md](../requirements/orches.md#req-orches-0149).
  Orchestrator returns node config that instructs whether and how to start the local inference backend.
  Inference backend instructions are derived from the node capability report.
- **Configuration Delivery:** [worker_node.md#spec-cynai-worker-configurationdelivery](../tech_specs/worker_node.md#spec-cynai-worker-configurationdelivery).
  Orchestrator derives inference backend instruction from capability report and policy.
  It includes it when the node is inference-capable and inference is enabled.
- **Payload schema:** [worker_node_payloads.md#spec-cynai-worker-payload-configuration-v1](../tech_specs/worker_node_payloads.md#spec-cynai-worker-payload-configuration-v1).
  `inference_backend` structure and semantics.
