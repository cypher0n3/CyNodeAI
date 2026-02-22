# Ports and Endpoints

- [Document Overview](#document-overview)
- [Default Port Assignments](#default-port-assignments)
- [Orchestrator Stack](#orchestrator-stack)
- [Worker Node](#worker-node)
- [Inference (Ollama and Proxy)](#inference-ollama-and-proxy)
- [CLI (Cynork)](#cli-cynork)
- [Conflict Avoidance](#conflict-avoidance)
- [E2E and BDD](#e2e-and-bdd)
- [Environment and Config Overrides](#environment-and-config-overrides)

## Document Overview

This document consolidates default ports and endpoint URLs for all CyNodeAI components.
It is the single reference for avoiding port conflicts in local and containerized deployments.

Implementations MUST use the defaults below unless overridden by environment or configuration.
Overrides are summarized at the end.

See [`docs/docs_standards/spec_authoring_writing_and_validation.md`](../docs_standards/spec_authoring_writing_and_validation.md) for spec authoring conventions.

## Default Port Assignments

- Spec ID: `CYNAI.STANDS.PortsAndEndpoints` <a id="spec-cynai-stands-portsandendpoints"></a>

Traces To:

- [REQ-WORKER-0114](../requirements/worker.md#req-worker-0114)
- [REQ-WORKER-0115](../requirements/worker.md#req-worker-0115)
- [REQ-SANDBX-0106](../requirements/sandbx.md#req-sandbx-0106)

- **5432** - PostgreSQL - Database (orchestrator)
- **8080** - User API Gateway - Auth, users, tasks, results (orchestrator)
- **8081** - Worker API - Run jobs on the node (worker node)
- **8082** - Control-plane - Node registration, dispatch, migrations (orchestrator)
- **8083** - MCP Gateway - MCP tool routing (orchestrator; optional profile)
- **8084** - API Egress - External API calls with credentials (orchestrator; optional profile)
- **11434** - Ollama - Inference API (orchestrator or node); also inference proxy listen port inside sandbox pods

No other CyNodeAI service defaults use these ports.
The CLI (Cynork) does not listen on any port.
It connects to the User API Gateway (default `http://localhost:8080`).

## Orchestrator Stack

- **PostgreSQL:** `5432` (container internal and host mapping).
  Override: `POSTGRES_PORT`.
- **Control-plane:** listens `:8082`.
  Override: `CONTROL_PLANE_LISTEN_ADDR` or `LISTEN_ADDR`; host mapping `CONTROL_PLANE_PORT`.
- **User API Gateway:** listens `:8080`.
  Override: `USER_GATEWAY_LISTEN_ADDR` or `LISTEN_ADDR`; host mapping `ORCHESTRATOR_PORT`.
- **MCP Gateway (optional):** listens `:8083`.
  Override: `LISTEN_ADDR`; host mapping `MCP_GATEWAY_PORT`.
- **API Egress (optional):** listens `:8084`.
  Override: `LISTEN_ADDR`; host mapping `API_EGRESS_PORT`.
- **Ollama (orchestrator dev stack):** host mapping `11434:11434`.
  No override in compose; change port mapping in compose if needed.

Orchestrator config may reference the worker node's Worker API URL (e.g. `WORKER_API_TARGET_URL`).
When nodes run on the host, that is typically <http://host.containers.internal:8081>.

## Worker Node

- **Worker API:** listens `:8081` by default.
  Override: `LISTEN_ADDR` (worker-api process).
  Node startup YAML `worker_api.listen_port` (and `worker_api.public_base_url`) override the port the node advertises to the orchestrator.
  If the process listens on a different port, config must match.
- **Node Manager:** does not expose a separate HTTP port.
  It starts Worker API, Ollama (if inference enabled), and pod workloads.
- **Ollama (node-local):** when started by Node Manager, published as `11434:11434` on the host.
  The inference proxy in pods can reach it at `host.containers.internal:11434`.

Recommended: use Worker API default `8081` to avoid conflict with User API Gateway (`8080`).
If `worker_api.listen_port` is set in node startup YAML, it should match the Worker API process listen port (implementation default is 8081; worker_node.md example uses 9090 for illustration).

## Inference (Ollama and Proxy)

- Spec ID: `CYNAI.STANDS.InferenceOllamaAndProxy` <a id="spec-cynai-stands-inferenceollamaandproxy"></a>

- **Ollama:** standard port `11434` (both orchestrator stack and node-local).
  Containers publish `11434:11434`.
- **Inference proxy (sidecar):** inside each sandbox pod, the proxy listens on `:11434` in the pod network namespace so the sandbox can use `OLLAMA_BASE_URL=http://localhost:11434`.
  The proxy forwards to the node's Ollama (e.g. <http://host.containers.internal:11434>).
  There is no port conflict because the proxy's 11434 is inside the pod; the host's 11434 is used only by Ollama.

See [`docs/tech_specs/worker_node.md`](worker_node.md#spec-cynai-worker-nodelocalinference) and [`docs/tech_specs/sandbox_container.md`](sandbox_container.md#spec-cynai-sandbx-nodelocalinf).

## CLI (Cynork)

- Spec ID: `CYNAI.STANDS.CliCynork` <a id="spec-cynai-stands-clicynork"></a>

The CLI does not bind any port.
It connects to the User API Gateway:

- **Default gateway URL:** `http://localhost:8080` (must match the User API Gateway listen port).
- Override: `CYNORK_GATEWAY_URL` or config file `gateway_url`.

Cynork expects the orchestrator's user-gateway to be reachable at the configured URL (default 8080).
It does not use 11434 or any other port.

See [`docs/tech_specs/cynork_cli.md`](cynork_cli.md).

## Conflict Avoidance

- **Single-host dev:** Ports 5432, 8080, 8081, 8082, 11434 must be free for a full stack (orchestrator + one node + Ollama).
  Optional: 8083, 8084.
- **Orchestrator-only (docker compose):** 5432, 8080, 8082, 11434.
  Node runs elsewhere and uses 8081 (and optionally 11434 for node-local Ollama).
- **Multiple nodes on one host:** Each node's Worker API needs a distinct port (e.g. 8081, 9081, 9082).
  Configure `worker_api.listen_port` and `worker_api.public_base_url` per node.

## E2E and BDD

E2E and BDD tests use the same default ports as production and dev.

- **Orchestrator stack:** User API 8080, control-plane 8082, PostgreSQL 5432, Ollama 11434 (e.g. `just e2e`, `./scripts/setup-dev.sh start`, `./scripts/dev-setup.sh`).
- **Worker node:** Worker API 8081 (or `NODE_PORT` in dev-setup); inference proxy in sandbox pods exposes `http://localhost:11434` to the sandbox per [Inference (Ollama and Proxy)](#inference-ollama-and-proxy).
- **Feature assertions:** Scenarios that verify inference-in-sandbox assert sandbox stdout contains `http://localhost:11434` (e.g. `features/e2e/single_node_happy_path.feature`, `features/worker_node/worker_node_sandbox_execution.feature`).

## Environment and Config Overrides

- **PostgreSQL** - Default: 5432 - Override: `POSTGRES_PORT` (host mapping)
- **User API Gateway** - Default: :8080 - Override: `USER_GATEWAY_LISTEN_ADDR`, `LISTEN_ADDR`; `ORCHESTRATOR_PORT` (host mapping)
- **Control-plane** - Default: :8082 - Override: `CONTROL_PLANE_LISTEN_ADDR`, `LISTEN_ADDR`; `CONTROL_PLANE_PORT` (host mapping)
- **Worker API** - Default: :8081 - Override: `LISTEN_ADDR`; node YAML `worker_api.listen_port`
- **MCP Gateway** - Default: :8083 - Override: `LISTEN_ADDR`; `MCP_GATEWAY_PORT` (host mapping)
- **API Egress** - Default: :8084 - Override: `LISTEN_ADDR`; `API_EGRESS_PORT` (host mapping)
- **Ollama** - Default: 11434 - Override: Change port mapping in compose or node-manager; inference proxy upstream via `OLLAMA_UPSTREAM_URL` (e.g. <http://host.containers.internal:11434>)
- **Cynork gateway** - Default: <http://localhost:8080> - Override: `CYNORK_GATEWAY_URL` or config `gateway_url`
