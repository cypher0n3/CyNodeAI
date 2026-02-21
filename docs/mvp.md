# Minimum Viable Product

## Overview

This document outlines what is the minimum viable feature set for an initial (alpha) release of CyNodeAI.

### In-Scope for MVP

For this document, "MVP" refers to completing the full phased build-out from Phase 1 through Phase 4.
Phase 0 items are treated as prerequisites for the Phase 1 through Phase 4 MVP build-out.

The canonical "what" is defined by [`docs/requirements/`](requirements/README.md), and the implementation guidance ("how") is defined by [`docs/tech_specs/`](tech_specs/_main.md).

The current implementation-oriented breakdown (4-8 hour chunks) is maintained in [`mvp_plan.md`](./mvp_plan.md).

#### MVP Target Capabilities

- **Foundations (Phase 0 prerequisites)**.
  - PostgreSQL schema for identity/auth, tasks/jobs, nodes, artifacts, and auditing.
    See [`docs/tech_specs/postgres_schema.md`](tech_specs/postgres_schema.md).
  - Canonical node payloads (capability report, bootstrap, config delivery, config ack).
    See [`docs/tech_specs/node_payloads.md`](tech_specs/node_payloads.md).
  - MCP gateway enforcement rules and initial allowlists (spec definition, with runtime integration in Phase 2).
    See [`docs/tech_specs/mcp_gateway_enforcement.md`](tech_specs/mcp_gateway_enforcement.md).
  - LangGraph MVP workflow contract (spec definition, with runtime integration in Phase 2).
    See [`docs/tech_specs/langgraph_mvp.md`](tech_specs/langgraph_mvp.md).

- **Single-node end-to-end execution (Phase 1)**.
  - Orchestrator control-plane supports node registration (PSK => JWT), capability ingest, config delivery, job dispatch, and result collection.
    See [`docs/tech_specs/orchestrator.md`](tech_specs/orchestrator.md) and [`docs/tech_specs/node_payloads.md`](tech_specs/node_payloads.md).
  - Worker node runs Node Manager and Worker API, executes sandbox jobs in containers, and returns bounded results.
    See [`docs/tech_specs/node.md`](tech_specs/node.md), [`docs/tech_specs/worker_api.md`](tech_specs/worker_api.md), and [`docs/tech_specs/sandbox_container.md`](tech_specs/sandbox_container.md).
  - User API Gateway supports local auth and task create/get/result.
    See [`docs/tech_specs/user_api_gateway.md`](tech_specs/user_api_gateway.md).
  - Readiness gating for inference availability and Project Manager model selection and warmup.
    See [`docs/tech_specs/orchestrator_bootstrap.md`](tech_specs/orchestrator_bootstrap.md) and [`docs/tech_specs/orchestrator.md`](tech_specs/orchestrator.md#project-manager-model-startup-selection-and-warmup).
  - Task input semantics:
    - Prompt text (plain text or Markdown) is interpreted by default and uses inference by default.
    - Script and commands modes are explicit raw execution modes and MUST run in the sandbox.
    See [`docs/requirements/orches.md`](requirements/orches.md) (REQ-ORCHES-0125, REQ-ORCHES-0126, REQ-ORCHES-0127) and [`docs/tech_specs/user_api_gateway.md`](tech_specs/user_api_gateway.md).

- **Single-node full capability slice (Phase 1.5)**.
  - Inference from inside the sandbox via node-local proxy sidecar and `OLLAMA_BASE_URL`.
    See [`docs/tech_specs/node.md`](tech_specs/node.md) and [`docs/tech_specs/sandbox_container.md`](tech_specs/sandbox_container.md).
  - CLI management app (cynork) exists as a separate Go module and can perform basic auth and task operations against the User API Gateway.
    See [`docs/tech_specs/cli_management_app.md`](tech_specs/cli_management_app.md).

- **MCP in the loop (Phase 2)**.
  - Orchestrator MCP tool gateway enforces allowlists, scoping, and auditing for tool calls in the runtime loop.
    See [`docs/tech_specs/mcp_gateway_enforcement.md`](tech_specs/mcp_gateway_enforcement.md) and [`docs/tech_specs/mcp_tool_call_auditing.md`](tech_specs/mcp_tool_call_auditing.md).
  - Orchestrator-side agents use MCP database tools (no direct PostgreSQL access).
    See [`docs/tech_specs/mcp_tooling.md`](tech_specs/mcp_tooling.md).
  - LangGraph MVP workflow drives tasks with persisted checkpoints and resumability.
    See [`docs/tech_specs/langgraph_mvp.md`](tech_specs/langgraph_mvp.md).

- **Multi-node robustness (Phase 3)**.
  - Node selection (capability, load, data locality, model availability).
  - Job leases, retries, idempotency, and heartbeats.
  - Dynamic node configuration updates and capability change reporting.
    See [`docs/tech_specs/orchestrator.md`](tech_specs/orchestrator.md), [`docs/tech_specs/node.md`](tech_specs/node.md), and [`docs/tech_specs/node_payloads.md`](tech_specs/node_payloads.md).
  - Worker Telemetry API integration for node operational signals.
    See [`docs/tech_specs/worker_telemetry_api.md`](tech_specs/worker_telemetry_api.md).

- **Controlled egress and integrations (Phase 4)**.
  - API Egress Server for controlled outbound API calls with policy enforcement and auditing.
    See [`docs/tech_specs/api_egress_server.md`](tech_specs/api_egress_server.md) and [`docs/tech_specs/access_control.md`](tech_specs/access_control.md).
  - External model routing via API Egress, including standalone fallback and external inference with node sandboxes (model via API Egress, sandbox for tools).
    See [`docs/tech_specs/external_model_routing.md`](tech_specs/external_model_routing.md).
  - Secure Browser Service (optional) for controlled web access and deterministic sanitization.
    See [`docs/tech_specs/secure_browser_service.md`](tech_specs/secure_browser_service.md).
  - CLI expansion for credentials, preferences, skills, and node management.
    See [`docs/tech_specs/cli_management_app.md`](tech_specs/cli_management_app.md) and [`docs/tech_specs/skills_storage_and_inference.md`](tech_specs/skills_storage_and_inference.md).

### Deferred Until After MVP

Items in this section are explicitly out of scope for the Phase 1 through Phase 4 MVP build-out.
They remain part of the overall roadmap and are covered by later work beyond Phase 4.

Deferred capabilities (explicit)

- **Admin web console**.
  See [`docs/tech_specs/admin_web_console.md`](tech_specs/admin_web_console.md).
- **Web egress proxy**.
  See [`docs/tech_specs/web_egress_proxy.md`](tech_specs/web_egress_proxy.md).
- **Git egress MCP**.
  See [`docs/tech_specs/git_egress_mcp.md`](tech_specs/git_egress_mcp.md).
- **Connector framework**.
  See [`docs/tech_specs/connector_framework.md`](tech_specs/connector_framework.md).
- **Runs and sessions API, streaming status, logs, and transcript retention**.
  See [`docs/tech_specs/runs_and_sessions_api.md`](tech_specs/runs_and_sessions_api.md).
- **Worker API long-running session sandboxes and interactive PTY mode**.
  See [`docs/requirements/worker.md`](requirements/worker.md) (REQ-WORKER-0150, REQ-WORKER-0151, REQ-WORKER-0153) and [`docs/tech_specs/worker_api.md`](tech_specs/worker_api.md).

### Phased MVP Implementation Plan

This phased plan summarizes the MVP roadmap in terms of prerequisites (Phase 0) and MVP build-out phases (Phase 1 through Phase 4).
For the full task breakdown with requirement and spec references, see [`docs/mvp_plan.md`](mvp_plan.md).

#### Phase 0 Foundations

- Define Postgres schema for users, local auth sessions, groups and RBAC, tasks, jobs, nodes, artifacts, and audit logging.
  See [`docs/tech_specs/postgres_schema.md`](tech_specs/postgres_schema.md).
- Define node capability report payload and node configuration payload.
  See [`docs/tech_specs/node_payloads.md`](tech_specs/node_payloads.md).
  - Specify registration-time bootstrap payload (PSK to JWT) and config versioning.
  - Specify capability report fields, hashing, and change reporting behavior.
  - Specify configuration refresh, acknowledgement payload, and rollback reporting.
- Define MCP gateway enforcement and initial tool allowlists by role.
  See [`docs/tech_specs/mcp_gateway_enforcement.md`](tech_specs/mcp_gateway_enforcement.md).
- Define the LangGraph MVP workflow contract and checkpointing requirements.
  See [`docs/tech_specs/langgraph_mvp.md`](tech_specs/langgraph_mvp.md).

#### Phase 1 Single Node Happy Path (MVP Phase 1)

- Orchestrator: node registration (PSK to JWT), capability ingest, config delivery, job dispatch, result collection.
  See [`docs/tech_specs/orchestrator.md`](tech_specs/orchestrator.md).
- Job dispatch: direct HTTP to Worker API using per-node URL and token from config delivery.
  MCP gateway is not in the loop for Phase 1.
  See [`docs/tech_specs/worker_api.md`](tech_specs/worker_api.md) and [`docs/tech_specs/node_payloads.md`](tech_specs/node_payloads.md).
- Node: Node Manager startup sequence that contacts orchestrator before starting the single Ollama container.
  See [`docs/tech_specs/node.md`](tech_specs/node.md).
- Node: Worker API can receive a job, run a sandbox container, and return a result.
  See [`docs/tech_specs/worker_api.md`](tech_specs/worker_api.md) and [`docs/tech_specs/sandbox_container.md`](tech_specs/sandbox_container.md).
- System: at least one inference-capable path must be available (node-local inference or external model routing with a configured provider key).
  In the single-node case, the system MUST refuse to enter a ready state if local inference is unavailable and no external provider key is configured.
  See [`docs/tech_specs/orchestrator_bootstrap.md`](tech_specs/orchestrator_bootstrap.md) and [`docs/tech_specs/external_model_routing.md`](tech_specs/external_model_routing.md).
- Orchestrator: on startup, select and warm up the effective Project Manager model.
  See [`docs/tech_specs/orchestrator.md`](tech_specs/orchestrator.md#project-manager-model-startup-selection-and-warmup).
- User API Gateway: local user auth (login and refresh), create task, and retrieve task result.
  See [`docs/tech_specs/user_api_gateway.md`](tech_specs/user_api_gateway.md).
- Phase 1 config refresh: node fetches configuration on startup only (no polling).
- Phase 1 node JWT: long-lived; node re-registers on expiry.
- Phase 1 workflow engine: tasks are executed as a single dispatched sandbox job.
  LangGraph is not integrated in the Phase 1 runtime loop.
- Task creation: user-facing input is plain text or Markdown, attachments, script, or a short series of commands.
  For script or commands, the system runs them in the sandbox.
  For plain text or Markdown, the system interprets the input and uses inference by default.
  The user task text MUST NOT be executed as a literal shell command unless the user explicitly selects a raw execution mode (script or commands).
  See [`docs/requirements/orches.md`](requirements/orches.md) (REQ-ORCHES-0125, REQ-ORCHES-0126, REQ-ORCHES-0127) and [`docs/tech_specs/user_api_gateway.md`](tech_specs/user_api_gateway.md).

#### Phase 1.5 Single Node Full Capability

- Enable node-local inference access from inside the sandbox.
  Implement the inference proxy sidecar approach so sandboxes can call `http://localhost:11434` without leaving the node.
  See [`docs/tech_specs/node.md`](tech_specs/node.md) and [`docs/tech_specs/sandbox_container.md`](tech_specs/sandbox_container.md).
- Extend E2E to exercise inference inside the sandbox for the single-node deployment.
- Add the minimum viable CLI slice as a separate Go module.
  See [`docs/tech_specs/cli_management_app.md`](tech_specs/cli_management_app.md).

#### Phase 2 MCP in the Loop

- Implement orchestrator MCP tool gateway with role-based access.
- Add MCP database tools for orchestrator-side agents and MCP artifact tools for worker agents.
- Ensure orchestrator-side agents use MCP database tools and do not connect to Postgres directly.
- Integrate the LangGraph MVP workflow as the orchestrator workflow engine for the Project Manager Agent.
  See [`docs/tech_specs/langgraph_mvp.md`](tech_specs/langgraph_mvp.md).

#### Phase 3 Multi Node Robustness

- Add node selection based on capability, load, data locality, and model availability.
- Add job leases, retries, idempotency, and heartbeats.
- Add dynamic node configuration updates and startup capability change reporting.
- Add Worker Telemetry API integration for node health and operational signals.
  See [`docs/tech_specs/worker_telemetry_api.md`](tech_specs/worker_telemetry_api.md).

#### Phase 4 Optional Controlled Egress and Integrations

- Add API Egress Server with ACL enforcement and auditing.
  See [`docs/tech_specs/api_egress_server.md`](tech_specs/api_egress_server.md).
- Add Secure Browser Service with deterministic sanitization and DB-backed rules.
  See [`docs/tech_specs/secure_browser_service.md`](tech_specs/secure_browser_service.md).
- Add external model routing per [`docs/tech_specs/external_model_routing.md`](tech_specs/external_model_routing.md).
  This includes routing policy and signals, external inference with node sandboxes, configurable settings, and per-agent overrides for Project Manager and Project Analyst.
- Expand the CLI management app surface for credentials, user preferences, skills, and node management.
  See [`docs/tech_specs/cli_management_app.md`](tech_specs/cli_management_app.md) and [`docs/tech_specs/skills_storage_and_inference.md`](tech_specs/skills_storage_and_inference.md).
- Defer the admin web console until after the CLI exists.
