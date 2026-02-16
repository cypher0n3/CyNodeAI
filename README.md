# CyNodeAI

[![License: Apache 2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

## Overview

Local-first multi-agent orchestrator for self-hosted teams and small enterprises.
Coordinates sandboxed workers across local nodes and optional cloud capacity, with centralized task management.

CyNodeAI coordinates agents on local nodes and, when configured, cloud-based agents.
A central orchestrator manages tasks, preferences, and vector storage (PostgreSQL + pgvector).
Local worker nodes register, receive jobs, run inference, and report results in isolated containers (Docker or Podman; Podman preferred for rootless); cloud-based agents are managed through the same registration, dispatch, and tool interfaces.
CyNodeAI also supports routing inference to external AI providers through controlled API egress.
Nodes may be inference-capable or sandbox-only, depending on configuration.

**Status:** Early prototype / design phase

**License:** [Apache 2.0](LICENSE)

**Technical specifications:** [Tech Specs Index](docs/tech_specs/_main.md)

## Goals

- Multi-agent system that can run fully local and private or incorporate cloud-based agents
- Easy scaling by adding local nodes (containers) or registering cloud-based agents
- Centralized intelligence with distributed compute across local and cloud workers

## High-Level Architecture

CyNodeAI uses a central orchestrator that can manage both node-local workers and cloud-based agents for execution.

### Orchestrator Node

- PostgreSQL + pgvector: stores tasks, agent state, user preferences, vector embeddings
- FastAPI server: REST + WebSocket API for node registration, job dispatch, result collection
- Authentication: pre-shared key (PSK) for node registration -> JWT for ongoing comms
- Workflow engine: LangGraph for multi-step and multi-agent flows
- Project Manager Agent: long-lived agent that coordinates tasks, enforces standards, and verifies completion using stored preferences
- API Egress Server: controlled external API access that keeps API keys out of sandbox containers
- Secure Browser Service: fetches and sanitizes web content for agents without exposing the open web to sandboxes
- External model routing: uses configured external AI APIs when local workers are overloaded or lack required capabilities
- Model management: caches models locally and tracks model capabilities for consistent node execution
- User API gateway: single user-facing endpoint that exposes orchestrator capabilities for UIs and integrations
- Sandbox image registry: manages sandbox container images and allowed images for node execution
- MCP tool interface: standard tool protocol for agents, with role-based access and support for external MCP servers
- Orchestrator bootstrap: optional YAML import at startup to seed PostgreSQL configuration and integrations
- Task scheduler: queues work, selects nodes, dispatches jobs; supports retries, leases, and a cron tool for scheduled jobs, wakeups, and automation
- Execution sandboxes: requests sandbox container execution on worker nodes
- Co-location preference: run sandbox containers on the same host that is assigned the AI work to minimize network traffic
- Orchestrator can also act as a worker by running the same worker services locally
- Can manage cloud-based agents via the same registration, capability reporting, and job dispatch model

See [`docs/tech_specs/orchestrator.md`](docs/tech_specs/orchestrator.md) for orchestrator responsibilities and cross-service links.

### Worker Nodes

- Node Manager (system service): manages Docker or Podman (Podman preferred for rootless) and container lifecycle on the worker host
  - Starts/stops the worker API container and the Ollama container (or host Ollama)
  - Spins up and tears down per-job or per-agent sandbox containers
  - Receives node configuration from the orchestrator, including registry endpoints and credentials
  - Contacts the orchestrator before starting Ollama so the orchestrator can select the correct Ollama container for the node
- Worker API service (FastAPI): exposes /register, /jobs, /status, /result endpoints
  - Registers with orchestrator using PSK
  - Polls/pushes via HTTP or WebSocket
- Optional single Ollama container (models loaded on-demand) with access to all GPUs/NPUs on the host
- Sandbox-only nodes can run sandboxes without running inference services

See [`docs/tech_specs/node.md`](docs/tech_specs/node.md) for node manager, registration, capability reporting, and configuration delivery.

### Cloud-Based Agents

Cloud-based agents are first-class workers that run outside your local network.
They use the same Worker API contract and are scheduled by the orchestrator based on capability, load, policy, and user preferences.

See [`docs/tech_specs/cloud_agents.md`](docs/tech_specs/cloud_agents.md) for contract details, security model, and practical OpenAI (ChatGPT) and Anthropic deployment examples.

### Sandbox Containers

- Isolated containers for agent and tool execution with restricted network access
- Agents can execute arbitrary commands inside the sandbox container
- Data ingress/egress via orchestrator-managed REST endpoints and file upload/download handling
- Sandbox containers are started and stopped by the worker node's Node Manager on the target host

See [`docs/tech_specs/node.md`](docs/tech_specs/node.md) for sandbox lifecycle and host execution responsibilities.

### API Egress Server

- Service that performs outbound API calls on behalf of agents
- API keys are stored in PostgreSQL and are never exposed to sandbox containers or agents
- Agents submit requests to the orchestrator, which routes approved calls to the API Egress Server
- Results are returned to the requesting agent, and calls are logged for auditing

See [`docs/tech_specs/api_egress_server.md`](docs/tech_specs/api_egress_server.md) and [`docs/tech_specs/access_control.md`](docs/tech_specs/access_control.md).

### Secure Browser Service

- Service that fetches web pages on behalf of agents and returns sanitized plain text
- Performs prompt-injection stripping using deterministic rules, not AI
- Access is policy-controlled and audited, similar to the API Egress Server

See [`docs/tech_specs/secure_browser_service.md`](docs/tech_specs/secure_browser_service.md) and [`docs/tech_specs/access_control.md`](docs/tech_specs/access_control.md).

### External Model Routing

- Orchestrator can route LLM calls to external AI APIs when configured and allowed
- Default preference is local execution, with external fallback for overload or missing capabilities
- External calls use the API Egress Server so credentials are never exposed to agents or sandbox containers

See [`docs/tech_specs/external_model_routing.md`](docs/tech_specs/external_model_routing.md).

### Model Management

- Orchestrator maintains a local model cache for worker nodes to load from
- Nodes SHOULD load models from the orchestrator cache instead of pulling models from the public internet
- Database tracks models, versions, and capabilities so the Project Manager Agent can select the right model per task
- Orchestrator can download and cache models when directed by users, subject to policy and auditing

See [`docs/tech_specs/model_management.md`](docs/tech_specs/model_management.md).

### Sandbox Image Registry

- Orchestrator provides or integrates with a container image registry for sandbox images
- Nodes pull sandbox container images from the configured registry
- Agents can submit images for publishing via orchestrator-controlled workflows
- Database tracks allowed images and image capabilities so tasks can request appropriate sandbox environments

See [`docs/tech_specs/sandbox_image_registry.md`](docs/tech_specs/sandbox_image_registry.md).

### User API Gateway

- Orchestrator exposes a single user-facing API endpoint for submitting work and retrieving results
- Designed to integrate with external tools such as Open WebUI and messaging services
- Provides stable auth, auditing, and capability discovery for user clients
- Provides live updates through user-connected messaging destinations and webhook subscriptions

See [`docs/tech_specs/user_api_gateway.md`](docs/tech_specs/user_api_gateway.md) and [`docs/tech_specs/data_rest_api.md`](docs/tech_specs/data_rest_api.md).

### MCP Tool Interface

- Agents use MCP as the standard interface for tools (web, API egress, artifacts, model and node operations)
- Tool access is role-based (worker agents vs orchestrator-side agents such as Project Manager and Project Analyst)
- Orchestrator can expose additional tools by connecting to external MCP servers, subject to policy and auditing

See [`docs/tech_specs/mcp_tooling.md`](docs/tech_specs/mcp_tooling.md).

### Orchestrator Bootstrap

- Orchestrator can import a bootstrap YAML at startup to seed PostgreSQL with system defaults and integration configuration
- YAML is not the source of truth; PostgreSQL remains the source of truth after import
- Orchestrator can run as the sole service with zero worker nodes by routing to external AI providers when allowed

See [`docs/tech_specs/orchestrator_bootstrap.md`](docs/tech_specs/orchestrator_bootstrap.md).

### Communication Flow

1. Node starts -> registers with PSK -> receives JWT
2. Orchestrator dispatches a job to the node (worker API) and specifies sandbox requirements
3. Node Manager creates the sandbox container on that host and runs the job inside it, when sandbox execution is required
4. Node runs sandboxed tools and optional local inference -> pushes result/status back
5. Central DB tracks everything (tasks, vectors, logs)

See [`docs/tech_specs/orchestrator.md`](docs/tech_specs/orchestrator.md) and [`docs/tech_specs/node.md`](docs/tech_specs/node.md).

## Key Technologies

- Go (orchestrator and node REST APIs; see [Go REST API standards](docs/tech_specs/go_rest_api_standards.md))
- Ollama (local inference)
- PostgreSQL + pgvector (state, embeddings)
- Docker or Podman (sandbox containers; Podman preferred for rootless)
- MCP (agent tool interface)
- JWT auth + HTTPS (self-signed or Let's Encrypt)

See [Tech Specs Index](docs/tech_specs/_main.md) for the full spec index.

## Future Considerations

- **Kubernetes support**: run orchestrator and/or workers on Kubernetes (scheduling, scaling, CRI-based nodes).
- **containerd via nerdctl**: support containerd as a sandbox runtime (e.g. via nerdctl) for environments where it is already the standard.
- **External service for RBAC**: integrate with an IdM/IdP and group directory (e.g. Google Workspace, Microsoft Entra ID) to source groups and memberships via SCIM-style provisioning and/or directory APIs.
