# DATAPI Requirements

- [1 Overview](#1-overview)
- [2 Requirements](#2-requirements)

## 1 Overview

This document consolidates requirements for the `DATAPI` domain.
It covers the Data REST API behavior and related data access semantics.

## 2 Requirements

- **REQ-DATAPI-0001:** Data REST API in User API Gateway; no raw SQL; authn/authz and audit on all endpoints; GORM repository layer and explicit transactions.
  [CYNAI.DATAPI.ScopeBoundaries](../tech_specs/data_rest_api.md#spec-cynai-datapi-scopebound)
  [CYNAI.DATAPI.DatabaseAccessOrm](../tech_specs/data_rest_api.md#spec-cynai-datapi-dbaccessorm)
  <a id="req-datapi-0001"></a>
- **REQ-DATAPI-0100:** The Data REST API MUST be implemented by the User API Gateway.
  [CYNAI.DATAPI.ScopeBoundaries](../tech_specs/data_rest_api.md#spec-cynai-datapi-scopebound)
  <a id="req-datapi-0100"></a>
- **REQ-DATAPI-0101:** The Data REST API MUST NOT expose raw SQL execution.
  [CYNAI.DATAPI.ScopeBoundaries](../tech_specs/data_rest_api.md#spec-cynai-datapi-scopebound)
  <a id="req-datapi-0101"></a>
- **REQ-DATAPI-0102:** The Data REST API MUST enforce authentication and authorization for all endpoints.
  [CYNAI.DATAPI.ScopeBoundaries](../tech_specs/data_rest_api.md#spec-cynai-datapi-scopebound)
  <a id="req-datapi-0102"></a>
- **REQ-DATAPI-0103:** The Data REST API MUST emit audit logs for reads and writes.
  [CYNAI.DATAPI.ScopeBoundaries](../tech_specs/data_rest_api.md#spec-cynai-datapi-scopebound)
  <a id="req-datapi-0103"></a>
- **REQ-DATAPI-0104:** The Data REST API MUST use a repository layer backed by GORM.
  [CYNAI.DATAPI.DatabaseAccessOrm](../tech_specs/data_rest_api.md#spec-cynai-datapi-dbaccessorm)
  <a id="req-datapi-0104"></a>
- **REQ-DATAPI-0105:** The Data REST API MUST use GORM's PostgreSQL driver with `pgx`.
  [CYNAI.DATAPI.DatabaseAccessOrm](../tech_specs/data_rest_api.md#spec-cynai-datapi-dbaccessorm)
  <a id="req-datapi-0105"></a>
- **REQ-DATAPI-0106:** The Data REST API MUST propagate `context.Context` to all database operations.
  [CYNAI.DATAPI.DatabaseAccessOrm](../tech_specs/data_rest_api.md#spec-cynai-datapi-dbaccessorm)
  <a id="req-datapi-0106"></a>
- **REQ-DATAPI-0107:** The Data REST API MUST define explicit transaction boundaries for multi-table writes and idempotent upserts.
  [CYNAI.DATAPI.DatabaseAccessOrm](../tech_specs/data_rest_api.md#spec-cynai-datapi-dbaccessorm)
  <a id="req-datapi-0107"></a>
- **REQ-DATAPI-0108:** The Data REST API MUST be compatible with an AutoMigrate-first schema workflow (GORM models as the schema definition).
  [CYNAI.DATAPI.DatabaseAccessOrm](../tech_specs/data_rest_api.md#spec-cynai-datapi-dbaccessorm)
  <a id="req-datapi-0108"></a>
- **REQ-DATAPI-0109:** The Data REST API MUST NOT execute schema-altering operations at request time.
  [CYNAI.DATAPI.DatabaseAccessOrm](../tech_specs/data_rest_api.md#spec-cynai-datapi-dbaccessorm)
  <a id="req-datapi-0109"></a>
- **REQ-DATAPI-0110:** The Data REST API MUST treat embedding dimension changes as a breaking schema change.
  [CYNAI.DATAPI.DatabaseAccessOrm](../tech_specs/data_rest_api.md#spec-cynai-datapi-dbaccessorm)
  <a id="req-datapi-0110"></a>
- **REQ-DATAPI-0111:** Vector similarity queries MUST be isolated behind a repository interface.
  [CYNAI.DATAPI.DatabaseAccessOrm](../tech_specs/data_rest_api.md#spec-cynai-datapi-dbaccessorm)
  <a id="req-datapi-0111"></a>
- **REQ-DATAPI-0112:** Vector similarity queries SHOULD use `db.Raw()` for pgvector operators rather than relying on ORM abstractions.
  [CYNAI.DATAPI.DatabaseAccessOrm](../tech_specs/data_rest_api.md#spec-cynai-datapi-dbaccessorm)
  <a id="req-datapi-0112"></a>
