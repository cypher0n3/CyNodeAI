# GORM Base Struct + Record Standard: Execution Plan

- [Plan Status](#plan-status)
- [Goal](#goal)
- [References](#references)
- [Constraints](#constraints)
- [Execution Plan](#execution-plan)

## Plan Status

**Created:** 2026-03-19.
**Scope:** Bring orchestrator (and future worker_node) GORM usage into alignment with REQ-SCHEMA-0120 and CYNAI.STANDS.GormModelStructure: domain base struct + GORM record struct embedding GormModelUUID; record structs only in the database package; shared base structs in go_shared_libs when used by more than one component.

## Goal

- Implement the standard GORM model structure across the codebase: (1) a shared **GormModelUUID** base (ID, CreatedAt, UpdatedAt, DeletedAt); (2) **domain base structs** for each logical entity; (3) **GORM record structs** that embed GormModelUUID and the domain struct, living only in the database package.
- Refactor existing orchestrator models so that persistence uses record types in `internal/database` and domain types remain (or are extracted) in `internal/models`; new tables (e.g. MCP tool definitions) follow the pattern from the start.
- Place domain base structs in `go_shared_libs` only when they are used by more than the orchestrator (e.g. worker_node); orchestrator-only bases stay in `orchestrator/internal/models`.

## References

- Requirements: [docs/requirements/schema.md](../../requirements/schema.md) (REQ-SCHEMA-0100, REQ-SCHEMA-0120).
- Tech specs: [docs/tech_specs/go_sql_database_standards.md](../../tech_specs/go_sql_database_standards.md) (CYNAI.STANDS.GoSqlGorm, CYNAI.STANDS.GormModelStructure), [docs/tech_specs/postgres_schema.md](../../tech_specs/postgres_schema.md) (Storing This Schema in Code).
- Draft spec (example pattern): [`docs/tech_specs/mcp/mcp_tool_definitions.md`](../../tech_specs/mcp/mcp_tool_definitions.md) (GORM wrappers, GormModelUUID, MCPToolDefinitionRecord).
- Implementation: `orchestrator/internal/models/models.go`, `orchestrator/internal/database/` (database.go, migrate.go, and per-entity files).

## Constraints

- Requirements and tech specs are source of truth; implementation is brought into compliance.
- Do not change external API or handler signatures that return domain types (e.g. `*models.User`); callers may keep receiving domain types with the record used only inside the database layer, or the database package may return the embedded domain view (e.g. convert record to domain on read) as decided per task.
- Use repo just targets: `just ci`, `just test-go-cover`, `just lint`, `just docs-check`.
- Do not modify Makefiles or Justfiles unless explicitly directed.
- Do not relax linter rules.

## Execution Plan

Execute tasks in order.
Each task is self-contained with Discovery, Red, Green, Refactor, Testing, and Closeout.
Do not start a later task until the current task's Testing gate and Closeout are complete.

---

### Task 1: Introduce GormModelUUID and Refactor One Table as Template

Add the shared UUID base type and refactor a single table (e.g. `users` / `User`) to the base+record pattern so the rest of the codebase can follow the same approach.
The database package will define the record struct and use it for GORM; callers continue to receive or work with the domain type where appropriate (Store interface may still expose `*models.User` with conversion from the record inside the database layer).

#### Task 1 Requirements and Specifications

- [docs/requirements/schema.md](../../requirements/schema.md) (REQ-SCHEMA-0120).
- [docs/tech_specs/go_sql_database_standards.md](../../tech_specs/go_sql_database_standards.md) (CYNAI.STANDS.GormModelStructure).
- [docs/tech_specs/postgres_schema.md](../../tech_specs/postgres_schema.md) (Storing This Schema in Code).
- [`docs/tech_specs/mcp/mcp_tool_definitions.md`](../../tech_specs/mcp/mcp_tool_definitions.md) (GormModelUUID, record embedding pattern).

#### Discovery (Task 1) Steps

- [ ] Read the requirements and specs listed in Task 1 Requirements and Specifications.
- [ ] Inspect `orchestrator/internal/models/models.go` for the current `User` struct (fields, GORM tags, TableName).
- [ ] Inspect `orchestrator/internal/database/` for all usages of `models.User` (CreateUser, GetUserByHandle, GetUserByID, etc.) and how the Store interface exposes User.
- [ ] Confirm whether `User` has soft delete (DeletedAt); if not, GormModelUUID still includes DeletedAt for consistency; User record may not use soft delete in behavior until a later requirement.
- [ ] Identify tests that create or query users (database tests, handler tests, integration tests) and what they assert.

#### Red (Task 1)

- [ ] Add or update unit tests in `orchestrator/internal/database` that create and fetch a user and assert on ID, CreatedAt, UpdatedAt, and key domain fields.
- [ ] If Store currently returns `*models.User`, decide: either (a) Store returns a record type internally and converts to domain for the interface, or (b) Store interface keeps returning `*models.User` and the database package converts the record to a User when returning.
  Document the decision in the plan or in a short comment in code.
- [ ] Run the targeted tests; ensure they pass with the current implementation (green baseline) before refactoring.
- [ ] Validation gate: tests must pass before proceeding to Green.

#### Green (Task 1)

- [ ] Define **GormModelUUID** in `orchestrator/internal/models` (or in a new file there): ID (uuid), CreatedAt, UpdatedAt, DeletedAt with GORM and JSON tags per [go_sql_database_standards](../../tech_specs/go_sql_database_standards.md#spec-cynai-stands-gormmodelstructure).
- [ ] Extract a **domain base struct** for User (e.g. `User` with only Handle, Email, IsActive, ExternalSource, ExternalID; no ID/CreatedAt/UpdatedAt).
  Name the domain struct so it is clear (e.g. keep `User` as the domain type and introduce `UserRecord` in the database package that embeds GormModelUUID and User, or name the domain `UserBase` and keep `User` as the record in models - per spec the **record** must live in the database package, so: domain `User` in models, record `UserRecord` in database embedding GormModelUUID + User).
  Prefer: domain `User` in models, record `UserRecord` in database.
- [ ] Add **UserRecord** in `orchestrator/internal/database` (e.g. in a new file `records.go` or `user_records.go`): embeds GormModelUUID and models.User (or the chosen domain type); implement TableName() returning `"users"`.
- [ ] Register UserRecord with GORM AutoMigrate (in migrate.go) instead of models.User; ensure existing `users` table schema is unchanged (same columns).
- [ ] Update database package CreateUser, GetUserByHandle, GetUserByID to use UserRecord for GORM operations; convert to *models.User when returning from Store interface so callers remain unchanged.
- [ ] Ensure JSON/API serialization still exposes id, created_at, updated_at.
  If the Store returns `*models.User` to handlers, models.User MUST have ID, CreatedAt, UpdatedAt for API responses.
  Simplest: keep models.User as the type returned from Store with all fields; define UserRecord in database as embedding GormModelUUID and a struct with only the non-PK, non-timestamp columns (Handle, Email, IsActive, ExternalSource, ExternalID).
  Database package converts UserRecord to models.User at read boundaries (record.ID, record.CreatedAt, record.UpdatedAt, record.Handle, etc.).
- [ ] Run database and handler tests; fix any breakage until green.
- [ ] Validation gate: do not proceed until all targeted tests pass.

#### Refactor (Task 1)

- [ ] Remove duplication: ensure GormModelUUID is the single definition; ensure TableName and column tags are consistent with postgres_schema.md.
- [ ] Re-run tests; keep green.

#### Testing (Task 1)

- [ ] Run `just test-go-cover` for orchestrator (or the database and handler packages).
- [ ] Run `just lint-go` (or `just ci`) for the changed files.
- [ ] Validation gate: do not start Task 2 until all checks pass.

#### Closeout (Task 1)

- [ ] Generate a **task completion report** for Task 1: what was done (GormModelUUID added, User refactored to UserRecord + domain User, conversion at Store boundary); what passed (tests, lint); any deviations or notes.
- [ ] Mark every completed step in this task with `- [x]`.

---

### Task 2: Refactor Remaining Orchestrator Tables to Base + Record Pattern

Apply the same pattern to all other GORM models in `orchestrator/internal/models` that map to PostgreSQL tables: introduce a record type in `internal/database` that embeds GormModelUUID (or equivalent for tables that do not use soft delete) and the domain/base fields; use the record for all GORM operations; convert to domain types at Store boundaries where the interface exposes domain types.
Process tables in logical groups (e.g. identity/auth, then tasks/jobs, then preferences, etc.) to limit blast radius and keep tests green after each group.

#### Task 2 Requirements and Specifications

- [docs/requirements/schema.md](../../requirements/schema.md) (REQ-SCHEMA-0120).
- [docs/tech_specs/go_sql_database_standards.md](../../tech_specs/go_sql_database_standards.md) (CYNAI.STANDS.GormModelStructure).
- [docs/tech_specs/postgres_schema.md](../../tech_specs/postgres_schema.md) (table definitions).

#### Discovery (Task 2) Steps

- [ ] List all structs in `orchestrator/internal/models/models.go` that have GORM tags and correspond to PostgreSQL tables (User, PasswordCredential, RefreshSession, AuthAuditLog, Task, Job, Node, etc.).
- [ ] For each, identify: (a) which have UUID primary key and timestamps (candidates for GormModelUUID); (b) which have different PK/timestamp patterns (e.g. audit logs may have different shape); (c) all call sites in `internal/database` that use these types.
- [ ] Group tables into batches (e.g. batch 1: PasswordCredential, RefreshSession, AuthAuditLog; batch 2: Task, Job; batch 3: Node; batch 4: preferences, system settings; etc.) so each batch can be implemented and tested independently.

#### Red (Task 2)

- [ ] For each batch, ensure existing tests cover the entities in that batch; run tests and confirm green before changing the batch.
- [ ] Validation gate: baseline green for the batch.

#### Green (Task 2)

- [ ] For each batch: add record structs in `internal/database` (embedding GormModelUUID and domain/base fields); update migrate.go to register record types; update all database package functions to use records and convert to domain types at Store boundaries.
- [ ] Update models to keep domain structs with only the fields needed for API/domain (and populate from record when returning).
- [ ] After each batch: run tests and fix until green.
- [ ] Validation gate: do not proceed to the next batch until the current batch's tests pass.

#### Refactor (Task 2)

- [ ] Consolidate record types in a single file or by domain (e.g. `user_records.go`, `task_records.go`) as preferred.
- [ ] Ensure no GORM model structs remain in `internal/models` that are used as the target of GORM Create/Find/Updates; only record types in database package.
- [ ] Re-run full orchestrator tests.

#### Testing (Task 2)

- [ ] Run `just test-go-cover` for orchestrator.
- [ ] Run `just lint-go` and `just ci`.
- [ ] Validation gate: do not start Task 3 until all checks pass.

#### Closeout (Task 2)

- [ ] Generate a **task completion report** for Task 2: list of tables refactored; any tables left with a different pattern (e.g. no soft delete) and why; what passed.
- [ ] Mark every completed step in this task with `- [x]`.

---

### Task 3: New Tables and `go_shared_libs`

Ensure any new table added (e.g. MCP tool definitions) follows the pattern from the start: domain base in models or go_shared_libs, record only in database package.
If any domain type is or becomes shared with worker_node or another module, move its base struct to go_shared_libs and reference it from the orchestrator database package.

#### Task 3 Requirements and Specifications

- [docs/requirements/schema.md](../../requirements/schema.md) (REQ-SCHEMA-0120).
- [docs/tech_specs/go_sql_database_standards.md](../../tech_specs/go_sql_database_standards.md) (Placement rules).
- [`docs/tech_specs/mcp/mcp_tool_definitions.md`](../../tech_specs/mcp/mcp_tool_definitions.md) (MCPTool, MCPToolDefinitionRecord).

#### Discovery (Task 3) Steps

- [ ] Check whether any orchestrator domain type is imported by worker_node or cynork; if so, that type MUST live in go_shared_libs per REQ-SCHEMA-0120.
- [ ] If MCP tool definitions table is to be added in this workstream: confirm the draft MCPTool type (base) and MCPToolDefinitionRecord (record); decide if MCPTool lives in orchestrator/internal/models or go_shared_libs (only if worker_node needs to reference the same struct).

#### Red (Task 3)

- [ ] If adding mcp_tool_definitions: add BDD or unit tests that expect the table to exist and that tool definition records can be created and loaded with the base+record pattern; run and see them fail until the table and record are implemented.
- [ ] Validation gate: failing tests for new behavior (if any).

#### Green (Task 3)

- [ ] For any new table: define domain base struct in the appropriate package (models or go_shared_libs); define record struct in internal/database embedding GormModelUUID and the base; register in migrate.go; implement Store methods using the record and returning domain type as needed.
- [ ] Move any domain base struct that is shared with another module to go_shared_libs; update orchestrator to import from go_shared_libs; ensure no circular dependency.
- [ ] Run tests until green.

#### Refactor (Task 3)

- [ ] Ensure migrate.go (or the central place that runs AutoMigrate) only registers record types from the database package.
- [ ] Confirm no `models.*` types are used for migration.
- [ ] Re-run tests.

#### Testing (Task 3)

- [ ] Run `just test-go-cover` and `just ci`.
- [ ] Validation gate: all pass.

#### Closeout (Task 3)

- [ ] Generate a **task completion report** for Task 3: which new tables added (if any); which types moved to go_shared_libs (if any); what passed.
- [ ] Mark every completed step in this task with `- [x]`.

---

### Task 4: Documentation and Plan Closeout

- [ ] Update orchestrator README or internal docs if the layout (models vs database package) is documented there.
- [ ] Verify REQ-SCHEMA-0120 and CYNAI.STANDS.GormModelStructure are satisfied: record structs only in database package; domain base structs in models or go_shared_libs; GormModelUUID used consistently for UUID-keyed tables.
- [ ] Generate a **final plan completion report**: which tasks were completed; overall validation status; any remaining risks or follow-up (e.g. worker_node SQLite models to align in a future plan).
- [ ] Mark all completed steps in the plan with `- [x]`.
