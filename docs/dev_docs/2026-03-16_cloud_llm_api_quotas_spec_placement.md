# Cloud LLM API Quotas Spec - Placement and Related Docs

- [1 Draft Spec Location](#1-draft-spec-location)
- [2 Placement Options](#2-placement-options)
- [3 Related Tech Specs](#3-related-tech-specs)
- [4 Requirements Traceability](#4-requirements-traceability)

## 1 Draft Spec Location

- **Date:** 2026-03-16
- **Purpose:** Recommend where to place the new cloud LLM API quotas tech spec and how it relates to existing specs.

- **Draft:** [docs/draft_specs/cloud_llm_api_quotas_spec_proposal.md](../draft_specs/cloud_llm_api_quotas_spec_proposal.md)

## 2 Placement Options

Three options for where to place the cloud LLM API quotas spec once promoted from draft.

### 2.1 Option A: Standalone Tech Spec (Recommended)

- **Path:** `docs/tech_specs/cloud_llm_api_quotas.md`
- **Index:** Add under **External Integration and Routing** in [docs/tech_specs/_main.md](../tech_specs/_main.md), after [External Model Routing](../tech_specs/external_model_routing.md) and [API Egress Server](../tech_specs/api_egress_server.md).
- **Rationale:** Cloud LLM API quotas are a distinct concern (provider-tier and per-credential limits, 429 handling) that touches both external model routing and API Egress; a single doc keeps the contract in one place and avoids bloating either of the two existing specs.

### 2.2 Option B: Merge Into Existing Specs

- **Sections:** Split content so that (1) quota config and per-credential state live in [External Model Routing](../tech_specs/external_model_routing.md) under a new "Cloud LLM API quota configuration" section, and (2) pre-call check and 429 handling live in [API Egress Server](../tech_specs/api_egress_server.md) under a new "Cloud LLM quota check and rate-limit handling" section.
- **Rationale:** Keeps the number of tech spec files lower; downside is two files to update and possible duplication of concepts (e.g. PerCredentialQuotaState).

### 2.3 Option C: Keep in `draft_specs` Until Requirements Are Accepted

- Leave the doc in `docs/draft_specs/` and add a line in [docs/draft_specs/README.md](../draft_specs/README.md) (if it lists draft specs) or in a dev_docs index until REQ-MODELS-0133, REQ-MODELS-0134, and REQ-APIEGR-0128 (or equivalents) are accepted and the spec is promoted to `docs/tech_specs/`.

**Recommendation:** Option A for promotion; Option C until requirements and spec are approved.

## 3 Related Tech Specs

- **Token Usage, Quotas, and Rate Limits** ([token_usage_quotas_spec_proposal.md](../draft_specs/token_usage_quotas_spec_proposal.md)): Records usage and enforces user/project quotas; cloud LLM spec focuses on provider-tier and per-credential limits and pre-call checks.
  Token recording feeds per-credential state where applicable.
- **External Model Routing** ([external_model_routing.md](../tech_specs/external_model_routing.md)): Routing and settings (`max_external_tokens`, `max_external_cost_usd`); cloud quota spec adds per-credential RPM/TPM so routing can avoid keys at limit.
- **API Egress Server** ([api_egress_server.md](../tech_specs/api_egress_server.md)): Per-user/per-task constraints (REQ-APIEGR-0114); cloud quota spec adds per-credential quota check and 429 handling.
- **Cloud Agents** ([cloud_agents.md](../tech_specs/cloud_agents.md)): Cloud worker `rate_limits` (RPM, TPM); cloud LLM spec defines how those limits are configured and enforced for API Egress-backed cloud LLM calls.
- **Personas and Task Scoping** ([personas_and_task_scoping_proposal.md](../draft_specs/personas_and_task_scoping_proposal.md)): Requires "available API quota" in model selection; cloud LLM spec defines `CloudQuotaInModelSelection` to satisfy that.

## 4 Requirements Traceability

- Proposed REQs (REQ-MODELS-0133, REQ-MODELS-0134, REQ-APIEGR-0128) are placeholders in the draft.
- If accepted, they belong in `docs/requirements/models.md` and `docs/requirements/apiegr.md`.
- After promotion, add links from those requirement entries to the Spec ID anchors in the new tech spec.
