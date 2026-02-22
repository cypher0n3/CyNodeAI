# Technical Specifications Index

- [Document Overview](#document-overview)
- [Architecture Summary](#architecture-summary)
- [Tech Spec Index](#tech-spec-index)
  - [Orchestrator and Nodes](#orchestrator-and-nodes)
  - [Ports and Endpoints](#ports-and-endpoints)
  - [User Interfaces](#user-interfaces)
  - [API Specifications](#api-specifications)
  - [Agents and Connectors](#agents-and-connectors)
  - [Identity, Policy, and Data](#identity-policy-and-data)
  - [Protocols and Standards](#protocols-and-standards)
  - [Execution and Artifacts](#execution-and-artifacts)
  - [External Integration and Routing](#external-integration-and-routing)
  - [Model Lifecycle](#model-lifecycle)
  - [AI Skills](#ai-skills)
  - [Agents Specification](#agents-specification)
  - [Bootstrap Configurations](#bootstrap-configurations)

## Document Overview

This document is the entrypoint for CyNodeAI technical specifications.
It provides a summary of the system and links to detailed specs.

For MVP scope and the phased MVP plan, see [`docs/mvp.md`](../mvp.md).

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

### Ports and Endpoints

- Ports and endpoints: [`docs/tech_specs/ports_and_endpoints.md`](ports_and_endpoints.md)

### User Interfaces

- CLI management app: [`docs/tech_specs/cli_management_app.md`](cli_management_app.md)
- Web Console: [`docs/tech_specs/web_console.md`](web_console.md)

### API Specifications

- User API Gateway: [`docs/tech_specs/user_api_gateway.md`](user_api_gateway.md)
- OpenAI-compatible chat API: [`docs/tech_specs/openai_compatible_chat_api.md`](openai_compatible_chat_api.md)
- Data REST API: [`docs/tech_specs/data_rest_api.md`](data_rest_api.md)
- Runs and sessions API: [`docs/tech_specs/runs_and_sessions_api.md`](runs_and_sessions_api.md)
- Chat threads and messages: [`docs/tech_specs/chat_threads_and_messages.md`](chat_threads_and_messages.md)
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
- Web egress proxy: [`docs/tech_specs/web_egress_proxy.md`](web_egress_proxy.md)
- Git egress MCP: [`docs/tech_specs/git_egress_mcp.md`](git_egress_mcp.md)
- External model routing: [`docs/tech_specs/external_model_routing.md`](external_model_routing.md)

### Model Lifecycle

- Model management: [`docs/tech_specs/model_management.md`](model_management.md)

### AI Skills

- Skills storage and inference exposure: [`docs/tech_specs/skills_storage_and_inference.md`](skills_storage_and_inference.md)

### Agents Specification

- Project Manager Agent: [`docs/tech_specs/project_manager_agent.md`](project_manager_agent.md)
- Project Analyst Agent: [`docs/tech_specs/project_analyst_agent.md`](project_analyst_agent.md)
- LangGraph MVP workflow: [`docs/tech_specs/langgraph_mvp.md`](langgraph_mvp.md)

### Bootstrap Configurations

- Orchestrator bootstrap: [`docs/tech_specs/orchestrator_bootstrap.md`](orchestrator_bootstrap.md)
