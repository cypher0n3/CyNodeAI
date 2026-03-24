# Worker Node Services

- [1 Overview](#1-overview)
- [2 What This Directory Contains](#2-what-this-directory-contains)
- [3 How To Run For Development](#3-how-to-run-for-development)
- [4 Testing And Linting](#4-testing-and-linting)
- [5 Cross-References](#5-cross-references)

## 1 Overview

This directory contains the Go implementation of CyNodeAI worker node services.
Worker nodes register with the orchestrator, report capabilities, accept jobs, execute sandboxed work, and report results.

The worker node runs as a **single process** (one binary, `cynodeai-wnm`): the node manager and Worker API run together.
There is no separate Worker API process or container.

## 2 What This Directory Contains

This directory is a standalone Go module defined by [`go.mod`](go.mod).

- [`cmd/`](cmd/): Entrypoints for worker node services.
  - [`cmd/node-manager/`](cmd/node-manager/): The **single binary** (`cynodeai-wnm`) that runs both the node manager and the Worker API in one process.
  - [`cmd/worker-api/`](cmd/worker-api/): Standalone Worker API entrypoint (used for tests or legacy compose; not the supported deployment topology).
  - [`cmd/inference-proxy/`](cmd/inference-proxy/): The inference proxy service for sandbox-side inference routing.
- [`docker-compose.yml`](docker-compose.yml): Development compose stack for **worker-api only** (e.g. telemetry/job endpoints in isolation).
  For full node behavior (registration, config fetch, PMA, sandbox), run the single binary on the host via `just setup-dev start` from the repo root.
- [`systemd/`](systemd/): Service definitions and notes for running on a host.

This module depends on shared contracts in [`go_shared_libs/`](../go_shared_libs/) via a local replace in [`go.mod`](go.mod).

## 3 How to Run for Development

Prefer repo-level tooling in the root [`justfile`](../justfile) so your workflow stays consistent across modules.

### 3.1 Run With the Orchestrator End-To-End Flow

Use the repo-level end-to-end recipe to start Postgres, the orchestrator, and a worker node, then run a basic happy path.
Run `just e2e` from the repository root.

### 3.2 Run With the Local Compose Stack

This directory includes a compose file at [`docker-compose.yml`](docker-compose.yml) that runs **worker-api only** (e.g. for telemetry or job endpoints in isolation).
For full node behavior (registration, config fetch, PMA, sandbox), run the **single binary** on the host via `just setup-dev start` from the repo root; it runs both the node manager and Worker API in one process and manages podman/docker for PMA and sandboxes.
Configuration is via environment variables documented inline in the compose file.

### 3.3 GPU and Ollama Image (ROCm vs CUDA)

Node-manager detects GPUs only via **`rocm-smi`** (AMD) and **`nvidia-smi`** (NVIDIA) in `PATH`.
If `rocm-smi` is missing or exits non-zero, **no AMD devices** are reported and the orchestrator cannot select the ROCm Ollama image from capability data.

To see raw tool output and the merged detection result the node sends in capability reports, run the node-manager binary with:

```bash
./worker_node/bin/cynodeai-wnm-dev --print-gpu-detect
```

(Adjust the path to match your build; see `worker_node/justfile`.) Use the JSON to confirm whether the host reports AMD VRAM before debugging orchestrator payloads or `inference_backend.variant` handling.

## 4 Testing and Linting

All Go modules in this repository are checked by repo-level `just` targets.

- Run `just test-go` to run Go tests across all modules.
- Run `just lint-go` or `just lint-go-ci` to run Go lint checks across all modules.
- Run `just ci` to run the local CI suite (lint, tests with coverage, and vulnerability checks).

When developing the worker executor, also verify behavior against orchestrator flows (for example via `just e2e`).

## 5 Cross-References

- Root project overview at [README.md](../README.md).
- Documentation index at [docs/README.md](../docs/README.md).
- Project meta at [meta.md](../meta.md).
- Orchestrator implementation at [orchestrator/README.md](../orchestrator/README.md).
- CLI management client at [cynork/README.md](../cynork/README.md).
- Shared contracts at [go_shared_libs/README.md](../go_shared_libs/README.md).
- Technical specifications index at [docs/tech_specs/_main.md](../docs/tech_specs/_main.md).
