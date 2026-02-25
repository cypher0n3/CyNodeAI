# In-Depth Code Review: SBA Code and Related Go Changes

- [Summary](#summary)
- [Specification Compliance](#specification-compliance)
- [Architectural Issues](#architectural-issues)
- [Concurrency / Safety Issues](#concurrency--safety-issues)
- [Security Risks](#security-risks)
- [Performance Concerns](#performance-concerns)
- [Maintainability Issues](#maintainability-issues)
- [Recommended Refactor Strategy](#recommended-refactor-strategy)
- [Files Reviewed](#files-reviewed)

## Summary

**Date:** 2026-02-25  
**Scope:** All unstaged `**/*.go` modifications and additions (SBA contracts, worker BDD, formatting).

The new `go_shared_libs/contracts/sbajob` package and worker_node SBA BDD steps implement the SBA job spec and result contract per `docs/tech_specs/cynode_sba.md`.
They trace to REQ-SBAGNT-0100, REQ-SBAGNT-0101, REQ-SBAGNT-0103.
Implementation is spec-compliant, tests are thorough, and CI (lint, tests, BDD) passes.

The main functional change is the node manager BDD step: it was changed to run the node-manager **binary** via `exec` instead of in-process `nodemanager.RunWithOptions`.
Running the binary for this test is not recommended; the review recommends keeping a black-box approach by calling an **exported library entrypoint** (e.g. `nodemanager.RunWithOptions`) so the real code path is exercised without subprocess and options like `failInferenceStartup` remain injectable.
Other Go diffs are formatting/alignment only.

## Specification Compliance

- **REQ-SBAGNT-0100 / ProtocolVersioning:** `ValidateJobSpec` refuses unknown major versions.
  `parseMajorVersion` and `SupportedProtocolMajor = 1` match spec.
- **REQ-SBAGNT-0101 / SchemaValidation:** `ParseAndValidateJobSpec` uses `dec.DisallowUnknownFields()`, validates before any step execution, and returns `ValidationError` (fail closed).
- **REQ-SBAGNT-0103 / ResultContract:** `Result` has `protocol_version`, `job_id`, `status`, `steps`, `artifacts`, `failure_code`, `failure_message`; matches minimum result shape.
  BDD feature tags `@req_sbagnt_*` and `@spec_cynai_sbagnt_*` are correct.
- **Traceability:** Package doc in `sbajob.go` references `docs/tech_specs/cynode_sba.md`.
  Feature file `worker_node_sba.feature` tags requirements and spec IDs.

**Gap:** Result `status` is not validated as one of `success|failure|timeout` in the contract package.
The spec defines that set; the package is used for emission, so this is acceptable but could be documented or enforced if the same types are used for ingestion.

## Architectural Issues

- **Node manager BDD step (worker_node/_bdd/steps.go):** The step "the node manager runs the startup sequence against the mock orchestrator" was changed to build/locate the node-manager binary and run it via `exec.CommandContext` with env vars and `NODE_MANAGER_SKIP_SERVICES=1`.
  Running the binary directly is not a good approach for this test: it drops use of `nodemanager.RunWithOptions` and `st.failInferenceStartup`, so the @wip fail-fast scenario cannot pass, and it adds binary discovery/build and subprocess overhead.
  **Recommendation:** Keep a black-box approach by having the BDD step call an **exported library entrypoint** (e.g. `nodemanager.RunWithOptions` or another exported runner from the node-manager package) instead of exec'ing the binary.
  That way the test still exercises the real node manager code path (black box) without subprocess, and options such as `failInferenceStartup` remain injectable for the fail-fast scenario.
  Remove or avoid reliance on `ensureNodeManagerBinary` for this step.

## Concurrency / Safety Issues

- **workerTestState:** New fields `sbaJobSpecBytes`, `sbaValidationErr`, `sbaValidationErrField`, `sbaResult`, `sbaResultJSON` are only written from step functions that run sequentially per scenario; no goroutine sharing of these fields.
  Safe.
- **Node manager step:** If the step uses an in-process exported entrypoint, ensure `context.WithTimeout` (or equivalent) and cancel are used so the runner can be stopped on timeout with no goroutine leak.

## Security Risks

- **sbajob:** No secrets, no network.
  Input is JSON only; `DisallowUnknownFields` and validation limit accepted shape.
  No logging of raw payloads.
- If the step is refactored to use an exported entrypoint only, binary discovery and `exec` are no longer in scope for this step; security considerations then reduce to the normal in-process test.

## Performance Concerns

- **ParseAndValidateJobSpec:** Single decode + validation; no allocation hotspots.
  Acceptable for job-sized payloads.
- **BDD node manager step:** Using an in-process exported entrypoint avoids binary build and subprocess startup latency.

## Maintainability Issues

- **sbajob:** Clear package API (`ParseAndValidateJobSpec`, `ValidateJobSpec`, `ValidationError`, types).
  `parseMajorVersion` is unexported and tested via `TestParseMajorVersion` in the same package; acceptable.
- **ContextSpec.
  Skills:** Type is `interface{}`; spec allows variable shape.
  Consider a short comment that it may be inline content or structured data per CYNAI.
    SBAGNT.
    JobContext.
- **ValidationError on decode error:** When `dec.Decode` fails (e.g. unknown field), error is wrapped as `ValidationError` with `Message` set and `Field` empty.
  BDD "unknown field" scenario does not assert on field; other validation paths set `Field` correctly.
- **Formatting-only changes:** agents, orchestrator, worker_node (non-SBA) diffs are struct alignment, import order, and blank lines.
  No behavior change; keep as-is for consistency.

## Recommended Refactor Strategy

1. **Node manager BDD step:** Refactor to preserve black-box by calling an **exported library entrypoint** (e.g. `nodemanager.RunWithOptions`) instead of exec'ing the binary.
   Pass config derived from env (or equivalent) and inject `st.failInferenceStartup` via options so the @wip fail-fast scenario can pass when enabled.
   Remove or avoid reliance on `ensureNodeManagerBinary` for this step.
2. **Optional:** Add a one-line comment on `ContextSpec.Skills` and on `Result.Status` (allowed values) for future readers.

## Files Reviewed

- `go_shared_libs/contracts/sbajob/sbajob.go` (new): job/result contract, validation, protocol versioning.
- `go_shared_libs/contracts/sbajob/sbajob_test.go` (new): table-driven and edge-case tests; 100% coverage.
- `worker_node/_bdd/steps.go` (modified): SBA steps, `ensureNodeManagerBinary`, node manager step now execs binary.
- `agents/cmd/cynode-pma/main.go` (modified): formatting (struct alignment).
- `agents/cmd/cynode-pma/main_test.go` (modified): formatting (goroutine call).
- `agents/internal/pma/config.go` (modified): formatting (const alignment).
- `agents/internal/pma/config_test.go` (modified): formatting (struct alignment).
- `orchestrator/_bdd/steps.go` (modified): formatting (map alignment).
- `orchestrator/cmd/mcp-gateway/main.go` (modified): formatting (struct alignment).
- `orchestrator/cmd/mcp-gateway/testcontainers_test.go` (modified): formatting (const alignment).
- `orchestrator/internal/database/integration_test.go` (modified): trailing blank line removed.
- `orchestrator/internal/handlers/openai_chat.go` (modified): import order, struct alignment.
- `orchestrator/internal/models/models.go` (modified): struct field alignment.
- `orchestrator/internal/testutil/mock_db.go` (modified): inline struct formatting.

CI (`just ci`): lint and tests pass; BDD passes (worker SBA scenarios and node manager config scenarios run).
