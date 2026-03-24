# Worker Node Host Requirements

- [1 Overview](#1-overview)
- [2 Container Runtime](#2-container-runtime)
- [3 Inference Backends and GPU Stacks](#3-inference-backends-and-gpu-stacks)
- [4 Capability Reporting (Worker Obligations)](#4-capability-reporting-worker-obligations)
- [5 Host Resources](#5-host-resources)
- [6 Traceability](#6-traceability)

## 1 Overview

This document describes **host and environment** expectations for **worker nodes**: **node-manager**, **Worker API**, and **local inference** (for example Ollama) typically running in **containers** on the worker host.

It does **not** describe the orchestrator control plane; see [Orchestrator backend hosts](orchestrator.md).

Normative behavior remains in [`docs/requirements/`](../requirements/README.md) (worker domain) and [`docs/tech_specs/`](../tech_specs/README.md).

## 2 Container Runtime

Workers use the same **Podman**-first, **Compose**-capable stack as the rest of the project when you run services in containers: [Common host requirements, section 2](common.md#2-container-runtime).

**GPU-backed inference** requires additional host software (drivers, **`nvidia-smi`** or **`rocm-smi`**, and container GPU access); follow [section 3](#3-inference-backends-and-gpu-stacks).

Local dev patterns: [`development_setup.md`](../development_setup.md) (worker node and node-manager sections).

## 3 Inference Backends and GPU Stacks

- **NVIDIA (CUDA):** [NVIDIA GPU systems](nvidia.md) - **`nvidia-smi`**, proprietary driver, **NVIDIA Container Toolkit**, `cuda` variant notes.
- **AMD (ROCm):** [AMD GPU systems](amd.md) - **`rocm-smi`**, AMDGPU firmware, ROCm userspace, `rocm` variant notes.
- **CPU-only and Intel-only (MVP):** [CPU-only and non-GPU nodes](cpu_only.md).

The orchestrator chooses **which** backend variant applies from **reported capabilities**; the worker must **start** the backend only under orchestrator-supplied configuration (see [section 4](#4-capability-reporting-worker-obligations)).

## 4 Capability Reporting (Worker Obligations)

Worker nodes **MUST** include **all** supported GPU devices from all supported vendors in capability data, each with **`vendor`** and **`vram_mb`**, so the orchestrator can compute **total VRAM per vendor** and select the correct inference backend variant.

When **multiple GPU vendors** are present, omitting devices would make orchestrator placement incorrect.

Workers **MUST NOT** override orchestrator-supplied inference backend variant or image with a conflicting node-local default (for example a single **`OLLAMA_IMAGE`** that contradicts the assigned variant).

If a host mixes **NVIDIA** and **AMD** GPUs, treat runtime behavior as driven by **reported capabilities** and orchestrator policy, not by a single undisciplined local environment variable.

**Orchestrator-side** rules for how those reports are turned into placement and variant selection: [Orchestrator backend hosts, section 5](orchestrator.md#5-inference-variant-and-placement-orchestrator-policy).

## 5 Host Resources

- **Disk:** model weights, container images, and logs; plan for pull and cache size.
- **Memory:** RAM for CPU inference; **VRAM** when using discrete GPUs (size to model footprint).
- **CPU:** throughput for CPU-only inference paths.
- **Network:** the worker must reach the **orchestrator** (registration, configuration) and honor egress policy for any approved external endpoints.

## 6 Traceability

- Worker capability reporting, inference backend startup, and GPU vendor rules: [`docs/requirements/worker.md`](../requirements/worker.md) (for example REQ-WORKER-0265, REQ-WORKER-0266, and inference backend sections).
