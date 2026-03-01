# Tenant / Organization Language Review - Single Deployment Clarification

- [Intent](#intent)
- [Scope](#scope)
- [Requirements](#requirements)
- [Canonical Tech Specs - Findings and Recommended Edits](#canonical-tech-specs---findings-and-recommended-edits)
- [Draft Specs and Dev Docs (Reference Only)](#draft-specs-and-dev-docs-reference-only)
- [Optional: Add a Single-Deployment Principle](#optional-add-a-single-deployment-principle)
- [Summary](#summary)

## Intent

Date: 2026-03-01.
CyNodeAI supports **RBAC** (roles, groups, project/user/system scope) but does **not** support multi-tenant or multi-organization within a single deployment.
Any wording that implies per-tenant or per-organization isolation or policies should be removed or rephrased so specs and requirements clearly reflect: **one deployment = one logical tenant/organization**.

## Scope

- **In scope:** `docs/requirements/`, `docs/tech_specs/` (canonical specs only).
- **Out of scope:** `docs/draft_specs/`, `docs/dev_docs/` (exploratory; can be updated later if those drafts are promoted).

## Requirements

- **Result:** No matches for "tenant", "organization", "multi-tenant", or "multi-org" in `docs/requirements/`.
  No changes needed.

## Canonical Tech Specs - Findings and Recommended Edits

Canonical specs that mention tenant or organization in a multi-tenant or multi-org sense, and recommended wording changes.

### Postgres Schema (Personas)

- **Location:** Personas section, ~line 542.
- **Current:** "They are stored per tenant/organization and are queriable by agents..."
- **Issue:** "Per tenant/organization" implies multiple tenants/orgs.
- **Recommended:** "They are stored in the deployment and are queriable by agents (PMA, PAA, orchestrator job builder) via the User API Gateway or MCP."

### Orchestrator Inference Container Decision

- **Location:** From Orchestrator Policy, ~line 96.
- **Current:** "Optional feature flag or tenant policy to disable node-local inference for specific nodes or globally."
- **Issue:** "Tenant policy" implies per-tenant policy.
- **Recommended:** "Optional feature flag or policy to disable node-local inference for specific nodes or globally."

### Skills Storage and Inference

- **Location 1:** Default skill scope, ~line 79.
- **Current:** "it is visible to all inference requests that receive skills (all users/tenants)."
- **Issue:** "all users/tenants" implies multiple tenants.
- **Recommended:** "it is visible to all inference requests that receive skills (all users)."

- **Location 2:** SkillScopeElevation outcomes, ~line 173.
- **Current:** "Global-scoped: all users (or all within a tenant) see the skill; only users with global/admin permission MAY set global scope."
- **Issue:** "or all within a tenant" implies multi-tenant.
- **Recommended:** "Global-scoped: all users in the deployment see the skill; only users with global/admin permission MAY set global scope."

### Project Git Repos (No Change)

- **Location:** ~line 49.
- **Current:** "optional fields for display and organization (e.g. `display_name`, `description`, `tags`, `metadata`)."
- **Assessment:** "organization" here means organizing/categorizing data (display, grouping), not "organization" as in multi-org.
  **No change recommended.**

## Draft Specs and Dev Docs (Reference Only)

The following contain tenant_id, multi-tenant, or organization in a multi-tenant sense; they are **not** canonical.
If any are later promoted to tech_specs or requirements, equivalent clarifications should be applied:

- `docs/draft_specs/agile_pm_rough_spec_addendum.md` - tenant_id, tenant scoping
- `docs/draft_specs/connector_framework_hardening.md` - Multi-Tenant Isolation Rules, multi-tenant safe
- `docs/draft_specs/pgvector_proposal_draft.md` - tenant_id, tenant isolation
- `docs/draft_specs/nats_messaging.md` - tenant_id in subjects, per-tenant NATS accounts
- `docs/draft_specs/agile_pm_rough_spec.md` - tenant_id, multi-tenant boundaries
- `docs/draft_specs/agile_pm_rough_spec_addendum2_lean_agile.md` - per-tenant concurrency
- `docs/dev_docs/draft_specs_priority_recommendations.md` - tenant_id in vector schema
- `docs/dev_docs/2026-02-27_proposed_plan_open_core_enterprise_agile_integration.md` - per-tenant feature flag
- `docs/dev_docs/2026-02-28_orchestrator_inference_container_decision_spec.md` - tenant policy (historical)
- `docs/draft_specs/default_messaging_connectors_proposal.md` - multi-tenant/multi-user Signal
- `docs/draft_specs/node_registration_bundle_no_tls.md` - multi-tenant scenarios
- `docs/draft_specs/model_hub_api_tool_spec.md` - "organization" as model author/org (provider-specific; can stay as-is if referring to external model hubs)

## Optional: Add a Single-Deployment Principle

To anchor future spec authoring, consider adding one sentence to a central doc:

- **Option A - `docs/tech_specs/_main.md` (Key Principles):**  
  "A single CyNodeAI deployment serves one logical tenant; there is no multi-tenant or multi-organization support.
    Access control is via RBAC (roles, groups, project/user/system scope) only."

- **Option B - New or existing requirement:**  
  Add a short REQ under identity/policy (e.g. in a requirements doc that covers deployment model) stating that the product is single-deployment only and does not implement multi-tenant or multi-organization isolation.

## Summary

- **Doc:** `docs/requirements/*`
  - action: None
- **Doc:** `postgres_schema.md`
  - action: Replace "per tenant/organization" with "in the deployment" in Personas.
- **Doc:** `orchestrator_inference_container_decision.md`
  - action: Replace "tenant policy" with "policy".
- **Doc:** `skills_storage_and_inference.md`
  - action: Remove "tenants" and "within a tenant"; use "all users" / "all users in the deployment".
- **Doc:** `project_git_repos.md`
  - action: No change (organization = organizing data).
- **Doc:** `_main.md` (optional)
  - action: Add single-deployment principle under Key Principles.

No code changes; documentation-only.
Apply the recommended edits when you are ready to update the specs.
