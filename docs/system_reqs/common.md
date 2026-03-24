# Common Host Requirements

- [1 Overview](#1-overview)
- [2 Container Runtime](#2-container-runtime)
- [3 Document Map](#3-document-map)

## 1 Overview

CyNodeAI splits **host** and **runtime** expectations by **role**:

- **Orchestrator backend** (control plane, user gateway, Postgres, and related services): see [Orchestrator backend hosts](orchestrator.md).
- **Worker nodes** (node-manager, Worker API, local inference backends): see [Worker node hosts](worker_node.md).

This file keeps **one** copy of **container engine** expectations (Podman, Compose) that apply when you run either stack with Compose locally.
Vendor GPU stacks (**NVIDIA**, **AMD**, CPU-only) apply to **workers** that run local inference; see [Worker node hosts](worker_node.md) and the linked vendor pages.

## 2 Container Runtime

CyNodeAI stacks use [**Compose**](https://docs.docker.com/compose/) with **Podman** **preferred** and **Docker** as an alternative.
See [`development_setup.md`](../development_setup.md) for how the repository runs Compose locally (orchestrator stack under `orchestrator/docker-compose.yml`, worker patterns in the same guide).

Install **Podman** and a Compose implementation (built-in **`podman compose`** where available, or **`podman-compose`**), or **Docker** with **`docker-compose-plugin`** if your environment standardizes on Docker.

- **Arch Linux:** **`podman`** (and **`podman-compose`** if needed), or **`docker`** and **`docker-compose-plugin`**.
- **Debian / Ubuntu:** **`podman`**, then use **`podman compose`** (built into Podman 4+) or install **`podman-compose`** if your release splits it out, or use **`docker.io`** and **`docker-compose-plugin`** for Docker Compose.
- **Fedora:** **`podman`** with **`podman compose`**, or **`docker`** and **`docker-compose-plugin`**.

**GPU-backed inference** on workers still needs the **vendor stack** on the host (drivers, userspace, container device access); see [Worker node hosts](worker_node.md) and [NVIDIA GPU systems](nvidia.md) or [AMD GPU systems](amd.md).

## 3 Document Map

- [Orchestrator backend hosts](orchestrator.md) - control-plane stack sizing, network, and orchestrator-side placement or variant policy.
- [Worker node hosts](worker_node.md) - inference backends, capability reporting, and worker-side resources.
- [NVIDIA GPU systems](nvidia.md) and [AMD GPU systems](amd.md) - vendor drivers, **`nvidia-smi`** / **`rocm-smi`**, and toolkits for GPU workers.
- [CPU-only and non-GPU nodes](cpu_only.md) - CPU inference and Intel GPU MVP policy.
