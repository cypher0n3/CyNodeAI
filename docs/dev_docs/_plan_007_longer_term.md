---
name: Longer-Term Maintenance and Debt
overview: |
  Address 6 longer-term maintenance and technical debt items from review
  reports 1-6.
  Tasks cover database migration infrastructure (versioned migrations),
  PMA startup alignment with the worker-instruction model, continuous PMA
  monitoring, centralized worker node config, replacing global mutable test
  hooks with dependency injection, and adding load/performance testing.
  Each task follows BDD/TDD with per-task validation gates.
todos:
  - id: lt-001
    content: "Read `orchestrator/internal/store/migrate.go` lines 17-22 (no migration transaction) and 28-69 (AutoMigrate every startup, no version tracking)."
    status: pending
  - id: lt-002
    content: "Read `worker_node/internal/securestore/store.go` lines 42-51 (AutoMigrate) and the SchemaVersion table placeholder."
    status: pending
    dependencies:
      - lt-001
  - id: lt-003
    content: "Evaluate migration libraries: `golang-migrate/migrate`, `pressly/goose`, or `atlas`; select one based on Go module compatibility, GORM integration, and CI support."
    status: pending
    dependencies:
      - lt-002
  - id: lt-004
    content: "Add a unit test: startup must apply only pending migrations (not re-run all); running on an up-to-date schema must be a no-op."
    status: pending
    dependencies:
      - lt-003
  - id: lt-005
    content: "Add a unit test: each migration must run in a transaction; a failing migration must roll back without leaving the schema in a partial state."
    status: pending
    dependencies:
      - lt-004
  - id: lt-006
    content: "Run `go test -v -run 'TestMigrationIdempotent|TestMigrationRollback' ./orchestrator/internal/store/...` and confirm failures."
    status: pending
    dependencies:
      - lt-005
  - id: lt-007
    content: "Create initial migration files from the current GORM AutoMigrate schema: one baseline migration per module (orchestrator, worker node)."
    status: pending
    dependencies:
      - lt-006
  - id: lt-008
    content: "Implement migration runner: replace AutoMigrate with the selected library; track version in a `schema_migrations` table."
    status: pending
    dependencies:
      - lt-007
  - id: lt-009
    content: "Wire the migration runner into startup for orchestrator (control-plane, user-gateway, mcp-gateway, api-egress) and worker node (node-manager)."
    status: pending
    dependencies:
      - lt-008
  - id: lt-010
    content: "Re-run `go test -v -run 'TestMigrationIdempotent|TestMigrationRollback' ./orchestrator/internal/store/...` and confirm green."
    status: pending
    dependencies:
      - lt-009
  - id: lt-011
    content: "Add a `just migrate-new <name>` recipe to the justfile for creating new migration files."
    status: pending
    dependencies:
      - lt-010
  - id: lt-012
    content: "Run `just lint-go` on all changed files and `go test -cover ./orchestrator/...` and `go test -cover ./worker_node/...`; confirm 90% threshold."
    status: pending
    dependencies:
      - lt-011
  - id: lt-013
    content: "Run `just e2e --tags no_inference` to verify startup and schema migration regression."
    status: pending
    dependencies:
      - lt-012
  - id: lt-014
    content: "Validation gate -- do not proceed to Task 2 until all checks pass."
    status: pending
    dependencies:
      - lt-013
  - id: lt-015
    content: "Generate task completion report for Task 1. Mark completed steps `- [x]`."
    status: pending
    dependencies:
      - lt-014
  - id: lt-016
    content: "Do not start Task 2 until Task 1 closeout is done."
    status: pending
    dependencies:
      - lt-015
  - id: lt-017
    content: "Read REQ-ORCHES-0150 in `docs/requirements/orches.md` for PMA startup via the worker-instruction model."
    status: pending
    dependencies:
      - lt-016
  - id: lt-018
    content: "Read `orchestrator/cmd/control-plane/main.go` and `orchestrator/internal/pmasubprocess/` (or equivalent) for the current PMA startup path."
    status: pending
    dependencies:
      - lt-017
  - id: lt-019
    content: "Read `docs/tech_specs/orchestrator_bootstrap.md` for the worker-instruction PMA startup spec."
    status: pending
    dependencies:
      - lt-018
  - id: lt-020
    content: "Add unit tests: PMA must be started via the worker node's managed-service instruction path (not a direct subprocess from the orchestrator)."
    status: pending
    dependencies:
      - lt-019
  - id: lt-021
    content: "Run `go test -v -run TestPmaWorkerInstruction ./orchestrator/...` and confirm failure (current path is direct subprocess)."
    status: pending
    dependencies:
      - lt-020
  - id: lt-022
    content: "Refactor PMA startup: remove direct subprocess launch from the orchestrator; instead, add PMA to the desired managed-service state and let the worker node reconcile it."
    status: pending
    dependencies:
      - lt-021
  - id: lt-023
    content: "Re-run `go test -v -run TestPmaWorkerInstruction ./orchestrator/...` and confirm green."
    status: pending
    dependencies:
      - lt-022
  - id: lt-024
    content: "Run `just lint-go` on changed files and `go test -cover ./orchestrator/...`; confirm 90% threshold."
    status: pending
    dependencies:
      - lt-023
  - id: lt-025
    content: "Run `just e2e --tags pma_inference` (requires inference; skip if unavailable) to verify PMA startup via worker instruction."
    status: pending
    dependencies:
      - lt-024
  - id: lt-026
    content: "Validation gate -- do not proceed to Task 3 until all checks pass."
    status: pending
    dependencies:
      - lt-025
  - id: lt-027
    content: "Generate task completion report for Task 2. Mark completed steps `- [x]`."
    status: pending
    dependencies:
      - lt-026
  - id: lt-028
    content: "Do not start Task 3 until Task 2 closeout is done."
    status: pending
    dependencies:
      - lt-027
  - id: lt-029
    content: "Read REQ-ORCHES-0129 in `docs/requirements/orches.md` for continuous PMA monitoring requirements."
    status: pending
    dependencies:
      - lt-028
  - id: lt-030
    content: "Read `orchestrator/internal/handlers/` for the current `readyzHandler` (health check only, no continuous loop)."
    status: pending
    dependencies:
      - lt-029
  - id: lt-031
    content: "Add unit tests: orchestrator must continuously monitor PMA health at a configurable interval (default 30s); detect PMA failure and trigger restart or re-provision."
    status: pending
    dependencies:
      - lt-030
  - id: lt-032
    content: "Run `go test -v -run TestPmaMonitoring ./orchestrator/...` and confirm failure."
    status: pending
    dependencies:
      - lt-031
  - id: lt-033
    content: "Implement continuous PMA monitoring: background goroutine that polls PMA health endpoint at configurable interval; on failure, update desired state to trigger restart."
    status: pending
    dependencies:
      - lt-032
  - id: lt-034
    content: "Add configurable monitoring interval (env var or config field) with a sensible default (e.g., 30s)."
    status: pending
    dependencies:
      - lt-033
  - id: lt-035
    content: "Re-run `go test -v -run TestPmaMonitoring ./orchestrator/...` and confirm green."
    status: pending
    dependencies:
      - lt-034
  - id: lt-036
    content: "Run `just lint-go` on changed files and `go test -cover ./orchestrator/...`; confirm 90% threshold."
    status: pending
    dependencies:
      - lt-035
  - id: lt-037
    content: "Run `just e2e --tags pma_inference` (requires inference; skip if unavailable) to verify PMA monitoring and restart."
    status: pending
    dependencies:
      - lt-036
  - id: lt-038
    content: "Validation gate -- do not proceed to Task 4 until all checks pass."
    status: pending
    dependencies:
      - lt-037
  - id: lt-039
    content: "Generate task completion report for Task 3. Mark completed steps `- [x]`."
    status: pending
    dependencies:
      - lt-038
  - id: lt-040
    content: "Do not start Task 4 until Task 3 closeout is done."
    status: pending
    dependencies:
      - lt-039
  - id: lt-041
    content: "Read `worker_node/cmd/node-manager/main.go` and `worker_node/internal/` to catalog all configuration sources: env vars, flags, hardcoded defaults, and config files."
    status: pending
    dependencies:
      - lt-040
  - id: lt-042
    content: "Identify inconsistencies: env vars read in multiple places, defaults that differ between call sites, missing validation."
    status: pending
    dependencies:
      - lt-041
  - id: lt-043
    content: "Design a `worker_node/internal/config/config.go` package: single validated `Config` struct loaded once at startup, with env var binding and default values."
    status: pending
    dependencies:
      - lt-042
  - id: lt-044
    content: "Add unit tests: `LoadConfig()` must populate all fields from env vars; missing required fields must return a clear error; defaults must be applied for optional fields."
    status: pending
    dependencies:
      - lt-043
  - id: lt-045
    content: "Run `go test -v -run TestLoadConfig ./worker_node/internal/config/...` and confirm failures (package does not exist)."
    status: pending
    dependencies:
      - lt-044
  - id: lt-046
    content: "Implement `config.go`: define `Config` struct, `LoadConfig() (*Config, error)` function, and validation."
    status: pending
    dependencies:
      - lt-045
  - id: lt-047
    content: "Wire `LoadConfig()` into `worker_node/cmd/node-manager/main.go` startup; replace scattered `os.Getenv` calls with config field access."
    status: pending
    dependencies:
      - lt-046
  - id: lt-048
    content: "Re-run `go test -v -run TestLoadConfig ./worker_node/internal/config/...` and confirm green."
    status: pending
    dependencies:
      - lt-047
  - id: lt-049
    content: "Remove all orphaned `os.Getenv` calls in `worker_node/` that are now served by the config package."
    status: pending
    dependencies:
      - lt-048
  - id: lt-050
    content: "Run `just lint-go` on changed files and `go test -cover ./worker_node/...`; confirm 90% threshold."
    status: pending
    dependencies:
      - lt-049
  - id: lt-051
    content: "Run `just e2e --tags worker,no_inference` to verify worker node startup and config regression."
    status: pending
    dependencies:
      - lt-050
  - id: lt-052
    content: "Validation gate -- do not proceed to Task 5 until all checks pass."
    status: pending
    dependencies:
      - lt-051
  - id: lt-053
    content: "Generate task completion report for Task 4. Mark completed steps `- [x]`."
    status: pending
    dependencies:
      - lt-052
  - id: lt-054
    content: "Do not start Task 5 until Task 4 closeout is done."
    status: pending
    dependencies:
      - lt-053
  - id: lt-055
    content: "Search all Go modules for package-level `var` used as test hooks: `orchestrator/`, `worker_node/`, `agents/`, `cynork/`."
    status: pending
    dependencies:
      - lt-054
  - id: lt-056
    content: "Catalog each test hook: variable name, file, line, what it replaces (HTTP client, time function, exec function, etc.), and which tests use it."
    status: pending
    dependencies:
      - lt-055
  - id: lt-057
    content: "For each module, design the replacement: inject the dependency via constructor parameter or struct field instead of a package-level var."
    status: pending
    dependencies:
      - lt-056
  - id: lt-058
    content: "Add unit tests: refactored code must accept injected dependencies; tests must use injected mocks, not mutate global state."
    status: pending
    dependencies:
      - lt-057
  - id: lt-059
    content: "Run `go test -v -run TestDI ./orchestrator/...`, `go test -v -run TestDI ./worker_node/...`, `go test -v -run TestDI ./agents/...`, `go test -v -run TestDI ./cynork/...` and confirm failures."
    status: pending
    dependencies:
      - lt-058
  - id: lt-060
    content: "Refactor orchestrator: replace each global test hook with constructor-injected dependency; update all test files to pass mocks via the constructor."
    status: pending
    dependencies:
      - lt-059
  - id: lt-061
    content: "Refactor worker node: same pattern as orchestrator."
    status: pending
    dependencies:
      - lt-060
  - id: lt-062
    content: "Refactor agents (PMA, SBA): same pattern as orchestrator."
    status: pending
    dependencies:
      - lt-061
  - id: lt-063
    content: "Refactor cynork: same pattern as orchestrator."
    status: pending
    dependencies:
      - lt-062
  - id: lt-064
    content: "Re-run DI tests across all modules and confirm green."
    status: pending
    dependencies:
      - lt-063
  - id: lt-065
    content: "Verify `t.Parallel()` can now be added to tests that were previously blocked by global mutable state."
    status: pending
    dependencies:
      - lt-064
  - id: lt-066
    content: "Run `just lint-go` on all changed files and `go test -race -cover` for each module; confirm 90% threshold."
    status: pending
    dependencies:
      - lt-065
  - id: lt-067
    content: "Run `just e2e --tags no_inference` to verify no regression from DI refactoring."
    status: pending
    dependencies:
      - lt-066
  - id: lt-068
    content: "Validation gate -- do not proceed to Task 6 until all checks pass."
    status: pending
    dependencies:
      - lt-067
  - id: lt-069
    content: "Generate task completion report for Task 5. Mark completed steps `- [x]`."
    status: pending
    dependencies:
      - lt-068
  - id: lt-070
    content: "Do not start Task 6 until Task 5 closeout is done."
    status: pending
    dependencies:
      - lt-069
  - id: lt-071
    content: "Identify key performance scenarios: concurrent chat streaming throughput, dispatcher under load (many tasks/jobs), PMA health check under contention, node registration burst."
    status: pending
    dependencies:
      - lt-070
  - id: lt-072
    content: "Read `docs/tech_specs/` for any existing performance or SLA requirements."
    status: pending
    dependencies:
      - lt-071
  - id: lt-073
    content: "Add Go benchmarks (`Benchmark*` functions) for: chat streaming handler, task dispatch handler, PMA health endpoint, node registration handler."
    status: pending
    dependencies:
      - lt-072
  - id: lt-074
    content: "Run `go test -bench=. -benchmem ./orchestrator/...` and `go test -bench=. -benchmem ./worker_node/...` to establish baseline performance."
    status: pending
    dependencies:
      - lt-073
  - id: lt-075
    content: "Add a load test script (Python or Go) that simulates concurrent users: 10 concurrent chat streams, 50 concurrent task creates, 5 concurrent node registrations."
    status: pending
    dependencies:
      - lt-074
  - id: lt-076
    content: "Add chaos/failure E2E scenarios: PMA crash during streaming (client must receive error, not hang), worker node disconnect during job execution, database connection loss during task create."
    status: pending
    dependencies:
      - lt-075
  - id: lt-077
    content: "Run load tests against the dev stack and document results: throughput, p99 latency, error rate."
    status: pending
    dependencies:
      - lt-076
  - id: lt-078
    content: "Run chaos scenarios and verify graceful degradation: error messages, no data corruption, recovery after reconnection."
    status: pending
    dependencies:
      - lt-077
  - id: lt-079
    content: "Add a `just perf-test` recipe to the justfile for running load and chaos tests."
    status: pending
    dependencies:
      - lt-078
  - id: lt-080
    content: "Run `just lint-go` and `just lint-python` on all new test files."
    status: pending
    dependencies:
      - lt-079
  - id: lt-081
    content: "Validation gate -- do not proceed to Task 7 until all checks pass."
    status: pending
    dependencies:
      - lt-080
  - id: lt-082
    content: "Generate task completion report for Task 6. Mark completed steps `- [x]`."
    status: pending
    dependencies:
      - lt-081
  - id: lt-083
    content: "Do not start Task 7 until Task 6 closeout is done."
    status: pending
    dependencies:
      - lt-082
  - id: lt-084
    content: "Update `docs/dev_docs/_todo.md` to mark all 6 Longer-Term items as complete."
    status: pending
    dependencies:
      - lt-083
  - id: lt-085
    content: "Verify no follow-up work was left undocumented."
    status: pending
    dependencies:
      - lt-084
  - id: lt-086
    content: "Run `just docs-check` on all changed documentation."
    status: pending
    dependencies:
      - lt-085
  - id: lt-087
    content: "Run `just ci` locally and confirm all targets pass."
    status: pending
    dependencies:
      - lt-086
  - id: lt-088
    content: "Run `just e2e --tags no_inference` as final E2E regression gate."
    status: pending
    dependencies:
      - lt-087
  - id: lt-089
    content: "Generate final plan completion report: tasks completed, overall validation, remaining risks."
    status: pending
    dependencies:
      - lt-088
  - id: lt-090
    content: "Mark all completed steps in the plan with `- [x]`. (Last step.)"
    status: pending
    dependencies:
      - lt-089
---

# Longer-Term Maintenance and Debt Plan

## Goal

Address 6 longer-term maintenance and technical debt items identified in review reports 1-6.
These cover foundational infrastructure (migration system, config centralization), architectural alignment (PMA startup, PMA monitoring), testability (global mutable state removal), and operational readiness (load and chaos testing).

## References

- Requirements: [REQ-ORCHES-0150](../requirements/orches.md#req-orches-0150) (PMA via worker instruction), [REQ-ORCHES-0129](../requirements/orches.md#req-orches-0129) (continuous PMA monitoring)
- Tech specs: [`docs/tech_specs/orchestrator.md`](../tech_specs/orchestrator.md), [`docs/tech_specs/orchestrator_bootstrap.md`](../tech_specs/orchestrator_bootstrap.md), [`docs/tech_specs/worker_node.md`](../tech_specs/worker_node.md)
- Review reports: [`2026-03-29_review_report_1_orchestrator.md`](old/2026-03-29_review_report_1_orchestrator.md), [`2026-03-29_review_report_2_worker_node.md`](old/2026-03-29_review_report_2_worker_node.md), [`2026-03-29_review_report_6_testing.md`](old/2026-03-29_review_report_6_testing.md)
- Consolidated summary: [`2026-03-29_review_consolidated_summary.md`](old/2026-03-29_review_consolidated_summary.md) section 2.6
- Implementation: `orchestrator/`, `worker_node/`, `agents/`, `cynork/`

## Constraints

- Requirements take precedence over tech specs; tech specs take precedence over current code.
- Follow BDD/TDD: failing tests before implementation.
- All changes must pass `just ci` before the task is considered complete.
- No changes that reduce test coverage below the 90% per-package threshold.
- No modifications to linter rules or suppression comments.
- Each task's validation gate must pass before starting the next task.
- Migration infrastructure (Task 1) should be in place before other database-touching tasks in other plans execute schema changes.
- PMA startup alignment (Task 2) depends on the per-session-binding plan (plan 005) being complete or in progress.

## Execution Plan

Tasks are ordered by foundational dependency: migration infrastructure first, then PMA architectural alignment, then cross-cutting refactors, finishing with operational testing.

### Task 1: Adopt Versioned Migrations to Replace AutoMigrate

Both orchestrator and worker node use GORM `AutoMigrate` on every startup with no version tracking, transaction wrapping, or rollback capability.

#### Task 1 Requirements and Specifications

- [Review Report 1](old/2026-03-29_review_report_1_orchestrator.md) -- `migrate.go:17-22` (no transaction), `migrate.go:28-69` (no version tracking)
- [Review Report 2](old/2026-03-29_review_report_2_worker_node.md) -- `store.go:42-51` (AutoMigrate), SchemaVersion placeholder

#### Discovery (Task 1) Steps

- [ ] Read `orchestrator/internal/store/migrate.go` lines 17-22 (no migration transaction) and 28-69 (AutoMigrate every startup, no version tracking).
- [ ] Read `worker_node/internal/securestore/store.go` lines 42-51 (AutoMigrate) and the SchemaVersion table placeholder.
- [ ] Evaluate migration libraries: `golang-migrate/migrate`, `pressly/goose`, or `atlas`; select one based on Go module compatibility, GORM integration, and CI support.

#### Red (Task 1)

- [ ] Add a unit test: startup must apply only pending migrations (not re-run all); running on an up-to-date schema must be a no-op.
- [ ] Add a unit test: each migration must run in a transaction; a failing migration must roll back without leaving the schema in a partial state.
- [ ] Run `go test -v -run 'TestMigrationIdempotent|TestMigrationRollback' ./orchestrator/internal/store/...` and confirm failures.

#### Green (Task 1)

- [ ] Create initial migration files from the current GORM AutoMigrate schema: one baseline migration per module (orchestrator, worker node).
- [ ] Implement migration runner: replace AutoMigrate with the selected library; track version in a `schema_migrations` table.
- [ ] Wire the migration runner into startup for orchestrator (control-plane, user-gateway, mcp-gateway, api-egress) and worker node (node-manager).
- [ ] Re-run `go test -v -run 'TestMigrationIdempotent|TestMigrationRollback' ./orchestrator/internal/store/...` and confirm green.

#### Refactor (Task 1)

- [ ] Add a `just migrate-new <name>` recipe to the justfile for creating new migration files.

#### Testing (Task 1)

- [ ] Run `just lint-go` on all changed files and `go test -cover ./orchestrator/...` and `go test -cover ./worker_node/...`; confirm 90% threshold.
- [ ] Run `just e2e --tags no_inference` to verify startup and schema migration regression.
- [ ] Validation gate -- do not proceed to Task 2 until all checks pass.

#### Closeout (Task 1)

- [ ] Generate task completion report for Task 1.
  Mark completed steps `- [x]`.
- [ ] Do not start Task 2 until Task 1 closeout is done.

---

### Task 2: Align PMA Startup With Worker-Instruction Model (REQ-ORCHES-0150)

The orchestrator currently starts PMA as a direct subprocess instead of going through the worker node's managed-service instruction path.

#### Task 2 Requirements and Specifications

- [REQ-ORCHES-0150](../requirements/orches.md#req-orches-0150) -- PMA started via worker instruction
- [`docs/tech_specs/orchestrator_bootstrap.md`](../tech_specs/orchestrator_bootstrap.md) -- PMA bootstrap via worker
- [Review Report 1](old/2026-03-29_review_report_1_orchestrator.md) -- `control-plane/main.go` + `pmasubprocess.Start`

#### Discovery (Task 2) Steps

- [ ] Read REQ-ORCHES-0150 in `docs/requirements/orches.md` for PMA startup via the worker-instruction model.
- [ ] Read `orchestrator/cmd/control-plane/main.go` and `orchestrator/internal/pmasubprocess/` (or equivalent) for the current PMA startup path.
- [ ] Read `docs/tech_specs/orchestrator_bootstrap.md` for the worker-instruction PMA startup spec.

#### Red (Task 2)

- [ ] Add unit tests: PMA must be started via the worker node's managed-service instruction path (not a direct subprocess from the orchestrator).
- [ ] Run `go test -v -run TestPmaWorkerInstruction ./orchestrator/...` and confirm failure (current path is direct subprocess).

#### Green (Task 2)

- [ ] Refactor PMA startup: remove direct subprocess launch from the orchestrator; instead, add PMA to the desired managed-service state and let the worker node reconcile it.
- [ ] Re-run `go test -v -run TestPmaWorkerInstruction ./orchestrator/...` and confirm green.

#### Refactor (Task 2)

No additional refactor needed.

#### Testing (Task 2)

- [ ] Run `just lint-go` on changed files and `go test -cover ./orchestrator/...`; confirm 90% threshold.
- [ ] Run `just e2e --tags pma_inference` (requires inference; skip if unavailable) to verify PMA startup via worker instruction.
- [ ] Validation gate -- do not proceed to Task 3 until all checks pass.

#### Closeout (Task 2)

- [ ] Generate task completion report for Task 2.
  Mark completed steps `- [x]`.
- [ ] Do not start Task 3 until Task 2 closeout is done.

---

### Task 3: Implement Continuous PMA Monitoring (REQ-ORCHES-0129)

The orchestrator only has a `readyzHandler` for PMA health; there is no continuous monitoring loop that detects PMA failure and triggers recovery.

#### Task 3 Requirements and Specifications

- [REQ-ORCHES-0129](../requirements/orches.md#req-orches-0129) -- continuous PMA monitoring
- [Review Report 1](old/2026-03-29_review_report_1_orchestrator.md) -- `readyzHandler` only, no continuous loop

#### Discovery (Task 3) Steps

- [ ] Read REQ-ORCHES-0129 in `docs/requirements/orches.md` for continuous PMA monitoring requirements.
- [ ] Read `orchestrator/internal/handlers/` for the current `readyzHandler` (health check only, no continuous loop).

#### Red (Task 3)

- [ ] Add unit tests: orchestrator must continuously monitor PMA health at a configurable interval (default 30s); detect PMA failure and trigger restart or re-provision.
- [ ] Run `go test -v -run TestPmaMonitoring ./orchestrator/...` and confirm failure.

#### Green (Task 3)

- [ ] Implement continuous PMA monitoring: background goroutine that polls PMA health endpoint at configurable interval; on failure, update desired state to trigger restart.
- [ ] Add configurable monitoring interval (env var or config field) with a sensible default (e.g., 30s).
- [ ] Re-run `go test -v -run TestPmaMonitoring ./orchestrator/...` and confirm green.

#### Refactor (Task 3)

No additional refactor needed.

#### Testing (Task 3)

- [ ] Run `just lint-go` on changed files and `go test -cover ./orchestrator/...`; confirm 90% threshold.
- [ ] Run `just e2e --tags pma_inference` (requires inference; skip if unavailable) to verify PMA monitoring and restart.
- [ ] Validation gate -- do not proceed to Task 4 until all checks pass.

#### Closeout (Task 3)

- [ ] Generate task completion report for Task 3.
  Mark completed steps `- [x]`.
- [ ] Do not start Task 4 until Task 3 closeout is done.

---

### Task 4: Extract Centralized Config Package for Worker Node

Worker node configuration is scattered across env var reads, flags, and hardcoded defaults with no single validated config struct.

#### Task 4 Requirements and Specifications

- [Review Report 2](old/2026-03-29_review_report_2_worker_node.md) -- section 3.3 (no single validated config package)

#### Discovery (Task 4) Steps

- [ ] Read `worker_node/cmd/node-manager/main.go` and `worker_node/internal/` to catalog all configuration sources: env vars, flags, hardcoded defaults, and config files.
- [ ] Identify inconsistencies: env vars read in multiple places, defaults that differ between call sites, missing validation.

#### Red (Task 4)

- [ ] Design a `worker_node/internal/config/config.go` package: single validated `Config` struct loaded once at startup, with env var binding and default values.
- [ ] Add unit tests: `LoadConfig()` must populate all fields from env vars; missing required fields must return a clear error; defaults must be applied for optional fields.
- [ ] Run `go test -v -run TestLoadConfig ./worker_node/internal/config/...` and confirm failures (package does not exist).

#### Green (Task 4)

- [ ] Implement `config.go`: define `Config` struct, `LoadConfig() (*Config, error)` function, and validation.
- [ ] Wire `LoadConfig()` into `worker_node/cmd/node-manager/main.go` startup; replace scattered `os.Getenv` calls with config field access.
- [ ] Re-run `go test -v -run TestLoadConfig ./worker_node/internal/config/...` and confirm green.

#### Refactor (Task 4)

- [ ] Remove all orphaned `os.Getenv` calls in `worker_node/` that are now served by the config package.

#### Testing (Task 4)

- [ ] Run `just lint-go` on changed files and `go test -cover ./worker_node/...`; confirm 90% threshold.
- [ ] Run `just e2e --tags worker,no_inference` to verify worker node startup and config regression.
- [ ] Validation gate -- do not proceed to Task 5 until all checks pass.

#### Closeout (Task 4)

- [ ] Generate task completion report for Task 4.
  Mark completed steps `- [x]`.
- [ ] Do not start Task 5 until Task 4 closeout is done.

---

### Task 5: Replace Global Mutable Test Hooks With Dependency Injection

Package-level `var` for test hooks appears in orchestrator, worker node, PMA, and cynork.
This pattern is not goroutine-safe and prevents `t.Parallel()`.

#### Task 5 Requirements and Specifications

- [Consolidated summary section 2.6](old/2026-03-29_review_consolidated_summary.md#26-global-mutable-state-for-test-injection) -- cross-module pattern
- All review reports

#### Discovery (Task 5) Steps

- [ ] Search all Go modules for package-level `var` used as test hooks: `orchestrator/`, `worker_node/`, `agents/`, `cynork/`.
- [ ] Catalog each test hook: variable name, file, line, what it replaces (HTTP client, time function, exec function, etc.), and which tests use it.

#### Red (Task 5)

- [ ] For each module, design the replacement: inject the dependency via constructor parameter or struct field instead of a package-level var.
- [ ] Add unit tests: refactored code must accept injected dependencies; tests must use injected mocks, not mutate global state.
- [ ] Run `go test -v -run TestDI ./orchestrator/...`, `go test -v -run TestDI ./worker_node/...`, `go test -v -run TestDI ./agents/...`, `go test -v -run TestDI ./cynork/...` and confirm failures.

#### Green (Task 5)

- [ ] Refactor orchestrator: replace each global test hook with constructor-injected dependency; update all test files to pass mocks via the constructor.
- [ ] Refactor worker node: same pattern as orchestrator.
- [ ] Refactor agents (PMA, SBA): same pattern as orchestrator.
- [ ] Refactor cynork: same pattern as orchestrator.
- [ ] Re-run DI tests across all modules and confirm green.

#### Refactor (Task 5)

- [ ] Verify `t.Parallel()` can now be added to tests that were previously blocked by global mutable state.

#### Testing (Task 5)

- [ ] Run `just lint-go` on all changed files and `go test -race -cover` for each module; confirm 90% threshold.
- [ ] Run `just e2e --tags no_inference` to verify no regression from DI refactoring.
- [ ] Validation gate -- do not proceed to Task 6 until all checks pass.

#### Closeout (Task 5)

- [ ] Generate task completion report for Task 5.
  Mark completed steps `- [x]`.
- [ ] Do not start Task 6 until Task 5 closeout is done.

---

### Task 6: Add Load/Performance Testing and Chaos/Failure E2E Scenarios

No load, performance, or chaos tests exist.
Streaming throughput, concurrent operations, and failure recovery are untested under realistic conditions.

#### Task 6 Requirements and Specifications

- [Review Report 6](old/2026-03-29_review_report_6_testing.md) -- no benchmarks, no load tests, no chaos scenarios

#### Discovery (Task 6) Steps

- [ ] Identify key performance scenarios: concurrent chat streaming throughput, dispatcher under load (many tasks/jobs), PMA health check under contention, node registration burst.
- [ ] Read `docs/tech_specs/` for any existing performance or SLA requirements.

#### Red (Task 6)

- [ ] Add Go benchmarks (`Benchmark*` functions) for: chat streaming handler, task dispatch handler, PMA health endpoint, node registration handler.
- [ ] Run `go test -bench=. -benchmem ./orchestrator/...` and `go test -bench=. -benchmem ./worker_node/...` to establish baseline performance.

#### Green (Task 6)

- [ ] Add a load test script (Python or Go) that simulates concurrent users: 10 concurrent chat streams, 50 concurrent task creates, 5 concurrent node registrations.
- [ ] Add chaos/failure E2E scenarios: PMA crash during streaming (client must receive error, not hang), worker node disconnect during job execution, database connection loss during task create.

#### Refactor (Task 6)

- [ ] Run load tests against the dev stack and document results: throughput, p99 latency, error rate.
- [ ] Run chaos scenarios and verify graceful degradation: error messages, no data corruption, recovery after reconnection.

#### Testing (Task 6)

- [ ] Add a `just perf-test` recipe to the justfile for running load and chaos tests.
- [ ] Run `just lint-go` and `just lint-python` on all new test files.
- [ ] Validation gate -- do not proceed to Task 7 until all checks pass.

#### Closeout (Task 6)

- [ ] Generate task completion report for Task 6.
  Mark completed steps `- [x]`.
- [ ] Do not start Task 7 until Task 6 closeout is done.

---

### Task 7: Documentation and Closeout

- [ ] Update `docs/dev_docs/_todo.md` to mark all 6 Longer-Term items as complete.
- [ ] Verify no follow-up work was left undocumented.
- [ ] Run `just docs-check` on all changed documentation.
- [ ] Run `just ci` locally and confirm all targets pass.
- [ ] Run `just e2e --tags no_inference` as final E2E regression gate.
- [ ] Generate final plan completion report: tasks completed, overall validation, remaining risks.
- [ ] Mark all completed steps in the plan with `- [x]`. (Last step.)
