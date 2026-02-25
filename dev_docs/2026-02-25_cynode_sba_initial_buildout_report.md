# CyNode SBA Initial Build-out Report

- [Summary](#summary)
- [Deliverables](#deliverables)
- [CI and Standards](#ci-and-standards)
- [Traceability](#traceability)

## Summary

**Date:** 2026-02-25.
Initial build-out of `docs/tech_specs/cynode_sba.md`, related feature files, unit tests, BDD tests, and local functional tests (no full orchestrator stack).
All work conforms to project spec authoring standards and testing/linting rules. `just ci` passes.

## Deliverables

The following items were added or updated.

### 1. Tech Spec (`docs/tech_specs/cynode_sba.md`)

- Added **Traces To** on the Document Overview spec item (CYNAI.SBAGNT.Doc.CyNodeSba) linking to REQ-SBAGNT-0001.
- No other spec content changes; the existing spec was already comprehensive and aligned with `docs/requirements/sbagnt.md`.

### 2. Shared SBA Job and Result Contracts (`go_shared_libs/contracts/sbajob/`)

- **`sbajob.go`**: Defines `JobSpec`, `JobConstraints`, `StepSpec`, `InferenceSpec`, `ContextSpec`, `Result`, `StepResult`, `ArtifactRef`, and `ValidationError`.
- **Validation**: `ParseAndValidateJobSpec` (strict JSON decode, unknown fields rejected), `ValidateJobSpec` (protocol version major, required fields, constraint bounds).
- **Protocol versioning**: Supported major version is 1; unknown major versions are refused per CYNAI.
  SBAGNT.
  ProtocolVersioning.
- **Unit tests** (`sbajob_test.go`): Full coverage (100%) for valid spec, unknown major version, unknown field rejection, missing required fields, invalid constraints, nil spec, result marshal round-trip, ValidationError.
  Error, invalid version format, parseMajorVersion table test.
  Helpers used to satisfy dupl/goconst linters.

### 3. Feature File (`features/worker_node/worker_node_sba.feature`)

- **Suite:** `@suite_worker_node`.
- **User story:** As a worker node or orchestrator, I want SBA job specifications to be validated and result contract well-defined, so that cynode-sba jobs run only with valid specs and results are auditable.
- **Scenarios (all with @req_* and @spec_* tags):**
  - Valid SBA job spec with supported protocol version passes validation (REQ-SBAGNT-0100, 0101; ProtocolVersioning, SchemaValidation).
  - SBA job spec with unknown major protocol version fails validation.
  - SBA job spec with unknown field fails validation (SchemaValidation).
  - SBA job spec with missing required job_id fails validation.
  - SBA result contract has required shape for orchestrator storage (REQ-SBAGNT-0103; ResultContract).

### 4. BDD Step Definitions (`worker_node/_bdd/steps.go`)

- **RegisterWorkerNodeSBASteps**: Steps for SBA job spec and result contract scenarios.
- State added: `sbaJobSpecBytes`, `sbaValidationErr`, `sbaValidationErrField`, `sbaResult`, `sbaResultJSON`.
- Steps use `go_shared_libs/contracts/sbajob` for parsing and validation; no orchestrator or container required (local functional behavior).

### 5. Local Functional Tests

- **Unit tests**: `go_shared_libs/contracts/sbajob` tests are runnable without any stack; they validate job spec parsing and validation logic only.
- **BDD**: Worker node BDD runs with in-process worker API and in-process SBA validation; no full orchestrator or Postgres required for the SBA scenarios.

## CI and Standards

- **`just ci`**: Passes (lint-sh, lint-go, lint-go-ci, vulncheck-go, lint-python, lint-md, validate-doc-links, validate-feature-files, test-go-cover, test-bdd, lint-containerfiles).
- Linter rules and testing standards were not modified; code was adjusted to satisfy existing dupl and goconst rules in the sbajob tests.

## Traceability

- Tech spec traces to REQ-SBAGNT-0001 on the doc overview.
- Feature scenarios trace to REQ-SBAGNT-0100, 0101, 0103 and to spec anchors ProtocolVersioning, SchemaValidation, ResultContract as applicable.
