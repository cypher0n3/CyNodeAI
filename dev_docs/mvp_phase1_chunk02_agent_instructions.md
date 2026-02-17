# MVP Phase 1 - Chunk 02 Agent Instructions

## Overview

This document is an execution-ready instruction set for implementing Chunk 02 from `dev_docs/mvp_phase1_completion_plan.md`.
Scope is limited to code changes needed to make the node registration bootstrap payload spec-compliant enough to enable config delivery.

## Goal

Make node registration return `node_bootstrap_payload_v1` with the Phase 1 minimal subset fields.
Make the Node Manager parse and use the returned endpoint URLs instead of hard-coded paths.

## Non-Goals

Do not implement node configuration delivery endpoints in this chunk.
Do not implement config acknowledgement handling in this chunk.
Do not implement TLS trust delivery, trust pinning, or initial config version hints in this chunk.
Do not change tech specs or requirements documents in this chunk.

## Inputs

- Chunk 02 definition and tasks in `dev_docs/mvp_phase1_completion_plan.md`.
- Canonical payload shape in `docs/tech_specs/node_payloads.md` under `node_bootstrap_payload_v1`.

### Requirements and Spec IDs

Implement the bootstrap response so it satisfies the following.
Do not change the requirement or spec documents; use them as the source of truth.

**Spec IDs** ([`docs/tech_specs/node_payloads.md`](../docs/tech_specs/node_payloads.md)):

- **`CYNAI.WORKER.Doc.NodePayloads`** - document; canonical payload shapes and versioning.
- **`CYNAI.WORKER.Payload.BootstrapV1`** - bootstrap payload schema `node_bootstrap_payload_v1` (registration response).
- **`CYNAI.WORKER.PayloadSecurity`** - secrets in payloads; do not expose to sandboxes; node-local secure store.
- **`CYNAI.WORKER.Payload.CompatibilityVersioning`** - optional new fields; no meaning change within same `version`.

#### Requirement IDs

- **REQ-ORCHES-0112** ([`docs/requirements/orches.md`](../docs/requirements/orches.md#req-orches-0112)): orchestrator MUST be able to configure worker nodes at registration time.
- **REQ-ORCHES-0113** ([`docs/requirements/orches.md`](../docs/requirements/orches.md#req-orches-0113)): orchestrator MUST support dynamic configuration updates after registration and must ingest node capability reports on registration and node startup.
- **REQ-WORKER-0131**, **REQ-WORKER-0132** ([`docs/requirements/worker.md`](../docs/requirements/worker.md#req-worker-0131)): payload security; secrets short-lived where possible; not exposed to sandbox; nodes store secrets in node-local secure store.
- **REQ-WORKER-0133**, **REQ-WORKER-0134** ([`docs/requirements/worker.md`](../docs/requirements/worker.md#req-worker-0133)): registry/cache tokens and rotation (trace to BootstrapV1; for Chunk 02 focus on bootstrap shape only).
- **REQ-WORKER-0136**, **REQ-WORKER-0137**, **REQ-WORKER-0138** ([`docs/requirements/worker.md`](../docs/requirements/worker.md#req-worker-0136)): payload compatibility; no meaning change within same version; nodes should reject unsupported version.

When adding comments or tests, prefer referencing these spec and requirement IDs (e.g. in code comments or test names) over inlining long spec text.

## Project Standards and Testing Requirements

Follow these project-wide rules for all work.
Authoritative sources: [`meta.md`](../meta.md), [`ai_files/ai_coding_instructions.md`](../ai_files/ai_coding_instructions.md), [`.github/copilot-instructions.md`](../.github/copilot-instructions.md).

### Tooling and Commands

- Use **justfile targets only** for setup, lint, and test; do not run raw `go test`, `golangci-lint`, or script paths directly.
- Run all commands from the repository root.

### CI Gate

- Work is not complete until **`just ci`** passes.
- `just ci` runs: `lint-go`, `lint-go-ci`, `lint-python`, `lint-md`, **`test-go-cover`**, `vulncheck-go`.

### Code Coverage

- **Minimum 90% Go coverage per module** is required.
- `just test-go-cover` enforces this for each of `go_shared_libs`, `orchestrator`, `worker_node`.
- Add or update unit tests so that new and changed code is covered; do not add linter suppressions to skip coverage.

### Linting and Style

- Do **not** modify lint rules or add linter suppression comments (e.g. `//nolint` in Go).
- Fix all reported lint issues; do not bypass or disable linters.
- Go: use `gofmt`; follow [`docs/tech_specs/go_rest_api_standards.md`](../docs/tech_specs/go_rest_api_standards.md).
- Markdown: follow `.editorconfig` and `.markdownlint.yml`; keep ASCII-only unless the linter allows otherwise.

### Specs and Requirements

- Do **not** change `docs/tech_specs/` or `docs/requirements/` unless the user explicitly asks.
- If the spec is ambiguous or contradicts the task, stop and ask the user for direction.

### Files and Commits

- Check existing files before making changes.
- When creating new files, create the file first (e.g. `touch path/to/file.go`), then edit.
- Use conventional commit messages; do not commit secrets (API keys, tokens, credentials).
- Do not commit generated artifacts under `tmp/` unless intentional.

### BDD and Tests

- If adding or changing user-visible behavior, add or update Gherkin scenarios under `features/` where appropriate.
- Prefer links to tech spec sections in code comments over long in-file design explanations.

## Expected Deliverables

- Control-plane registration returns a spec-shaped bootstrap payload with:
  - `version`, `issued_at`.
  - `orchestrator.base_url`.
  - `orchestrator.endpoints.worker_registration_url`.
  - `orchestrator.endpoints.node_report_url`.
  - `orchestrator.endpoints.node_config_url`.
  - `auth.node_jwt`.
  - `auth.expires_at`.
- Node Manager can register and then use the returned URLs for follow-on calls.
- No Node Manager reliance on hard-coded endpoint paths for registration, capability reporting, or config URL discovery.

## Implementation Steps

These steps are intentionally prescriptive so an AI coding agent can execute them without introducing new design decisions.

### 1. Identify the Current Registration Response Shape

Locate where the control-plane handles node registration and builds the response payload.
Confirm the current response does not match `node_bootstrap_payload_v1`.

### 2. Update Shared Contracts

Edit `go_shared_libs/contracts/nodepayloads/nodepayloads.go`.
Add types and JSON field names required for the Phase 1 minimal bootstrap subset in `node_bootstrap_payload_v1` (spec **CYNAI.WORKER.Payload.BootstrapV1**).
Do not remove or rename existing fields that may already be used.
Prefer adding optional fields in a forward-compatible way when needed.

### 3. Update Control-Plane Registration Handler

Edit `orchestrator/internal/handlers/nodes.go`.
Make the registration response match the `node_bootstrap_payload_v1` shape for the minimal subset (spec **CYNAI.WORKER.Payload.BootstrapV1**; req **REQ-ORCHES-0112**).

Guidance:

- Populate `version` and `issued_at`.
- Populate `auth.node_jwt` and `auth.expires_at`.
- Populate `orchestrator.base_url`.
- Populate `orchestrator.endpoints.*_url` fields with concrete URLs.
- Emit absolute URLs.
- Ensure the node can discover `node_report_url` and `node_config_url` from this payload.

### 4. Update the Node Manager Client

Edit `worker_node/cmd/node-manager/main.go`.
Parse the updated bootstrap payload shape.
Persist the JWT as needed for subsequent orchestrator calls.
Use `orchestrator.endpoints.node_report_url` and `orchestrator.endpoints.node_config_url` from the response.
Do not hard-code endpoint paths in the Node Manager for these follow-on calls.

### 5. Add or Update Tests

Add or update Go unit tests as needed to validate:

- Bootstrap payload JSON shape and required fields.
- Node Manager bootstrap parsing.
- Any helper functions used to build endpoint URLs.

## Validation and Test Plan

Per project standards, **`just ci`** is the mandatory gate before considering work complete.
It runs lint (Go quick + full, Python, Markdown), tests with 90% coverage, and Go vulnerability check.

### Required Gate

```bash
just ci
```

Chunk 02 plan also calls out Go tests and Go lint explicitly; both are included in `just ci`:

- `just test-go-cover` (tests + 90% coverage per module)
- `just lint-go-ci` (golangci-lint)

### Optional E2E

If the repo has a stable local E2E path and it is not prohibitively slow:

```bash
just e2e
```

## Definition of Done (Chunk 02)

- Registration returns a bootstrap payload that matches the `node_bootstrap_payload_v1` minimal subset (**CYNAI.WORKER.Payload.BootstrapV1**).
- Node Manager consumes the bootstrap payload and uses returned URLs for follow-on calls (supporting **REQ-ORCHES-0112**, **REQ-ORCHES-0113**).
- Payload secrets (e.g. `auth.node_jwt`) are handled per **CYNAI.WORKER.PayloadSecurity** / **REQ-WORKER-0131**, **REQ-WORKER-0132** (not logged; not exposed to sandboxes).
- **`just ci`** passes (lint, 90% Go coverage, vuln check).
- New or changed code has unit test coverage; no linter suppressions added.

## Notes for the Agent

- Keep changes tightly scoped to Chunk 02.
- Do not introduce new design decisions in code.
- Do not log secrets such as `auth.node_jwt`.
