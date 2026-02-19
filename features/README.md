# Feature Specifications

- [1 Overview](#1-overview)
- [2 What This Directory Contains](#2-what-this-directory-contains)
- [3 How to Use These Features](#3-how-to-use-these-features)
  - [3.1 Suite Tags (Component Ownership)](#31-suite-tags-component-ownership)
  - [3.2 Traceability Tags](#32-traceability-tags)
- [4 Testing and Validation](#4-testing-and-validation)
- [5 Cross-References](#5-cross-references)

## 1 Overview

This directory contains Gherkin `.feature` files that describe acceptance-level behavior for CyNodeAI.
These files are intended to be readable by humans and usable as executable specifications when a BDD runner is wired into the repo.

Treat feature files as a high-level contract for system behavior rather than as implementation notes.

## 2 What This Directory Contains

Feature files in this directory describe acceptance behavior.
Each feature is tagged to a single suite so BDD can run per major component.

Feature files are organized by suite directory:

- `features/orchestrator/`: Orchestrator suite features.
- `features/worker_node/`: Worker-node suite features.
- `features/e2e/`: End-to-end suite features spanning multiple major components.

## 3 How to Use These Features

Use these files as a reference when implementing endpoints, workflows, and integration behavior.
Keep scenarios aligned with the technical specs and with the actual implementation.

If a scenario becomes outdated, update the feature file and the corresponding tests together.

### 3.1 Suite Tags (Component Ownership)

Each feature file MUST declare exactly one `@suite_*` tag immediately above the `Feature:` line.
Suites exist even if there is no current Godog runner yet.
This keeps feature files scoped to a single major component and allows per-component BDD execution.

Suite directory rule:

- A feature file tagged `@suite_<name>` MUST live under `features/<name>/`.
  Example: `@suite_orchestrator` files live under `features/orchestrator/`.

Suite tag registry:

- `@suite_orchestrator`: Orchestrator suite (control-plane and user-gateway behavior).
- `@suite_worker_node`: Worker-node suite (node manager and worker API behavior).
- `@suite_cli_management_app`: CLI management app suite.
- `@suite_admin_web_console`: Admin web console suite.
- `@suite_api_egress_server`: API Egress Server suite.
- `@suite_secure_browser_service`: Secure Browser Service suite.
- `@suite_mcp_gateway`: MCP gateway suite.
- `@suite_e2e`: End-to-end suite spanning multiple major components.

### 3.2 Traceability Tags

Each `Scenario` SHOULD include:

- A requirement tag: `@req_<domain>_<nnnn>` derived from `REQ-<DOMAIN>-<NNNN>` (lowercased).
- A spec tag: `@spec_<spec_id>` derived from a Spec ID (dots replaced with underscores, lowercased).
- A suite tag at the Feature level: one `@suite_*` tag from the registry above.

Example:

- `REQ-IDENTY-0104` => `@req_identy_0104`
- `CYNAI.IDENTY.AuthenticationModel` => `@spec_cynai_identy_authenticationmodel`

## 4 Testing and Validation

This repository currently validates behavior primarily through Go tests and end-to-end developer tooling.

- Run `just test-go` to run Go tests across all modules.
- Run `just e2e` to run the repository happy path that exercises orchestrator and worker node behavior.

Feature scenarios may also be reflected in orchestrator integration tests under [`orchestrator/internal/handlers/`](../orchestrator/internal/handlers/).

## 5 Cross-References

- Root project overview at [`README.md`](../README.md).
- Technical specifications index at [`docs/tech_specs/_main.md`](../docs/tech_specs/_main.md).
- Orchestrator implementation at [`orchestrator/README.md`](../orchestrator/README.md).
- Worker node implementation at [`worker_node/README.md`](../worker_node/README.md).
