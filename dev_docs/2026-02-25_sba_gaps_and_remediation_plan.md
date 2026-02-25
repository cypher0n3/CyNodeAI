# SBA Gaps and Remediation Plan

- [Summary](#summary)
- [Gaps](#gaps)
  - [Spec and contract design](#spec-and-contract-design)
  - [Implementation and tests](#implementation-and-tests)
- [Remediation Plan](#remediation-plan)
- [Source Docs](#source-docs)

## Summary

**Date:** 2026-02-25.

This doc consolidates SBA-related gaps and remediation from:

- Job contract design analysis (spec/requirement clarifications and optional evolutions).
- SBA code review (node-manager BDD step, optional doc comments).
- Initial build-out report (one-time deliverable; delete after branch merge; no open gaps).

Remediation is ordered by dependency: spec updates first, then code/contract changes that follow spec approval.

## Gaps

Open items from design analysis and code review.

### Spec and Contract Design

All from analysis of `go_shared_libs/contracts/sbajob/sbajob.go` vs `docs/tech_specs/cynode_sba.md` and `docs/requirements/sbagnt.md`.
No code changes proposed in the design analysis; implementation follows spec approval.

- **Inference model selection.**  
  Concern: `AllowedModels []string` may be insufficient (e.g. provider, fallback order, capabilities).
  Spec today: allowlist of opaque model identifiers; runtime maps to endpoints.
  Gap: No explicit "future considerations" for richer model entries; optional extended shape not defined.

- **Context preferences shape.**  
  Concern: `preferences` as `map[string]string` is flat vs scoped semantics (scope_type, scope_id, key, value, value_type).
  Spec today: effective preference map (key to value); orchestrator resolves and attaches.
  Gap: Spec does not clearly state that `context.preferences` is the effective map (key to JSON value) and link to user_preferences.md; no note on possible future structured form.

- **Skills type.**  
  Concern: `Skills` as `interface{}` lacks type safety and clear contract.
  Spec today: skills may be inline content, stable ids, or paths; multiple shapes allowed.
  Gap: Spec does not enumerate allowed JSON shapes for `skills` (e.g. array of ids, array of objects with id/content, map id to content); implementors cannot replace `interface{}` with a union type without spec definition.

- **Step structure and execution flexibility.**  
  Concern: Fixed step `type` + `args` may be too rigid for dynamic inference and tools.
  Spec today: MVP step types; SBA builds todo list and may use inference and MCP throughout.
  Gap: Not explicitly stated that job steps are initial/suggested and SBA MAY use inference and MCP at any time; no extensibility note for future step types (e.g. `call_tool`, `call_llm`) or custom types.

### Implementation and Tests

- **Node manager BDD step (worker_node/_bdd/steps.go).**  
  The step "the node manager runs the startup sequence against the mock orchestrator" was changed to run the node-manager **binary** via `exec` instead of in-process `nodemanager.RunWithOptions`.
  Gap: Exec'ing the binary drops use of `nodemanager.RunWithOptions` and `st.failInferenceStartup`, so the @wip fail-fast scenario cannot pass; adds binary discovery/build and subprocess overhead.
  Required fix: Refactor to call an **exported library entrypoint** (e.g. `nodemanager.RunWithOptions`) so the real code path is exercised and options like `failInferenceStartup` remain injectable; remove or avoid reliance on `ensureNodeManagerBinary` for this step.

- **Result status (optional).**  
  Result `status` is not validated as one of `success|failure|timeout` in the contract package.
  Spec defines that set; package is used for emission, so acceptable but could be documented or enforced if same types used for ingestion.

- **ContextSpec.
  Skills (optional).**  
  Add a short comment that it may be inline content or structured data per CYNAI.
    SBAGNT.
    JobContext.

## Remediation Plan

Ordered by dependency: spec first, then code.

### 1. Spec Updates (`cynode_sba.md`)

- **Inference model.**  
  Option A (recommended for MVP): Add one paragraph under JobInferenceModel that `allowed_models` is an allowlist of opaque model identifiers and the runtime maps them to endpoints; add "Future considerations" for structured model entries (provider, priority, capabilities) if requirements emerge.
  Option B: Add optional extended shape (e.g. object with id, optional source, priority/capabilities) with "array of strings" remaining valid; then update Go contract in a separate change.

- **Preferences shape.**  
  Option A (recommended): Clarify in cynode_sba.md that `context.preferences` is the effective preference map (key to JSON value) as produced by the orchestrator's resolution; link to user_preferences.md; note that value types may be string, number, boolean, object, or array; document possible future structured form (e.g. array of entries with key, value, value_type, scope_type).
  Option B: Define PreferenceEntry in spec and allow map or array form; then update Go contract.

- **Skills type.**  
  Option A (recommended): In cynode_sba.md, define allowed JSON shapes for `skills` (e.g. array of strings (ids), array of objects with id and optional content, or map id to content).
  Then in a separate code change update the Go contract to use a tagged union or generic type instead of `interface{}`.
  Option B (minimal): Keep spec as-is; in Go, replace `interface{}` with `json.RawMessage` and add comment that SBA/orchestrator unmarshals into one of the documented shapes.

- **Step flexibility.**  
  Option A (recommended): Add a short subsection under Step Types (MVP) or Execution Model: job steps are the initial/suggested sequence; the SBA MAY use inference and MCP tools at any time; the SBA builds and updates a todo list from requirements, acceptance criteria, and steps.
  Clarify that current step types are MVP primitives and future minor versions MAY add step types (e.g. `call_tool`, `call_llm`) or an extension point for custom step types.
  Option B: Define that `steps[].type` may be MVP types or implementation-defined (e.g. namespaced); document unknown-type handling per protocol version; then Go contract can keep `Type string` and `Args json.RawMessage` with documented behavior.

### 2. Code Refactor (`worker_node`)

- Refactor the node manager BDD step to call an exported library entrypoint (e.g. `nodemanager.RunWithOptions`) instead of exec'ing the binary.
- Pass config derived from env and inject `st.failInferenceStartup` via options so the @wip fail-fast scenario can pass when enabled.
- Remove or avoid reliance on `ensureNodeManagerBinary` for this step.
- When using the in-process entrypoint, ensure `context.WithTimeout` (or equivalent) and cancel are used so the runner can be stopped on timeout with no goroutine leak.

### 3. Optional Doc Comments

- Add one-line comment on `ContextSpec.Skills` (allowed shapes per spec).
- Add one-line comment on `Result.Status` (allowed values: success|failure|timeout).

## Source Docs

This plan consolidates:

- **2026-02-25_sba_contract_design_analysis.md** - Design analysis and Options A/B; content merged above; doc can be deleted after this consolidation.
- **2026-02-25_sba_code_review.md** - Code review findings and refactor strategy; content merged above; doc can be deleted after node-manager BDD refactor.
- **2026-02-25_cynode_sba_initial_buildout_report.md** - One-time build-out deliverable (spec trace, contracts, BDD, CI).
  No open gaps; deleted.
  Key content preserved here: spec trace REQ-SBAGNT-0001; sbajob contract types and validation; `worker_node_sba.feature` scenarios (REQ-SBAGNT-0100/0101/0103, ProtocolVersioning, SchemaValidation, ResultContract); RegisterWorkerNodeSBASteps; `just ci` passes.
