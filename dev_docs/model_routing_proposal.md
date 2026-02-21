# Intelligent Model Routing Proposal (OpenAI-Compatible Clients)

- [Document metadata](#document-metadata)
- [Purpose](#purpose)
- [Goals](#goals)
- [Non-goals (for initial implementation)](#non-goals-for-initial-implementation)
- [Requirements alignment (high-signal)](#requirements-alignment-high-signal)
- [Problem statement](#problem-statement)
- [Proposed concept: virtual models backed by a routing policy](#proposed-concept-virtual-models-backed-by-a-routing-policy)
- [Architecture overview](#architecture-overview)
- [Routing inputs and signals](#routing-inputs-and-signals)
- [Routing decision output](#routing-decision-output)
- [How this plugs into OpenAI-compatible clients](#how-this-plugs-into-openai-compatible-clients)
- [Preferences model (task-execution)](#preferences-model-task-execution)
- [Auditing, safety, and policy](#auditing-safety-and-policy)
- [Observability and operator UX](#observability-and-operator-ux)
- [Rollout plan (incremental)](#rollout-plan-incremental)
- [Open questions / decisions to make](#open-questions--decisions-to-make)

## Document Metadata

Date: 2026-02-20

## Purpose

Add an intelligent model-routing capability that lets CyNodeAI expose "virtual models" to OpenAI-compatible clients (e.g. OpenWebUI, Cline) while selecting the best execution target per request.
This proposal is a dev-doc design draft.
It is intended to align with the existing MODELS requirements and the current external routing technical spec.

Key references:

- MODELS requirements: `docs/requirements/models.md`
- External model routing spec: `docs/tech_specs/external_model_routing.md`
- Model management spec: `docs/tech_specs/model_management.md`
- User API Gateway spec (client compatibility): `docs/tech_specs/user_api_gateway.md`
- OpenWebUI integration notes: `dev_docs/openwebui_cynodeai_integration.md`

## Goals

- Provide a single, consistent routing decision point in the orchestrator for LLM calls.
- Support OpenAI-compatible clients by exposing stable "model IDs" that map to routing policies.
- Prefer local inference when it satisfies capability and latency needs, and fall back to external providers only when policy allows it.
- Produce auditable, explainable routing decisions with clear reasons and thresholds.
- Make routing behavior configurable via PostgreSQL preferences (task-execution preferences and constraints, consistent with existing specs).

## Non-Goals (For Initial Implementation)

- Automatically learning policies from production data without an explicit offline evaluation loop.
- Introducing new external-provider credential handling paths outside API Egress.
- Exposing raw provider model catalogs directly to end users without governance.

## Requirements Alignment (High-Signal)

This proposal is designed to comply with existing requirements and specs, including:

- `REQ-MODELS-0002`: External calls routed via API Egress, with no provider keys in agents or sandboxes.
- `REQ-MODELS-0112` to `REQ-MODELS-0119`: Prefer local when possible, allow external when needed and allowed, record decision reasons, and route via API Egress.
- `REQ-MODELS-0120` to `REQ-MODELS-0122`: No keys in sandboxes, prefer MCP-mediated access, preferences-driven behavior.

## Problem Statement

OpenAI-compatible clients typically require the caller to choose a "model" string at request time.
In practice, users want a small set of stable choices (e.g. "auto", "fast", "coder") while the system dynamically selects the best underlying model and execution target (local worker vs external provider).
We also want deterministic, policy-governed routing that is safe in a local-first, controlled-egress architecture.

## Proposed Concept: Virtual Models Backed by a Routing Policy

Expose a set of stable "virtual model IDs" via the User API Gateway OpenAI-compatibility layer.
Each virtual model corresponds to a routing policy, not a single underlying provider model.

Examples (names are illustrative):

- `cynai/auto`: Best-effort routing based on request signals and configured constraints.
- `cynai/fast`: Prefer low latency models and local inference when available.
- `cynai/coder`: Prefer code-capable models and tool/function support.
- `cynai/vision`: Require vision-capable models.
- `cynai/local-only`: Hard-deny external routing.
- `cynai/external-only`: Use API Egress only (subject to policy and configured providers).

The gateway can also expose "physical models" when desired (e.g. a specific local model version), but the default should emphasize a small, stable virtual set to avoid client churn.

## Architecture Overview

This section describes where routing lives in the architecture, and the minimum components needed to keep routing deterministic, auditable, and policy-constrained.

### Where Routing Lives

Routing logic should live in the orchestrator as a single service (library + internal API) used by:

- Orchestrator-side agents (e.g. Project Manager Agent) when they need an LLM call.
- The User API Gateway OpenAI-compatibility endpoints (`/v1/chat/completions`, `/v1/models`).
- Any workflow engine steps that perform model inference (e.g. LangGraph-hosted workflows).

Routing must remain compatible with the existing separation of concerns:

- Local inference runs on worker nodes (e.g. Ollama) and is selected via orchestrator dispatch.
- External inference is performed only via the API Egress Server.
- Sandboxes do not receive provider keys and should not have direct external access.

### Proposed Components (Logical)

- `ModelRouter` (orchestrator service)
  - Entry point: `Route(request, context) -> RoutingDecision`.
  - Implements policy evaluation and candidate selection.
- `CandidateEnumerator`
  - Enumerates eligible local candidates from the model registry + node availability.
  - Enumerates eligible external candidates from configured providers and allowlists.
- `PolicyEngine`
  - Enforces "must/deny" constraints (allowed providers, budgets, data sensitivity, RBAC).
  - Evaluates preference keys described in `external_model_routing.md`.
- `Scorer`
  - Produces a ranking across candidates using signals and weights.
  - Should be deterministic and explainable (rule-based) for the first iteration.
- `DecisionRecorder`
  - Emits an audit event and stores a compact "routing reasons" record with request context.
  - Links to API Egress logs when external is selected.

## Routing Inputs and Signals

This section defines the signals the router can consider without violating CyNodeAI's controlled-egress and sandbox constraints.

### Routing Context (What the Router Sees)

- **Client type**: OpenAI-compat client name if known (OpenWebUI, Cline, other).
- **Requested model ID**: virtual model ID or physical model ID.
- **User identity and project**: for policy and preferences.
- **Operation**: chat completion, embedding, image, etc (future).
- **Request characteristics**:
  - Prompt token estimate.
  - Required context length.
  - Tool/function calling required (or desired).
  - Response format requirements (JSON mode, structured output).
  - Modalities required (text, vision).
- **Constraints**:
  - Deadline / latency budget (if provided).
  - Cost budget ceiling (if configured).
  - External-allowed flag (explicit or implied by policy).
- **Environment**:
  - Worker load signals (queue depth, avg latency, health).
  - Node model availability (from `node_model_availability`).

### "Intelligence" Without LLM-in-the-Router (Initial)

For the first iteration, implement an explicit, deterministic rule-based scorer.
This keeps routing stable, explainable, and auditable.

Later, optionally add an LLM-assisted router as a feature flag.
If added, it must remain policy-constrained, logged, and testable, and it must not be the sole decision authority.

## Routing Decision Output

The router returns a decision object that is usable by the orchestrator without further interpretation.

At minimum:

- **target_kind**: `local_worker` or `api_egress`.
- **selected_model_version_id** (when local) and any runtime hints (e.g. Ollama model name).
- **selected_node_id** (when local inference is required on a node).
- **selected_provider + provider_model** (when external).
- **reasons**: a compact list of reason codes and key numeric thresholds (for audit and debugging).
- **routing_trace_id**: stable id to correlate orchestrator logs, API Egress logs, and user-visible run logs.

## How This Plugs Into OpenAI-Compatible Clients

This section describes how routing is exposed to external tools via the User API Gateway OpenAI-compatibility surface.

### Models Endpoint (`/v1/models`)

Return a list containing:

- Virtual model IDs (the policy-backed set).
- Optionally, selected physical models (admin-controlled), especially local models intended for direct pinning.

Notes:

- OpenWebUI and Cline primarily need stable IDs, not exhaustive catalogs.
- Descriptions should clearly state the policy intent (e.g. "Auto routing, prefers local, external allowed by policy").

### Chat Completions Endpoint (`/v1/chat/completions`)

Flow:

- The gateway authenticates the user and builds a routing context.
- The gateway interprets the requested `model`:
  - If it matches a virtual model ID, load its routing policy.
  - If it matches a physical model ID, treat as a constrained request (subject to policy).
- The gateway calls the orchestrator `ModelRouter`.
- The orchestrator executes the model call via:
  - Local worker dispatch, or
  - API Egress invocation (external provider), per `external_model_routing.md`.
- The gateway returns an OpenAI-compatible response.

### Mapping Model IDs to Chat Agents (OpenWebUI)

OpenWebUI usage often mixes "models" and "agents" conceptually.
CyNodeAI can support both:

- A model ID can select the backing agent (e.g. Project Manager Agent vs a general chat agent).
- Within that agent, the model ID can also select a routing policy.

This implies an internal mapping table like:

- `openai_compat.model_id -> agent_id + routing_policy_id`

This mapping should be admin-configurable (Admin Web Console parity with CLI required by `meta.md`).

### Cline-Specific Considerations

Cline tends to rely on:

- Tool/function calling support.
- Long-context prompts (repo context).
- Consistent JSON or structured outputs for certain steps.

The router should treat "tool support required" as a hard capability constraint.
If local models do not support a required tool protocol, the router must select an external provider only if policy allows it, otherwise it must fail with an actionable error.

## Preferences Model (Task-Execution)

This proposal assumes the preference-based approach in `external_model_routing.md` and extends it with virtual-model policy definitions.
These preferences are intended to be read by agents and the router during task execution, not to represent deployment or service configuration.

### Suggested Preference Keys (Additive)

These are intentionally aligned with existing patterns and naming.

- `model_routing.virtual_models` (json)
  - Defines the virtual model IDs and their policies.
- `model_routing.virtual_models.<id>.constraints` (json)
  - Required capabilities (tool_use, json_mode, vision, context_length_min, etc).
  - Allowed target kinds (local-only, external-allowed).
  - Allowed providers and fallback order.
- `model_routing.virtual_models.<id>.weights` (json)
  - Weights for latency, cost, quality tiers, and locality.
- `model_routing.virtual_models.<id>.budgets` (json)
  - Max tokens, max cost, and/or per-time-window limits.

If the project prefers fewer preference keys, `model_routing.virtual_models` can be the single JSON blob, with server-side validation.

## Auditing, Safety, and Policy

This section captures the minimum auditability and safety invariants that routing must preserve.

### Auditing Expectations

- Log each routing decision (including local decisions), not just external decisions.
- Store reason codes and key signals that drove the decision (queue depth, capability mismatch, policy deny, user override).
- When external is selected, correlate with the API Egress outbound call log (required by `REQ-MODELS-0124`).

### Safety Constraints (Must Hold)

- No provider keys in agent prompts, sandbox env, or worker jobs.
- External calls only via API Egress, even if the caller is the gateway itself.
- Virtual models must be governed by allowlists and policy constraints (per user, project, task).

## Observability and Operator UX

Add a small set of metrics and dashboards early, because routing is hard to debug without them.

Minimum metrics:

- Decision counts by target kind (local vs external) and by virtual model ID.
- p50/p95 latency by target kind and by selected provider/model.
- Local worker saturation signals (queue depth, concurrency) at routing time.
- Fallback rate and top fallback reasons.

Admin UX needs:

- A read-only "why was this routed externally" view per run.
- A safe way to adjust virtual model policies and constraints.

## Rollout Plan (Incremental)

This section proposes an incremental rollout that delivers value early while keeping behavior predictable.

### Phase A: Rule-Based Router + Virtual Model IDs

- Implement `ModelRouter` as deterministic scorer + policy engine.
- Expose a minimal `/v1/models` list with a few virtual models.
- Route `/v1/chat/completions` through the router.
- Persist routing decisions and correlate with API Egress.

### Phase B: Richer Capability Constraints and Client-Specific Defaults

- Add capability constraints for tool use, json mode, and context length.
- Add client type hints (OpenWebUI vs Cline) as soft signals.
- Add safe "physical model pinning" for advanced users (admin-controlled).

### Phase C: Optional LLM-Assisted Scoring (Guardrailed)

- Add an optional assistant that suggests a candidate ranking.
- Keep policy engine as the ultimate gatekeeper.
- Log router prompts and outputs for offline evaluation.

## Open Questions / Decisions to Make

- What is the initial set of virtual model IDs we want to expose by default.
- Whether `/v1/models` should include any physical models or only virtual models in early phases.
- How to represent capability requirements for OpenAI tool/function calling in the model registry.
- How to express per-project budgets and rate limits for external calls in preferences.
