# Proposed Spec Update: `/status` and Underlying Tooling for Detailed Stack Health

- [Scope and Metadata](#scope-and-metadata)
- [Summary](#summary)
- [Goals](#goals)
- [Non-Goals](#non-goals)
- [Proposed Specification Changes](#proposed-specification-changes)
  - [Client requirements](#1-client-requirements)
  - [Backend requirements](#2-backend-requirements)
  - [User API Gateway detailed status endpoint](#3-user-api-gateway-detailed-status-endpoint)
  - [Control-plane optional richer status](#4-control-plane-optional-richer-status)
  - [CLI cynork status](#5-cli-cynork-status)
  - [Chat slash status](#6-chat-slash-status)
  - [Traceability and references](#7-traceability-and-references)
- [PMA Status Source of Truth](#pma-status-source-of-truth)
- [Open Points](#open-points)
- [References](#references)

## Scope and Metadata

- Date: 2026-03-10
- Status: Proposal (draft_specs; not merged to specs)
- Scope: `/status` chat slash command, `cynork status` CLI command, and gateway/control-plane endpoints that back them.

## Summary

Extend the `/status` slash command (and `cynork status`) to return detailed health of every component in the stack, including explicit PMA (Project Manager Agent) up/healthy status, instead of only gateway reachability.

Current behavior (per [cli_management_app_commands_core.md](../tech_specs/cynork/cli_management_app_commands_core.md) and [cli_management_app_commands_chat.md](../tech_specs/cynork/cli_management_app_commands_chat.md)): the CLI calls the gateway `GET /healthz`, treats HTTP 200 with body containing `ok` as healthy, and prints `ok` (table) or `{"gateway":"ok"}` (JSON).
Chat `/status` is defined as "same as cynork status".
There is no visibility into control-plane, MCP gateway, API Egress, PostgreSQL, worker nodes, or PMA.

This proposal adds a detailed status response and keeps backward compatibility with the simple health check where needed.

## Goals

- Users and operators can run `/status` or `cynork status` and see at a glance:
  - User API Gateway: up / unreachable
  - Control-plane (orchestrator): liveness and readiness (including inference path and PMA)
  - PMA: up and healthy (from worker-reported `managed_services_status`), or not yet ready / unhealthy / absent
  - Optionally: MCP Gateway, API Egress, PostgreSQL, and worker nodes (with per-node health)
- Same data supports both chat `/status` and `cynork status` (parity per REQ-CLIENT-0004).
- One gateway endpoint provides the aggregate so clients do not need to call internal ports or multiple services.
- Backward compatibility: existing `GET /healthz` behavior unchanged; new behavior behind a new endpoint or optional query/format.

## Non-Goals

- Changing orchestrator or gateway liveness/readiness semantics (`/healthz`, `/readyz`).
- Exposing internal hostnames or ports to the client in the response (only logical component names and status).
- Replacing the Web Console node/health views; this is a concise summary for CLI/chat.

## Proposed Specification Changes

This section lists concrete spec edits: requirements (client and backend), gateway endpoint, optional control-plane endpoint, CLI and chat behavior, and traceability.

### 1 Client Requirements

Add at least two new requirements in `docs/requirements/client.md`.

- REQ-CLIENT-0XXX (proposed): When the User API Gateway exposes a detailed status endpoint, the CLI and chat `/status` command MUST be able to call it and display the returned stack health to the user.
  Trace: new spec item in CLI core commands and chat slash commands; gateway spec for the detailed status endpoint.

- REQ-CLIENT-0YYY (proposed): The detailed stack health displayed by `/status` and `cynork status` MUST include at least: User API Gateway reachability, control-plane (orchestrator) liveness and readiness, and PMA status (up/healthy or not).
  The detailed stack health MAY additionally include: MCP Gateway, API Egress, PostgreSQL, and worker node health.
  Trace: gateway spec for the detailed status endpoint response shape; CLI and chat spec for what is shown.

### 2 Backend Requirements

Add at least two new requirements in `docs/requirements/usrgwy.md` (User API Gateway) so the backend is obligated to expose and populate the detailed status.

- REQ-USRGWY-0XXX (proposed): The User API Gateway MUST expose an authenticated endpoint (e.g. `GET /v1/status`) that returns aggregated stack component health as JSON for use by the CLI and chat `/status` command.
  Trace: [user_api_gateway.md](../tech_specs/user_api_gateway.md) detailed status endpoint spec.

- REQ-USRGWY-0YYY (proposed): The User API Gateway detailed status response MUST include at least: the gateway's own liveness, control-plane (orchestrator) liveness and readiness (with an actionable reason when not ready), and PMA status.
  PMA status MUST be derived from worker-reported `managed_services_status` (same source as orchestrator readiness).
  The response MAY additionally include MCP Gateway, API Egress, PostgreSQL, and worker node health.
  Trace: gateway spec response shape; [worker_node_payloads.md](../tech_specs/worker_node_payloads.md) `managed_services_status`; [orchestrator_bootstrap.md](../tech_specs/orchestrator_bootstrap.md).

### 3 User API Gateway Detailed Status Endpoint

- Spec ID (proposed): `CYNAI.USRGWY.DetailedStatusEndpoint`
- Location: [user_api_gateway.md](../tech_specs/user_api_gateway.md) (new subsection under Support for Cynork Chat Slash Commands or Data REST API).

#### 3.1 Endpoint Contract

- `GET /v1/status` (or `GET /status`; prefer under `/v1/` for consistency with Data REST API).
- Authentication: same as other read-only user endpoints (authenticated).
- Response: `200 OK` with a JSON body describing component health.

#### 3.2 Response Shape (Proposed)

- `gateway`: `{ "status": "ok" | "unreachable" }` (gateway's own liveness; when the client gets 200, this is always `ok`).
- `orchestrator`: `{ "liveness": "ok" | "unreachable", "readiness": "ready" | "not_ready", "reason": "<string>" }`.
  When readiness is `not_ready`, `reason` MUST be present and actionable (e.g. "no inference path", "PMA not ready").
- `pma`: `{ "status": "ready" | "starting" | "unhealthy" | "absent" | "unknown", "source": "worker_reported" | "none" }`.
  Values align with `managed_services_status.services[].state` where applicable; `absent` when no worker has reported a PMA service; `unknown` when gateway cannot determine (e.g. store error).
- Optional components (gateway MAY include when it has access to probe or store):
  - `mcp_gateway`: `{ "status": "ok" | "unreachable" }`
  - `api_egress`: `{ "status": "ok" | "unreachable" }`
  - `postgres`: `{ "status": "ok" | "unreachable" }` (if the gateway or control-plane can expose this without raw DB access)
  - `nodes`: array of `{ "node_id": "<id>", "status": "ok" | "unreachable" | "unknown", "last_heartbeat": "<RFC3339>" }` (from existing node list + health/heartbeat data already available to orchestrator).

#### 3.3 Implementation Note

- Gateway obtains orchestrator liveness and readiness by calling control-plane `GET /healthz` and `GET /readyz` (internal network).
- PMA status is derived from the same store/capability data the gateway or control-plane already uses for readiness (worker-reported `managed_services_status` with `service_type`/`service_id` for PMA and `state` = `ready` | `starting` | `unhealthy` | etc.).
- Optional components require the gateway (or a control-plane helper) to probe internal services or read from store.
  See [worker_node_payloads.md](../tech_specs/worker_node_payloads.md) `managed_services_status`, [orchestrator_bootstrap.md](../tech_specs/orchestrator_bootstrap.md).

### 4 Control-Plane Optional Richer Status

- Current: `GET /readyz` returns 200 with body `ready` or 503 with an actionable reason string.
  No JSON breakdown.
- Proposed (optional): control-plane MAY expose a dedicated endpoint, e.g. `GET /v1/status` or `GET /status`, returning JSON: `{ "ready": bool, "reason": string, "components": { "pma": "ready"|"starting"|"unhealthy"|"absent", "inference_path": bool } }`.
  This would allow the user-gateway to call one internal endpoint instead of combining healthz + readyz + store queries.
- If control-plane does not add this, the user-gateway MUST derive the same information from existing `/healthz`, `/readyz`, and store (node capabilities / managed_services_status).

### 5 CLI Cynork Status

- Spec ID (proposed): extend `CYNAI.CLIENT.CliStatus` in [cli_management_app_commands_core.md](../tech_specs/cynork/cli_management_app_commands_core.md).

#### 5.1 CLI Status Behavior

- The CLI MUST call the gateway health endpoint for backward compatibility when no "detailed" output is requested.
- When detailed status is requested (e.g. `cynork status --detailed` or `cynork status -v`), the CLI MUST call `GET /v1/status` (or the chosen detailed endpoint).
  If the endpoint returns 200, the CLI MUST display the component breakdown.
  If the endpoint is not implemented (404), the CLI MUST fall back to existing behavior (gateway `GET /healthz` and print `ok` or `gateway: ok`).
- Table mode (detailed): one line per component, e.g. `gateway=ok orchestrator=ready pma=ready mcp_gateway=ok api_egress=ok` or a short multi-line summary.
- JSON mode (detailed): output the gateway response JSON (or a normalized subset) so scripts can parse it.
- Exit code: 0 when gateway is reachable and (if detailed) when overall stack is acceptable per policy; 7 when gateway is unreachable or (optional) when detailed status indicates not ready and a strict flag is set.

#### 5.2 Backward Compatibility

- `cynork status` with no flags continues to call `GET /healthz` and print `ok` / `{"gateway":"ok"}` so existing scripts and chat behavior remain valid.
- Detailed view is opt-in unless we later make it the default and keep `--short` for the old behavior.

### 6 Chat Slash Status

- Spec ID (proposed): extend `CYNAI.CLIENT.CliChatSlashStatus` in [cli_management_app_commands_chat.md](../tech_specs/cynork/cli_management_app_commands_chat.md).

#### 6.1 Chat Status Behavior

- `/status` MUST show gateway reachability (current behavior).
- `/status` SHOULD show detailed stack health (gateway, orchestrator, PMA, and optionally MCP Gateway, API Egress, nodes) when the gateway exposes the detailed status endpoint.
  Display format is implementation-defined (e.g. inline table or short summary in chat).
- If the detailed endpoint is not available, `/status` MUST show at least gateway reachability (same as today).

### 7 Traceability and References

- New/updated requirements: REQ-CLIENT-0167 (unchanged or extended), REQ-CLIENT-0XXX and REQ-CLIENT-0YYY (client); REQ-USRGWY-0XXX and REQ-USRGWY-0YYY (backend gateway).
- Specs: `cli_management_app_commands_core.md`, `cli_management_app_commands_chat.md`, `user_api_gateway.md`; optionally `orchestrator.md` if control-plane adds a detailed status endpoint.

## PMA Status Source of Truth

- PMA is up/healthy when at least one worker has reported, in `managed_services_status.services[]`, a service with the designated PMA `service_id`/`service_type` and `state=ready`.
- The orchestrator already uses this for readiness ([orchestrator_bootstrap.md](../tech_specs/orchestrator_bootstrap.md), [orchestrator.md](../tech_specs/orchestrator.md) HealthEndpoints).
- The gateway (or control-plane) MUST derive PMA status from the same data: e.g. scan stored node capability reports for `managed_services_status.services[]` where `service_type` is PMA and take the best state (e.g. ready if any, else starting, unhealthy, etc.) or `absent` if none.

## Open Points

- Exact endpoint path: `GET /v1/status` vs `GET /status` (recommend `/v1/status`).
- Default for `cynork status`: always detailed vs detailed only with `--detailed`/`-v` (recommend detailed as default, `--short` for legacy).
- Whether control-plane should expose its own `GET /v1/status` or gateway aggregates from existing healthz/readyz + store.
- Whether to include PostgreSQL and optional components in the first implementation or phase them.

## References

- [REQ-CLIENT-0167](../requirements/client.md#req-client-0167) (slash commands `/status`, `/whoami`)
- [cli_management_app_commands_core.md](../tech_specs/cynork/cli_management_app_commands_core.md) (`cynork status`)
- [cli_management_app_commands_chat.md](../tech_specs/cynork/cli_management_app_commands_chat.md) (Status and Identity Slash Commands)
- [user_api_gateway.md](../tech_specs/user_api_gateway.md) (Support for Cynork Chat Slash Commands)
- [orchestrator.md](../tech_specs/orchestrator.md) (Health Checks)
- [orchestrator_bootstrap.md](../tech_specs/orchestrator_bootstrap.md) (PMA startup, worker-reported status)
- [worker_node_payloads.md](../tech_specs/worker_node_payloads.md) (`managed_services_status`)
