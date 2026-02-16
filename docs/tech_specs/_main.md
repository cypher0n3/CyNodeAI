# Technical Specifications Index

- [Document Overview](#document-overview)
- [Architecture Summary](#architecture-summary)
- [Tech Spec Index](#tech-spec-index)
- [MVP Development Plan](#mvp-development-plan)

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

Core documents

- Orchestrator: [`docs/tech_specs/orchestrator.md`](orchestrator.md)
- Worker nodes: [`docs/tech_specs/node.md`](node.md)
- Cloud-based agents: [`docs/tech_specs/cloud_agents.md`](cloud_agents.md)
- User API Gateway: [`docs/tech_specs/user_api_gateway.md`](user_api_gateway.md)
- Admin web console: [`docs/tech_specs/admin_web_console.md`](admin_web_console.md)
- CLI management app: [`docs/tech_specs/cli_management_app.md`](cli_management_app.md)
- Data REST API: [`docs/tech_specs/data_rest_api.md`](data_rest_api.md)
- Go REST API standards: [`docs/tech_specs/go_rest_api_standards.md`](go_rest_api_standards.md)
- MCP tooling: [`docs/tech_specs/mcp_tooling.md`](mcp_tooling.md)
- User preferences: [`docs/tech_specs/user_preferences.md`](user_preferences.md)
- Access control: [`docs/tech_specs/access_control.md`](access_control.md)

Execution and artifacts

- Sandbox container: [`docs/tech_specs/sandbox_container.md`](sandbox_container.md)
- Sandbox image registry: [`docs/tech_specs/sandbox_image_registry.md`](sandbox_image_registry.md)
- Sandbox web browsing: [`docs/tech_specs/secure_browser_service.md`](secure_browser_service.md)

External integration and routing

- API egress server: [`docs/tech_specs/api_egress_server.md`](api_egress_server.md)
- Git egress MCP: [`docs/tech_specs/git_egress_mcp.md`](git_egress_mcp.md)
- External model routing: [`docs/tech_specs/external_model_routing.md`](external_model_routing.md)

Model lifecycle

- Model management: [`docs/tech_specs/model_management.md`](model_management.md)

Agents

- Project Manager Agent: [`docs/tech_specs/project_manager_agent.md`](project_manager_agent.md)
- Project Analyst Agent: [`docs/tech_specs/project_analyst_agent.md`](project_analyst_agent.md)
- LangGraph MVP workflow: [`docs/tech_specs/langgraph_mvp.md`](langgraph_mvp.md)

Bootstrap

- Orchestrator bootstrap: [`docs/tech_specs/orchestrator_bootstrap.md`](orchestrator_bootstrap.md)

## MVP Development Plan

This plan targets the minimum components needed for end-to-end task execution.
Items are grouped by phase and can be implemented incrementally.

### Phase 0 Foundations

- Define Postgres schema for tasks, jobs, nodes, artifacts, and audit logging.
- Define node capability report payload and node configuration payload.
- Define MCP tool envelope and initial tool allowlists by role.

### Phase 1 Single Node Happy Path

- Orchestrator: node registration (PSK to JWT), capability ingest, config delivery, job dispatch, result collection.
- Node: Node Manager startup sequence that contacts orchestrator before starting the single Ollama container.
- Node: worker API can receive a job, run a sandbox container, and return a result.
- User API Gateway: create task and retrieve task result.

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
