# CPU-Only and Non-GPU Nodes

- [1 Summary](#1-summary)
- [2 When Nodes Run Without Discrete GPU Acceleration](#2-when-nodes-run-without-discrete-gpu-acceleration)
- [3 Intel GPUs (MVP Policy)](#3-intel-gpus-mvp-policy)
- [4 Resource Planning](#4-resource-planning)
- [5 Traceability](#5-traceability)

## 1 Summary

CyNodeAI supports worker nodes that run local inference **without** NVIDIA or AMD discrete GPU acceleration.
These nodes rely on **CPU** execution (and whatever acceleration the inference runtime provides on CPU).

This document clarifies how **non-GPU** and **Intel-only GPU** hosts fit MVP expectations.

**Role:** [Worker node hosts](worker_node.md).
**Containers and shared tooling:** [Common host requirements](common.md#2-container-runtime); **worker resources:** [Worker node hosts, section 5](worker_node.md#5-host-resources).

## 2 When Nodes Run Without Discrete GPU Acceleration

Typical cases include:

- **No GPU present** or GPU not exposed to the runtime.
- **CPU-only policy** by design (small footprint hosts, CI-like workers, or air-gapped CPU clusters).

Capacity and latency differ sharply from GPU-backed inference; size RAM and CPU cores against model choice and concurrent jobs.

## 3 Intel GPUs (MVP Policy)

**Intel GPU-specific** inference backend variants and Intel-specific orchestrator selection are **deferred until post-MVP**.

### 3.1 For Mvp

- Nodes **MUST NOT** use an Intel-specific inference backend variant.
- If **only Intel GPUs** are present, the orchestrator **SHALL** treat the node as **CPU for inference** (or follow "do not start" policy where requirements specify it).

Plan Intel-GPU hosts as **CPU inference nodes** until post-MVP Intel support is defined in requirements and tech specs.

## 4 Resource Planning

- **RAM:** CPU inference often needs substantial system memory for larger models; undersized hosts fail with OOM or extreme swapping.
- **CPU:** Throughput scales with cores and memory bandwidth up to a point; linear scaling is not guaranteed for all models and runtimes.
- **Disk:** Same as GPU nodes: model artifacts and images still consume storage.

## 5 Traceability

- Intel deferral and CPU treatment: [`docs/requirements/orches.md`](../requirements/orches.md) (REQ-ORCHES-0175) and [`docs/requirements/worker.md`](../requirements/worker.md) (REQ-WORKER-0266).
