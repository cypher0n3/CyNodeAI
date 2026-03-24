# Orchestrator Backend Host Requirements

- [1 Overview](#1-overview)
- [2 Container Runtime](#2-container-runtime)
- [3 Host Resources](#3-host-resources)
- [4 Network](#4-network)
- [5 Inference Variant and Placement (Orchestrator Policy)](#5-inference-variant-and-placement-orchestrator-policy)
- [6 Traceability](#6-traceability)

## 1 Overview

This document describes **host and environment** expectations for machines that run the **orchestrator backend**: Postgres, **control-plane**, **user-gateway**, and (in the default dev stack) **Ollama** alongside them.

It does **not** describe worker nodes that execute jobs or host local inference; see [Worker node hosts](worker_node.md).

Normative behavior remains in [`docs/requirements/`](../requirements/README.md) (orchestrator domain) and [`docs/tech_specs/`](../tech_specs/README.md).

## 2 Container Runtime

Local development uses **Compose** for the orchestrator stack (`orchestrator/docker-compose.yml`).

**Podman** or **Docker** plus Compose: [Common host requirements, section 2](common.md#2-container-runtime).

Full walkthrough: [`development_setup.md`](../development_setup.md) (orchestrator stack section).

## 3 Host Resources

Size the host for **Postgres** (data and **pgvector** workloads), **control-plane** and **user-gateway** processes, migrations, and container images for those services plus optional **Ollama** when it runs in the same stack.

- **Disk:** database growth, image layers, and logs; plan retention and backups per your policy.
- **Memory:** sufficient RAM for Postgres working set and Go services under expected concurrency.
- **CPU:** enough cores for control-plane work, user-gateway traffic, and database load.

Exact ports and defaults: [`ports_and_endpoints.md`](../tech_specs/ports_and_endpoints.md).

## 4 Network

The orchestrator **accepts** client and node traffic on documented listen addresses and must reach **worker nodes** (and optional external endpoints) per deployment policy.

Orchestrator services must be reachable where your design places the user API, node registration, and internal dependencies (for example Postgres only on trusted networks).

## 5 Inference Variant and Placement (Orchestrator Policy)

The **orchestrator** decides **inference backend** instructions for a registered node (for example container image and **CUDA** vs **ROCm** variant) from **node capability reports** and **model** constraints.

When a node reports **multiple GPU vendors**, the orchestrator uses **total VRAM per vendor** to choose the variant for the vendor with the largest aggregate VRAM.

The orchestrator selects variants using **VRAM and model** needs, not vendor alone; see requirements for the full rule.

## 6 Traceability

- Orchestrator MVP GPU vendor rules and placement: [`docs/requirements/orches.md`](../requirements/orches.md) (for example REQ-ORCHES-0175 and related inference configuration sections).
