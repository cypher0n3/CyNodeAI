# Draft Specs: Incorporation Status and Conflict Review

- [1. Document metadata and scope](#1-document-metadata-and-scope)
- [2. Inventory of draft specs reviewed](#2-inventory-of-draft-specs-reviewed)
- [3. Incorporated but not marked as such](#3-incorporated-but-not-marked-as-such)
- [4. Partially incorporated or explicitly marked](#4-partially-incorporated-or-explicitly-marked)
- [5. Conflicts and tensions between draft specs](#5-conflicts-and-tensions-between-draft-specs)
- [6. Summary of recommendations](#6-summary-of-recommendations)
- [7. References](#7-references)

## 1. Document Metadata and Scope

- **Date:** 2026-03-16
- **Scope:** All documents in `docs/draft_specs/` (excluding README)
- **Purpose:** Identify drafts already incorporated but not marked; identify conflicts between draft specs.
- **Output:** Report only; no code or normative doc changes.

## 2. Inventory of Draft Specs Reviewed

The following drafts were reviewed for incorporation status and cross-draft conflicts.

- **Draft:** api_egress_sanity_checker_spec_proposal.md
  - purpose (summary): LLM-based sanity checker in API Egress path
- **Draft:** chat_threads_pma_context_and_backend_env_followups.md
  - purpose (summary): Follow-ups: PMA history, explicit thread creation, backend env
- **Draft:** connector_framework_hardening.md
  - purpose (summary): Connector framework: async jobs, status, retry, governance
- **Draft:** cynork_tui_spec_proposal.md
  - purpose (summary): Chat QOL + cynork TUI as primary; slash commands, layout
- **Draft:** default_messaging_connectors_proposal.md
  - purpose (summary): Default messaging connectors (Signal, Discord, Mattermost); bidirectional
- **Draft:** llm_routing_and_model_handling_spec_draft.md
  - purpose (summary): Per-model routing, capability record, thinking-block handling
- **Draft:** local_inference_backend_alternatives_spec_proposal.md
  - purpose (summary): Abstraction over Ollama; backend-agnostic local inference
- **Draft:** model_capabilities_update_blob_spec_proposal.md
  - purpose (summary): Bulk YAML blob ingest for model capability updates
- **Draft:** model_hub_api_tool_spec.md
  - purpose (summary): Model Hub API tool: search, pull, cache; multi-provider
- **Draft:** model_warm_up_proposal.md
  - purpose (summary): Model warm-up behavior
- **Draft:** nats_messaging.md
  - purpose (summary): NATS/JetStream subject taxonomy, streams, event schemas
- **Draft:** node_manager_restart_and_pma_redeploy_spec_proposal.md
  - purpose (summary): Independent node-manager restart; orchestrator-triggered PMA redeploy
- **Draft:** node_registration_bundle_no_tls.md
  - purpose (summary): Registration via password-protected bundle; no TLS PKI
- **Draft:** orchestrator_specifications_table.md
  - purpose (summary): `specifications` (and related) tables for project-scoped spec refs
- **Draft:** personas_and_task_scoping_proposal.md
  - purpose (summary): Persona catalog, task-scoped persona, task bundles
- **Draft:** pgvector_proposal_draft.md
  - purpose (summary): Pgvector + strict RBAC (documents/chunks, tenant, sensitivity)
- **Draft:** pma_plan_creation_skill_spec_integration.md
  - purpose (summary): Promote PMA plan-creation skill into requirements/specs
- **Draft:** status_command_detailed_health_spec_proposal.md
  - purpose (summary): Detailed `/status` and `cynork status` stack health
- **Draft:** task_routing_pma_first_task_state.md
  - purpose (summary): PMA-first task routing; planning_state draft/ready
- **Draft:** token_usage_quotas_spec_proposal.md
  - purpose (summary): Token usage, quotas, rate limits, cost tracking
- **Draft:** user_directed_job_kill_proposal.md
  - purpose (summary): User-directed job kill; slash command + PMA + gateway
- **Draft:** worker_node_agent_draft_spec.md
  - purpose (summary): Worker Node Agent (WNA): host-level SBA, restricted commands
- **Draft:** zero_trust_tech_specs_assessment.md
  - purpose (summary): Zero-trust gap analysis and recommended spec changes

## 3. Incorporated but Not Marked as Such

One draft is fully reflected in canonical docs but does not state that it has been incorporated.

### 3.1 API Egress Sanity Checker (Fully Incorporated)

- **Draft:** `api_egress_sanity_checker_spec_proposal.md`
- **Evidence:** The canonical [`docs/tech_specs/api_egress_server.md`](../tech_specs/api_egress_server.md) contains a full "Sanity Check (Semantic Safety)" section.
  It covers placement in request flow, inputs, outputs, escalate-to-human, detection categories, security, audit, configuration, and model configuration.
  Requirements [`docs/requirements/apiegr.md`](../requirements/apiegr.md) include REQ-APIEGR-0121 through REQ-APIEGR-0126, all tracing to the sanity check spec.
- **Recommendation:** Mark the draft as incorporated (e.g. add "Status: Incorporated into api_egress_server.md and apiegr.md (REQ-APIEGR-0121--0126)" at the top) or archive/remove it per draft_specs README so the single source of truth is canonical only.

## 4. Partially Incorporated or Explicitly Marked

Drafts that are either partially integrated (and correctly marked) or whose relationship to canonical docs is clarified.

### 4.1 Cynork TUI Spec Proposal

- **Draft:** `cynork_tui_spec_proposal.md`
- **Status:** Correctly marked.
  It states "Draft; partially integrated via a TUI-first MVP" and documents promoted vs deferred (REQ IDs, entry point, thread summary/archive deferred, etc.).
  No change needed for marking.

### 4.2 Chat Threads, PMA Context, and Backend Env Follow-Ups

- **Draft:** `chat_threads_pma_context_and_backend_env_followups.md`
- **Status:** Implementation work described in the draft (PMA history preservation, explicit thread creation, inference-backend env delivery) has been done.
  The draft's **Promotion Checklist** (Section 6) is still a pending to-do list.
  The draft does not claim "incorporated"; it is a follow-up doc.
  Consider adding a short note at the top: "Implementation completed for items in Section 1.2; promotion checklist (Section 6) remains pending for normative doc updates."

### 4.3 Pgvector Proposal vs Canonical Schema

- **Draft:** `pgvector_proposal_draft.md`
- **Canonical:** [`postgres_schema.md`](../tech_specs/postgres_schema.md) already has "Vector Storage (pgvector)" with `vector_items`, RBAC (project_id, namespace, sensitivity_level), and retrieval rules.
- **Relationship:** The draft proposes a different shape (e.g. `documents` and `chunks` with tenant_id, owner_group_id).
  The canonical schema is a simpler, existing design.
  The draft is **not** incorporated; it is an alternative or extension (e.g. richer RBAC model).
  If the project adopts the draft's model, it would require schema evolution and a decision to supersede or extend the current vector_items design.

## 5. Conflicts and Tensions Between Draft Specs

Cross-draft dependencies and tensions that should be resolved or documented when promoting.

### 5.1 Connector Framework: Hardening vs Canonical

- **Draft:** `connector_framework_hardening.md` extends the connector model (async job model, status enums, retry policy, dead-letter, interactive auth).
- **Canonical:** [`connector_framework.md`](../tech_specs/connector_framework.md) defines connector type/instance, credentials, policy, and a smaller field set.
- **Conflict type:** Extension vs current.
  No direct contradiction; the hardening draft adds behavior and fields.
  Risk: if both are updated independently, duplicate or inconsistent definitions of "connector instance" or "operation" could appear.
- **Recommendation:** When promoting hardening, fold additions into the canonical connector_framework.md (and requirements) so there remains one source of truth.

### 5.2 Node Registration: Bundle (No TLS) vs Current Model

- **Draft:** `node_registration_bundle_no_tls.md` "extends or replaces" the current registration PSK and TLS-optional model in worker_node.md and worker_node_payloads.md.
- **Conflict type:** Proposed replacement.
  The draft is an alternative design (bundle as one-time credential, rotating keys, no TLS PKI).
  Until the project chooses one model, both "current spec" and "draft" describe different approaches.
- **Recommendation:** Resolve by either (a) adopting the bundle model and updating worker_node / worker_node_payloads, or (b) explicitly documenting "current: PSK/TLS-optional; proposed alternative: bundle (draft)" so readers know which is authoritative.

### 5.3 Local Inference Backend vs Ollama-Specific Specs

- **Draft:** `local_inference_backend_alternatives_spec_proposal.md` introduces a backend-agnostic "local inference backend" (configurable URL, OpenAI-compatible API, no hardcoded 11434).
- **Canonical:** Specs and requirements refer to "Ollama" and fixed ports (e.g. 11434) in worker_node, ports_and_endpoints, and REQ-WORKER-0114/0115.
- **Conflict type:** Abstraction vs current naming.
  The draft does not contradict behavior but proposes a layer that would eventually replace direct "Ollama" references.
- **Recommendation:** When promoting, update worker_node, ports_and_endpoints, and relevant requirements to use the abstraction (e.g. "local inference backend") and treat Ollama as one implementation so there is no long-term split between "Ollama-only" and "backend-agnostic" wording.

### 5.4 Personas and Task Scoping vs Task Routing (PMA-First)

- **Drafts:** `personas_and_task_scoping_proposal.md` and `task_routing_pma_first_task_state.md`
- **Relationship:** Personas explicitly defers to task_routing for execution gating: "only tasks with planning_state=ready are eligible" (personas doc).
  Task routing introduces `planning_state` (draft/ready).
  No conflict; they are aligned.
  Personas depends on task_routing being adopted for consistent semantics.

### 5.5 Default Messaging Connectors vs User-Directed Job Kill

- **Drafts:** `default_messaging_connectors_proposal.md` and `user_directed_job_kill_proposal.md`
- **Relationship:** Job kill specifies that slash commands for kill/cancel are part of the default slash-command set in the messaging connectors draft.
  Messaging draft references API Egress sanity checker (now in canonical) for "escalate" notification.
  No conflict; job kill depends on messaging for the slash-command surface.
  Order of promotion: messaging connector slash-command set first, then job kill can reference the canonical spec.

### 5.6 LLM Routing vs Model Capabilities Update Blob

- **Drafts:** `llm_routing_and_model_handling_spec_draft.md` and `model_capabilities_update_blob_spec_proposal.md`
- **Relationship:** LLM routing references the capabilities update blob draft for bulk capability updates.
  No conflict; complementary.
  If one is promoted, the other can be promoted or referenced as a separate capability.

### 5.7 Orchestrator Specifications Table vs Postgres Schema

- **Draft:** `orchestrator_specifications_table.md` proposes new tables: `specifications`, `plan_specifications`, `task_specifications`, and resolution algorithm.
- **Canonical:** [`postgres_schema.md`](../tech_specs/postgres_schema.md) does not yet include these tables.
- **Conflict type:** Additive.
  No clash with existing schema; the draft is a net addition.
- **Recommendation:** When accepted, add the table and algorithm definitions to postgres_schema.md (and link from orchestrator/plan/task specs) so schema remains single source of truth.

## 6. Summary of Recommendations

1. **Mark as incorporated:** Add a clear "Incorporated" status to `api_egress_sanity_checker_spec_proposal.md`, or archive/remove it and keep api_egress_server.md and apiegr.md as the only source of truth.
2. **Optional clarification:** In `chat_threads_pma_context_and_backend_env_followups.md`, add one line noting that implementation for Section 1.2 is done and that the promotion checklist is still pending.
3. **Resolve replacement vs current:** For `node_registration_bundle_no_tls.md`, either adopt and update worker_node/worker_node_payloads or document "current vs proposed" explicitly.
4. **Avoid duplicate definitions:** When promoting `connector_framework_hardening.md`, integrate into `connector_framework.md` (and requirements) rather than maintaining two connector specs.
5. **Backend-agnostic wording:** When promoting `local_inference_backend_alternatives_spec_proposal.md`, update worker and ports specs to the abstraction and keep Ollama as one backend.
6. **Schema single source of truth:** When promoting `orchestrator_specifications_table.md` or `pgvector_proposal_draft.md`, fold table and algorithm definitions into `postgres_schema.md` (and resolve any overlap with existing vector_items if adopting the pgvector draft's document/chunk model).
7. **Dependency order:** Personas and task routing are consistent; job kill depends on default messaging connectors for slash commands.
  No blocking conflicts identified among these.

## 7. References

- [Draft Specs README](../draft_specs/README.md)
- [Tech Specs Index](../tech_specs/_main.md)
- [Spec authoring and validation](../docs_standards/spec_authoring_writing_and_validation.md)
- [meta.md](../../meta.md)
