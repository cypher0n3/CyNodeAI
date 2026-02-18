# MVP Phase 1 Chunk 02 Validation Report

- [1 Summary](#1-summary)
- [2 Deliverables Checklist](#2-deliverables-checklist)
  - [2.1 Shared Contracts](#21-shared-contracts)
  - [2.2 Control-Plane Registration Handler](#22-control-plane-registration-handler)
  - [2.3 Node Manager](#23-node-manager)
  - [2.4 No Hard-Coded Endpoint Paths for Follow-on Calls](#24-no-hard-coded-endpoint-paths-for-follow-on-calls)
- [3 Tests](#3-tests)
- [4 Definition of Done (Chunk 02)](#4-definition-of-done-chunk-02)
  - [4.1 CI Failure Detail](#41-ci-failure-detail)
- [5 Recommendation](#5-recommendation)
- [6 References](#6-references)

## 1 Summary

Validated: 2026-02-17.
References: `dev_docs/mvp_phase1_chunk02_agent_instructions.md`, `dev_docs/mvp_phase1_completion_plan.md`.

| Criterion | Status |
|-----------|--------|
| Chunk 02 deliverables (implementation) | **Complete** |
| Definition of Done (`just ci` passes) | **Not met** (coverage gate) |

Chunk 02 implementation is complete.
Bootstrap payload is spec-shaped; Node Manager parses and uses returned URLs; tests cover the new behavior.
The mandatory CI gate fails due to **module-level** Go coverage below 90% for `go_shared_libs`, `orchestrator`, and `worker_node`.
That is a project-wide bar, not limited to Chunk 02 code.

## 2 Deliverables Checklist

Per chunk 02 agent instructions and completion plan Section 3.2.

### 2.1 Shared Contracts

Location: `go_shared_libs/contracts/nodepayloads/nodepayloads.go`.

- **Done.** `BootstrapResponse` includes Phase 1 minimal subset: `version`, `issued_at`; `orchestrator.base_url`, `orchestrator.endpoints` (`worker_registration_url`, `node_report_url`, `node_config_url`); `auth.node_jwt`, `auth.expires_at`.
- Types align with `docs/tech_specs/node_payloads.md` (CYNAI.WORKER.Payload.BootstrapV1).

### 2.2 Control-Plane Registration Handler

Location: `orchestrator/internal/handlers/nodes.go`.

- **Done.** `buildBootstrapResponse` sets `version: 1`, `issued_at` (RFC3339), `orchestrator.base_url`, all three `orchestrator.endpoints.*_url` (absolute URLs), and `auth.node_jwt`, `auth.expires_at`.
- Used for both new and existing node registration.
- JWT is not logged (PayloadSecurity / REQ-WORKER-0131, REQ-WORKER-0132).

### 2.3 Node Manager

Location: `worker_node/cmd/node-manager/main.go`.

- **Done.** Registration response is decoded into `nodepayloads.BootstrapResponse`; `validateBootstrap` checks version, `auth.node_jwt`, and required endpoint URLs.
- `bootstrapData` holds `NodeJWT`, `ExpiresAt`, `NodeReportURL`, `NodeConfigURL` from the payload.
- Capability reporting uses `bootstrap.NodeReportURL` and `bootstrap.NodeJWT` (no hard-coded path for follow-on calls).
- Only `expires_at` is logged at registration; JWT is not logged.

### 2.4 No Hard-Coded Endpoint Paths for Follow-on Calls

- **Done.** Node report URL and config URL come from the bootstrap payload.
- Registration URL is still built from `ORCHESTRATOR_URL + "/v1/nodes/register"` for the initial call only; acceptable per plan (discovery applies to capability and config).

## 3 Tests

- **go_shared_libs/contracts/nodepayloads:** `TestBootstrapResponseJSON` (round-trip JSON, required fields).
- **orchestrator/internal/handlers:** `TestNodeBootstrapResponseJSON`, `TestBuildBootstrapResponse` (bootstrap shape and URLs); integration and mock DB tests assert bootstrap structure and `NodeReportURL`/`NodeConfigURL` in registration response.
- **worker_node/cmd/node-manager:** `TestValidateBootstrap` (version, node_jwt, node_report_url, node_config_url); `TestRegisterUsesBootstrapURLs` (register uses bootstrap-derived `NodeReportURL`).

Chunk 02-related code is covered by unit/integration tests.

## 4 Definition of Done (Chunk 02)

| DoD Item | Result |
|----------|--------|
| Registration returns bootstrap matching `node_bootstrap_payload_v1` minimal subset | Met |
| Node Manager consumes bootstrap and uses returned URLs for follow-on calls | Met |
| Payload secrets not logged / not exposed to sandbox | Met (JWT not logged) |
| `just ci` passes | **Not met** |
| New/changed code has unit test coverage; no linter suppressions | Met for Chunk 02 code |

### 4.1 CI Failure Detail

Command run: `just ci`.

- Lint: **pass** (lint-go, lint-go-ci, lint-python, lint-md).
- `test-go-cover`: **fail** - per-module coverage below 90%: `go_shared_libs` 0.0%, `orchestrator` 51.6%, `worker_node` 12.0%.
- `vulncheck-go`: not reached (runs after test-go-cover).

The coverage requirement is **per module** (each of `go_shared_libs`, `orchestrator`, `worker_node`).
The shortfall is from overall module coverage, not from Chunk 02 code being untested.

## 5 Recommendation

- **Chunk 02 scope:** Treat the chunk **implementation** as complete; all Chunk 02 deliverables and DoD items except the project-wide CI gate are satisfied.
- **Before marking Phase 1 or chunk done in the plan:** Either bring the three Go modules to >=90% coverage and pass `just ci`, or explicitly relax the gate for the current phase and document the decision.

## 6 References

- Spec: `docs/tech_specs/node_payloads.md` (BootstrapV1, PayloadSecurity).
- Requirements: REQ-ORCHES-0112, REQ-ORCHES-0113; REQ-WORKER-0131, REQ-WORKER-0132, REQ-WORKER-0136-0138.
- Plan: `dev_docs/mvp_phase1_completion_plan.md` Section 3.2 (Chunk 02).
- Agent instructions: `dev_docs/mvp_phase1_chunk02_agent_instructions.md`.
