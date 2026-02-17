# Requirements Reconciliation Plan

- [1 Scope](#1-scope)
- [2 Inputs and Constraints](#2-inputs-and-constraints)
  - [2.1 Alignment With `spec_authoring_writing_and_validation.md`](#21-alignment-with-spec_authoring_writing_and_validationmd)
- [3 Proposed Requirements Set Structure](#3-proposed-requirements-set-structure)
- [4 Requirements Item Format (Draft)](#4-requirements-item-format-draft)
- [5 Requirements Domains (Draft Proposal)](#5-requirements-domains-draft-proposal)
- [6 Consolidation and Reconciliation Workflow](#6-consolidation-and-reconciliation-workflow)
- [7 Anchor and Linking Plan](#7-anchor-and-linking-plan)
- [8 `.markdownlint.yml` Updates (Anchor Allowlist)](#8-markdownlintyml-updates-anchor-allowlist)
- [9 Work Breakdown (Phased)](#9-work-breakdown-phased)
- [10 Open Questions for User](#10-open-questions-for-user)
- [11 Acceptance Criteria](#11-acceptance-criteria)

## 1 Scope

- Date: 2026-02-17.
- Goal: Consolidate requirement-level statements currently spread across [`docs/tech_specs/`](../docs/tech_specs/) into a unified set under `docs/requirements/`.
- Non-goal: Replace tech specs as the normative source of truth.
  Requirements should primarily normalize, index, and cross-link spec obligations rather than restate entire designs.

## 2 Inputs and Constraints

- Primary inputs: All files under [`docs/tech_specs/`](../docs/tech_specs/).
- Standards: All new requirements docs must follow [`docs/docs_standards/`](../docs/docs_standards/), especially:
  - [`docs/docs_standards/markdown_conventions.md`](../docs/docs_standards/markdown_conventions.md)
  - [`docs/docs_standards/spec_authoring_writing_and_validation.md`](../docs/docs_standards/spec_authoring_writing_and_validation.md)
- Validation: Use `just lint-md` for Markdown validation.
- Inline HTML: Only the allowed anchor forms should be used.
- Scale estimate: `docs/tech_specs/` currently contains approximately:
  - ~431 occurrences of RFC-2119-style `MUST`.
  - ~209 occurrences of RFC-2119-style `SHOULD`.
  - ~54 occurrences of RFC-2119-style `MAY`.
- Structure signal: `docs/tech_specs/` currently contains many informal "Normative requirements" blocks.
  There are currently 64 occurrences of the literal phrase "Normative requirements".

### 2.1 Alignment With `spec_authoring_writing_and_validation.md`

The current standards document defines:

- Spec IDs as the canonical cross-link target (`spec-*` anchors appended to `- Spec ID: ...` lines).
- Allowed inline HTML anchors limited to `spec-*`, `ref-*`, `algo-*`, and `algo-*-step-*`.
- Requirements documents as lightweight cross-references that should not restate spec content.

This plan aligns with that intent (single source of truth is `docs/tech_specs/`) but proposes two additions:

- Fine-grained anchors for individual normative requirement bullets in tech specs (`norm-*`).
- Stable anchors for requirement list entries in requirements docs (`req-*`).

Those additions require minimal updates to `docs/docs_standards/spec_authoring_writing_and_validation.md`:

- Update **Requirement IDs** to explicitly allow `REQ-<DOMAIN>-<NNNN>` (this project uses 4 digits).
- Extend **Allowed Inline HTML** to include:
  - Requirement anchors: `<a id="req-..."></a>` (requirements docs only; on a continuation line under the requirement list item, after any spec reference link lines).
  - Normative anchors: `<a id="norm-..."></a>` (tech specs only; end of normative list item line under a standardized "Normative Requirements" heading).
- Extend **Requirements to Spec References** to allow a list-style requirements format:
  - Each `REQ-*` list item has continuation lines: one or more spec reference links (Spec ID as link text, href to `norm-*` or `spec-*` anchor), then the `req-*` anchor on its own line.
  - No dedicated "Spec References" section; references are these continuation lines under the list item.

## 3 Proposed Requirements Set Structure

- `docs/requirements/README.md`
  - Index of domains.
  - Pointers to authoring rules and ID conventions.
- `docs/requirements/<domain>.md` (one file per domain).
  - Contains a list-style index of requirements for that domain.
  - Each requirement entry is a single list item with:
    - requirement ID
    - a short label (non-normative, for readability only)
    - one or more links to canonical tech spec anchors (fine-grained, subsection or bullet-level)

Notes:

- Domain-to-file mapping must be stable.
- Keep files reasonably sized to avoid exceeding doc-length validation.
- There MUST be one canonical location for a given spec obligation (the tech specs).
  Requirements documents MUST only reference those obligations via stable anchors.

## 4 Requirements Item Format (Draft)

The requirements docs should be list-like (similar to `novuspack/docs/requirements/basic_ops.md`) and should not duplicate spec content.
Each entry has: (1) a first line with requirement ID and short label, (2) one or more continuation lines each giving a spec reference as a link (Spec ID as link text, target the canonical `norm-*` or `spec-*` anchor), (3) a final continuation line with the `req-*` anchor.

```markdown
- REQ-ACCESS-0001: Default deny stance for access control policy evaluation.
  [`CYNAI.ACCESS.Control.Policy.Evaluation`](../tech_specs/access_control.md#norm-should-cynai-access-control-policy-evaluation-0001)
  <a id="req-access-0001"></a>
```

Draft anchor rule for requirements:

- Anchor IDs are derived from the requirement ID by lowercasing and converting `REQ-<DOMAIN>-<NNNN>` to `req-<domain>-<nnnn>`.
- Example: `REQ-ACCESS-0001` => `req-access-0001`.

## 5 Requirements Domains (Draft Proposal)

Canonical domains are defined in [`docs/docs_standards/requirements_domains.md`](../docs/docs_standards/requirements_domains.md).
That file is the single source of truth for the domain list and domain-to-file mapping.

Domain constraints:

- Domain tags are exactly 6 uppercase letters (A-Z), with no underscores.
- Requirement IDs use `REQ-<DOMAIN>-<NNNN>` (e.g. `REQ-ACCESS-0001`).

Draft domain candidates (to be confirmed):

- `ACCESS`: Authorization, policy evaluation, RBAC, scopes, and enforcement points.
  - [access_control.md](../docs/tech_specs/access_control.md)
  - [rbac_and_groups.md](../docs/tech_specs/rbac_and_groups.md)
  - [projects_and_scopes.md](../docs/tech_specs/projects_and_scopes.md)
- `IDENTY`: Local user accounts, sessions, and authentication flows.
  - [local_user_accounts.md](../docs/tech_specs/local_user_accounts.md)
- `ORCHES`: Orchestrator control-plane behavior, task lifecycle, dispatch, and state.
  - [orchestrator.md](../docs/tech_specs/orchestrator.md)
- `WORKER`: Worker-node behavior, job execution, reporting, and node management.
  - [node.md](../docs/tech_specs/node.md)
  - [node_payloads.md](../docs/tech_specs/node_payloads.md)
  - [worker_api.md](../docs/tech_specs/worker_api.md)
- `SANDBX`: Sandbox execution model, container constraints, and isolation.
  - [sandbox_container.md](../docs/tech_specs/sandbox_container.md)
- `IMGREG`: Sandbox image registry requirements.
  - [sandbox_image_registry.md](../docs/tech_specs/sandbox_image_registry.md)
- `MCPGAT`: MCP gateway enforcement, auditing, and tool invocation policy controls.
  - [mcp_gateway_enforcement.md](../docs/tech_specs/mcp_gateway_enforcement.md)
  - [mcp_tool_call_auditing.md](../docs/tech_specs/mcp_tool_call_auditing.md)
  - [mcp_tooling.md](../docs/tech_specs/mcp_tooling.md)
  - [mcp_tool_catalog.md](../docs/tech_specs/mcp_tool_catalog.md)
  - [mcp_sdk_installation.md](../docs/tech_specs/mcp_sdk_installation.md)
- `APIEGR`: Controlled egress services and API egress behavior.
  - [api_egress_server.md](../docs/tech_specs/api_egress_server.md)
- `GITEGR`: Git egress requirements.
  - [git_egress_mcp.md](../docs/tech_specs/git_egress_mcp.md)
- `CONNEC`: Connector framework and external connector integration.
  - [connector_framework.md](../docs/tech_specs/connector_framework.md)
- `MODELS`: Model lifecycle, routing, and model management requirements.
  - [model_management.md](../docs/tech_specs/model_management.md)
  - [external_model_routing.md](../docs/tech_specs/external_model_routing.md)
- `AGENTS`: Agent behaviors, responsibilities, and orchestration integration.
  - [cloud_agents.md](../docs/tech_specs/cloud_agents.md)
  - [project_manager_agent.md](../docs/tech_specs/project_manager_agent.md)
  - [project_analyst_agent.md](../docs/tech_specs/project_analyst_agent.md)
  - [langgraph_mvp.md](../docs/tech_specs/langgraph_mvp.md)
- `CLIMGT`: CLI management app requirements.
  - [cli_management_app.md](../docs/tech_specs/cli_management_app.md)
  - [user_preferences.md](../docs/tech_specs/user_preferences.md)
- `ADMWEB`: Admin web console requirements.
  - [admin_web_console.md](../docs/tech_specs/admin_web_console.md)
- `PERSIST`: Data persistence and storage requirements.
  - (no dedicated spec; persistence is described in postgres_schema and related docs)
- `SCHEMA`: Schema-level invariants and database constraints.
  - [postgres_schema.md](../docs/tech_specs/postgres_schema.md)
- `USRGWY`: User-facing API gateway and runs/sessions APIs.
  - [user_api_gateway.md](../docs/tech_specs/user_api_gateway.md)
  - [runs_and_sessions_api.md](../docs/tech_specs/runs_and_sessions_api.md)
- `DATAPI`: Data REST API behavior and contracts.
  - [data_rest_api.md](../docs/tech_specs/data_rest_api.md)
- `STANDS`: Cross-cutting REST API and documentation standards.
  - [go_rest_api_standards.md](../docs/tech_specs/go_rest_api_standards.md)
- `BOOTST`: Bootstrap configuration for orchestrator and nodes.
  - [orchestrator_bootstrap.md](../docs/tech_specs/orchestrator_bootstrap.md)
- `BROWSR`: Secure browser service requirements.
  - [secure_browser_service.md](../docs/tech_specs/secure_browser_service.md)

## 6 Consolidation and Reconciliation Workflow

The workflow is anchored around making the tech specs linkable at the right granularity.

- Normalize tech spec structure for normative content.
  - Convert informal "Normative requirements" labels into consistent headings.
  - Ensure each normative block has a subsection-level Spec ID with a stable `spec-*` anchor.
- Add fine-grained anchors for normative requirements.
  - Add stable anchors for each normative bullet (MUST, SHOULD, MAY, MUST NOT).
  - These anchors become the canonical link targets for requirements docs.
- Build requirements as an index, not a restatement.
  - Create one requirement list entry per normative statement (or a tightly-coupled unit explicitly defined by the spec).
  - Keep requirement entry text minimal and non-normative.
  - Always include at least one canonical anchor link into `docs/tech_specs/`.
- Reconcile duplicates and conflicts.
  - If multiple specs define the same obligation, one requirement ID links to multiple canonical anchors.
  - If there is a conflict, surface it with links to the conflicting anchors.

## 7 Anchor and Linking Plan

The requirements docs should point to tech specs at subsection or bullet-level, not doc-level.

Proposed anchor layers:

- Spec Item anchors (existing `spec-*`):
  - Purpose: stable anchors for subsection-level spec items.
  - Placement: appended to `- Spec ID:`...`` bullet lines.
- Normative requirement anchors (new, fine-grained):
  - Purpose: stable anchors for individual MUST, SHOULD, MAY, MUST NOT obligations.
  - Placement: appended to the end of the corresponding list item line inside a normative requirements list.
  - Note: This intentionally mirrors the existing "Algorithm step anchors" pattern (anchor appended to the list item line), but for normative requirement bullets.
  - Proposed IDs:
    - `norm-must-<normalized-spec-id>-<nnnn>`
    - `norm-mustnot-<normalized-spec-id>-<nnnn>`
    - `norm-should-<normalized-spec-id>-<nnnn>`
    - `norm-may-<normalized-spec-id>-<nnnn>`
  - Example:
    - Spec ID `CYNAI.CLIENT.AdminWebConsole.SecurityModel` => `normalized-spec-id=cynai-client-adminwebconsole-securitymodel`
    - `norm-must-cynai-client-adminwebconsole-securitymodel-0001`

Requirements docs linking rules:

- Each `REQ-*` entry MUST link to at least one `norm-*` anchor in `docs/tech_specs/`.
- A `REQ-*` entry MAY additionally link to a subsection-level `spec-*` anchor when the spec defines a cluster of obligations under a single spec item.
- Requirements internal links:
  - Each requirement entry defines a stable `req-*` anchor derived from the requirement ID.
  - Requirements should cross-reference via `#req-*` anchors.

## 8 `.markdownlint.yml` Updates (Anchor Allowlist)

The lint rules need to support:

- requirement entry anchors (`req-*`) at the end of requirement list items, and
- fine-grained normative anchors (`norm-*`) at the end of normative list items in tech specs.

Proposed allowed ID patterns (tight):

- Requirement entry anchors:
  - Pattern: `^req-[a-z]{6}-[0-9]{4}$`
- Normative anchors:
  - Pattern: `^norm-(?:must|mustnot|should|may)-[a-z0-9-]+-[0-9]{4}$`

Proposed placement constraints (tight):

- `req-*` anchors:
  - Must appear on a continuation line under the requirement list item (after the `- REQ-...:` line and after any spec reference link lines).
  - Requirement list items must match `^- REQ-[A-Z]{6}-[0-9]{4}:`.
  - Exactly one `req-*` anchor per requirement entry.
- `norm-*` anchors:
  - Must be appended to the end of a list item line.
  - Must only appear under a heading that includes the exact phrase "Normative Requirements" (standardized heading text).
  - Optionally enforce that the list item text includes the corresponding RFC-2119 token.

## 9 Work Breakdown (Phased)

- Phase A: Standardize normative blocks in tech specs.
  - Identify all "Normative requirements" blocks across `docs/tech_specs/`.
  - Convert each to a consistent heading form (for example `### <Topic> Normative Requirements`).
  - Define a subsection-level Spec ID per normative block (so requirements can reference subsections).
- Phase B: Add fine-grained normative anchors.
  - Add a stable `norm-*` anchor to each normative list item (MUST, SHOULD, MAY, MUST NOT).
  - Ensure anchor IDs are derived from the Spec ID for the normative block plus a stable ordinal.
- Phase C: Adjust lint constraints.
  - Update `.markdownlint.yml` to allow `norm-*` anchors with strict placement rules.
  - Update `.markdownlint.yml` so `req-*` anchors can be appended to `- REQ-...:` requirement list items.
- Phase D: Rebuild requirements docs as lists.
  - Replace per-requirement multi-section writeups with list-style entries.
  - Create one `REQ-*` entry per normative statement (or explicitly grouped unit).
  - Ensure every `REQ-*` links to at least one canonical `norm-*` anchor in tech specs.
- Phase E: Reconcile and validate.
  - Deduplicate requirements that point to the same canonical obligation.
  - Surface conflicts explicitly with links to the conflicting anchors.
  - Run `just lint-md` and fix violations.

## 10 Open Questions for User

Decisions captured (2026-02-17):

- Domains: Define the full long-term domain list upfront.
- Coverage: Include `MUST`, `SHOULD`, and `MAY` statements as requirements candidates.
- Anchors: Use lowercase derived requirement anchors (`REQ-ACCESS-0001` => `req-access-0001`).
- Layout: One file per domain under `docs/requirements/`.
- Traceability: Each requirement item includes at least one link back into `docs/tech_specs/`.
- ID width: 4 digits (`REQ-ACCESS-0001`).

## 11 Acceptance Criteria

- `docs/tech_specs/` defines subsection-level Spec IDs and anchors for normative blocks.
- Each normative statement in tech specs has a stable, linkable `norm-*` anchor.
- `docs/requirements/` is list-style and contains no duplicated canonical spec content.
- Each `REQ-*` entry links to at least one canonical `norm-*` anchor in tech specs.
- Lint rules allow and enforce anchor placement for `spec-*`, `norm-*`, and `req-*`.
- `just lint-md` passes for all changed Markdown.
