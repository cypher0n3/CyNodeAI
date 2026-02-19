# Chunk 03 (Node Config Delivery) - Gaps and Issues Report

- [Summary](#summary)
- [Gaps and Issues](#gaps-and-issues)
- [What was verified](#what-was-verified)
- [References](#references)
- [Validation Performed](#validation-performed)

## Summary

**Report generated:** 2026-02-19T05:46:34-05:00

**Scope:** Review of work performed for MVP Phase 1 Chunk 03 (Implement Minimum Node Config Delivery API in the Control Plane) per `dev_docs/mvp_phase1_completion_plan.md` Section 4.3 and `dev_docs/chunk03_node_config_delivery_report.md`.

Chunk 03 deliverables are implemented and validated with unit/integration tests and lint.
The following gaps and issues were identified for follow-up or documentation.
**Status:** All listed items have been addressed (see resolution notes below).

## Gaps and Issues

The following items were gaps or minor issues; each has been addressed.

### 1. Documentation Typo in Completion Report (Resolved)

In `dev_docs/chunk03_node_config_delivery_report.md`, the GET config endpoint was written as:

- **As written:** `GET /V1/nodes/config/nodes/config`
- **Correct:** `GET /v1/nodes/config`

**Resolution:** Fixed in the completion report.
Headings and text now use `GET /v1/nodes/config` and `POST /v1/nodes/config` (lowercase `v1`, no duplicated path).

### 2. BDD Steps for Config Delivery (Resolved)

The feature file defines scenarios for config fetch and config acknowledgement; the step definitions for these flows had been stubbed with `godog.ErrSkip`.

**Resolution:** Config-related step definitions in `orchestrator/_bdd/steps.go` were implemented.
The scenarios "Node fetches config after registration", "Node sends config acknowledgement", "GET config without node JWT returns 401", and "Config ack with wrong node_slug is rejected" now execute against the Chunk 03 API when `POSTGRES_TEST_DSN` is set.
The completion report documents the BDD feature and steps.

### 3. Config Ack Error Storage: Message Only (Documented)

`docs/tech_specs/node_payloads.md` defines `node_config_ack_v1.error` as an object with `type`, `message`, and optional `details`.
The implementation persists only the error message in `config_ack_error`.

**Resolution:** Documented in `dev_docs/chunk03_node_config_delivery_report.md` under Config acknowledgement: Phase 1 stores only the error message; type and details can be added in a later phase if needed.

### 4. Plan Validation vs. Full CI (Noted)

Chunk 03 validation required `just test-go` and `just lint-go-ci`.
Full local CI (`just ci`) also runs the BDD suite.

**Resolution:** No change required.
With BDD config steps implemented, `just ci` now exercises the config delivery API via BDD when a database is available.

## What Was Verified

- GET `/v1/nodes/config` and POST `/v1/nodes/config` are registered in the control-plane with node JWT auth.
- Handlers return/accept the normative payload shapes from `docs/tech_specs/node_payloads.md` (node_configuration_payload_v1, node_config_ack_v1).
- Per-node config_version is set on new node init and when GET config is called for a node with nil version; Worker API bearer token and target URL are included when configured.
- Config ack validates version, node_slug, and status; persists config_ack_at, config_ack_status, config_ack_error.
- Unit tests in `orchestrator/internal/handlers/handlers_mockdb_test.go` and DB integration tests in `orchestrator/internal/database/integration_test.go` cover config get/ack paths.
- Contracts in `go_shared_libs/contracts/nodepayloads/nodepayloads.go` and node model/schema align with `docs/tech_specs/postgres_schema.md`.

## References

- Plan: `dev_docs/mvp_phase1_completion_plan.md` Section 4.3.
- Completion report: `dev_docs/chunk03_node_config_delivery_report.md`.
- Specs: `docs/tech_specs/node_payloads.md`, `docs/tech_specs/worker_api.md`, `docs/tech_specs/postgres_schema.md`.
- Implementation: `orchestrator/internal/handlers/nodes.go`, `orchestrator/cmd/control-plane/main.go`, `orchestrator/internal/database/nodes.go`, `orchestrator/_bdd/steps.go`.

## Validation Performed

Validation run to confirm all reported gaps are closed:

| Gap | Status | Evidence |
|-----|--------|----------|
| 1. Doc typo | **Closed** | `dev_docs/chunk03_node_config_delivery_report.md` headings corrected to `GET /v1/nodes/config` and `POST /v1/nodes/config` (lowercase `v1`, no duplicated path). |
| 2. BDD steps stubbed | **Closed** | `orchestrator/_bdd/steps.go`: config steps perform real HTTP GET/POST to `/v1/nodes/config`, parse payloads, assert config_version and worker_api token, send ack, verify DB has config_ack_status and config_version. |
| 3. Config ack error storage | **Closed** | Documented in completion report (Config acknowledgement): "Phase 1 stores only the error message... Type and details can be added in a later phase." |
| 4. Plan validation vs CI | **Closed** | BDD config scenarios run when `POSTGRES_TEST_DSN` is set; `just ci` exercises config delivery via BDD. |

All four gaps are now closed.
