# System Requirements

- [1 Purpose](#1-purpose)
- [2 Scope](#2-scope)
- [3 Document Index](#3-document-index)
- [4 Cross-Cutting Expectations](#4-cross-cutting-expectations)
- [5 Related Canonical Docs](#5-related-canonical-docs)

## 1 Purpose

This directory collects **practical system and environment guidance** for running CyNodeAI components on common hardware profiles.

It complements normative requirements in [`docs/requirements/`](../requirements/README.md) and implementation detail in [`docs/tech_specs/`](../tech_specs/README.md).
Use it for onboarding, capacity planning, and troubleshooting host setup (drivers, containers, inference backends).

## 2 Scope

- **In scope:** Host OS assumptions, **orchestrator** vs **worker** roles, GPU vendor notes, driver and container-runtime expectations, and how those choices map to worker inference backend variants described in requirements.
- **Out of scope:** Product feature lists, API contracts, and security policy (see requirements and tech specs instead).

MVP GPU support for node-local inference backend selection is **NVIDIA (CUDA)** and **AMD (ROCm)** per orchestrator and worker requirements.
**Intel** GPU-specific inference variants are **post-MVP**; until then, nodes with only Intel GPUs are treated like CPU-only nodes for inference policy.
See [CPU-only and non-GPU nodes](cpu_only.md).

## 3 Document Index

The sections below group pages by **role** (orchestrator vs worker), **shared** tooling, and **worker** GPU or CPU topics.

### 3.1 Orchestrator and Worker Host Pages

- [Orchestrator backend hosts](orchestrator.md) - Postgres, control-plane, user-gateway, optional Ollama in the dev stack; **orchestrator-side** variant and placement policy.
- [Worker node hosts](worker_node.md) - node-manager, Worker API, local inference; **worker-side** capability reporting and resources.

### 3.2 Shared Host Requirements

- [Common host requirements](common.md) - **container runtime** (Podman, Compose) shared by local dev stacks; **document map** to the rest of this folder.

### 3.3 Worker GPU and CPU Topics

- [NVIDIA GPU systems](nvidia.md) - CUDA stack, **`nvidia-smi`**, NVIDIA Container Toolkit, and the `cuda` inference variant.
- [AMD GPU systems](amd.md) - ROCm, **`rocm-smi`**, AMDGPU firmware, and the `rocm` inference variant.
- [CPU-only and non-GPU nodes](cpu_only.md) - CPU inference, Intel-only hosts, and resource expectations without discrete GPU acceleration.

## 4 Cross-Cutting Expectations

- **Orchestrator vs worker:** Read [Orchestrator backend hosts](orchestrator.md) for control-plane hosts and [Worker node hosts](worker_node.md) for execution and inference hosts.
- **Containers:** Podman-first Compose expectations live in [Common host requirements](common.md#2-container-runtime).

## 5 Related Canonical Docs

- **Development stack and tests:** [`development_setup.md`](../development_setup.md).
- **Worker capability and GPU reporting:** [`docs/requirements/worker.md`](../requirements/worker.md).
- **Orchestrator variant selection:** [`docs/requirements/orches.md`](../requirements/orches.md).
