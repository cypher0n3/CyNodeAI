# GORM Table Definition Standard: Execution Plan

## Plan Status

**Created:** 2026-03-20.
**Scope:** Update all orchestrator GORM table definitions to comply with the updated standard: domain base struct + GORM record struct embedding GormModelUUID; record structs only in the database package; shared base structs in go_shared_libs when used by more than one component.

## Goal

Refactor all orchestrator PostgreSQL table models to follow the updated GORM model structure standard:

1. **GormModelUUID** base struct in `go_shared_libs` (ID, CreatedAt, UpdatedAt, DeletedAt)
2. **Domain base structs** in `orchestrator/internal/models` (or `go_shared_libs` if shared with other components)
3. **GORM record structs** in `orchestrator/internal/database` that embed both GormModelUUID and the domain base struct

This separation ensures domain types are reusable outside the database layer and provides a single definition for identity and timestamps across all tables.

## References

- Requirements: [docs/requirements/schema.md](../requirements/schema.md) (REQ-SCHEMA-0100, REQ-SCHEMA-0120).
- Tech specs: [docs/tech_specs/go_sql_database_standards.md](../tech_specs/go_sql_database_standards.md) (CYNAI.STANDS.GoSqlGorm, CYNAI.STANDS.GormModelStructure).
- Schema definitions: [docs/tech_specs/postgres_schema.md](../tech_specs/postgres_schema.md) (Storing This Schema in Code).
- Current implementation: `orchestrator/internal/models/models.go`, `orchestrator/internal/database/` (migrate.go and per-entity files).

## Constraints

- Requirements and tech specs are source of truth; implementation must be brought into compliance.
- Do not change external API or handler signatures that return domain types (e.g. `*models.User`); callers may keep receiving domain types with the record used only inside the database layer.
- Use repo just targets: `just ci`, `just test-go-cover`, `just lint`, `just docs-check`.
- Do not modify Makefiles or Justfiles unless explicitly directed.
- Do not relax linter rules.
- All GORM operations (Create, Find, Updates, AutoMigrate) must use record types from the database package.
- Domain types returned from Store interfaces must be populated from records at read boundaries.

## Execution Plan

Execute tasks in order.
Each task is self-contained with Discovery, Red, Green, Refactor, Testing, and Closeout.
Do not start a later task until the current task's Testing gate and Closeout are complete.

---

### Task 1: Create GormModelUUID Base and Refactor One Table as Template

Add the shared UUID base type in `go_shared_libs` and refactor a single table (e.g. `users` / `User`) to the base+record pattern so the rest of the codebase can follow the same approach.
The database package will define the record struct and use it for GORM; callers continue to receive or work with the domain type where appropriate.

#### Task 1 Requirements and Specifications

- [docs/requirements/schema.md](../requirements/schema.md) (REQ-SCHEMA-0120).
- [docs/tech_specs/go_sql_database_standards.md](../tech_specs/go_sql_database_standards.md) (CYNAI.STANDS.GormModelStructure).
- [docs/tech_specs/postgres_schema.md](../tech_specs/postgres_schema.md) (Storing This Schema in Code, Users Table).

#### Discovery (Task 1) Steps

- [x] Read the requirements and specs listed in Task 1 Requirements and Specifications.
- [x] Inspect `orchestrator/internal/models/models.go` for the current `User` struct (fields, GORM tags, TableName).
- [x] Inspect `orchestrator/internal/database/` for all usages of `models.User` (CreateUser, GetUserByHandle, GetUserByID, etc.) and how the Store interface exposes User.
- [x] Check if `go_shared_libs` already exists and has a models package; identify where GormModelUUID should be defined.
- [x] Confirm whether `User` has soft delete (DeletedAt); if not, GormModelUUID still includes DeletedAt for consistency; User record may not use soft delete in behavior until a later requirement.
- [x] Identify tests that create or query users (database tests, handler tests, integration tests) and what they assert.
- [x] Check `orchestrator/internal/database/migrate.go` to see how User is currently registered with AutoMigrate.

#### Red (Task 1)

- [x] Add or update unit tests in `orchestrator/internal/database` that create and fetch a user and assert on ID, CreatedAt, UpdatedAt, and key domain fields (Handle, Email, IsActive).
- [x] If Store currently returns `*models.User`, decide: either (a) Store returns a record type internally and converts to domain for the interface, or (b) Store interface keeps returning `*models.User` and the database package converts the record to a User when returning.
  Document the decision in the plan or in a short comment in code.
- [x] Run the targeted tests; ensure they pass with the current implementation (green baseline) before refactoring.
- [x] Validation gate: tests must pass before proceeding to Green.

#### Green (Task 1)

- [x] Define **GormModelUUID** in `go_shared_libs` (create package if needed, e.g. `go_shared_libs/models` or `go_shared_libs/gorm`):
  - Fields: `ID uuid.UUID`, `CreatedAt time.Time`, `UpdatedAt time.Time`, `DeletedAt gorm.DeletedAt`
  - Add appropriate GORM tags: `gorm:"type:uuid;primaryKey"` for ID, `gorm:"column:created_at"` for CreatedAt, etc.
  - Add JSON tags: `json:"id"`, `json:"created_at"`, `json:"updated_at"`, `json:"deleted_at,omitempty"`
  - Import required packages: `github.com/google/uuid`, `time`, `gorm.io/gorm`
- [x] Extract a **domain base struct** for User in `orchestrator/internal/models`:
  - Create `UserBase` struct (or keep `User` as domain and introduce `UserRecord` in database)
  - Fields: Handle, Email, IsActive, ExternalSource, ExternalID (no ID, CreatedAt, UpdatedAt, DeletedAt)
  - Keep GORM column tags and JSON tags on domain fields
  - Prefer naming: domain `User` in models, record `UserRecord` in database
- [x] Add **UserRecord** in `orchestrator/internal/database` (e.g. in a new file `user_records.go` or `records.go`):
  - Embeds `GormModelUUID` (from go_shared_libs) and `models.UserBase` (domain base struct)
  - Implement `TableName()` returning `"users"`
  - Ensure GORM tags on embedded fields are preserved or re-applied as needed
- [x] Update `orchestrator/internal/database/migrate.go`:
  - Register `UserRecord` with GORM AutoMigrate instead of `models.User`
  - Ensure existing `users` table schema is unchanged (same columns, same constraints)
- [x] Update database package functions (CreateUser, GetUserByHandle, GetUserByID, etc.):
  - Use `UserRecord` for all GORM operations (Create, Find, Updates)
  - Convert `UserRecord` to `*models.User` when returning from Store interface so callers remain unchanged
  - Ensure conversion preserves all fields including ID, CreatedAt, UpdatedAt
- [x] Update `orchestrator/internal/models/models.go`:
  - Keep `User` struct with all fields (ID, CreatedAt, UpdatedAt, Handle, Email, etc.) for API/handler consumption
  - Remove GORM tags and TableName from User if it's now only a domain type (or keep them if User is still used for some GORM operations during transition)
- [x] Ensure JSON/API serialization still exposes id, created_at, updated_at correctly.
- [x] Run database and handler tests; fix any breakage until green.
- [x] Validation gate: do not proceed until all targeted tests pass.

#### Refactor (Task 1)

- [x] Remove duplication: ensure GormModelUUID is the single definition; ensure TableName and column tags are consistent with postgres_schema.md.
- [x] Verify no GORM operations use `models.User` directly; all should use `UserRecord`.
- [x] Re-run tests; keep green.

#### Testing (Task 1)

- [x] Run `just test-go-cover` for orchestrator (or the database and handler packages).
- [x] Run `just lint-go` (or `just ci`) for the changed files.
- [x] Verify AutoMigrate still creates the `users` table with the same schema as before.
- [x] Validation gate: do not start Task 2 until all checks pass.

#### Closeout (Task 1)

- [x] Generate a **task completion report** for Task 1:
  - What was done (GormModelUUID added in go_shared_libs, User refactored to UserRecord + domain User, conversion at Store boundary).
  - What passed (tests, lint, AutoMigrate validation).
  - Any deviations or notes (e.g. decision on Store interface return type).
- [x] Mark every completed step in this task with `- [x]`.

---

### Task 2: Refactor Identity and Authentication Tables

Apply the same pattern to identity and authentication tables: `PasswordCredential`, `RefreshSession`, `AuthAuditLog`.
These tables are closely related to User and should follow the same pattern.

#### Task 2 Requirements and Specifications

- [docs/requirements/schema.md](../requirements/schema.md) (REQ-SCHEMA-0120).
- [docs/tech_specs/go_sql_database_standards.md](../tech_specs/go_sql_database_standards.md) (CYNAI.STANDS.GormModelStructure).
- [docs/tech_specs/postgres_schema.md](../tech_specs/postgres_schema.md) (Identity and Authentication section).

#### Discovery (Task 2) Steps

- [x] Inspect `orchestrator/internal/models/models.go` for `PasswordCredential`, `RefreshSession`, `AuthAuditLog` structs.
- [x] Identify all call sites in `orchestrator/internal/database` that use these types.
- [x] Check if these tables have soft delete requirements (likely not for audit logs).
- [x] Identify tests that create or query these entities.
- [x] Check `orchestrator/internal/database/migrate.go` to see how these are registered.

#### Red (Task 2)

- [x] Ensure existing tests cover PasswordCredential, RefreshSession, and AuthAuditLog operations.
- [x] Run tests and confirm green before changing these tables.
- [x] Validation gate: baseline green for these tables.

#### Green (Task 2)

- [x] For each table (PasswordCredential, RefreshSession, AuthAuditLog):
  - Extract domain base struct in `orchestrator/internal/models` (fields without ID, CreatedAt, UpdatedAt, DeletedAt)
  - Add record struct in `orchestrator/internal/database` (e.g. `PasswordCredentialRecord`, `RefreshSessionRecord`, `AuthAuditLogRecord`) that embeds GormModelUUID and the domain base
  - Implement `TableName()` for each record
  - Update `migrate.go` to register record types instead of domain types
  - Update all database package functions to use records and convert to domain types at Store boundaries
- [x] After each table: run tests and fix until green.
- [x] Validation gate: do not proceed to the next batch until these tables' tests pass.

#### Refactor (Task 2)

- [x] Consolidate record types in a single file or by domain (e.g. `user_records.go` for User, PasswordCredential, RefreshSession, `audit_records.go` for AuthAuditLog) as preferred.
- [x] Ensure no GORM model structs remain in `internal/models` that are used as the target of GORM Create/Find/Updates for these tables.
- [x] Re-run full orchestrator tests.

#### Testing (Task 2)

- [x] Run `just test-go-cover` for orchestrator.
- [x] Run `just lint-go` and `just ci`.
- [x] Validation gate: do not start Task 3 until all checks pass.

#### Closeout (Task 2)

- [x] Generate a **task completion report** for Task 2: list of tables refactored (PasswordCredential, RefreshSession, AuthAuditLog); any tables left with a different pattern and why; what passed.
- [x] Mark every completed step in this task with `- [x]`.

---

### Task 3: Refactor Tasks, Jobs, and Nodes Tables

Apply the pattern to task and job execution tables: `Task`, `Job`, `Node`, `NodeCapability`, `TaskDependency`.
These are core execution entities and may have more complex relationships.

#### Task 3 Requirements and Specifications

- [docs/requirements/schema.md](../requirements/schema.md) (REQ-SCHEMA-0120).
- [docs/tech_specs/go_sql_database_standards.md](../tech_specs/go_sql_database_standards.md) (CYNAI.STANDS.GormModelStructure).
- [docs/tech_specs/postgres_schema.md](../tech_specs/postgres_schema.md) (Tasks, Jobs, and Nodes section).

#### Discovery (Task 3) Steps

- [x] Inspect `orchestrator/internal/models/models.go` for `Task`, `Job`, `Node`, `NodeCapability`, `TaskDependency` structs.
- [x] Identify all call sites in `orchestrator/internal/database` that use these types (tasks.go, nodes.go, etc.).
- [x] Check for foreign key relationships and ensure record types preserve them.
- [x] Identify tests that create or query these entities.
- [x] Check `orchestrator/internal/database/migrate.go` to see how these are registered.

#### Red (Task 3)

- [x] Ensure existing tests cover Task, Job, Node, NodeCapability, and TaskDependency operations.
- [x] Run tests and confirm green before changing these tables.
- [x] Validation gate: baseline green for these tables.

#### Green (Task 3)

- [x] For each table (Task, Job, Node, NodeCapability, TaskDependency):
  - Extract domain base struct in `orchestrator/internal/models` (fields without ID, CreatedAt, UpdatedAt, DeletedAt)
  - Add record struct in `orchestrator/internal/database` (e.g. `TaskRecord`, `JobRecord`, `NodeRecord`, etc.) that embeds GormModelUUID and the domain base
  - Implement `TableName()` for each record
  - Update `migrate.go` to register record types instead of domain types
  - Update all database package functions to use records and convert to domain types at Store boundaries
  - Ensure foreign key relationships are preserved (e.g. Task.ProjectID, Job.TaskID, TaskDependency.task_id and depends_on_task_id)
- [x] After each table: run tests and fix until green.
- [x] Validation gate: do not proceed to the next batch until these tables' tests pass.

#### Refactor (Task 3)

- [x] Consolidate record types by domain (e.g. `task_records.go` for Task, Job, TaskDependency; `node_records.go` for Node, NodeCapability).
- [x] Ensure no GORM model structs remain in `internal/models` that are used as the target of GORM Create/Find/Updates for these tables.
- [x] Re-run full orchestrator tests.

#### Testing (Task 3)

- [x] Run `just test-go-cover` for orchestrator.
- [x] Run `just lint-go` and `just ci`.
- [x] Validation gate: do not start Task 4 until all checks pass.

#### Closeout (Task 3)

- [x] Generate a **task completion report** for Task 3: list of tables refactored; any tables left with a different pattern and why; what passed.
- [x] Mark every completed step in this task with `- [x]`.

---

### Task 4: Refactor Remaining Orchestrator Tables

Apply the pattern to all remaining GORM models in `orchestrator/internal/models` that map to PostgreSQL tables.
Process tables in logical groups to limit blast radius and keep tests green after each group.

#### Task 4 Requirements and Specifications

- [docs/requirements/schema.md](../requirements/schema.md) (REQ-SCHEMA-0120).
- [docs/tech_specs/go_sql_database_standards.md](../tech_specs/go_sql_database_standards.md) (CYNAI.STANDS.GormModelStructure).
- [docs/tech_specs/postgres_schema.md](../tech_specs/postgres_schema.md) (all remaining table definitions).

#### Discovery (Task 4) Steps

- [x] List all remaining structs in `orchestrator/internal/models/models.go` that have GORM tags and correspond to PostgreSQL tables:
  - Preferences: `PreferenceEntry`, `PreferenceAuditLog`
  - Projects: `Project`, `ProjectPlan`
  - Chat: `Session`, `ChatThread`, `ChatMessage`, `ChatAuditLog`
  - Workflow: `WorkflowCheckpoint`, `TaskWorkflowLease`
  - Sandbox: `SandboxImage`, `SandboxImageVersion`, `NodeSandboxImageAvailability`
  - Artifacts: `TaskArtifact`
  - Skills: `Skill`
  - Access Control: `AccessControlRule`, `AccessControlAuditLog`
  - API Credentials: `ApiCredential`
- [x] For each, identify: (a) which have UUID primary key and timestamps (candidates for GormModelUUID); (b) which have different PK/timestamp patterns; (c) all call sites in `internal/database` that use these types.
- [x] Group tables into batches (e.g. batch 1: PreferenceEntry, PreferenceAuditLog; batch 2: Project, ProjectPlan; batch 3: Chat tables; batch 4: Workflow tables; batch 5: Sandbox tables; batch 6: remaining) so each batch can be implemented and tested independently.

#### Red (Task 4)

- [x] For each batch, ensure existing tests cover the entities in that batch; run tests and confirm green before changing the batch.
- [x] Validation gate: baseline green for each batch.

#### Green (Task 4)

- [x] For each batch:
  - Extract domain base structs in `internal/models` (fields without ID, CreatedAt, UpdatedAt, DeletedAt)
  - Add record structs in `internal/database` (embedding GormModelUUID and domain/base fields)
  - Update `migrate.go` to register record types instead of domain types
  - Update all database package functions to use records and convert to domain types at Store boundaries
- [x] After each batch: run tests and fix until green.
- [x] Validation gate: do not proceed to the next batch until the current batch's tests pass.

#### Refactor (Task 4)

- [x] Consolidate record types in a single file or by domain (e.g. `preference_records.go`, `project_records.go`, `chat_records.go`, etc.) as preferred.
- [x] Ensure no GORM model structs remain in `internal/models` that are used as the target of GORM Create/Find/Updates; only record types in database package.
- [x] Re-run full orchestrator tests.

#### Testing (Task 4)

- [x] Run `just test-go-cover` for orchestrator.
- [x] Run `just lint-go` and `just ci`.
- [x] Verify AutoMigrate still creates all tables with the same schema as before.
- [x] Validation gate: do not start Task 5 until all checks pass.

#### Closeout (Task 4)

- [x] Generate a **task completion report** for Task 4: list of all tables refactored; any tables left with a different pattern (e.g. no soft delete) and why; what passed.
- [x] Mark every completed step in this task with `- [x]`.

---

### Task 5: Verify `go_shared_libs` Placement and New Tables

Ensure any domain type that is or becomes shared with worker_node or another module is moved to go_shared_libs.
Ensure any new table added follows the pattern from the start.

#### Task 5 Requirements and Specifications

- [docs/requirements/schema.md](../requirements/schema.md) (REQ-SCHEMA-0120).
- [docs/tech_specs/go_sql_database_standards.md](../tech_specs/go_sql_database_standards.md) (Placement rules).

#### Discovery (Task 5) Steps

- [x] Check whether any orchestrator domain type is imported by worker_node or cynork; if so, that type MUST live in go_shared_libs per REQ-SCHEMA-0120.
- [x] Review all domain base structs in `orchestrator/internal/models` and identify which ones might be shared in the future.
- [x] Check if any new tables are planned (e.g. MCP tool definitions) and ensure they follow the pattern from the start.

#### Red (Task 5)

- [x] If any domain types need to move to go_shared_libs: add tests that verify the types can be imported from go_shared_libs by orchestrator and other modules.
- [x] Validation gate: failing tests for new behavior (if any).

#### Green (Task 5)

- [x] Move any domain base struct that is shared with another module (or will be) to go_shared_libs:
  - Create appropriate package structure in go_shared_libs
  - Move the domain struct
  - Update orchestrator to import from go_shared_libs
  - Ensure no circular dependency
  - Update record structs in orchestrator/internal/database to reference the moved type
- [x] For any new table: define domain base struct in the appropriate package (models or go_shared_libs); define record struct in internal/database embedding GormModelUUID and the base; register in migrate.go; implement Store methods using the record and returning domain type as needed.
- [x] Run tests until green.

#### Refactor (Task 5)

- [x] Ensure `migrate.go` (or the central place that runs AutoMigrate) only registers record types from the database package.
- [x] Confirm no `models.*` types are used for migration.
- [x] Re-run tests.

#### Testing (Task 5)

- [x] Run `just test-go-cover` and `just ci`.
- [x] Validation gate: all pass.

#### Closeout (Task 5)

- [x] Generate a **task completion report** for Task 5: which types moved to go_shared_libs (if any); which new tables added (if any); what passed.
- [x] Mark every completed step in this task with `- [x]`.

---

### Task 6: Documentation and Plan Closeout

Update documentation and verify compliance with the standard.

#### Task 6 Requirements and Specifications

- [docs/requirements/schema.md](../requirements/schema.md) (REQ-SCHEMA-0120).
- [docs/tech_specs/go_sql_database_standards.md](../tech_specs/go_sql_database_standards.md) (CYNAI.STANDS.GormModelStructure).

#### Discovery (Task 6) Steps

- [x] Check if orchestrator README or internal docs document the layout (models vs database package).
- [x] Verify all tables have been refactored by checking migrate.go only registers record types.

#### Red (Task 6)

- [x] N/A (documentation task).

#### Green (Task 6)

- [x] Update orchestrator README or internal docs if the layout (models vs database package) is documented there.
- [x] Add comments in code explaining the pattern (e.g. in migrate.go, in record struct files).
- [x] Verify REQ-SCHEMA-0120 and CYNAI.STANDS.GormModelStructure are satisfied:
  - Record structs only in database package
  - Domain base structs in models or go_shared_libs
  - GormModelUUID used consistently for UUID-keyed tables
  - All GORM operations use record types
  - Store interfaces return domain types (converted from records)

#### Refactor (Task 6)

- [x] Ensure documentation is clear and consistent.
- [x] Re-run tests to ensure nothing broke.

#### Testing (Task 6)

- [x] Run `just test-go-cover` and `just ci`.
- [x] Run `just docs-check` to verify documentation.
- [x] Validation gate: all pass.

#### Closeout (Task 6)

- [x] Generate a **final plan completion report**:
  - Which tasks were completed
  - Overall validation status
  - List of all tables refactored
  - Any remaining risks or follow-up (e.g. worker_node SQLite models to align in a future plan)
  - Confirmation that REQ-SCHEMA-0120 and CYNAI.STANDS.GormModelStructure are satisfied
- [x] Mark all completed steps in the plan with `- [x]`.
