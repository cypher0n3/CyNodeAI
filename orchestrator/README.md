# Orchestrator Services

- [1 Overview](#1-overview)
- [2 What This Directory Contains](#2-what-this-directory-contains)
- [3 How To Run For Development](#3-how-to-run-for-development)
- [4 Testing And Linting](#4-testing-and-linting)
- [5 Cross-References](#5-cross-references)

## 1 Overview

This directory contains the Go implementation of the CyNodeAI orchestrator services.
The orchestrator owns system state such as users, authentication, task and job state, and worker node registration.
The orchestrator persists state in PostgreSQL and exposes REST APIs consumed by users and worker nodes.

For system-level requirements and normative behavior, start with the technical specs index at [`docs/tech_specs/_main.md`](../docs/tech_specs/_main.md).

## 2 What This Directory Contains

This directory is a standalone Go module defined by [`go.mod`](go.mod).

- [`cmd/`](cmd/): Entrypoints for the orchestrator services.
  - [`cmd/user-gateway/`](cmd/user-gateway/): User-facing REST API gateway.
  - [`cmd/control-plane/`](cmd/control-plane/): Control plane API and dispatcher for node and job coordination.
  - [`cmd/mcp-gateway/`](cmd/mcp-gateway/): Optional gateway for MCP tool access.
  - [`cmd/api-egress/`](cmd/api-egress/): Optional API egress proxy service.
- [`internal/`](internal/): Private implementation packages (handlers, middleware, auth, database, models, config).
- [`migrations/`](migrations/): SQL migrations applied to the orchestrator database.
- [`docker-compose.yml`](docker-compose.yml): Development compose stack for Postgres and orchestrator services.
- [`systemd/`](systemd/): Service definitions and notes for running on a host.

Notable services under [`cmd/`](cmd/) include the user-facing API gateway, the control plane, and optional gateways for MCP and egress.

## 3 How to Run for Development

Prefer repo-level tooling in the root [`justfile`](../justfile) so your workflow stays consistent across modules.

### 3.1 Run the Full Happy Path

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

This module also includes integration-style tests under [`internal/handlers/`](internal/handlers/) and database tests under [`internal/database/`](internal/database/).

## 5 Cross-References

- Root project overview at [README.md](../README.md).
- Project meta at [meta.md](../meta.md).
- Technical specifications index at [docs/tech_specs/_main.md](../docs/tech_specs/_main.md).
- Go REST API standards at [docs/tech_specs/go_rest_api_standards.md](../docs/tech_specs/go_rest_api_standards.md).
- Worker node implementation at [worker_node/README.md](../worker_node/README.md).
- CLI management client at [cynork/README.md](../cynork/README.md).
- Shared contracts at [go_shared_libs/README.md](../go_shared_libs/README.md).
