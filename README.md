# CyNodeAI

[![License: Apache 2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Go Workspace](https://img.shields.io/badge/Go-workspace-00ADD8?logo=go)](go.work)
[![Docs](https://img.shields.io/badge/docs-tech%20specs-blueviolet)](docs/README.md)

## Overview

CyNodeAI is a local-first multi-agent orchestrator for self-hosted teams and small enterprises.
It coordinates sandboxed worker execution across local nodes and optional cloud capacity, with centralized task, state, and policy management.

The system combines a central orchestrator, worker nodes, sandboxed execution, MCP-based tooling, and optional controlled access to external AI providers.
It is designed to keep credentials, policy decisions, and audit trails on the trusted side of the system instead of inside execution sandboxes.

> [!IMPORTANT]
> This repository is still in an early prototype and design phase.
> The canonical normative "what" lives in [docs/requirements/](docs/requirements/README.md), and the implementation guidance "how" lives in [docs/tech_specs/](docs/tech_specs/_main.md).

## Highlights

- `🏠 Local-first`: Run fully on your own infrastructure, with optional cloud-based worker capacity.
- `🧱 Sandboxed execution`: Scripts and commands run only in isolated containers under worker-node control.
- `🔐 Security-first`: Default-deny policy, credential isolation, auditable actions, and controlled egress.
- `🧰 MCP-first tooling`: Agents use MCP as the standard interface for privileged tools and system access.
- `🤖 Multi-agent orchestration`: Coordinate orchestrator-side agents, worker-side agents, and node-local execution.
- `🌐 OpenAI-compatible chat`: Expose `/v1/models` and `/v1/chat/completions` for tools such as Open WebUI and `cynork`.

## Current Status

MVP Phases 1, 1.5, and 1.7 are complete for the current single-node product slice, including single-node execution, sandbox inference, the basic `cynork` CLI, Project Manager Agent (PMA) integration, and SandBox Agent (SBA) runner support.
Phase 2 is in progress, with MCP gateway scoping/auditing, preference tools, and workflow/SBA foundations already landed.

For the roadmap, see [docs/mvp.md](docs/mvp.md) and [docs/mvp_plan.md](docs/mvp_plan.md).

## Quick Start

Install [`just`](https://github.com/casey/just), then use the repo [justfile](justfile) for all common workflows.

```bash
just setup
just setup-dev full-demo --stop-on-success
just docs-check README.md
just ci
```

Useful commands:

- `just setup`: Install Podman, Go, markdown tooling, and other local prerequisites.
- `just setup-dev <command>`: Run the Python-based local dev workflow such as `start`, `stop`, `build`, `test-e2e`, or `full-demo`.
- `just e2e`: Run the Python E2E suite after the stack is already running.
- `just docs-check <path>`: Run docs-only validation for changed Markdown.
- `just lint`: Run the repo lint suite.
- `just test`: Run the repo test suite.
- `just go-fmt`: Format Go code across workspace modules.
- `just ci`: Run the full local CI suite.

For the local development stack and ports, see [docs/development_setup.md](docs/development_setup.md).

## Repository Map

Primary workspace modules from [go.work](go.work):

- [agents/](agents/) - Go module for the CyNode PMA and CyNode SBA binaries plus bundled instruction sets.
- [cynork/](cynork/) - Go CLI management client built with Cobra.
- [e2e/](e2e/) - Go BDD support module used by the repository test harness.
- [go_shared_libs/](go_shared_libs/) - Shared Go contracts and types used across services.
- [orchestrator/](orchestrator/) - Control-plane services such as the control plane, user gateway, MCP gateway, and API egress.
- [worker_node/](worker_node/) - Node manager, worker API, and inference proxy services.

Other important directories:

- [docs/](docs/README.md) - Main documentation hub.
- [features/](features/README.md) - Gherkin acceptance specs and BDD traceability.
- [scripts/](scripts/README.md) - Python dev setup and E2E support tooling.
- [docs/examples/](docs/examples/README.md) - Example bootstrap configuration files.

## Architecture Snapshot

This section summarizes the major runtime layers and service boundaries in the current design.

### Orchestrator Layer

- `🧠` Owns task state, user task-execution preferences, audit logs, and model metadata.
- `🗄️` Persists state in PostgreSQL with `pgvector`.
- `🚪` Exposes user-facing and worker-facing Go APIs.
- `🛡️` Enforces policy, access control, and controlled service boundaries.

See [docs/tech_specs/orchestrator.md](docs/tech_specs/orchestrator.md).

### Worker Nodes

- `⚙️` Register with the orchestrator and report capabilities.
- `📦` Run sandbox containers for agent work and optional local inference.
- `🔌` Host the worker API, node manager, and inference proxy components.
- `📡` Return results, logs, and status back to the orchestrator.

See [docs/tech_specs/worker_node.md](docs/tech_specs/worker_node.md).

### Agents and Tooling

- `🧭` The Project Manager Agent coordinates work using stored user preferences and project context.
- `🧪` The Sandbox Agent executes task steps inside restricted containers.
- `🛠️` MCP is the standard tool interface for orchestrator-side and worker-side agents.

See [docs/tech_specs/project_manager_agent.md](docs/tech_specs/project_manager_agent.md), [docs/tech_specs/cynode_pma.md](docs/tech_specs/cynode_pma.md), [docs/tech_specs/cynode_sba.md](docs/tech_specs/cynode_sba.md), and [docs/tech_specs/mcp_tooling.md](docs/tech_specs/mcp_tooling.md).

### Controlled Egress

- `🌍` External API access flows through the API Egress Server.
- `🧼` Web access flows through the Secure Browser Service, which sanitizes content before agents consume it.
- `🔑` Credentials stay out of sandboxes and are managed centrally.

See [docs/tech_specs/api_egress_server.md](docs/tech_specs/api_egress_server.md) and [docs/tech_specs/secure_browser_service.md](docs/tech_specs/secure_browser_service.md).

## Security Model

- Plain-text and Markdown prompts use inference by default.
- Script and shell execution are explicit opt-in behaviors that run only inside isolated sandboxes.
- Worker sandboxes are untrusted and network-restricted by default.
- Policy evaluation and tool use are auditable.
- API credentials must not be exposed to agents or sandboxes.

For details, see [docs/tech_specs/access_control.md](docs/tech_specs/access_control.md), [docs/tech_specs/api_egress_server.md](docs/tech_specs/api_egress_server.md), and [docs/tech_specs/secure_browser_service.md](docs/tech_specs/secure_browser_service.md).

## Documentation

- [docs/README.md](docs/README.md) - Entry point for project documentation.
- [docs/requirements/README.md](docs/requirements/README.md) - Canonical requirements.
- [docs/tech_specs/_main.md](docs/tech_specs/_main.md) - Technical specifications index.
- [docs/openwebui_cynodeai_integration.md](docs/openwebui_cynodeai_integration.md) - Open WebUI integration guide.
- [meta.md](meta.md) - Project summary, repository layout, and contributor expectations.

AI-assisted coding workflows must follow [ai_files/ai_coding_instructions.md](ai_files/ai_coding_instructions.md).

## License and Contributing

- CyNodeAI is licensed under [Apache 2.0](LICENSE).
- See [CONTRIBUTING.md](CONTRIBUTING.md) for contribution guidance.
