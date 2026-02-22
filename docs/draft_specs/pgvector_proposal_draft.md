# Proposal - Pgvector Usage With Strict RBAC in CyNodeAI

## 1. Purpose

Define how pgvector integrates with CyNodeAI's strict RBAC model (users, groups, projects, roles) while preserving:

- Hard tenant isolation
- Project-scoped access
- Least-privilege enforcement
- Deterministic policy controls
- Auditable retrieval

## 2. RBAC Model Assumptions

CyNodeAI enforces:

- Users belong to zero or more Groups
- Groups are assigned Roles
- Roles grant scoped permissions
- Permissions are scoped to:

  - Tenant
  - Project
  - Repository
  - Skill
  - Sensitivity level

Vector retrieval must respect the same scoping model as all other resources.

## 3. Core Principle

Vector retrieval must never bypass RBAC.

Similarity search is only allowed inside an already-authorized document set.

Authorization must be applied before similarity ranking.

## 4. Data Model Adjustments for RBAC

Tables and fields are extended for RBAC as follows.

### 4.1 Authoritative Metadata Tables

Add explicit access metadata:

- `documents`

  - doc_id
  - tenant_id
  - project_id
  - repo_id (nullable)
  - sensitivity_level
  - owner_group_id
  - version
  - source_hash

- `chunks`

  - chunk_id
  - doc_id
  - tenant_id
  - project_id
  - sensitivity_level
  - namespace
  - content_hash

- `embeddings_<model_version>`

  - chunk_id
  - tenant_id
  - project_id
  - namespace
  - embedding vector
  - created_at

Important: tenant_id and project_id must be indexed and enforced.

## 5. Retrieval Flow With RBAC Enforcement

Queries must apply RBAC before similarity ranking.

### 5.1 Query Flow

1. Authenticate user
2. Resolve effective permissions:

   - tenant_id
   - allowed project_ids
   - allowed namespaces
   - allowed sensitivity levels
3. Build candidate set:

   - WHERE tenant_id = ?
   - AND project_id IN (authorized_projects)
   - AND namespace IN (authorized_namespaces)
   - AND sensitivity_level <= allowed_level
4. Apply similarity ranking within filtered candidate set
5. Return results with provenance metadata

RBAC filtering must occur in SQL before similarity scoring.

### 5.2 Why Pre-Filtering Matters

If similarity ranking occurs first:

- Unauthorized documents could influence ranking
- Timing or scoring differences could leak information
- Cross-project inference becomes possible

Therefore, strict pre-filtering is mandatory.

## 6. Namespace-Based Isolation

Use namespaces as coarse-grained policy boundaries:

Examples:

- docs
- skills
- project_memory
- global_memory
- code_index
- incidents

Role-based rules may allow:

- PM role -> docs + skills + project_memory
- Analyst role -> docs + skills + incidents
- Developer role -> docs + code_index
- Sandbox role -> none (default)

Namespaces must not substitute project-level filtering; they are additive.

## 7. Sensitivity Model

Introduce ordered sensitivity levels:

- public
- internal
- confidential
- restricted

Each role has a max_sensitivity_level.

Query must enforce:

- chunk.sensitivity_level <= role.max_sensitivity_level

This prevents high-privilege documents from being retrieved by lower roles even inside same project.

## 8. Cross-Project and Global Content

Global and cross-project retrieval are restricted as follows.

### 8.1 Global Content

Certain namespaces (e.g., core skills, global docs) may be marked:

- project_id = null
- scope = global

Access rule:

- Must still match tenant_id
- Must match role permission for namespace

### 8.2 Cross-Project Restrictions

By default:

- No cross-project retrieval
- No implicit aggregation across projects
- No union queries unless explicitly allowed by role

If a role allows multi-project visibility, project_id filter must still be explicit.

## 9. Write Path Controls

Ingestion and agent writes are controlled as follows.

### 9.1 Ingestion Authorization

Embedding ingestion must require:

- write permission on document
- write permission in namespace
- correct project association

Only controlled services may insert embeddings.

### 9.2 Agent Write Restrictions

cynode-pm may not:

- Directly write embeddings
- Write arbitrary memory

Instead:

- It submits a "store_memory" request
- Host validates:

  - namespace allowed
  - size limits
  - no secrets
  - sensitivity tagging
- Only then is content embedded

## 10. Memory Segmentation Strategy

Memory is segmented by project, user, and global scope.

### 10.1 Project Memory

Scoped strictly to project_id.

Never retrievable outside project.

### 10.2 User Memory (Optional)

If implemented:

- Scoped to user_id + project_id
- Not visible to other users unless role allows

### 10.3 Global Memory

Only for platform-level knowledge:

- skill registry
- platform architecture docs

Requires elevated role.

## 11. Query Guardrails

All vector queries must enforce:

- explicit tenant_id
- explicit project filter
- explicit namespace filter
- top_k limit
- maximum result size
- rate limits per user

No "open" vector queries allowed.

## 12. Audit Requirements

Every retrieval must log:

- user_id
- role
- tenant_id
- project_id
- namespaces queried
- chunk_ids returned
- similarity scores (optional)
- timestamp

This ensures traceability.

## 13. Isolation Model Summary

Isolation must exist at three levels:

1. Tenant isolation (hard boundary)
2. Project isolation (default boundary)
3. Sensitivity isolation (role-based boundary)

Vector similarity must operate only inside the smallest allowed boundary.

## 14. Performance Considerations With RBAC

Because filtering reduces candidate set size:

- Create composite indexes on:

  - (tenant_id, project_id, namespace)
- Partition large deployments by tenant_id
- Consider per-tenant schemas if scale demands it

Similarity search should run only against already filtered rows.

## 15. Threat Model Considerations

Key risks and mitigations are summarized below.

### 15.1 Embedding Leakage Risk

Embeddings may encode sensitive information.

Mitigations:

- No secret ingestion
- No raw credential embedding
- Redaction pipeline before embedding
- Sensitivity tagging

### 15.2 Side-Channel Risks

Mitigate:

- Consistent query timing
- Fixed top_k
- No partial result metadata leaks

### 15.3 Model Prompt Leakage

Only return:

- chunk text
- provenance metadata

Never return:

- full document bodies beyond chunk
- hidden metadata fields

## 16. MVP With RBAC

MVP must include:

- tenant_id scoping
- project_id scoping
- namespace filtering
- sensitivity level enforcement
- ingestion authorization checks
- retrieval audit logs

Defer:

- user-scoped memory
- multi-project aggregation roles
- advanced row-level security policies if app-layer enforcement is sufficient

## 17. Acceptance Criteria

- A user cannot retrieve content from a project they are not assigned to.
- A lower-privilege role cannot retrieve higher-sensitivity chunks.
- Vector similarity never runs outside an authorized subset.
- All retrievals are auditable.
- Embedding rebuild does not break RBAC metadata.

## 18. Architectural Outcome

pgvector becomes:

- A scoped retrieval accelerator
- Not an authority
- Not a global knowledge pool

RBAC remains authoritative.

Similarity operates strictly inside authorized boundaries.

This preserves CyNodeAI's core security model while enabling efficient contextual retrieval.
