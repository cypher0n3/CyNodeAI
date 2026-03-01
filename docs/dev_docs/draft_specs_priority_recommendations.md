# Draft Specs: High-Value, Low-Effort Priority Recommendations

- [Overview](#overview)
- [Promotion Status](#promotion-status)
- [Methodology](#methodology)
- [Rank-Ordered Recommendations](#rank-ordered-recommendations)
- [Summary Table](#summary-table)
- [Deferred or Lower-Priority Drafts](#deferred-or-lower-priority-drafts)
- [References](#references)

## Overview

This report reviews the draft specifications in `docs/draft_specs/` and ranks them by **highest value, lowest effort** to guide which drafts to promote to canonical requirements and tech specs first.

Draft specs are non-canonical; promotion means splitting content into `docs/requirements/` and `docs/tech_specs/` per [draft_specs/README.md](../draft_specs/README.md) and [spec authoring standards](../docs_standards/spec_authoring_writing_and_validation.md).

## Promotion Status

The following drafts have been promoted to canonical requirements and tech specs.
The draft files in `docs/draft_specs/` remain for reference until archived or removed.

- **Rank 1 - Model Warm-up Proposal:** Promoted.
  Requirements: [REQ-USRGWY-0134](../requirements/usrgwy.md#req-usrgwy-0134), [REQ-CLIENT-0177](../requirements/client.md#req-client-0177).
  Tech spec: [Chat Model Warm-Up](../tech_specs/openai_compatible_chat_api.md#spec-cynai-usrgwy-openaichatapi-warmup), [Chat Session Warm-Up](../tech_specs/cli_management_app_commands_chat.md#spec-cynai-client-clichatwarmup); Web Console note in [web_console.md](../tech_specs/web_console.md) (UI Requirements by Domain).
- **Rank 2 - API Egress Sanity Checker:** Promoted.
  Requirements: [REQ-APIEGR-0121](../requirements/apiegr.md#req-apiegr-0121) through [REQ-APIEGR-0125](../requirements/apiegr.md#req-apiegr-0125).
  Tech spec: [Sanity Check (Semantic Safety)](../tech_specs/api_egress_server.md#spec-cynai-apiegr-sanitycheck); optional note in [access_control.md](../tech_specs/access_control.md) (Service Integration).
- **Rank 3 - Pgvector With Strict RBAC:** Promoted.
  Requirements: [REQ-ACCESS-0121](../requirements/access.md#req-access-0121) through [REQ-ACCESS-0125](../requirements/access.md#req-access-0125), [REQ-SCHEMA-0111](../requirements/schema.md#req-schema-0111), [REQ-SCHEMA-0112](../requirements/schema.md#req-schema-0112).
  Tech spec: [Vector retrieval and RBAC](../tech_specs/postgres_schema.md#spec-cynai-schema-vectorretrievalrbac); [vector_items](../tech_specs/postgres_schema.md#vector-items-table) extended with `tenant_id`, `namespace`, `sensitivity_level`.

All other drafts in the table below are not yet promoted.

## Methodology

Value and effort are defined as follows.

- **Value:** Impact on product (UX, security, correctness, scalability, alignment with MVP and existing tech specs).
- **Effort:** Scope of doc and implementation change: number of touched requirements/specs, new subsystems, dependencies, and complexity.
- **Ranking:** Ordered by value-to-effort ratio; ties favor smaller scope and clearer promotion path.

## Rank-Ordered Recommendations

Drafts are listed from highest value-to-effort (promote first) to lowest.

### 1. Model Warm-Up Proposal (`model_warm_up_proposal.md`)

- **Status:** Promoted to canonical (see [Promotion status](#promotion-status)).
- **Value:** High.
  Directly improves perceived latency for chat (CLI and Web) by warming the default model before the first user message.
- **Effort:** Low.
  Single optional gateway endpoint (e.g. `POST /v1/chat/warm`), one call from cynork and Web on session start; best-effort, no blocking.
- **Promotion path:** Add requirements (e.g. REQ-APIEGR or chat-domain) for optional warm-up; add a short section to the OpenAI-compatible chat API spec and CLI/Web console specs.
  Implementation is a small gateway handler and client-side calls.
- **Recommendation:** Promote first.
  Clear scope, immediate UX gain, minimal risk.

### 2. API Egress Sanity Checker (`api_egress_sanity_checker_spec_proposal.md`)

- **Status:** Promoted to canonical (see [Promotion status](#promotion-status)).
- **Value:** High.
  Adds a semantic safety layer (allow/deny/escalate) for outbound API calls, catching dangerous intents (bulk delete, secret exposure, high-impact actions) that policy alone cannot express.
- **Effort:** Medium.
  New layer after access control; LLM integration (local or external), config, audit.
  No change to sandbox or credential flow; well-bounded.
- **Promotion path:** New requirements in `apiegr.md` (e.g. REQ-APIEGR-0121 through 0125); new section in `api_egress_server.md` with Spec ID; optional note in `access_control.md`.
- **Recommendation:** Promote early.
  Strong safety payoff; effort is contained to API Egress Server and config.

### 3. Pgvector With Strict RBAC (`pgvector_proposal_draft.md`)

- **Status:** Promoted to canonical (see [Promotion status](#promotion-status)).
- **Value:** High.
  Ensures vector retrieval never bypasses RBAC; tenant/project/namespace/sensitivity scoping is mandatory for correctness and security.
- **Effort:** Medium.
  Schema extensions (tenant_id, project_id, sensitivity_level, etc.), query flow changes (filter-before-similarity), indexes, and audit.
  No new infrastructure; builds on existing pgvector use.
- **Promotion path:** Requirements in the appropriate domain (e.g. vector or RAG); tech spec section or dedicated doc for pgvector RBAC flow; align with existing access control and postgres_schema.
- **Recommendation:** Promote once vector/RAG is in active use.
  Foundational for safe retrieval.

### 4. Node Registration Bundle Without TLS (`node_registration_bundle_no_tls.md`)

- **Value:** High.
  Simplifies ops (no TLS PKI), improves security (one-time bundle, password-encrypted, rotating keys post-registration).
- **Effort:** Medium-high.
  Bundle issuance and consumption, registration flow changes, crypto and key rotation, cynork commands (e.g. generate bundle, single-node setup).
  Well specified; open points are wire format and algorithm pinning.
- **Promotion path:** Requirements in `worker.md` and possibly `bootst.md`; updates to `worker_node.md` and `worker_node_payloads.md`; traceability to new Spec IDs.
- **Recommendation:** Promote when prioritizing node onboarding and security over current PSK model.

### 5. Local Inference Backend Alternatives (`local_inference_backend_alternatives_spec_proposal.md`)

- **Value:** High.
  Backend-agnostic abstraction (Ollama, vLLM, LocalAI) supports scaling and heterogeneous hardware without changing sandbox contract.
- **Effort:** Medium-high.
  Edits across worker, models, sandbox requirements; worker_node, ports_and_endpoints, SBA/sandbox specs; config and proxy upstream URL.
  Minimal implementation path keeps Ollama default and adds abstraction first.
- **Promotion path:** Draft already lists concrete requirement edits and spec section changes; promote in phases (abstraction + Ollama default, then add vLLM/LocalAI).
- **Recommendation:** Promote after core single-node path is stable; enables future backends without big-bang change.

### 6. Model Hub API Tool (`model_hub_api_tool_spec.md`)

- **Value:** High.
  Unified search/pull/resolve from multiple providers (Hugging Face, Ollama library, ModelScope, private) via API Egress with filtering and audit.
- **Effort:** Medium-high.
  Egress integration, provider adapters, content/model-class filters, system settings, cache and registry integration.
  Draft is already structured with Spec IDs and traceability.
- **Promotion path:** Requirements in `models.md`; tech spec in `model_management.md` or dedicated model hub doc; link to API Egress and access control.
- **Recommendation:** Promote when user-directed model discovery and cache population are on the roadmap.

### 7. Lean Agile Amendment (`agile_pm_rough_spec_addendum2_lean_agile.md`)

- **Value:** Medium-high.
  WIP limits, flow metrics, value-based prioritization (e.g. WSJF), pull-based behavior improve flow and predictability.
- **Effort:** Medium-high.
  Depends on agile work item model (epics, stories, tasks, subtasks).
  New config fields, enforcement in status transitions, metrics, optional cynode-pm behavior changes.
- **Promotion path:** Add to or extend the agile PM spec once the base work-item model is canonical; add requirements for WIP and flow.
- **Recommendation:** Promote after the base structured work model is accepted and implemented.

### 8. NATS Messaging (`nats_messaging.md`)

- **Value:** High.
  Durable job dispatch, work-item and requirement events, node presence, progress streaming with a clear subject taxonomy and JetStream streams.
- **Effort:** High.
  New messaging stack, stream definitions, envelope and payload schemas, consumer patterns, idempotency, RBAC at NATS and message level.
  MVP scope in the doc is a subset (jobs + node heartbeat/capacity + artifact.available).
- **Promotion path:** New requirements for messaging/events; dedicated tech spec for NATS/JetStream; align with postgres_schema and job lifecycle.
- **Recommendation:** Promote when moving to multi-node dispatch and event-driven read models; start with MVP subset.

### 9. Requirements and Acceptance Criteria Addendum (`agile_pm_rough_spec_addendum.md`)

- **Value:** High.
  Formal requirements and acceptance criteria linked to stories and validation (automated/manual) improve traceability and "done" definition.
- **Effort:** High.
  New tables (requirements, acceptance_criteria), cynode-pm integration (generate requirements, block Story Done until verified), RBAC and audit.
- **Promotion path:** Requirements in a PM or governance domain; tech spec for data model and validation flow; link to agile_pm_rough_spec.
- **Recommendation:** Promote with or after the base agile PM spec; elevates governance and testability.

### 10. Agile PM Rough Spec (`agile_pm_rough_spec.md`)

- **Value:** High.
  Epic/Feature/Story/Task/Sub-task hierarchy, PM agent decomposition, artifact and job traceability, RBAC-aligned work items.
- **Effort:** High.
  Schema, APIs, status model, cynode-pm integration, sprint support, definition of done.
  Foundation for addenda (requirements, lean agile).
- **Promotion path:** Split into requirements (work item model, RBAC, traceability) and tech spec (data model, APIs, agent behavior); align with postgres_schema and jobs/tasks.
- **Recommendation:** Promote when structured PM and work-item-driven execution are product priorities; do before or with addenda.

### 11. Connector Framework Hardening (`connector_framework_hardening.md`)

- **Value:** High.
  Execution model, idempotency, versioning, rate limiting, credentials, observability, and optional YAML/CRD-style spec close large operational and security gaps.
- **Effort:** Very high.
  Broad framework changes: execution and async job model, schema validation, circuit breaker, policy expansion, reconciliation controller, certification.
  Draft is a long amendment list plus YAML feasibility.
- **Promotion path:** Incremental: map amendment sections to new Spec IDs and requirements; consider a v2 connector spec or phased requirements (e.g. idempotency and retry first, then governance).
- **Recommendation:** Defer full promotion; extract highest-impact slices (e.g. idempotency, timeout, retry) as targeted spec updates when connector work is scheduled.

## Summary Table

- **Rank:** 1
  - draft spec: model_warm_up_proposal
  - value: High
  - effort: Low
  - suggested timing: Next (UX quick win)
  - status: Promoted
- **Rank:** 2
  - draft spec: api_egress_sanity_checker_spec_proposal
  - value: High
  - effort: Medium
  - suggested timing: Early (safety)
  - status: Promoted
- **Rank:** 3
  - draft spec: pgvector_proposal_draft
  - value: High
  - effort: Medium
  - suggested timing: When vector/RAG active
  - status: Promoted
- **Rank:** 4
  - draft spec: node_registration_bundle_no_tls
  - value: High
  - effort: Medium-high
  - suggested timing: When node onboarding prioritized
  - status: Not yet promoted
- **Rank:** 5
  - draft spec: local_inference_backend_alternatives_spec_proposal
  - value: High
  - effort: Medium-high
  - suggested timing: After single-node stable
  - status: Not yet promoted
- **Rank:** 6
  - draft spec: model_hub_api_tool_spec
  - value: High
  - effort: Medium-high
  - suggested timing: When model discovery on roadmap
  - status: Not yet promoted
- **Rank:** 7
  - draft spec: agile_pm_rough_spec_addendum2_lean_agile
  - value: Medium-high
  - effort: Medium-high
  - suggested timing: After base agile PM
  - status: Not yet promoted
- **Rank:** 8
  - draft spec: nats_messaging
  - value: High
  - effort: High
  - suggested timing: When multi-node dispatch/events needed
  - status: Not yet promoted
- **Rank:** 9
  - draft spec: agile_pm_rough_spec_addendum
  - value: High
  - effort: High
  - suggested timing: With or after base agile PM
  - status: Not yet promoted
- **Rank:** 10
  - draft spec: agile_pm_rough_spec
  - value: High
  - effort: High
  - suggested timing: When structured PM is priority
  - status: Not yet promoted
- **Rank:** 11
  - draft spec: connector_framework_hardening
  - value: High
  - effort: Very high
  - suggested timing: Phased or deferred
  - status: Not yet promoted

## Deferred or Lower-Priority Drafts

- **Connector framework hardening:** Treat as a backlog of amendments; promote in slices when connector work is scheduled.
- **Agile PM suite (rough spec + addenda + lean agile):** High value but high effort and dependency on product commitment to structured work management; promote as a coordinated set when PM roadmap is clear.
- **NATS:** Promote when the system moves beyond single-node or when event-driven read models are required; MVP subset reduces initial effort.

## References

- [docs/draft_specs/README.md](../draft_specs/README.md) - Role of draft specs and promotion process
- [docs/docs_standards/spec_authoring_writing_and_validation.md](../docs_standards/spec_authoring_writing_and_validation.md) - Spec authoring and validation
- [meta.md](../../meta.md) - Project layout and canonical docs
- [docs/tech_specs/_main.md](../tech_specs/_main.md) - Tech specs index
