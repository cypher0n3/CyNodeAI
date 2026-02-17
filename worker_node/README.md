# Worker Node Services

- [1 Overview](#1-overview)
- [2 What This Directory Contains](#2-what-this-directory-contains)
- [3 How To Run For Development](#3-how-to-run-for-development)
- [4 Testing And Linting](#4-testing-and-linting)
- [5 Cross-References](#5-cross-references)

## 1 Overview

This directory contains the Go implementation of CyNodeAI worker node services.
Worker nodes register with the orchestrator, report capabilities, accept jobs, execute sandboxed work, and report results.

At a high level, the worker side is split into a node manager (registration and orchestration-facing control) and a worker API (job execution surface).

## 2 What This Directory Contains

This directory is a standalone Go module defined by [`go.mod`](go.mod).

- [`cmd/`](cmd/): Entrypoints for worker node services.
  - [`cmd/node-manager/`](cmd/node-manager/): The node manager service entrypoint and container definition.
  - [`cmd/worker-api/`](cmd/worker-api/): The worker API service entrypoint, container definition, and executor package.
- [`docker-compose.yml`](docker-compose.yml): Development compose stack for local worker services.
- [`systemd/`](systemd/): Service definitions and notes for running on a host.

This module depends on shared contracts in [`go_shared_libs/`](../go_shared_libs/) via a local replace in [`go.mod`](go.mod).

## 3 How to Run for Development

Prefer repo-level tooling in the root [`justfile`](../justfile) so your workflow stays consistent across modules.

### 3.1 Run With the Orchestrator End-to-End Flow

Use the repo-level end-to-end recipe to start Postgres, the orchestrator, and a worker node, then run a basic happy path.
Run `just e2e` from the repository root.

### 3.2 Run With the Local Compose Stack

This directory includes a compose file for local development at [`docker-compose.yml`](docker-compose.yml).
Configuration is provided via environment variables documented inline in that compose file.

## 4 Testing and Linting

All Go modules in this repository are checked by repo-level `just` targets.

- Run `just test-go` to run Go tests across all modules.
- Run `just lint-go` or `just lint-go-ci` to run Go lint checks across all modules.
- Run `just ci` to run the local CI suite (lint, tests with coverage, and vulnerability checks).

When developing the worker executor, also verify behavior against orchestrator flows (for example via `just e2e`).

## 5 Cross-References

- Root project overview at [`README.md`](../README.md).
- Orchestrator implementation at [`orchestrator/README.md`](../orchestrator/README.md).
- Shared contracts at [`go_shared_libs/README.md`](../go_shared_libs/README.md).
- Technical specifications index at [`docs/tech_specs/_main.md`](../docs/tech_specs/_main.md).
