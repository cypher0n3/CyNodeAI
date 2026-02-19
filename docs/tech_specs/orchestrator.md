# Orchestrator Technical Spec

- [Spec IDs](#spec-ids)
- [Document Overview](#document-overview)
- [Core Responsibilities](#core-responsibilities)
- [Task Scheduler](#task-scheduler)
- [Project Manager Agent](#project-manager-agent)
- [API Egress Server](#api-egress-server)
- [Secure Browser Service](#secure-browser-service)
- [External Model Routing](#external-model-routing)
- [Model Management](#model-management)
- [User API Gateway](#user-api-gateway)
- [Sandbox Image Registry](#sandbox-image-registry)
- [Node Bootstrap and Configuration](#node-bootstrap-and-configuration)
- [MCP Tool Interface](#mcp-tool-interface)
- [Orchestrator Bootstrap Configuration](#orchestrator-bootstrap-configuration)
- [Workflow Engine](#workflow-engine)

## Spec IDs

- Spec ID: `CYNAI.ORCHES.Doc.Orchestrator` <a id="spec-cynai-orches-doc-orchestrator"></a>

## Document Overview

This document describes the orchestrator responsibilities and its relationship to orchestrator-side agents.

## Core Responsibilities

- Acts as the control plane for nodes, jobs, tasks, and agent workflows.
- Owns the source of truth for task state, results, logs, and user preferences in PostgreSQL.
- Dispatches sandboxed execution to worker nodes (via the worker API).
- Routes model inference to local nodes or to external providers when allowed.
- Schedules sandbox execution independently of where inference occurs.

## Task Scheduler

The orchestrator MUST include a task scheduler that decides when and where to run work.

Responsibilities

- **Queue**: Maintain a queue of pending work (tasks and jobs) backed by PostgreSQL so state survives restarts.
- **Dispatch**: Select eligible nodes based on capability, load, data locality, and model availability; dispatch jobs to the worker API; collect results and update task state.
- **Retries and leases**: Support job leases, retries on failure, and idempotency so work is not lost or duplicated when nodes fail or restart.
- **Cron tool**: MUST support a cron (or equivalent) facility for scheduled jobs, wakeups, and automation.
  Users and agents MUST be able to enqueue work at a future time or on a recurrence (cron expression or calendar-like).
  The scheduler is responsible for firing at the scheduled time and enqueueing the corresponding tasks or jobs.
  Schedule evaluation MUST be time-zone aware (schedules specify or inherit a time zone; next-run and history use that zone).
  Schedules MUST support create, update, disable (temporarily stop firing without deleting), and cancellation (cancel the schedule or the next run).
  The system MUST retain run history per schedule (past execution times and outcomes) for visibility and debugging.
  The cron facility SHOULD be exposed to agents (e.g. via MCP tools) so they can create and manage scheduled jobs.

The scheduler MAY be implemented as a background process, a worker that consumes the queue, or integrated into the workflow engine; it MUST use the same node selection and job-dispatch contracts as the rest of the orchestrator.
Agents (e.g. Project Manager) and the cron facility enqueue work; the scheduler is responsible for dequeueing and dispatching to nodes.
The scheduler MUST be available via the User API Gateway so users can create and manage scheduled jobs, query queue and schedule state, and trigger wakeups or automation.

See job dispatch and node selection in [`docs/tech_specs/node.md`](node.md), the roadmap in [`docs/tech_specs/_main.md`](_main.md), and [`docs/tech_specs/user_api_gateway.md`](user_api_gateway.md).

## Project Manager Agent

The Project Manager Agent is a long-lived orchestrator-side agent that continuously drives work to completion.

- Reads tasks and their acceptance criteria from the database.
- Retrieves user preferences and standards from the database and applies them during planning and verification.
- Assigns work to worker nodes, monitors progress, and requests remediation when results fail checks.
- Continuously updates task state in PostgreSQL so the system remains resumable and auditable.

See [`docs/tech_specs/project_manager_agent.md`](project_manager_agent.md), [`docs/tech_specs/project_analyst_agent.md`](project_analyst_agent.md), and [`docs/tech_specs/user_preferences.md`](user_preferences.md).

Orchestrator-side agents MAY use external AI providers for planning and verification when policy allows it.
External provider calls MUST use API Egress and SHOULD use agent-specific routing preferences.
See [`docs/tech_specs/external_model_routing.md`](external_model_routing.md) and [`docs/tech_specs/api_egress_server.md`](api_egress_server.md).

## API Egress Server

The orchestrator provides controlled external API access through an API Egress Server.
This prevents API keys from being exposed to sandbox containers and enables policy and auditing.

See [`docs/tech_specs/api_egress_server.md`](api_egress_server.md).

## Secure Browser Service

The orchestrator provides controlled web browsing through a Secure Browser Service.
This enables agents to retrieve web information without granting direct network access to sandbox containers.

See [`docs/tech_specs/secure_browser_service.md`](secure_browser_service.md).

## External Model Routing

The orchestrator can route model calls to configured external AI APIs when local workers are overloaded or lack required capabilities.
External calls MUST use the API Egress Server so credentials are not exposed to agents or sandbox containers.

See [`docs/tech_specs/external_model_routing.md`](external_model_routing.md).

## Model Management

The orchestrator maintains a model registry and a local model cache that worker nodes can load from.
This enables consistent model availability and reduces node internet dependencies.

See [`docs/tech_specs/model_management.md`](model_management.md).

## User API Gateway

The orchestrator exposes a single user-facing API endpoint that surfaces its capabilities to external clients.
This is intended for UIs and integrations such as Open WebUI and messaging services.

See [`docs/tech_specs/user_api_gateway.md`](user_api_gateway.md) and [`docs/tech_specs/data_rest_api.md`](data_rest_api.md).

## Sandbox Image Registry

The orchestrator integrates with a sandbox container image registry for worker nodes to pull sandbox images from.
Allowed sandbox images and their capabilities are tracked in PostgreSQL so tasks can request safe, appropriate execution environments.

See [`docs/tech_specs/sandbox_image_registry.md`](sandbox_image_registry.md).

## Node Bootstrap and Configuration

The orchestrator MUST be able to configure worker nodes at registration time.
This includes distributing the correct endpoints, certificates, and pull credentials for orchestrator-provided services.
The orchestrator MUST support dynamic configuration updates after registration and must ingest node capability reports on registration and node startup.

Config delivery

- The orchestrator exposes the node config URL in the bootstrap payload (`node_config_url` in `node_bootstrap_payload_v1`).
- GET on that URL returns `node_configuration_payload_v1` for the authenticated node.
- POST on that URL accepts `node_config_ack_v1` and persists the acknowledgement; see [`docs/tech_specs/postgres_schema.md`](postgres_schema.md) Nodes table columns `config_ack_at`, `config_ack_status`, `config_ack_error`.
- Endpoint paths are not mandated here; the bootstrap payload carries the concrete URLs so nodes do not rely on hard-coded paths.

Job dispatch (initial implementation)

- For the initial single-node implementation (Phase 1), the orchestrator dispatches jobs to the Worker API via direct HTTP.
- The dispatcher uses the per-node `worker_api_target_url` and per-node bearer token stored from config delivery (see [`docs/tech_specs/postgres_schema.md`](postgres_schema.md) Nodes table).
- The MCP gateway is not in the loop for job dispatch in Phase 1.

See [`docs/tech_specs/node.md`](node.md) and [`docs/tech_specs/node_payloads.md`](node_payloads.md).

## MCP Tool Interface

The orchestrator adopts MCP as the standard tool interface for agents.
This enables a consistent tool protocol and role-based tool access.

See [`docs/tech_specs/mcp_tooling.md`](mcp_tooling.md).

## Workflow Engine

The orchestrator uses LangGraph to implement multi-step and multi-agent workflows.
The Project Manager Agent's behavior is implemented by the LangGraph MVP workflow.

See [`docs/tech_specs/langgraph_mvp.md`](langgraph_mvp.md) for the graph topology, state model, and node behaviors.
For how the graph is hosted, invoked, checkpointed, and wired to orchestrator capabilities (MCP, Worker API, model routing), see the "Integration with the Orchestrator" section of that document.

## Orchestrator Bootstrap Configuration

The orchestrator MAY import bootstrap configuration from a YAML file at startup to seed PostgreSQL and external integrations.
The orchestrator SHOULD support running as the sole service with zero worker nodes and using external AI providers when allowed.

See [`docs/tech_specs/orchestrator_bootstrap.md`](orchestrator_bootstrap.md).
