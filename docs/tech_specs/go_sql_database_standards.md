# Go SQL Database Standards (GORM)

- [Document Overview](#document-overview)
- [`GoSqlGorm` Rule](#gosqlgorm-rule)
  - [`GoSqlGorm` Rule Requirements Traces](#gosqlgorm-rule-requirements-traces)
  - [`GoSqlGorm` Scope](#gosqlgorm-scope)
  - [`GoSqlGorm` Preconditions](#gosqlgorm-preconditions)
  - [`GoSqlGorm` Outcomes](#gosqlgorm-outcomes)
  - [`GoSqlGorm` Error Conditions](#gosqlgorm-error-conditions)
- [Drivers and Backends](#drivers-and-backends)
- [Relationship to Other Specs](#relationship-to-other-specs)

## Document Overview

This document is the single canonical specification for Go SQL database access in the CyNodeAI codebase.
All Go code that reads from or writes to a SQL database (PostgreSQL, SQLite, or any other SQL backend) MUST use GORM as the access layer.

Normative obligations for this behavior are in [docs/requirements/stands.md](../requirements/stands.md) (REQ-STANDS-0117, REQ-STANDS-0134).
This spec prescribes implementation guidance and scope.

## `GoSqlGorm` Rule

- Spec ID: `CYNAI.STANDS.GoSqlGorm` <a id="spec-cynai-stands-gosqlgorm"></a>

Implementations MUST use GORM for all SQL database access from Go: model definitions (structs with GORM tags), queries, and schema application (AutoMigrate or explicit migrations).
Raw `database/sql` or other ORMs MUST NOT be used for SQL persistence in this codebase.

### `GoSqlGorm` Rule Requirements Traces

- [REQ-STANDS-0117](../requirements/stands.md#req-stands-0117)
- [REQ-STANDS-0134](../requirements/stands.md#req-stands-0134)

### `GoSqlGorm` Scope

- **In scope:** Orchestrator PostgreSQL (control-plane, user-gateway, api-egress, and any service that connects to the orchestrator database).
- **In scope:** Worker node SQLite (e.g. telemetry database at `${storage.state_dir}/telemetry/telemetry.db`).
- **In scope:** Any future SQL backend introduced in the repo (e.g. additional Postgres or SQLite databases).
- **Out of scope:** Non-Go components; external systems that use their own DB drivers.

### `GoSqlGorm` Preconditions

- The component requires persistent storage backed by a SQL database.
- The database is owned or directly accessed by CyNodeAI Go code (not only via an external API).

### `GoSqlGorm` Outcomes

- All table definitions are represented as GORM models (Go structs with appropriate GORM struct tags).
- All reads and writes go through GORM (e.g. `db.WithContext(ctx).Find()`, `Create()`, `Updates()`, `Raw()` only where necessary for DB-specific SQL such as pgvector).
- Schema application uses GORM's migration workflow: AutoMigrate and/or explicit DDL as specified by the schema spec for that database (e.g. [postgres_schema.md](postgres_schema.md), [worker_telemetry_api.md](worker_telemetry_api.md)).
- Context propagation: all database operations use a context-aware API (e.g. `db.WithContext(ctx)`) so that cancellation and timeouts apply.

### `GoSqlGorm` Error Conditions

- Use GORM's error handling; check `result.Error` and treat `gorm.ErrRecordNotFound` where appropriate.
- Do not bypass GORM to run arbitrary raw SQL for normal CRUD; raw SQL is allowed only for DB-specific features (e.g. pgvector similarity) and MUST be isolated behind repository or query helpers.

## Drivers and Backends

- **PostgreSQL:** Use GORM with the `pgx` driver as required by [REQ-STANDS-0118](../requirements/stands.md#req-stands-0118).
  See [go_rest_api_standards.md - Database Access](go_rest_api_standards.md#spec-cynai-stands-dbaccess) for connection pooling, logging, and pgvector guidance.
- **SQLite:** Use a GORM-compatible SQLite driver (e.g. `gorm.io/driver/sqlite`) for node-local databases such as worker telemetry.
  Schema and location are defined by [worker_telemetry_api.md](worker_telemetry_api.md); implementations MUST still use GORM models and GORM-based schema application to satisfy REQ-STANDS-0134.

## Relationship to Other Specs

- **Orchestrator PostgreSQL:** Table definitions, columns, and constraints are in [postgres_schema.md](postgres_schema.md).
  GORM models MUST reflect that schema; DDL bootstrap (extensions, advanced indexes) may use idempotent SQL in addition to AutoMigrate.
- **Worker Telemetry SQLite:** Contract, schema, and retention are in [worker_telemetry_api.md](worker_telemetry_api.md).
  Implementations MUST use GORM for all access and schema application; the logical schema in that spec is the source of truth for what the GORM models and migrations must produce.
- **REST services and connection behavior:** Timeouts, pooling, logging, and request-time constraints for services that expose REST APIs are in [go_rest_api_standards.md - Database Access](go_rest_api_standards.md#spec-cynai-stands-dbaccess).
