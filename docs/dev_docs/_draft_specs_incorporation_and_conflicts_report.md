# Draft Specs: Incorporation Status and Conflict Review

- [1. Document metadata and scope](#1-document-metadata-and-scope)
- [2. Inventory of draft specs reviewed](#2-inventory-of-draft-specs-reviewed)
- [3. Incorporated but not marked as such](#3-incorporated-but-not-marked-as-such)
- [4. Partially incorporated or explicitly marked](#4-partially-incorporated-or-explicitly-marked)
- [5. Conflicts and tensions between draft specs](#5-conflicts-and-tensions-between-draft-specs)
- [6. Summary of recommendations](#6-summary-of-recommendations)
- [7. References](#7-references)
- [8. Changes since report date (2026-03-16)](#8-changes-since-report-date-2026-03-16)

## 1. Document Metadata and Scope

- **Date:** 2026-03-16
- **Last updated:** 2026-03-19 (Report review; docs-check link fixes.)
- **Scope:** All documents in `docs/draft_specs/` (excluding README)
- **Purpose:** Identify drafts already incorporated but not marked; identify conflicts between draft specs.
- **Output:** Report only; no code or normative doc changes.

## 2. Inventory of Draft Specs Reviewed

The following drafts were reviewed for incorporation status and cross-draft conflicts.
The draft `api_egress_sanity_checker_spec_proposal.md` was fully incorporated and was removed from `draft_specs/` on 2026-03-17; see Section 3.

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
- **Draft:** orchestrator_self_metadata_and_logging_spec_proposal.md
  - purpose (summary): Orchestrator self-metadata and logging (blob utilization, utilization windows, structured logging)
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
- **Draft:** cloud_llm_api_quotas_spec_proposal.md
  - purpose (summary): Cloud LLM API quotas (provider/per-credential limits, 429 handling, model-selection integration)
- **Draft:** worker_llm_proxy_sba_inference_capture_proposal.md
  - purpose (summary): Worker LLM proxy SBA inference capture, redaction, and reporting to orchestrator
- **Draft:** workspace_provisioning_and_project_repos_spec_proposal.md
  - purpose (summary): Workspace provisioning (repo to sandbox) and project repos allowlist for SBA; credential boundary, job spec workspace source, node/API contract

## 3. Incorporated but Not Marked as Such

No drafts in `draft_specs/` remain in this category.

### 3.1 API Egress Sanity Checker (Resolved)

- **Draft (removed):** `api_egress_sanity_checker_spec_proposal.md` was fully reflected in canonical docs ([`api_egress_server.md`](../tech_specs/api_egress_server.md) "Sanity Check (Semantic Safety)" section, [`apiegr.md`](../requirements/apiegr.md) REQ-APIEGR-0121 through REQ-APIEGR-0126).
- **Action taken:** The draft was removed from `docs/draft_specs/` on 2026-03-17 so the single source of truth is canonical only (per draft_specs README).

## 4. Partially Incorporated or Explicitly Marked

Drafts that are either partially integrated (and correctly marked) or whose relationship to canonical docs is clarified.

### 4.1 Cynork TUI Spec Proposal

- **Draft:** `cynork_tui_spec_proposal.md`
- **Status:** Full incorporation **complete** (2026-03-18); specs refined for implementation specificity (2026-03-18).
  All content from the draft has been incorporated into canonical requirements and tech specs.
  Formerly deferred items are fully specified with explicit contracts, constants, error semantics, and "Deferred Implementation" subsections per [spec authoring](../docs_standards/spec_authoring_writing_and_validation.md).
  The draft can be removed from `docs/draft_specs/`; the single source of truth is canonical.

Verified and extended in canonical:

- **Requirements:** REQ-USRGWY-0135 through 0145, REQ-CLIENT-0181 through 0207 (and later CLIENT IDs), REQ-ORCHES-0167/0168, REQ-SCHEMA-0114, REQ-PMAGNT-0115.
- **Tech specs:** [cynork_tui.md](../tech_specs/cynork_tui.md), [cynork_tui_slash_commands.md](../tech_specs/cynork_tui_slash_commands.md), [chat_threads_and_messages.md](../tech_specs/chat_threads_and_messages.md), [cli_management_app_commands_chat.md](../tech_specs/cli_management_app_commands_chat.md), [openai_compatible_chat_api.md](../tech_specs/openai_compatible_chat_api.md), [orchestrator.md](../tech_specs/orchestrator.md), [cynode_pma.md](../tech_specs/cynode_pma.md), [postgres_schema.md](../tech_specs/postgres_schema.md).

Refined with explicit implementation detail:

- **chat_threads_and_messages.md:** Thread summary (MaxLength 500, PATCH body, deferred server generation); archive (PATCH `archived`, GET query `archived`, semantics); download refs (metadata shape, GET endpoint or signed URL, 200/404/410/403); file upload (Option A `POST /v1/chat/uploads` vs Option B inline, MaxFileSizeBytes 10 MiB, AllowedMediaTypes minimum set, link to postgres_schema chat_message_attachments).
- **openai_compatible_chat_api.md:** At-Reference (client behavior at send time, search path default cwd and config key `tui.at_search_path`, gateway contract linked to FileUploadStorage).
- **cynork_tui.md:** Queued drafts (Ctrl+Q, session-scoped, send-all sequential with wait, deferred impl); default entry point (help preservation, code path); auth recovery (MaxConsecutiveFailures 2); web login (TimeoutSeconds 300, token handling, deferred impl).

Open design choices (direction needed):

- **File upload:** Prefer Option A (dedicated `POST /v1/chat/uploads`) or Option B (inline in completion request) for the first implementation?
- **AllowedMediaTypes:** Pin an exact allowlist in the spec (e.g. text/plain, text/markdown, image/png, image/jpeg, image/webp, application/pdf) or leave to gateway/config only?
- **Thread summary server generation:** Trigger on first message, on every N messages, or only when client sets/requests? (Currently deferred; no trigger specified.)

### 4.2 Chat Threads, PMA Context, and Backend Env Follow-Ups

- **Draft:** `chat_threads_pma_context_and_backend_env_followups.md`
- **Status:** Implementation work described in the draft (PMA history preservation, explicit thread creation, inference-backend env delivery) has been done.
  The draft's **Promotion Checklist** (Section 6) is still a pending to-do list.
  The draft does not claim "incorporated"; it is a follow-up doc.
  Consider adding a short note at the top: "Implementation completed for items in Section 1.2; promotion checklist (Section 6) remains pending for normative doc updates."

### 4.3 Personas, Task Routing, and Orchestrator Specifications Table (Incorporated 2026-03-18)

- **Drafts:** `personas_and_task_scoping_proposal.md`, `task_routing_pma_first_task_state.md`, `orchestrator_specifications_table.md`
- **Status:** Full incorporation **complete** (2026-03-18).
  All three drafts have been restated in canonical requirements and tech specs; the draft files are marked in the Overview (or Document Overview) section with "Incorporated into canonical specs as of 2026-03-18" and a pointer to this report (Section 4.3 and 8).
  Single source of truth is canonical.

#### 4.3.1 Task Routing (PMA-First, `planning_state`)

- **Requirements:** REQ-ORCHES-0176 through 0180 (orches.md), REQ-USRGWY-0158 (usrgwy.md).
- **Tech specs:** postgres_schema (Tasks.planning_state, migration note), user_api_gateway, cli_management_app_commands_tasks (create returns draft, ready transition, get/list/result include planning_state), langgraph_mvp (workflow start gate, planning_state=ready), project_manager_agent (Task review and ready transition, PMA review contract), orchestrator (Task Create Handoff).

#### 4.3.2 Orchestrator Specifications Table

- **Requirements:** REQ-SCHEMA-0115, 0116 (schema.md), REQ-PROJCT-0125 (projct.md).
- **Tech specs:** postgres_schema (specifications table, plan_specifications, task_specifications, meta/jsonb, ResolveSpecificationsForPlanOrTask algorithm, SpecificationObject contract), mcp_tools (specification.help, specification.*, plan.specifications.set, task.specifications.set; PMA allowlist for specification tools in access_allowlists_and_scope).

#### 4.3.3 Personas and Task Scoping

- **Requirements:** REQ-SCHEMA-0117-0119, REQ-ORCHES-0181-0183, REQ-PMAGNT-0127, REQ-MCPTOO-0120 (and cross-domain refs in projct, agents, etc.).
- **Tech specs:**
  - postgres_schema (Task vs Job terminology; Personas table extended with default_skill_ids, recommended_cloud_models, recommended_local_model_ids; Tasks.persona_id, recommended_skill_ids; Jobs.task_ids, allowed model allowlists; projects.allowed_model_ids)
  - data_rest_api (persona/task/project resource fields, edit semantics)
  - cynode_sba (task_ids, task_contexts for bundles, single model per job, skill merge, bundle result contract)
  - orchestrator (Job builder: persona resolution, allowed set, model/node selection, task bundle, planning_state ref)
  - project_manager_agent (task-level persona and skills, task bundles, persona list/get)
  - mcp_tools/persona_tools (persona.get returns new fields, SBA via worker proxy)
  - mcp_tools/access_allowlists_and_scope (persona.list, persona.get on PM/PAA/worker allowlists)
  - default_skills/pma_task_creation_skill (persona_id, recommended_skill_ids, bundles)

### 4.4 Pgvector Proposal vs Canonical Schema

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

### 5.4 Personas and Task Scoping vs Task Routing (PMA-First) - Resolved by Incorporation

- **Drafts:** `personas_and_task_scoping_proposal.md` and `task_routing_pma_first_task_state.md`
- **Relationship:** Personas explicitly defers to task_routing for execution gating: "only tasks with planning_state=ready are eligible" (personas doc).
  Task routing introduces `planning_state` (draft/ready).
  No conflict; they are aligned.
  **As of 2026-03-18 both drafts are incorporated** (Section 4.3); canonical specs are the single source of truth.

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

### 5.7 Orchestrator Specifications Table vs Postgres Schema (Resolved 2026-03-18)

- **Draft:** `orchestrator_specifications_table.md` proposed new tables: `specifications`, `plan_specifications`, `task_specifications`, and resolution algorithm.
- **Canonical:** [`postgres_schema.md`](../tech_specs/postgres_schema.md) now includes these tables and the ResolveSpecificationsForPlanOrTask algorithm (incorporated 2026-03-18; see Section 4.3.2).
- **Status:** Resolved by incorporation; single source of truth is postgres_schema.md.

### 5.8 Cloud LLM API Quotas vs Token Usage Quotas

- **Drafts:** `cloud_llm_api_quotas_spec_proposal.md` and `token_usage_quotas_spec_proposal.md`
- **Relationship:** Complementary.
  Cloud LLM spec covers provider-tier and per-credential limits (RPM, TPM, 429 handling); token usage spec covers user/project usage recording and quotas.
  The cloud LLM draft states that token recording may feed per-credential state where applicable.
- **Placement:** A dev_doc ([2026-03-16_cloud_llm_api_quotas_spec_placement.md](2026-03-16_cloud_llm_api_quotas_spec_placement.md)) recommends promoting cloud LLM quotas as a standalone tech spec under External Integration and Routing when accepted.
- **No conflict.**

### 5.9 Worker LLM Proxy SBA Inference Capture vs Canonical

- **Draft:** `worker_llm_proxy_sba_inference_capture_proposal.md` specifies SBA inference capture, proxy-to-job binding, opportunistic redaction (shared library), and worker-to-orchestrator inference reporting.
- **Canonical:** [`worker_node.md`](../tech_specs/worker_node.md), [`cynode_sba.md`](../tech_specs/cynode_sba.md), [`openai_compatible_chat_api.md`](../tech_specs/openai_compatible_chat_api.md) cover inference proxy, UDS path, and PMA streaming redaction; they do not yet specify SBA capture/report or orchestrator redaction before storage.
- **Conflict type:** Additive.
  The draft extends worker/orchestrator behavior without contradicting existing specs.
- **Recommendation:** When promoting, fold the capture, binding, redaction, and report contract into worker_node and a dedicated subsection or linked spec so SBA inference visibility is canonical in one place.

### 5.10 Orchestrator Self-Metadata and Logging vs Canonical

- **Draft:** `orchestrator_self_metadata_and_logging_spec_proposal.md` specifies orchestrator self-metadata (blob storage utilization, utilization windows, component/scheduler health) and structured logging for operators.
- **Canonical:** [`orchestrator.md`](../tech_specs/orchestrator.md), [`orchestrator_artifacts_storage.md`](../tech_specs/orchestrator_artifacts_storage.md) cover artifacts storage and orchestrator behavior; they do not yet define utilization snapshots, utilization windows, or self-metadata logging.
- **Conflict type:** Additive.
  The draft extends orchestrator behavior without contradicting existing specs; it traces to REQ-ORCHES and orchestrator_artifacts_storage.
- **Recommendation:** When promoting, fold into orchestrator.md (and optionally orchestrator_artifacts_storage.md) or add a dedicated subsection so self-metadata and logging remain in one place.

### 5.11 Workspace Provisioning and Project Repos vs Canonical

- **Draft:** `workspace_provisioning_and_project_repos_spec_proposal.md`
- **Canonical:** [`project_git_repos.md`](../tech_specs/project_git_repos.md), [`git_egress.md`](../tech_specs/mcp_tools/git_egress.md), [`cynode_sba.md`](../tech_specs/cynode_sba.md), [`worker_node.md`](../tech_specs/worker_node.md), [`worker_api.md`](../tech_specs/worker_api.md), [`sandbox_container.md`](../tech_specs/sandbox_container.md) define project-repo association, egress allowlist, job spec, and sandbox mounts; they do not specify how repo content reaches `/workspace` or who performs clone.
- **Conflict type:** Additive.
  The draft adds implementation "how" (workspace provisioning flow, credential boundary, job workspace source, node population contract) without contradicting existing specs.
- **Recommendation:** When promoting, fold the new subsections into the listed canonical docs so workspace provisioning and the single allowlist (project_git_repos) for both egress and provisioning are in one place; add or extend REQ-APIEGR, REQ-WORKER, REQ-SANDBX (or REQ-PROJCT) as proposed in the draft.

## 6. Summary of Recommendations

1. **Done:** `api_egress_sanity_checker_spec_proposal.md` was removed from draft_specs on 2026-03-17 (fully incorporated; canonical source in api_egress_server.md and apiegr.md).
2. **Optional clarification:** In `chat_threads_pma_context_and_backend_env_followups.md`, add one line noting that implementation for Section 1.2 is done and that the promotion checklist is still pending.
3. **Resolve replacement vs current:** For `node_registration_bundle_no_tls.md`, either adopt and update worker_node/worker_node_payloads or document "current vs proposed" explicitly.
4. **Avoid duplicate definitions:** When promoting `connector_framework_hardening.md`, integrate into `connector_framework.md` (and requirements) rather than maintaining two connector specs.
5. **Backend-agnostic wording:** When promoting `local_inference_backend_alternatives_spec_proposal.md`, update worker and ports specs to the abstraction and keep Ollama as one backend.
6. **Schema single source of truth:** For `orchestrator_specifications_table.md`, done (2026-03-18; Section 5.7).
  When promoting `pgvector_proposal_draft.md`, fold table and algorithm definitions into `postgres_schema.md` (and resolve any overlap with existing vector_items if adopting the pgvector draft's document/chunk model).
7. **Dependency order:** Personas and task routing are consistent; job kill depends on default messaging connectors for slash commands.
  No blocking conflicts identified among these.
8. **Cloud LLM API quotas:** When promoting `cloud_llm_api_quotas_spec_proposal.md`, use the placement recommended in [2026-03-16_cloud_llm_api_quotas_spec_placement.md](2026-03-16_cloud_llm_api_quotas_spec_placement.md) (standalone tech spec under External Integration and Routing).
9. **Worker LLM proxy SBA capture:** When promoting `worker_llm_proxy_sba_inference_capture_proposal.md`, integrate into worker_node (and related specs) so SBA inference capture and reporting are canonical in one place.
10. **Orchestrator self-metadata and logging:** When promoting `orchestrator_self_metadata_and_logging_spec_proposal.md`, integrate into orchestrator.md (and optionally orchestrator_artifacts_storage.md) so self-metadata and logging are canonical in one place.
11. **Workspace provisioning and project repos:** When promoting `workspace_provisioning_and_project_repos_spec_proposal.md`, fold new subsections into project_git_repos.md, git_egress.md, cynode_sba.md, worker_node.md, worker_api.md, and sandbox_container.md; add or extend requirements per the draft so workspace provisioning and credential boundary are canonical.

## 7. References

- [Draft Specs README](../draft_specs/README.md)
- [Tech Specs Index](../tech_specs/_main.md)
- [Spec authoring and validation](../docs_standards/spec_authoring_writing_and_validation.md)
- [meta.md](../../meta.md)
- [Cloud LLM API Quotas spec placement](2026-03-16_cloud_llm_api_quotas_spec_placement.md) (dev_doc)

## 8. Changes Since Report Date (2026-03-16)

Re-review performed 2026-03-17.

- **New drafts added to inventory:** Two drafts present in `docs/draft_specs/` were not in the original report and are now included: `cloud_llm_api_quotas_spec_proposal.md` and `worker_llm_proxy_sba_inference_capture_proposal.md`.
- **Incorporation status:** The API Egress Sanity Checker draft was fully incorporated; the draft file was removed from draft_specs on 2026-03-17 per recommendation (Section 3.1).
- **Partially incorporated / marked:** No change to the drafts previously listed in Section 4.
- **Canonical docs:** No new incorporations detected in `docs/tech_specs/` or `docs/requirements/` that would change the status of any draft (e.g. specifications tables still not in postgres_schema; cloud_llm_api_quotas and worker SBA inference capture not yet in tech_specs).
- **New relationship/conflict notes:** Section 5 now includes 5.8 (Cloud LLM API Quotas vs Token Usage Quotas) and 5.9 (Worker LLM Proxy SBA Inference Capture vs canonical worker/sba specs).
  Section 6 recommendations extended with items 8 and 9 for the two new drafts.

**2026-03-18: Cynork TUI draft incorporation verification.**

- Section 4.1 was updated after a full pass comparing the Cynork TUI draft to canonical requirements and tech specs.
- Outcome: all first-rollout items from the draft are already in canonical; no outstanding normative doc changes.
- Section 4.1 now includes a verification checklist (requirements and specs confirmed), an explicit deferred list, and an optional note about updating the draft's "proposed" REQ wording.

**2026-03-18: Cynork TUI draft full incorporation (formerly deferred items).**

- All formerly deferred items were incorporated into canonical specs with "Implementation note (TODO)" stubs where implementation is pending.
- **chat_threads_and_messages.md:** Thread summary (generation, max length), archive (archived_at, list filter, PATCH), download refs (optional metadata, retrieval contract), file upload storage (gateway endpoint or inline, limits).
- **openai_compatible_chat_api.md:** At-Reference Workflow expanded (send-time resolve, syntax, gateway contract); TODO for upload endpoint and limits.
- **cynork_tui.md:** Queued drafts and deferred send (new subsection), default entry point bare cynork (new subsection), auth recovery (consecutive-failure limit), web login (device-code/browser, timeout); TODOs added.
- Section 4.1 updated to state full incorporation complete and that the draft can be removed from `docs/draft_specs/`.

**2026-03-18: Personas, task routing, and orchestrator specifications table incorporated.**

- The three drafts `personas_and_task_scoping_proposal.md`, `task_routing_pma_first_task_state.md`, and `orchestrator_specifications_table.md` were fully incorporated into canonical requirements and tech specs (see Section 4.3).
- Each draft file was marked in its Overview (or Document Overview) section with "Incorporated into canonical specs as of 2026-03-18" and a link to this report; the drafts were not removed so that historical context remains in `draft_specs/`.
- Section 4.3 added with a concise map of where each draft's content landed (requirements and tech specs).
- Section 5.4 (Personas vs Task Routing) updated to note that both are now incorporated.

**2026-03-18: Report review and inventory update.**

- **Inventory:** Added `orchestrator_self_metadata_and_logging_spec_proposal.md` to Section 2 (new draft in `draft_specs/`).
- **Section reference fixes:** Section 5.4 now correctly references Section 4.3 (not 4.4) for personas/task routing incorporation; Section 8 changelog corrected from "Section 4.4 added" to "Section 4.3 added."
- **Section 5.7:** Updated to "Resolved 2026-03-18"; specifications tables and algorithm are now in postgres_schema.md per Section 4.3.2.
- **Section 5.10 and 6:** Added conflict/relationship note and recommendation for orchestrator self-metadata and logging draft.

**2026-03-18: Workspace provisioning and project repos draft added.**

- **New draft:** `workspace_provisioning_and_project_repos_spec_proposal.md` added to `docs/draft_specs/`.
- **Inventory:** Added to Section 2.
- **Section 5.11 and 6 (item 11):** Added relationship note (additive to canonical) and recommendation for promotion: fold into project_git_repos, git_egress_mcp, cynode_sba, worker_node, worker_api, sandbox_container; add/extend REQ as in draft.

**2026-03-19: Report review and docs-check.**

- Report reviewed; metadata "Last updated" set to 2026-03-19.
- Broken doc links to removed dev_docs files fixed in dev_docs and draft_specs so `just docs-check` passes (links replaced with plain-text references).
