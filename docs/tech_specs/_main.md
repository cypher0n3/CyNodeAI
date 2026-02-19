# Technical Specifications Index

- [Document Overview](#document-overview)
- [Architecture Summary](#architecture-summary)
- [Tech Spec Index](#tech-spec-index)
  - [Orchestrator and Nodes](#orchestrator-and-nodes)
  - [User Interfaces](#user-interfaces)
  - [API Specifications](#api-specifications)
  - [Agents and Connectors](#agents-and-connectors)
  - [Identity, Policy, and Data](#identity-policy-and-data)
  - [Protocols and Standards](#protocols-and-standards)
  - [Execution and Artifacts](#execution-and-artifacts)
  - [External Integration and Routing](#external-integration-and-routing)
  - [Model Lifecycle](#model-lifecycle)
  - [Agents Specification](#agents-specification)
  - [Bootstrap Configurations](#bootstrap-configurations)
- [MVP Development Plan](#mvp-development-plan)
  - [Phase 0 Foundations](#phase-0-foundations)
  - [Phase 1 Single Node Happy Path](#phase-1-single-node-happy-path)
  - [Phase 2 MCP in the Loop](#phase-2-mcp-in-the-loop)
  - [Phase 3 Multi Node Robustness](#phase-3-multi-node-robustness)
  - [Phase 4 Optional Controlled Egress and Integrations](#phase-4-optional-controlled-egress-and-integrations)

## Document Overview

This document is the entrypoint for CyNodeAI technical specifications.
It provides a summary of the system and links to detailed specs.

## Architecture Summary

CyNodeAI is a local-first multi-agent orchestrator for self-hosted teams and small enterprises.
It uses a central orchestrator to coordinate node-local workers, sandboxed execution, and tool access.

Key principles

- Agents use MCP as the standard tool interface.
- Worker agents run in sandbox containers with restricted network access.
- Nodes are configured by the orchestrator at registration time and can receive dynamic updates.
- User clients interact through a single User API Gateway.
- All REST APIs in this system MUST be implemented in Go.

## Tech Spec Index

<!-- no-empty-heading allow -->

### Orchestrator and Nodes

- Orchestrator: [`docs/tech_specs/orchestrator.md`](orchestrator.md)
- Worker nodes: [`docs/tech_specs/node.md`](node.md)
- Node payloads: [`docs/tech_specs/node_payloads.md`](node_payloads.md)

### User Interfaces

- CLI management app: [`docs/tech_specs/cli_management_app.md`](cli_management_app.md)
- Admin web console: [`docs/tech_specs/admin_web_console.md`](admin_web_console.md)

### API Specifications

- User API Gateway: [`docs/tech_specs/user_api_gateway.md`](user_api_gateway.md)
- Data REST API: [`docs/tech_specs/data_rest_api.md`](data_rest_api.md)
- Runs and sessions API: [`docs/tech_specs/runs_and_sessions_api.md`](runs_and_sessions_api.md)
- Worker API (node contract): [`docs/tech_specs/worker_api.md`](worker_api.md)
- Worker Telemetry API (node ops): [`docs/tech_specs/worker_telemetry_api.md`](worker_telemetry_api.md)

### Agents and Connectors

- Cloud-based agents: [`docs/tech_specs/cloud_agents.md`](cloud_agents.md)
- Connector framework: [`docs/tech_specs/connector_framework.md`](connector_framework.md)

### Identity, Policy, and Data

- Local user accounts: [`docs/tech_specs/local_user_accounts.md`](local_user_accounts.md)
- Projects and scopes: [`docs/tech_specs/projects_and_scopes.md`](projects_and_scopes.md)
- RBAC and groups: [`docs/tech_specs/rbac_and_groups.md`](rbac_and_groups.md)
- Access control: [`docs/tech_specs/access_control.md`](access_control.md)
- User preferences: [`docs/tech_specs/user_preferences.md`](user_preferences.md)
- Postgres schema: [`docs/tech_specs/postgres_schema.md`](postgres_schema.md)

### Protocols and Standards

- MCP tooling: [`docs/tech_specs/mcp_tooling.md`](mcp_tooling.md)
- MCP gateway enforcement: [`docs/tech_specs/mcp_gateway_enforcement.md`](mcp_gateway_enforcement.md)
- MCP tool catalog: [`docs/tech_specs/mcp_tool_catalog.md`](mcp_tool_catalog.md)
- MCP tool call auditing: [`docs/tech_specs/mcp_tool_call_auditing.md`](mcp_tool_call_auditing.md)
- MCP SDK installation: [`docs/tech_specs/mcp_sdk_installation.md`](mcp_sdk_installation.md)
- Go REST API standards: [`docs/tech_specs/go_rest_api_standards.md`](go_rest_api_standards.md)

### Execution and Artifacts

- Sandbox container: [`docs/tech_specs/sandbox_container.md`](sandbox_container.md)
- Sandbox image registry: [`docs/tech_specs/sandbox_image_registry.md`](sandbox_image_registry.md)
- Sandbox web browsing: [`docs/tech_specs/secure_browser_service.md`](secure_browser_service.md)

### External Integration and Routing

- API egress server: [`docs/tech_specs/api_egress_server.md`](api_egress_server.md)
- Git egress MCP: [`docs/tech_specs/git_egress_mcp.md`](git_egress_mcp.md)
- External model routing: [`docs/tech_specs/external_model_routing.md`](external_model_routing.md)

### Model Lifecycle

- Model management: [`docs/tech_specs/model_management.md`](model_management.md)

### Agents Specification

- Project Manager Agent: [`docs/tech_specs/project_manager_agent.md`](project_manager_agent.md)
- Project Analyst Agent: [`docs/tech_specs/project_analyst_agent.md`](project_analyst_agent.md)
- LangGraph MVP workflow: [`docs/tech_specs/langgraph_mvp.md`](langgraph_mvp.md)

### Bootstrap Configurations

- Orchestrator bootstrap: [`docs/tech_specs/orchestrator_bootstrap.md`](orchestrator_bootstrap.md)

## MVP Development Plan

This plan targets the minimum components needed for end-to-end task execution.
Items are grouped by phase and can be implemented incrementally.

### Phase 0 Foundations

- Define Postgres schema for users, local auth sessions, groups and RBAC, tasks, jobs, nodes, artifacts, and audit logging (see [`docs/tech_specs/postgres_schema.md`](postgres_schema.md)).
- Define node capability report payload and node configuration payload (see [`docs/tech_specs/node_payloads.md`](node_payloads.md)).
  - Specify registration-time bootstrap payload (PSK to JWT) and config versioning.
  - Specify capability report fields, hashing, and change reporting behavior.
  - Specify configuration refresh, acknowledgement payload, and rollback reporting.
- Define MCP gateway enforcement and initial tool allowlists by role (see [`docs/tech_specs/mcp_gateway_enforcement.md`](mcp_gateway_enforcement.md)).
  - Use standard MCP protocol messages on the wire.
  - Enforce allowlists, access control, and auditing centrally at the orchestrator gateway.
  - Require strict tool argument schemas for task-scoped tools (tool args include `task_id`).
  - Define policy mapping to `access_control_rules` using action `mcp.tool.invoke`.

### Phase 1 Single Node Happy Path

- Orchestrator: node registration (PSK to JWT), capability ingest, config delivery, job dispatch, result collection.
- Job dispatch: direct HTTP to Worker API using per-node URL and token from config delivery; MCP gateway not in loop.
- Node: Node Manager startup sequence that contacts orchestrator before starting the single Ollama container.
- Node: worker API can receive a job, run a sandbox container, and return a result.
- System: at least one inference-capable path must be available (node-local inference container such as Ollama, or external model routing with a configured provider key).
- System: in the single-node case, startup must fail fast (or refuse to enter a ready state) if the node cannot run an inference container and no external provider key is configured.
- User API Gateway: local user auth (login and refresh), create task, and retrieve task result.
- Phase 1 config refresh: node fetches configuration on startup only (no polling).
- Phase 1 node JWT: long-lived; node re-registers on expiry.

### Phase 2 MCP in the Loop

- Implement orchestrator MCP tool gateway with role-based access.
- Add MCP database tools for orchestrator-side agents and MCP artifact tools for worker agents.
- Ensure orchestrator-side agents use MCP database tools and do not connect to Postgres directly.

### Phase 3 Multi Node Robustness

- Add node selection based on capability, load, data locality, and model availability.
- Add job leases, retries, idempotency, and heartbeats.
- Add dynamic node configuration updates and startup capability change reporting.

### Phase 4 Optional Controlled Egress and Integrations

- Add API Egress Server with ACL enforcement and auditing.
- Add Secure Browser Service with deterministic sanitization and DB-backed rules.
- Add external model routing fallback for standalone orchestrator operation, subject to policy.
- Add CLI management app for credentials, preferences, and basic node management.
- Defer the admin web console until after the CLI exists.
