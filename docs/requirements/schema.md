# SCHEMA Requirements

- [1 Overview](#1-overview)
- [2 Requirements](#2-requirements)

## 1 Overview

This document consolidates requirements for the `SCHEMA` domain.
It covers data persistence requirements, schema-level invariants, and database constraints.

## 2 Requirements

- REQ-SCHEMA-0001: Schema in code (GORM); AutoMigrate + explicit DDL bootstrap; pgvector extension and vector columns with explicit dimension and scope.
  [CYNAI.SCHEMA.StoringInCode](../tech_specs/postgres_schema.md#spec-cynai-schema-storingcode)
  [CYNAI.SCHEMA.VectorStorage](../tech_specs/postgres_schema.md#spec-cynai-schema-vectorstorage)
  <a id="req-schema-0001"></a>

- REQ-SCHEMA-0100: The schema MUST be represented in Go as GORM models (structs + tags).
  [CYNAI.SCHEMA.StoringInCode](../tech_specs/postgres_schema.md#spec-cynai-schema-storingcode)
  <a id="req-schema-0100"></a>
- REQ-SCHEMA-0101: The orchestrator MUST provide a supported way to apply schema changes in dev and CI using GORM `AutoMigrate`.
  [CYNAI.SCHEMA.StoringInCode](../tech_specs/postgres_schema.md#spec-cynai-schema-storingcode)
  <a id="req-schema-0101"></a>
- REQ-SCHEMA-0102: Production deployments MUST have an explicit, supported schema-application step.
  [CYNAI.SCHEMA.StoringInCode](../tech_specs/postgres_schema.md#spec-cynai-schema-storingcode)
  <a id="req-schema-0102"></a>
- REQ-SCHEMA-0103: Implementations MUST pin and align the database stack versions used to generate schema changes.
  [CYNAI.SCHEMA.StoringInCode](../tech_specs/postgres_schema.md#spec-cynai-schema-storingcode)
  <a id="req-schema-0103"></a>
- REQ-SCHEMA-0104: Implementations MUST NOT rely on GORM `AutoMigrate` alone for all PostgreSQL DDL.
  [CYNAI.SCHEMA.StoringInCode](../tech_specs/postgres_schema.md#spec-cynai-schema-storingcode)
  <a id="req-schema-0104"></a>
- REQ-SCHEMA-0105: The DDL bootstrap step MUST be idempotent and safe to run repeatedly.
  [CYNAI.SCHEMA.StoringInCode](../tech_specs/postgres_schema.md#spec-cynai-schema-storingcode)
  <a id="req-schema-0105"></a>
- REQ-SCHEMA-0106: The database MUST enable the pgvector extension via the DDL bootstrap step.
  [CYNAI.SCHEMA.VectorStorage](../tech_specs/postgres_schema.md#spec-cynai-schema-vectorstorage)
  <a id="req-schema-0106"></a>
- REQ-SCHEMA-0107: Vector columns MUST use an explicit pgvector dimension (for example `vector(1536)`).
  [CYNAI.SCHEMA.VectorStorage](../tech_specs/postgres_schema.md#spec-cynai-schema-vectorstorage)
  <a id="req-schema-0107"></a>
- REQ-SCHEMA-0108: Embedding dimension changes MUST be treated as a breaking schema change and handled via a deterministic schema change process.
  [CYNAI.SCHEMA.VectorStorage](../tech_specs/postgres_schema.md#spec-cynai-schema-vectorstorage)
  <a id="req-schema-0108"></a>
- REQ-SCHEMA-0109: Vector rows MUST be scoped so queries can filter by `task_id` and `project_id` when applicable.
  [CYNAI.SCHEMA.VectorStorage](../tech_specs/postgres_schema.md#spec-cynai-schema-vectorstorage)
  <a id="req-schema-0109"></a>
- REQ-SCHEMA-0110: Vector rows MUST record the embedding model identifier used to produce the embedding.
  [CYNAI.SCHEMA.VectorStorage](../tech_specs/postgres_schema.md#spec-cynai-schema-vectorstorage)
  <a id="req-schema-0110"></a>
