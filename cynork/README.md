# Cynork CLI

- [1 Overview](#1-overview)
- [2 What This Directory Contains](#2-what-this-directory-contains)
- [3 How to Run for Development](#3-how-to-run-for-development)
- [4 Testing and Linting](#4-testing-and-linting)
- [5 Cross-References](#5-cross-references)

## 1 Overview

This directory contains the CyNodeAI CLI management client (**cynork**), implemented in Go with Cobra.
`cynork` operates against the User API Gateway for authentication, task operations, and admin capabilities (credentials, user preferences, nodes, skills).
It MUST offer the same administrative capabilities as the Admin Web Console; see [CLI management app spec](../docs/tech_specs/cli_management_app.md).

## 2 What This Directory Contains

This directory is a standalone Go module defined by [`go.mod`](go.mod).

- [`cmd/`](cmd/): Cobra command definitions (root, auth, task, status, version).
- [`internal/config/`](internal/config/): Config loading, env overrides, validation.
- [`internal/gateway/`](internal/gateway/): Typed gateway client and auth for the User API Gateway.

Cynork does not depend on [go_shared_libs](../go_shared_libs/); it talks to the orchestrator via the public User API Gateway.

## 3 How to Run for Development

Prefer repo-level tooling in the root [`justfile`](../justfile).

- Build from repo root: `cd cynork && go build -o cynork .` (or build from root with appropriate module path).
- Run against a local User API Gateway (e.g. after `just e2e` or running the orchestrator compose stack).
- Configuration: config file (default `~/.config/cynork/config.yaml`) and environment overrides; see [CLI management app spec - Authentication and Configuration](../docs/tech_specs/cli_management_app.md#authentication-and-configuration).

## 4 Testing and Linting

All Go modules in this repository are checked by repo-level `just` targets.

- Run `just test-go` to run Go tests across all modules.
- Run `just lint-go` or `just lint-go-ci` to run Go lint checks across all modules.
- Run `just ci` to run the local CI suite (lint, tests with coverage, and vulnerability checks).

## 5 Cross-References

- Root project overview at [README.md](../README.md).
- Project meta and repository layout at [meta.md](../meta.md).
- CLI specification at [docs/tech_specs/cli_management_app.md](../docs/tech_specs/cli_management_app.md).
- Technical specifications index at [docs/tech_specs/_main.md](../docs/tech_specs/_main.md).
- Orchestrator (User API Gateway) at [orchestrator/README.md](../orchestrator/README.md).
- Worker node at [worker_node/README.md](../worker_node/README.md).
- Shared contracts at [go_shared_libs/README.md](../go_shared_libs/README.md).
