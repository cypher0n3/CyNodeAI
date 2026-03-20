# Vector Storage (Pgvector)

- [Document Overview](#document-overview)
- [Postgres Schema](#postgres-schema)
  - [Vector Storage Applicable Requirements](#vector-storage-applicable-requirements)
  - [Vector Items Table](#vector-items-table)
  - [Vector Retrieval and RBAC](#vector-retrieval-and-rbac)

## Document Overview

- Spec ID: `CYNAI.SCHEMA.VectorStorage` <a id="spec-cynai-schema-vectorstorage"></a>

CyNodeAI uses PostgreSQL and pgvector for vector storage and similarity search.
Vector storage supports retrieval and semantic search over task-related content.

Related documents

- [Postgres Schema Index](postgres_schema.md#spec-cynai-schema-vectorstorage)

## Postgres Schema

- Spec ID: `CYNAI.SCHEMA.VectorStorage` <a id="spec-cynai-schema-vectorstorage-schema"></a>

### Vector Storage Applicable Requirements

Vector storage requirements and recommended behavior.

#### Recommended Behavior

- Prefer cosine distance for similarity search.
- Use an approximate index (HNSW when available; otherwise IVFFLAT) to keep queries fast at scale.
- Store only sanitized, policy-allowed content in vector storage.
- Keep similarity search queries isolated in a repository layer so they can be tuned without changing callers.

#### Vector Storage Applicable Requirements Requirements Traces

- [REQ-SCHEMA-0106](../requirements/schema.md#req-schema-0106)
- [REQ-SCHEMA-0107](../requirements/schema.md#req-schema-0107)
- [REQ-SCHEMA-0108](../requirements/schema.md#req-schema-0108)
- [REQ-SCHEMA-0109](../requirements/schema.md#req-schema-0109)
- [REQ-SCHEMA-0110](../requirements/schema.md#req-schema-0110)
- [REQ-SCHEMA-0111](../requirements/schema.md#req-schema-0111)
- [REQ-ACCESS-0125](../requirements/access.md#req-access-0125)

### Vector Items Table

- Spec ID: `CYNAI.SCHEMA.VectorItemsTable` <a id="spec-cynai-schema-vectoritemstable"></a>

Table name: `vector_items`.

This table stores chunked text content and its embedding.
It is intentionally generic so multiple sources can be indexed (artifacts, run logs, connector documents, sanitized web pages).

- `id` (uuid, pk)
- `project_id` (uuid, fk to `projects.id`, nullable)
- `task_id` (uuid, fk to `tasks.id`, nullable)
- `namespace` (text, nullable)
  - coarse-grained policy boundary (e.g. docs, skills, project_memory, code_index); used for RBAC filtering
- `sensitivity_level` (text, nullable)
  - ordered level for role-based filtering (e.g. public, internal, confidential, restricted); queries enforce `chunk.sensitivity_level <= role.max_sensitivity_level`
- `source_type` (text)
  - examples: task_artifact, run_log, connector_doc, web_page, note
- `source_ref` (text, nullable)
  - stable identifier for the source (e.g. artifact path, URL, connector id)
- `chunk_index` (int)
- `content_text` (text)
- `content_sha256` (text)
- `embedding_model` (text)
  - examples: text-embedding-3-small, bge-m3, nomic-embed-text
- `embedding_dim` (int)
- `embedding` (vector(1536))
  - dimension is an example and is set to the system's configured embedding dimension
  - the configured embedding dimension matches `embedding_dim`
- `metadata` (jsonb, nullable)
- `created_at` (timestamptz)
- `updated_at` (timestamptz)

#### Vector Items Table Constraints

- Index: (`project_id`)
- Index: (`project_id`, `namespace`) for RBAC-filtered similarity queries
- Index: (`task_id`)
- Index: (`source_type`)
- Index: (`content_sha256`)

#### Indexing Guidance

- Create a pgvector index on `embedding` using cosine operators.
- Pair the index with filters (for example `task_id`) to avoid cross-scope retrieval.
- Index type and parameters are explicit and versioned in migrations.
- Use IVFFLAT for approximate search when HNSW is unavailable.
- Use HNSW when pgvector supports it and it meets performance requirements for the expected dataset size.

#### Example Index Definitions

```sql
-- IVFFLAT (approximate).
-- Tune lists and probes based on dataset size and recall requirements.
CREATE INDEX CONCURRENTLY IF NOT EXISTS vector_items_embedding_ivfflat
ON vector_items USING ivfflat (embedding vector_cosine_ops)
WITH (lists = 100);

-- HNSW (newer pgvector).
-- Tune m and ef_construction based on dataset size and recall requirements.
CREATE INDEX CONCURRENTLY IF NOT EXISTS vector_items_embedding_hnsw
ON vector_items USING hnsw (embedding vector_cosine_ops)
WITH (m = 16, ef_construction = 200);
```

### Vector Retrieval and RBAC

- Spec ID: `CYNAI.SCHEMA.VectorRetrievalRbac` <a id="spec-cynai-schema-vectorretrievalrbac"></a>

Vector retrieval does not bypass RBAC.
Similarity search is only allowed within an already-authorized document set; authorization is applied in SQL before similarity ranking.

#### Vector Retrieval and RBAC Requirements Traces

- [REQ-ACCESS-0121](../requirements/access.md#req-access-0121)
- [REQ-ACCESS-0122](../requirements/access.md#req-access-0122)
- [REQ-ACCESS-0123](../requirements/access.md#req-access-0123)
- [REQ-ACCESS-0124](../requirements/access.md#req-access-0124)
- [REQ-SCHEMA-0111](../requirements/schema.md#req-schema-0111)
- [REQ-SCHEMA-0112](../requirements/schema.md#req-schema-0112)

#### Vector Query Flow

1. Authenticate the caller and resolve effective permissions (allowed project_ids, allowed namespaces, max_sensitivity_level for the role).
2. Build the candidate set with explicit filters: WHERE project_id IN (authorized_projects), AND namespace IN (authorized_namespaces), AND (sensitivity_level IS NULL OR sensitivity_level <= allowed_max).
3. Run similarity ranking only against the filtered candidate set (e.g. ORDER BY embedding <=> query_embedding LIMIT top_k).
4. Return results with provenance metadata; do not return full document bodies or hidden metadata beyond chunk text and allowed provenance.

RBAC filtering occurs in SQL before similarity scoring.
No "open" vector queries are allowed; every query includes explicit project/namespace/sensitivity constraints derived from the authenticated subject.

#### Vector Ingestion

- Only controlled services may insert into vector storage; ingestion requires write permission on the target scope (project, namespace) and correct project association per [REQ-ACCESS-0125](../requirements/access.md#req-access-0125).

#### Vector Audit

- Every retrieval is logged (e.g. user_id, role, project_id, namespaces queried, chunk count returned, timestamp) per [REQ-ACCESS-0124](../requirements/access.md#req-access-0124).

#### Vector Performance

- Use composite indexes on (project_id, namespace) so that filtering reduces the candidate set before similarity search; similarity search runs only against already filtered rows.
