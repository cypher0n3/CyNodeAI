# Chunk 03: Node Config Delivery API - Completion Report

- [Summary](#summary)
- [Deliverables](#deliverables)
- [Validation](#validation)
- [References](#references)

## Summary

Completed MVP Phase 1 Chunk 03: **Implement Minimum Node Config Delivery API in the Control Plane** (see `dev_docs/mvp_phase1_completion_plan.md` Section 4.3).

Date: 2026-02-19.

## Deliverables

Implemented the following per the plan.

### Node Configuration Retrieval (GET /V1/nodes/config/nodes/config)

- Control-plane returns normative `node_configuration_payload_v1` from `docs/tech_specs/node_payloads.md`.
- Requires node JWT auth; returns 401 without auth, 404 if node not found.
- Payload includes `config_version`, `issued_at`, `node_slug`, `orchestrator.base_url` and `orchestrator.endpoints` (`worker_api_target_url`, `node_report_url`), minimal `sandbox_registry` and `model_cache`, and `worker_api.orchestrator_bearer_token` when configured.

### Config Acknowledgement (POST /V1/nodes/config/nodes/config)

- Accepts `node_config_ack_v1` body; validates `version`, `node_slug` (must match authenticated node), and `status` (`applied` or `failed`).
- Records acknowledgement on the node row: `config_ack_at`, `config_ack_status`, `config_ack_error` (optional).
- Phase 1 stores only the error message in `config_ack_error`; the spec's `node_config_ack_v1.error` object (type, message, details) is accepted but only `message` is persisted.
  Type and details can be added in a later phase if needed.
- Returns 204 on success.

### Per-Node Config Version

- `config_version` stored on `nodes` table; initial version `"1"` set on new node registration (`initializeNewNode`) and when GET config is first called for a node with nil version.
- Emitted in the config payload.

### Worker API Bearer Token

- Orchestrator config: `WORKER_API_BEARER_TOKEN` (default `phase1-static-token`), `WORKER_API_TARGET_URL` (optional).
- Token included in config payload as `worker_api.orchestrator_bearer_token` so nodes can authenticate Worker API requests per `docs/tech_specs/worker_api.md`.
- `worker_api_target_url` in payload: from node row if set, else from `WORKER_API_TARGET_URL`.

### Bootstrap and Routes

- `node_config_url` and `node_report_url` were already emitted in bootstrap (Chunk 02); paths are GET/POST `/v1/nodes/config` and POST `/v1/nodes/capability`.
- Routes registered in control-plane with node auth middleware.

### Contracts and Schema

- `go_shared_libs/contracts/nodepayloads`: added `NodeConfigurationPayload`, `ConfigAck`, and nested types per spec.
- Node model extended with `WorkerAPITargetURL`, `WorkerAPIBearerToken`, `ConfigAckAt`, `ConfigAckStatus`, `ConfigAckError` per `docs/tech_specs/postgres_schema.md`.
- Store: `UpdateNodeConfigVersion`, `UpdateNodeConfigAck`; MockDB and integration tests updated.

## Validation

- `just test-go`: all modules pass (coverage threshold met).
- `just lint-go-ci`: no issues.
- `just ci`: passes (includes feature file validation and orchestrator BDD suite).

## BDD Feature Files and Steps

- **Feature**: `features/orchestrator/node_registration_and_config.feature`.
- **Scenarios** (Chunk 03):
  - Node fetches config after registration: register then GET config; assert payload has config_version and worker_api bearer token.
  - Node sends config acknowledgement: given node that has received config, POST config ack with status "applied"; assert orchestrator records ack and config_version stored.
  - GET config without node JWT returns 401: unauthenticated GET config; assert 401.
  - Config ack with wrong node_slug is rejected: POST ack with node_slug "wrong-slug"; assert 400.
- **Step definitions**: `orchestrator/_bdd/steps.go` (config request, config payload assertions, config ack send, ack/version stored, unauthenticated request, status code assertions).
  BDD test server wires GET/POST `/v1/nodes/config` with node auth and passes `WorkerAPIBearerToken` and `WorkerAPITargetURL` from config to `NewNodeHandler`.

## References

- Plan: `dev_docs/mvp_phase1_completion_plan.md` Section 4.3.
- Specs: `docs/tech_specs/node_payloads.md`, `docs/tech_specs/worker_api.md`, `docs/tech_specs/postgres_schema.md`.
- Implementation: `orchestrator/internal/handlers/nodes.go` (GetConfig, ConfigAck), `orchestrator/cmd/control-plane/main.go` (routes), `orchestrator/internal/database/nodes.go`, `go_shared_libs/contracts/nodepayloads/nodepayloads.go`.
