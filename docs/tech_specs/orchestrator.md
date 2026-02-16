# Orchestrator Technical Spec

- [Document Overview](#document-overview)
- [Core Responsibilities](#core-responsibilities)
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

## Document Overview

This document describes the orchestrator responsibilities and its relationship to orchestrator-side agents.

## Core Responsibilities

- Acts as the control plane for nodes, jobs, tasks, and agent workflows.
- Owns the source of truth for task state, results, logs, and user preferences in PostgreSQL.
- Dispatches sandboxed execution to worker nodes (via the worker API).
- Routes model inference to local nodes or to external providers when allowed.
- Schedules sandbox execution independently of where inference occurs.

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

See [`docs/tech_specs/node.md`](node.md).

## MCP Tool Interface

The orchestrator adopts MCP as the standard tool interface for agents.
This enables a consistent tool protocol, role-based tool access, and support for external MCP servers.

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
