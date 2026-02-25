# SBA Job Contract Design Analysis and Suggested Spec Updates

- [Summary](#summary)
- [Inference Model Selection](#inference-model-selection)
- [Context Preferences Shape](#context-preferences-shape)
- [Skills Type](#skills-type)
- [Step Structure and Execution Flexibility](#step-structure-and-execution-flexibility)
- [Plans of Action](#plans-of-action)

## Summary

**Date:** 2026-02-25  
**Scope:** Documentation-only review of four design concerns raised about the SBA job contract in `go_shared_libs/contracts/sbajob/sbajob.go`.

This document analyzes each concern against the current tech spec (`docs/tech_specs/cynode_sba.md`) and requirements (`docs/requirements/sbagnt.md`), and suggests plans of action in the form of spec/requirement updates.
No code changes are proposed in this document.

## Inference Model Selection

The following subsections address the concern that a simple list of model identifiers may be insufficient for model selection.

### Inference Model Concern

A simple list of strings (`AllowedModels []string`) seems insufficient; more detail may be needed for selecting a model (e.g. provider, endpoint hint, capabilities).

### Inference Model Current Spec

Spec `CYNAI.SBAGNT.JobInferenceModel` defines:

- `allowed_models`: array of strings, required when inference is used; allowlist of model identifiers (e.g. `llama3.2` for node-local Ollama, or provider-specific ids for API Egress).
- `source`: string, optional; `worker`, `api_egress`, or unset; when unset, runtime MAY infer which path(s) to enable per model id or policy.

### Inference Model Analysis

- The spec intentionally keeps the job-side contract minimal: the job says *which* models are allowed; the runtime (node and/or orchestrator) is responsible for injecting the actual endpoint(s) and making at least one allowed model reachable.
- Model selection today is "identifier only" (e.g. `llama3.2`, or an API Egress model id).
- Use cases that may require richer selection later: multiple providers for the same logical model, fallback order, model capabilities (vision, long context), or per-step model hints.
- None of these are mandated by current requirements; REQ-SBAGNT-0109 and the spec focus on allowlist and runtime-provided endpoints.

### Inference Model Suggested Plan

- **Option A (minimal):** Leave spec and contract as-is for MVP; document in the spec that `allowed_models` is an allowlist of opaque model identifiers and that the runtime maps them to endpoints.
  Add a short "Future considerations" note that a later revision could introduce structured model entries (e.g. provider, priority, capabilities) if requirements emerge.
- **Option B (evolve spec first):** Add a small subsection under `CYNAI.SBAGNT.JobInferenceModel` that defines an optional extended shape for model selection (e.g. object with `id`, optional `source`, optional `priority` or `capabilities`) while keeping "array of strings" as valid for backward compatibility.
  Then update the Go contract and validation to support both forms once the spec is approved.

## Context Preferences Shape

The following subsections address whether preferences should be a structured type instead of a flat map.

### Preferences Shape Concern

`preferences` as `map[string]string` is too flat; a preference struct (or slice of preferences) would better align with scoped preference semantics (scope_type, scope_id, key, value, value_type).

### Preferences Shape Current Spec

Spec `CYNAI.SBAGNT.JobContext` states:

- "Relevant preferences" are user or task-scoped preferences that affect how work is done; only preferences relevant to the job need be included; the orchestrator resolves and attaches them.
- Example job JSON shows `"preferences": {}` (empty object).
- The orchestrator's effective preference resolution (user_preferences.md) produces a map: key to value (JSON), with optional metadata (source scope, version).

### Preferences Shape Analysis

- In the orchestrator, preference entries have `scope_type`, `scope_id`, `key`, `value` (jsonb), `value_type`.
- When the orchestrator supplies "relevant preferences" to the job, it typically passes the *effective* map (key -> value) so the SBA does not need to re-resolve scope.
- So for the job payload, a key-value map is a reasonable serialization of "effective preferences for this task."
  A struct that includes scope metadata (e.g. `[]PreferenceEntry` or `map[PreferenceKey]PreferenceValue`) would allow the SBA to distinguish source scope or value_type if needed for auditing or display; the spec does not currently require that.

### Preferences Shape Suggested Plan

- **Option A (spec clarification only):** In cynode_sba.md, clarify that `context.preferences` is the effective preference map (key -> JSON value) as produced by the orchestrator's resolution, and that value types may be string, number, boolean, object, or array per user_preferences.md.
  Document that a future revision could add a structured form (e.g. array of `{ key, value, value_type, scope_type? }`) if the SBA or auditing needs scope metadata.
- **Option B (struct in spec and contract):** Define in the spec a small `PreferenceEntry` or `Preference` shape (key, value, optional value_type and/or scope_type) and allow either `preferences: { "key": value }` or `preferences: [ { "key": "...", "value": ... } ]` for backward compatibility.
  Then update the Go contract to use a slice or map of that struct; validation would accept both forms if the spec allows both.

## Skills Type

The following subsections address whether `Skills` should be a generic or typed structure instead of `interface{}`.

### Skills Type Concern

`Skills` as bare `interface{}` should be a generic or a more specific type for type safety and clearer contracts.

### Skills Type Current Spec

Spec `CYNAI.SBAGNT.JobContext` states:

- Skills (or skill identifiers/content) may be supplied in job context as inline content, stable skill ids, or paths under the job directory; may also be fetched via MCP (`skills.list`, `skills.get`).
- Example shows `"skill_ids": ["skill-uuid-1"]` and `"skills": null`.

### Skills Type Analysis

- The spec intentionally allows multiple shapes: inline content (e.g. markdown or structured blocks), stable ids (references), or paths.
- A single field that can be "array of ids" or "array of inline skill objects" or "map of id to content" fits a sum type or a generic over a small set of concrete types.
- In Go, options include: `interface{}` (current; flexible but no type safety), `json.RawMessage` (defer shape to consumers), or a union type (e.g. struct with optional fields or a slice of a generic type).
- The tech spec does not mandate a single schema for inline skills; it says "inline content, stable skill ids, or paths."

### Skills Type Suggested Plan

- **Option A (spec-first):** In cynode_sba.md, define the allowed shapes for `skills` explicitly (e.g. one of: array of strings (ids), array of objects with `id` and optional `content`, or map id -> content).
  Once the spec enumerates the shapes, the Go contract can use a tagged union struct or a type parameterized type (e.g. slice of a generic skill item type) instead of `interface{}`.
- **Option B (minimal code-level improvement without spec change):** Keep spec as-is; in the Go package, replace `interface{}` with `json.RawMessage` and add a short comment that the SBA or orchestrator unmarshals into one of the documented shapes.
  This remains docs-only if the "comment" is the only change; any actual type change would be a code change and is out of scope for this doc-only review.

## Step Structure and Execution Flexibility

The following subsections address whether the step schema is too rigid for dynamic inference and tools.

### Step Flexibility Concern

The current step structure (fixed `type` + `args` per step type) implies over-specification of how jobs are executed and may not allow enough room for dynamic inference and use of additional tools as needed.

### Step Flexibility Current Spec

- **Step Types (MVP):** `CYNAI.SBAGNT.Enforcement` defines deterministic step types: `run_command`, `write_file`, `read_file`, `apply_unified_diff`, `list_tree`.
- **Execution model:** The SBA reads the job spec and produces a result; it "MUST be able to build and manage its own todo list based on the job" derived from "job context (requirements, acceptance criteria, and any initial or suggested steps)" and updates it as it executes (add sub-tasks, mark complete, reorder).
- Steps in the job are validated and executed; the SBA may also add sub-tasks dynamically (todo list).
- Outbound channels (inference, MCP tools) are described separately; the SBA uses inference and MCP in addition to executing steps.

### Step Flexibility Analysis

- The spec already separates (1) validated job steps (MVP primitives), (2) todo list (SBA-managed, derived from context and steps), and (3) inference and MCP (available to the SBA regardless of step type).
- So "dynamic inference and use of additional tools" is allowed: the SBA can call LLMs and MCP tools during execution; the step list is not the only driver of behavior.
- The concern may be that the *step schema* (type + args) is too rigid for future step kinds (e.g. "call_llm" or "call_tool" as first-class step types) or for extensible args (e.g. plugin-defined step types).
- The spec currently defines a closed set of MVP step types; it does not yet define an extension mechanism (e.g. custom step type names or a generic "tool" step).

### Step Flexibility Suggested Plan

- **Option A (spec clarification):** In cynode_sba.md, add a short subsection under Step Types (MVP) or under Execution Model that states: job steps are the initial/suggested sequence; the SBA MAY use inference and MCP tools at any time during execution; the SBA builds and updates a todo list from requirements, acceptance criteria, and steps.
  Clarify that the current step types are MVP primitives and that future minor versions MAY add step types (e.g. `call_tool`, `call_llm`) or an extension point for custom step types, without changing the core contract.
- **Option B (extensibility in spec):** Define in the spec that `steps[].type` may be one of the MVP types or an implementation-defined type (e.g. namespaced); unknown types are either rejected at validation or passed through for the SBA to interpret, per schema version.
  Then the Go contract could keep `Type string` and `Args json.RawMessage` but document that unknown types are allowed or disallowed per protocol version.

## Plans of Action

- **Inference model selection.**
  Recommended: Option A for MVP (document current semantics and add "Future considerations" for richer model entries).
  Next step: Edit cynode_sba.md to add one paragraph under JobInferenceModel and optionally add "Future considerations" for structured model entries.
- **Preferences shape.**
  Recommended: Option A (clarify in spec that `preferences` is the effective map key to JSON value; note possible future structured form).
  Next step: Edit cynode_sba.md to clarify type and semantics of `context.preferences` and link to user_preferences.md effective result.
- **Skills type.**
  Recommended: Option A (define in spec the allowed JSON shapes for `skills`; then implementors can replace `interface{}` with a union or generic).
  Next step: Edit cynode_sba.md to enumerate allowed shapes for `skills` (and optionally `skill_ids`); then in a separate change update the Go contract.
- **Step flexibility.**
  Recommended: Option A (clarify that steps are initial/suggested; SBA may use inference and MCP throughout); Option B if extensible step types are desired.
  Next step: Edit cynode_sba.md to add clarification under Execution Model or Step Types and optionally add an extensibility note for future step types.

All of the above are documentation and specification updates.
Implementation changes to the Go contract should follow spec approval and be done in a separate, code-change review.
