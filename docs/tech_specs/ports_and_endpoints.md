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

### Traces To

- [REQ-WORKER-0114](../requirements/worker.md#req-worker-0114)
- [REQ-WORKER-0115](../requirements/worker.md#req-worker-0115)
- [REQ-SANDBX-0106](../requirements/sandbx.md#req-sandbx-0106)

- **5432** - PostgreSQL - Database (orchestrator)
- **8080** - Web Console - Nuxt/Vue web UI (orchestrator; own container)
- **9000** - Artifacts storage (S3 API) - MinIO in dev stack; see [Orchestrator Artifacts Storage](orchestrator_artifacts_storage.md#spec-cynai-orches-artifactsdevstack)
- **12080** - User API Gateway - Auth, users, tasks, results (orchestrator)
- **12082** - Control-plane - Node registration, dispatch, migrations (orchestrator)
- **12083** - MCP Gateway - MCP tool routing (orchestrator; optional profile)
- **12084** - API Egress - External API calls with credentials (orchestrator; optional profile)
- **12090** - Worker API - Run jobs on the node (worker node)
- **11434** - Ollama - Inference API (orchestrator or node); also inference proxy listen port inside sandbox pods

The Web Console uses port **8080** by default so it is easy to remember and does not conflict with the 12080-12090 orchestrator API block.
All other CyNodeAI HTTP service defaults use the **12080-12090** block.
No other CyNodeAI service defaults use port 8080.
The CLI (Cynork) does not listen on any port.
It connects to the User API Gateway (default `http://localhost:12080`).

## Orchestrator Stack

- **PostgreSQL:** `5432` (container internal and host mapping).
  Override: `POSTGRES_PORT`.
- **Artifacts (MinIO, dev stack):** S3 API on `9000` (container and host mapping).
  Override: `ARTIFACTS_S3_PORT`.
  See [Orchestrator Artifacts Storage](orchestrator_artifacts_storage.md#spec-cynai-orches-artifactsdevstack).
- **Web Console:** listens `:8080` in its own container.
  Override: `WEBCON_PORT` or `NITRO_PORT` (Nuxt); host mapping `WEBCON_PORT`.
  The console is a separate service in the orchestrator stack; it calls the User API Gateway (e.g. `http://user-gateway:12080`) for all operations.
  See [Web Console](web_console.md#spec-cynai-webcon-runtimeanddeployment).
- **Control-plane:** listens `:12082`.
  Override: `CONTROL_PLANE_LISTEN_ADDR` or `LISTEN_ADDR`; host mapping `CONTROL_PLANE_PORT`.
- **User API Gateway:** listens `:12080`.
  Override: `USER_GATEWAY_LISTEN_ADDR` or `LISTEN_ADDR`; host mapping `ORCHESTRATOR_PORT`.
- **MCP Gateway (optional):** listens `:12083`.
  Override: `LISTEN_ADDR`; host mapping `MCP_GATEWAY_PORT`.
- **API Egress (optional):** listens `:12084`.
  Override: `LISTEN_ADDR`; host mapping `API_EGRESS_PORT`.
- **Ollama (orchestrator dev stack):** host mapping `11434:11434`.
  No override in compose; change port mapping in compose if needed.

Orchestrator config may reference the worker node's Worker API URL (e.g. `WORKER_API_TARGET_URL`).
When nodes run on the host, that is typically <http://host.containers.internal:12090>.

## Worker Node

- **Worker API:** listens `:12090` by default.
  All CyNodeAI HTTP defaults use the 12080-12090 block.
  The node reports its Worker API address (`worker_api.base_url`) at registration and in capability reports; the orchestrator uses that URL for dispatch unless an explicit operator override is set (e.g. same-host dev: `WORKER_API_TARGET_URL`); see [`worker_node.md`](worker_node.md) and [`worker_node_payloads.md`](worker_node_payloads.md).
  Override: `LISTEN_ADDR` (worker-api process).
  Node startup YAML `worker_api.listen_port` (and `worker_api.public_base_url`) define what the node advertises; any orchestrator-side override for the dispatch URL is explicit and documented as an override.
  If the process listens on a different port, config must match.
- **Node Manager:** does not expose a separate HTTP port.
  It starts Worker API, Ollama (if inference enabled), and pod workloads.
- **Ollama (node-local):** when started by Node Manager, published as `11434:11434` on the host.
  The inference proxy in pods can reach it at `host.containers.internal:11434`.

## Inference (Ollama and Proxy)

- Spec ID: `CYNAI.STANDS.InferenceOllamaAndProxy` <a id="spec-cynai-stands-inferenceollamaandproxy"></a>

- **Ollama (host/node):** standard port `11434` (orchestrator stack and node-local).
  Ollama listens on TCP on the host; the inference proxy (worker-owned) connects to it (e.g. <http://host.containers.internal:11434>) and MUST NOT expose that TCP endpoint to agent or sandbox containers.
- **Container-facing inference (unified UDS):** Per [Unified UDS Path](worker_node.md#spec-cynai-worker-unifiedudspath), the sandbox and managed agents MUST reach inference only via Unix domain sockets.
  The inference proxy exposes a UDS to the sandbox; the worker injects an inference proxy URL (e.g. `INFERENCE_PROXY_URL=http+unix://...`) into the sandbox, not `OLLAMA_BASE_URL=http://localhost:11434`.
  Tests and feature assertions MUST validate UDS-based inference (e.g. presence of inference proxy socket or `http+unix` URL in sandbox env), not a TCP URL.

See [`docs/tech_specs/worker_node.md`](worker_node.md#spec-cynai-worker-unifiedudspath) and [Node-Local Inference](worker_node.md#spec-cynai-worker-nodelocalinference), and [`docs/tech_specs/sandbox_container.md`](sandbox_container.md#spec-cynai-sandbx-nodelocalinf).

## CLI (Cynork)

- Spec ID: `CYNAI.STANDS.CliCynork` <a id="spec-cynai-stands-clicynork"></a>

The CLI does not bind any port.
It connects to the User API Gateway:

- **Default gateway URL:** `http://localhost:12080` (must match the User API Gateway listen port).
- Override: `CYNORK_GATEWAY_URL` or config file `gateway_url`.

Cynork expects the orchestrator's user-gateway to be reachable at the configured URL (default 12080).
It does not use 11434 or any other port.

See [`docs/tech_specs/cynork_cli.md`](cynork_cli.md).

## Conflict Avoidance

- **Single-host dev:** Ports 5432, 8080, 12080, 12082, 12090, 11434 must be free for a full stack (orchestrator + Web Console + one node + Ollama).
  Optional: 12083, 12084.
- **Orchestrator-only (docker compose):** 5432, 8080, 12080, 12082, 11434.
  Node runs elsewhere and uses 12090 (and optionally 11434 for node-local Ollama).
- **Multiple nodes on one host:** Each node's Worker API needs a distinct port (e.g. 12090, 12091, 12092).
  Configure `worker_api.listen_port` and `worker_api.public_base_url` per node.

## E2E and BDD

E2E and BDD tests use the same default ports as production and dev.

- **Orchestrator stack:** User API 12080, control-plane 12082, PostgreSQL 5432, Ollama 11434 (e.g. `just e2e`, `just setup-dev start`).
- **Worker node:** Worker API 12090 (or `WORKER_PORT`); inference proxy exposes only UDS to the sandbox per [Inference (Ollama and Proxy)](#inference-ollama-and-proxy) and [Unified UDS Path](worker_node.md#spec-cynai-worker-unifiedudspath).
- **Feature assertions:** Scenarios that verify inference-in-sandbox MUST assert UDS-based inference (e.g. sandbox env contains `INFERENCE_PROXY_URL` with `http+unix`, or socket path present); they MUST NOT assert `http://localhost:11434` or `OLLAMA_BASE_URL` in container-facing contract.

## Environment and Config Overrides

- **PostgreSQL** - Default: 5432 - Override: `POSTGRES_PORT` (host mapping)
- **Web Console** - Default: :8080 - Override: `WEBCON_PORT` or `NITRO_PORT` (Nuxt); host mapping `WEBCON_PORT`
- **User API Gateway** - Default: :12080 - Override: `USER_GATEWAY_LISTEN_ADDR`, `LISTEN_ADDR`; `ORCHESTRATOR_PORT` (host mapping)
- **Control-plane** - Default: :12082 - Override: `CONTROL_PLANE_LISTEN_ADDR`, `LISTEN_ADDR`; `CONTROL_PLANE_PORT` (host mapping)
- **Worker API** - Default: :12090 - Override: `LISTEN_ADDR`; node YAML `worker_api.listen_port`
- **MCP Gateway** - Default: :12083 - Override: `LISTEN_ADDR`; `MCP_GATEWAY_PORT` (host mapping)
- **API Egress** - Default: :12084 - Override: `LISTEN_ADDR`; `API_EGRESS_PORT` (host mapping)
- **Ollama** - Default: 11434 - Override: Change port mapping in compose or node-manager; inference proxy upstream via `OLLAMA_UPSTREAM_URL` (e.g. <http://host.containers.internal:11434>)
- **Cynork gateway** - Default: <http://localhost:12080> - Override: `CYNORK_GATEWAY_URL` or config `gateway_url`
- **Web Console gateway URL** - Default (in container): <http://user-gateway:12080> - Override: `NUXT_PUBLIC_GATEWAY_URL` or equivalent runtime config
