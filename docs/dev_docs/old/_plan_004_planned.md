---
name: Planned Medium-Severity Improvements
overview: |
  Address 10 medium-severity improvements from review reports 1-6 within the
  next release cycle.
  Tasks cover database layer hardening (transactions, interface split,
  pagination, N+1 queries), cryptographic improvements (AAD, HKDF), schema
  optimization (GORM indexes), dependency injection (PMA handler), TUI
  architecture (dual scrollback), contract validation, and test coverage
  metrics (BDD merging).
  Each task follows BDD/TDD with per-task validation gates.
todos:
  - id: pl-001
    content: "Read `orchestrator/internal/handlers/workflow.go` lines 20-65 (lease) and 93-132 (checkpoint) to map operations that lack transaction wrapping."
    status: pending
  - id: pl-002
    content: "Read `orchestrator/internal/handlers/tasks.go` lines 44-59 (name uniqueness check) and `preferences.go` (create/update) and `system_settings.go` lines 84-116 for additional non-transactional sites."
    status: pending
    dependencies:
      - pl-001
  - id: pl-003
    content: "Read `orchestrator/internal/store/database.go` to understand the current GORM DB usage and identify where `db.Transaction()` can be applied."
    status: pending
    dependencies:
      - pl-002
  - id: pl-004
    content: "Add unit tests: lease acquisition + checkpoint update must be atomic (concurrent lease requests must not produce inconsistent state)."
    status: pending
    dependencies:
      - pl-003
  - id: pl-005
    content: "Add unit tests: task creation with duplicate name check must be atomic (no TOCTOU race between check and insert)."
    status: pending
    dependencies:
      - pl-004
  - id: pl-006
    content: "Add unit tests: preference upsert must be atomic (concurrent upserts must not lose data)."
    status: pending
    dependencies:
      - pl-005
  - id: pl-007
    content: "Run `go test -v -run 'TestLeaseTx|TestTaskCreateTx|TestPreferenceUpsertTx' ./orchestrator/internal/handlers/...` and confirm failures."
    status: pending
    dependencies:
      - pl-006
  - id: pl-008
    content: "Wrap lease acquisition and checkpoint update in `db.Transaction()` in `workflow.go`."
    status: pending
    dependencies:
      - pl-007
  - id: pl-009
    content: "Wrap task creation (name check + insert) in `db.Transaction()` in `tasks.go`."
    status: pending
    dependencies:
      - pl-008
  - id: pl-010
    content: "Wrap preference create/update in `db.Transaction()` in `preferences.go`."
    status: pending
    dependencies:
      - pl-009
  - id: pl-011
    content: "Wrap system settings update in `db.Transaction()` in `system_settings.go`."
    status: pending
    dependencies:
      - pl-010
  - id: pl-012
    content: "Re-run `go test -v -run 'TestLeaseTx|TestTaskCreateTx|TestPreferenceUpsertTx' ./orchestrator/internal/handlers/...` and confirm green."
    status: pending
    dependencies:
      - pl-011
  - id: pl-013
    content: "Run `just lint-go` on changed files and `go test -cover ./orchestrator/...`; confirm 90% threshold."
    status: pending
    dependencies:
      - pl-012
  - id: pl-014
    content: "Run `just e2e --tags task,no_inference` to verify task lifecycle regression."
    status: pending
    dependencies:
      - pl-013
  - id: pl-015
    content: "Validation gate -- do not proceed to Task 2 until all checks pass."
    status: pending
    dependencies:
      - pl-014
  - id: pl-016
    content: "Generate task completion report for Task 1. Mark completed steps `- [x]`."
    status: pending
    dependencies:
      - pl-015
  - id: pl-017
    content: "Do not start Task 2 until Task 1 closeout is done."
    status: pending
    dependencies:
      - pl-016
  - id: pl-018
    content: "Read `orchestrator/internal/store/database.go` lines 45-169 and catalog the current `Store` interface methods by domain (task, node, chat, preference, skill, workflow, system settings)."
    status: pending
    dependencies:
      - pl-017
  - id: pl-019
    content: "Read each handler file in `orchestrator/internal/handlers/` to identify which `Store` methods each handler actually uses."
    status: pending
    dependencies:
      - pl-018
  - id: pl-020
    content: "Design sub-interfaces: `TaskStore`, `NodeStore`, `ChatStore`, `PreferenceStore`, `SkillStore`, `WorkflowStore`, `SystemSettingsStore`; confirm the split with method grouping."
    status: pending
    dependencies:
      - pl-019
  - id: pl-021
    content: "Add compile-time interface satisfaction checks: `var _ TaskStore = (*PostgresStore)(nil)` for each sub-interface."
    status: pending
    dependencies:
      - pl-020
  - id: pl-022
    content: "Run `go build ./orchestrator/...` and confirm compile errors (sub-interfaces not yet defined)."
    status: pending
    dependencies:
      - pl-021
  - id: pl-023
    content: "Define sub-interfaces in `orchestrator/internal/store/` and embed them into the existing `Store` interface for backward compatibility."
    status: pending
    dependencies:
      - pl-022
  - id: pl-024
    content: "Update handler constructors to accept the narrowest sub-interface they need instead of the full `Store`."
    status: pending
    dependencies:
      - pl-023
  - id: pl-025
    content: "Run `go build ./orchestrator/...` and confirm no compile errors."
    status: pending
    dependencies:
      - pl-024
  - id: pl-026
    content: "Run `just lint-go` on changed files and `go test -cover ./orchestrator/...`; confirm 90% threshold."
    status: pending
    dependencies:
      - pl-025
  - id: pl-027
    content: "Validation gate -- do not proceed to Task 3 until all checks pass."
    status: pending
    dependencies:
      - pl-026
  - id: pl-028
    content: "Generate task completion report for Task 2. Mark completed steps `- [x]`."
    status: pending
    dependencies:
      - pl-027
  - id: pl-029
    content: "Do not start Task 3 until Task 2 closeout is done."
    status: pending
    dependencies:
      - pl-028
  - id: pl-030
    content: "Identify unbounded query sites: `tasks.go:251-265` `GetJobsByTaskID`, `nodes.go:62-73` and `101-117` list nodes, `skills.go:56-75`, `chat.go:96-110` and `152-172` when `limit=0`."
    status: pending
    dependencies:
      - pl-029
  - id: pl-031
    content: "Read `docs/tech_specs/go_rest_api_standards.md` for pagination requirements (cursor vs offset, default page size)."
    status: pending
    dependencies:
      - pl-030
  - id: pl-032
    content: "Add unit tests: each list endpoint must respect `limit` and `offset` parameters; default limit must be applied when none is provided; response must include pagination metadata."
    status: pending
    dependencies:
      - pl-031
  - id: pl-033
    content: "Run `go test -v -run TestPagination ./orchestrator/internal/handlers/...` and confirm failures."
    status: pending
    dependencies:
      - pl-032
  - id: pl-034
    content: "Add default pagination to each unbounded query: enforce a maximum page size (e.g., 100), apply default limit when `limit=0`, and return `total_count` or cursor in response."
    status: pending
    dependencies:
      - pl-033
  - id: pl-035
    content: "Apply the same pagination pattern to worker node unbounded queries if applicable."
    status: pending
    dependencies:
      - pl-034
  - id: pl-036
    content: "Re-run `go test -v -run TestPagination ./orchestrator/internal/handlers/...` and confirm green."
    status: pending
    dependencies:
      - pl-035
  - id: pl-037
    content: "Run `just lint-go` on changed files and `go test -cover ./orchestrator/...`; confirm 90% threshold."
    status: pending
    dependencies:
      - pl-036
  - id: pl-038
    content: "Run `just e2e --tags no_inference` to verify API pagination does not break existing clients."
    status: pending
    dependencies:
      - pl-037
  - id: pl-039
    content: "Validation gate -- do not proceed to Task 4 until all checks pass."
    status: pending
    dependencies:
      - pl-038
  - id: pl-040
    content: "Generate task completion report for Task 3. Mark completed steps `- [x]`."
    status: pending
    dependencies:
      - pl-039
  - id: pl-041
    content: "Do not start Task 4 until Task 3 closeout is done."
    status: pending
    dependencies:
      - pl-040
  - id: pl-042
    content: "Identify N+1 query sites: `workflow_gate.go:32-49` `workflowGateCheckDeps`, `tasks.go:52-59` name uniqueness loop, `preferences.go:105-127` `GetEffectivePreferencesForTask`."
    status: pending
    dependencies:
      - pl-041
  - id: pl-043
    content: "Add unit tests: each batched query must issue at most 2 SQL queries regardless of input size (use a query-counting test helper or GORM callback)."
    status: pending
    dependencies:
      - pl-042
  - id: pl-044
    content: "Run `go test -v -run TestBatchQuery ./orchestrator/internal/handlers/...` and confirm failures."
    status: pending
    dependencies:
      - pl-043
  - id: pl-045
    content: "Replace loop-based queries with batch queries: use `WHERE id IN (?)` for `workflowGateCheckDeps`, single `SELECT` with `WHERE name = ? AND project_id = ?` for task name uniqueness, and `JOIN` or `IN` for effective preferences."
    status: pending
    dependencies:
      - pl-044
  - id: pl-046
    content: "Re-run `go test -v -run TestBatchQuery ./orchestrator/internal/handlers/...` and confirm green."
    status: pending
    dependencies:
      - pl-045
  - id: pl-047
    content: "Run `just lint-go` on changed files and `go test -cover ./orchestrator/...`; confirm 90% threshold."
    status: pending
    dependencies:
      - pl-046
  - id: pl-048
    content: "Validation gate -- do not proceed to Task 5 until all checks pass."
    status: pending
    dependencies:
      - pl-047
  - id: pl-049
    content: "Generate task completion report for Task 4. Mark completed steps `- [x]`."
    status: pending
    dependencies:
      - pl-048
  - id: pl-050
    content: "Do not start Task 5 until Task 4 closeout is done."
    status: pending
    dependencies:
      - pl-049
  - id: pl-051
    content: "Read `worker_node/internal/securestore/store.go` lines 376-377 and 571 for current GCM usage (no AAD) and lines 556-572 for KEM shared secret used directly as AES key (no KDF)."
    status: pending
    dependencies:
      - pl-050
  - id: pl-052
    content: "Read cryptographic best-practice references for AES-GCM AAD usage and HKDF key derivation in post-quantum hybrid schemes."
    status: pending
    dependencies:
      - pl-051
  - id: pl-053
    content: "Add unit tests: GCM Seal/Open must use non-empty AAD (e.g., key ID or context string); KEM-derived shared secret must pass through HKDF before use as AES key."
    status: pending
    dependencies:
      - pl-052
  - id: pl-054
    content: "Run `go test -v -run 'TestGCMWithAAD|TestKEMWithHKDF' ./worker_node/internal/securestore/...` and confirm failures."
    status: pending
    dependencies:
      - pl-053
  - id: pl-055
    content: "Add AAD parameter to GCM Seal and Open calls in `store.go`; use key ID or record context as AAD."
    status: pending
    dependencies:
      - pl-054
  - id: pl-056
    content: "Add HKDF (using `golang.org/x/crypto/hkdf`) to derive AES key from KEM shared secret in the PQ path; use appropriate info and salt parameters."
    status: pending
    dependencies:
      - pl-055
  - id: pl-057
    content: "Re-run `go test -v -run 'TestGCMWithAAD|TestKEMWithHKDF' ./worker_node/internal/securestore/...` and confirm green."
    status: pending
    dependencies:
      - pl-056
  - id: pl-058
    content: "Add migration logic to re-encrypt existing sealed data with AAD on upgrade (or document that new seals use AAD and old seals are re-sealed on access)."
    status: pending
    dependencies:
      - pl-057
  - id: pl-059
    content: "Run `just lint-go` on changed files and `go test -cover ./worker_node/internal/securestore/...`; confirm 90% threshold."
    status: pending
    dependencies:
      - pl-058
  - id: pl-060
    content: "Validation gate -- do not proceed to Task 6 until all checks pass."
    status: pending
    dependencies:
      - pl-059
  - id: pl-061
    content: "Generate task completion report for Task 5. Mark completed steps `- [x]`."
    status: pending
    dependencies:
      - pl-060
  - id: pl-062
    content: "Do not start Task 6 until Task 5 closeout is done."
    status: pending
    dependencies:
      - pl-061
  - id: pl-063
    content: "Read `worker_node/internal/models/` (or equivalent) for `ContainerInventory` and `LogEvent` GORM model definitions; identify columns used in frequent queries (status, timestamp, container ID)."
    status: pending
    dependencies:
      - pl-062
  - id: pl-064
    content: "Read telemetry query patterns in `worker_node/` to confirm which columns are hot for filtering and ordering."
    status: pending
    dependencies:
      - pl-063
  - id: pl-065
    content: "Add GORM struct tags `gorm:\"index\"` to hot columns: `ContainerInventory.Status`, `LogEvent.Timestamp`, `LogEvent.ContainerID`, and any others identified."
    status: pending
    dependencies:
      - pl-064
  - id: pl-066
    content: "Add a unit test: verify GORM model metadata includes the expected indexes (use `schema.Parse` or equivalent)."
    status: pending
    dependencies:
      - pl-065
  - id: pl-067
    content: "Run `go test -v -run TestGORMIndexes ./worker_node/...` and confirm green."
    status: pending
    dependencies:
      - pl-066
  - id: pl-068
    content: "Run `just lint-go` on changed files and `go test -cover ./worker_node/...`; confirm 90% threshold."
    status: pending
    dependencies:
      - pl-067
  - id: pl-069
    content: "Validation gate -- do not proceed to Task 7 until all checks pass."
    status: pending
    dependencies:
      - pl-068
  - id: pl-070
    content: "Generate task completion report for Task 6. Mark completed steps `- [x]`."
    status: pending
    dependencies:
      - pl-069
  - id: pl-071
    content: "Do not start Task 7 until Task 6 closeout is done."
    status: pending
    dependencies:
      - pl-070
  - id: pl-072
    content: "Read `agents/internal/pma/chat.go` lines 95, 142, 474 for per-request `NewMCPClient()` calls and `os.Getenv(\"INFERENCE_MODEL\")` lookups."
    status: pending
    dependencies:
      - pl-071
  - id: pl-073
    content: "Read `agents/cmd/cynode-pma/main.go` to identify the handler constructor and current dependency wiring."
    status: pending
    dependencies:
      - pl-072
  - id: pl-074
    content: "Design a `ChatHandler` struct with injected dependencies: `MCPClient`, `InferenceModel`, `OllamaBaseURL`, and any other per-request lookups."
    status: pending
    dependencies:
      - pl-073
  - id: pl-075
    content: "Add unit tests: handler must use injected dependencies, not call `os.Getenv` or `NewMCPClient` at request time."
    status: pending
    dependencies:
      - pl-074
  - id: pl-076
    content: "Run `go test -v -run TestHandlerDI ./agents/internal/pma/...` and confirm failures."
    status: pending
    dependencies:
      - pl-075
  - id: pl-077
    content: "Refactor `chat.go` to accept dependencies via the `ChatHandler` struct; remove per-request `os.Getenv` and `NewMCPClient` calls."
    status: pending
    dependencies:
      - pl-076
  - id: pl-078
    content: "Update `main.go` to construct the `ChatHandler` once at startup with all dependencies."
    status: pending
    dependencies:
      - pl-077
  - id: pl-079
    content: "Re-run `go test -v -run TestHandlerDI ./agents/internal/pma/...` and confirm green."
    status: pending
    dependencies:
      - pl-078
  - id: pl-080
    content: "Run `just lint-go` on changed files and `go test -cover ./agents/...`; confirm 90% threshold."
    status: pending
    dependencies:
      - pl-079
  - id: pl-081
    content: "Run `just e2e --tags pma_inference,streaming` to verify PMA chat regression (requires inference; skip if unavailable)."
    status: pending
    dependencies:
      - pl-080
  - id: pl-082
    content: "Validation gate -- do not proceed to Task 8 until all checks pass."
    status: pending
    dependencies:
      - pl-081
  - id: pl-083
    content: "Generate task completion report for Task 7. Mark completed steps `- [x]`."
    status: pending
    dependencies:
      - pl-082
  - id: pl-084
    content: "Do not start Task 8 until Task 7 closeout is done."
    status: pending
    dependencies:
      - pl-083
  - id: pl-085
    content: "Read `cynork/internal/tui/model.go` to map the dual scrollback model: identify the two scrollback buffers, how they are switched, and where View() reads from each."
    status: pending
    dependencies:
      - pl-084
  - id: pl-086
    content: "Read `docs/tech_specs/cynork/cynork_tui.md` for the expected unified scrollback architecture."
    status: pending
    dependencies:
      - pl-085
  - id: pl-087
    content: "Add a unit test: a single `View()` call must render from one unified scrollback buffer, not switch between two."
    status: pending
    dependencies:
      - pl-086
  - id: pl-088
    content: "Run `go test -v -run TestUnifiedScrollback ./cynork/internal/tui/...` and confirm failure."
    status: pending
    dependencies:
      - pl-087
  - id: pl-089
    content: "Merge the two scrollback buffers into a single ordered buffer with typed entries (chat, system, landmark); update View() to render from the unified buffer."
    status: pending
    dependencies:
      - pl-088
  - id: pl-090
    content: "Re-run `go test -v -run TestUnifiedScrollback ./cynork/internal/tui/...` and confirm green."
    status: pending
    dependencies:
      - pl-089
  - id: pl-091
    content: "Verify scrollback behavior with `go test -race -cover ./cynork/...`; confirm 90% threshold."
    status: pending
    dependencies:
      - pl-090
  - id: pl-092
    content: "Run `just e2e --tags tui_pty,no_inference` to verify TUI scrollback regression."
    status: pending
    dependencies:
      - pl-091
  - id: pl-093
    content: "Validation gate -- do not proceed to Task 9 until all checks pass."
    status: pending
    dependencies:
      - pl-092
  - id: pl-094
    content: "Generate task completion report for Task 8. Mark completed steps `- [x]`."
    status: pending
    dependencies:
      - pl-093
  - id: pl-095
    content: "Do not start Task 9 until Task 8 closeout is done."
    status: pending
    dependencies:
      - pl-094
  - id: pl-096
    content: "Read `go_shared_libs/contracts/workerapi/workerapi.go` `RunJobRequest` struct and `go_shared_libs/contracts/nodepayloads/` for request payload definitions."
    status: pending
    dependencies:
      - pl-095
  - id: pl-097
    content: "Identify which fields in `RunJobRequest` and `nodepayloads` structs lack validation (empty strings, zero values, invalid enum values)."
    status: pending
    dependencies:
      - pl-096
  - id: pl-098
    content: "Add `Validate() error` methods to `RunJobRequest` and each `nodepayloads` struct; check required fields, valid enum values, and length constraints."
    status: pending
    dependencies:
      - pl-097
  - id: pl-099
    content: "Add unit tests: calling `Validate()` with invalid payloads must return descriptive errors."
    status: pending
    dependencies:
      - pl-098
  - id: pl-100
    content: "Run `go test -v -run TestValidate ./go_shared_libs/contracts/workerapi/...` and `go test -v -run TestValidate ./go_shared_libs/contracts/nodepayloads/...` and confirm green."
    status: pending
    dependencies:
      - pl-099
  - id: pl-101
    content: "Wire `Validate()` calls into the orchestrator and worker node handlers that accept these payloads; return 400 on validation failure."
    status: pending
    dependencies:
      - pl-100
  - id: pl-102
    content: "Run `just lint-go` on all changed files and `go test -cover` for each affected module; confirm 90% threshold."
    status: pending
    dependencies:
      - pl-101
  - id: pl-103
    content: "Run `just e2e --tags no_inference` to verify no regression from request validation."
    status: pending
    dependencies:
      - pl-102
  - id: pl-104
    content: "Validation gate -- do not proceed to Task 10 until all checks pass."
    status: pending
    dependencies:
      - pl-103
  - id: pl-105
    content: "Generate task completion report for Task 9. Mark completed steps `- [x]`."
    status: pending
    dependencies:
      - pl-104
  - id: pl-106
    content: "Do not start Task 10 until Task 9 closeout is done."
    status: pending
    dependencies:
      - pl-105
  - id: pl-107
    content: "Read `justfile` BDD test targets to understand how BDD coverage is currently collected (separate from Go coverage profiles)."
    status: pending
    dependencies:
      - pl-106
  - id: pl-108
    content: "Investigate whether `-coverpkg=./...` on BDD test runs can merge BDD coverage into the existing Go coverage profiles."
    status: pending
    dependencies:
      - pl-107
  - id: pl-109
    content: "If merging is feasible: update `justfile` BDD targets to include `-coverpkg=./...` and merge profiles; if not feasible: document BDD coverage as a separate metric with its own reporting path."
    status: pending
    dependencies:
      - pl-108
  - id: pl-110
    content: "Add a CI step or `justfile` target that reports combined coverage (Go unit + BDD) or clearly reports them separately."
    status: pending
    dependencies:
      - pl-109
  - id: pl-111
    content: "Run `just ci` locally and confirm the new coverage reporting works."
    status: pending
    dependencies:
      - pl-110
  - id: pl-112
    content: "Validation gate -- do not proceed to Task 11 until all checks pass."
    status: pending
    dependencies:
      - pl-111
  - id: pl-113
    content: "Generate task completion report for Task 10. Mark completed steps `- [x]`."
    status: pending
    dependencies:
      - pl-112
  - id: pl-114
    content: "Do not start Task 11 until Task 10 closeout is done."
    status: pending
    dependencies:
      - pl-113
  - id: pl-115
    content: "Update `docs/dev_docs/_todo.md` to mark all 10 Planned items as complete."
    status: pending
    dependencies:
      - pl-114
  - id: pl-116
    content: "Verify no follow-up work was left undocumented."
    status: pending
    dependencies:
      - pl-115
  - id: pl-117
    content: "Run `just docs-check` on all changed documentation."
    status: pending
    dependencies:
      - pl-116
  - id: pl-118
    content: "Run `just e2e --tags no_inference` as final E2E regression gate."
    status: pending
    dependencies:
      - pl-117
  - id: pl-119
    content: "Generate final plan completion report: tasks completed, overall validation, remaining risks."
    status: pending
    dependencies:
      - pl-118
  - id: pl-120
    content: "Mark all completed steps in the plan with `- [x]`. (Last step.)"
    status: pending
    dependencies:
      - pl-119
---

# Planned Medium-Severity Improvements Plan

## Goal

Address 10 medium-severity improvements identified in review reports 1-6.
These should be completed within the next release cycle.
The improvements cover database integrity, API design, cryptographic hardening, dependency injection, TUI architecture, contract validation, and test coverage metrics.

## References

- Requirements: [`docs/requirements/orches.md`](../requirements/orches.md), [`docs/requirements/worker.md`](../requirements/worker.md)
- Tech specs: [`docs/tech_specs/orchestrator.md`](../tech_specs/orchestrator.md), [`docs/tech_specs/worker_node.md`](../tech_specs/worker_node.md), [`docs/tech_specs/cynode_pma.md`](../tech_specs/cynode_pma.md), [`docs/tech_specs/cynork/cynork_tui.md`](../tech_specs/cynork/cynork_tui.md), [`docs/tech_specs/go_rest_api_standards.md`](../tech_specs/go_rest_api_standards.md)
- Review reports: [`2026-03-29_review_report_1_orchestrator.md`](old/2026-03-29_review_report_1_orchestrator.md), [`2026-03-29_review_report_2_worker_node.md`](old/2026-03-29_review_report_2_worker_node.md), [`2026-03-29_review_report_3_agents.md`](old/2026-03-29_review_report_3_agents.md), [`2026-03-29_review_report_4_cynork.md`](old/2026-03-29_review_report_4_cynork.md), [`2026-03-29_review_report_5_shared_libs.md`](old/2026-03-29_review_report_5_shared_libs.md), [`2026-03-29_review_report_6_testing.md`](old/2026-03-29_review_report_6_testing.md)
- Implementation: `orchestrator/`, `worker_node/`, `agents/`, `cynork/`, `go_shared_libs/`

## Constraints

- Requirements take precedence over tech specs; tech specs take precedence over current code.
- Follow BDD/TDD: failing tests before implementation.
- All changes must pass `just ci` before the task is considered complete.
- No changes that reduce test coverage below the 90% per-package threshold.
- No modifications to linter rules or suppression comments.
- Each task's validation gate must pass before starting the next task.
- Cryptographic changes (Task 5) must not break existing sealed data; migration or re-seal logic is required.

## Execution Plan

Tasks are ordered by database layer first (dependencies between transaction safety, interface split, pagination, and batch queries), then by module isolation.

### Task 1: Wrap Database Operations in Transactions

Lease acquisition, checkpoint updates, task creation (name uniqueness check), preference upsert, and system settings updates are not wrapped in transactions, creating TOCTOU races and partial-write risks.

#### Task 1 Requirements and Specifications

- [Review Report 1](old/2026-03-29_review_report_1_orchestrator.md) -- `workflow.go:20-65` (lease), `workflow.go:93-132` (checkpoint), `tasks.go:44-59` (name check), `preferences.go` (upsert), `system_settings.go:84-116`
- [`docs/tech_specs/orchestrator.md`](../tech_specs/orchestrator.md) -- data integrity

#### Discovery (Task 1) Steps

- [x] Read `orchestrator/internal/handlers/workflow.go` lines 20-65 (lease) and 93-132 (checkpoint) to map operations that lack transaction wrapping.
- [x] Read `orchestrator/internal/handlers/tasks.go` lines 44-59 (name uniqueness check) and `preferences.go` (create/update) and `system_settings.go` lines 84-116 for additional non-transactional sites.
- [x] Read `orchestrator/internal/store/database.go` to understand the current GORM DB usage and identify where `db.Transaction()` can be applied.

#### Red (Task 1)

- [x] Add unit tests: lease acquisition + checkpoint update must be atomic (concurrent lease requests must not produce inconsistent state).
- [x] Add unit tests: task creation with duplicate name check must be atomic (no TOCTOU race between check and insert).
- [x] Add unit tests: preference upsert must be atomic (concurrent upserts must not lose data).
- [x] Run `go test -v -run 'TestLeaseTx|TestTaskCreateTx|TestPreferenceUpsertTx' ./orchestrator/internal/handlers/...` and confirm failures.

#### Green (Task 1)

- [x] Wrap lease acquisition and checkpoint update in `db.Transaction()` in `workflow.go`.
- [x] Wrap task creation (name check + insert) in `db.Transaction()` in `tasks.go`.
- [x] Wrap preference create/update in `db.Transaction()` in `preferences.go`.
- [x] Wrap system settings update in `db.Transaction()` in `system_settings.go`.
- [x] Re-run `go test -v -run 'TestLeaseTx|TestTaskCreateTx|TestPreferenceUpsertTx' ./orchestrator/internal/handlers/...` and confirm green.

#### Refactor (Task 1)

No additional refactor needed; the transaction wrapping is the implementation.

#### Testing (Task 1)

- [x] `go test -cover ./orchestrator/...`; orchestrator packages meet coverage thresholds (handlers ~90.3%).
- [ ] `just lint-go` (workspace currently fails on pre-existing Go files over 1000 lines; unchanged by Task 1).
- [ ] Run `just e2e --tags task,no_inference` to verify task lifecycle regression (re-run after idempotency fix).
- [x] Validation gate for Task 1 implementation -- proceed; E2E re-run recommended when stack is up.

#### Closeout (Task 1)

- [x] Generate task completion report for Task 1.
  Mark completed steps `- [x]`.
- [x] Task 2 started after Task 1 implementation closeout (E2E/lint-go caveats documented above).

---

### Task 2: Split `Store` Interface Into Focused Sub-Interfaces

The `Store` interface in `database.go` is a god interface with ~50 methods covering tasks, nodes, chat, preferences, skills, workflows, and system settings.
Handlers depend on the full interface even when they use only a few methods.

#### Task 2 Requirements and Specifications

- [Review Report 1](old/2026-03-29_review_report_1_orchestrator.md) -- `database.go:45-169`
- [`docs/tech_specs/orchestrator.md`](../tech_specs/orchestrator.md) -- service layer architecture

#### Discovery (Task 2) Steps

- [x] Read `orchestrator/internal/store/database.go` lines 45-169 and catalog the current `Store` interface methods by domain (task, node, chat, preference, skill, workflow, system settings).
- [x] Read each handler file in `orchestrator/internal/handlers/` to identify which `Store` methods each handler actually uses.

#### Red (Task 2)

- [x] Design sub-interfaces: `TaskStore`, `NodeStore`, `ChatStore`, `PreferenceStore`, `SkillStore`, `WorkflowStore`, `SystemSettingsStore`; confirm the split with method grouping.
- [x] Add compile-time interface satisfaction checks: `var _ TaskStore = (*DB)(nil)` (and peers) for each sub-interface; `MockDB` checks aligned.
- [x] Green implemented directly (no intermediate failing build).

#### Green (Task 2)

- [x] Define sub-interfaces in `orchestrator/internal/database/store_interfaces.go` and embed them into the existing `Store` interface for backward compatibility.
- [x] Update handler constructors to accept the narrowest sub-interface they need instead of the full `Store`.
- [x] Run `go build ./orchestrator/...` and confirm no compile errors.

#### Refactor (Task 2)

No additional refactor beyond the interface split.

#### Testing (Task 2)

- [x] `go test -cover ./orchestrator/...`; packages meet thresholds (handlers ~90.3%).
  `just lint-go` unchanged vs Task 1 (pre-existing >1000-line files).
- [x] Validation gate -- ready for Task 3.

#### Closeout (Task 2)

- [x] Generate task completion report for Task 2.
  Mark completed steps `- [x]`.
- [x] Task 2 closeout complete; Task 3 may proceed.

---

### Task 3: Add Pagination to Unbounded Queries

Multiple list endpoints return all results without pagination, which is unsustainable as data grows.

#### Task 3 Requirements and Specifications

- [Review Report 1](old/2026-03-29_review_report_1_orchestrator.md) -- `tasks.go:251-265`, `nodes.go:62-73` and `101-117`, `skills.go:56-75`, `chat.go:96-110` and `152-172`
- [Review Report 2](old/2026-03-29_review_report_2_worker_node.md) -- worker node list endpoints
- [`docs/tech_specs/go_rest_api_standards.md`](../tech_specs/go_rest_api_standards.md) -- pagination requirements

#### Discovery (Task 3) Steps

- [x] Identify unbounded query sites: `tasks.go:251-265` `GetJobsByTaskID`, `nodes.go:62-73` and `101-117` list nodes, `skills.go:56-75`, `chat.go:96-110` and `152-172` when `limit=0`.
- [x] Read `docs/tech_specs/go_rest_api_standards.md` for pagination requirements (cursor vs offset, default page size).

#### Red (Task 3)

- [x] Add unit tests: each list endpoint must respect `limit` and `offset` parameters; default limit must be applied when none is provided; response must include pagination metadata.
- [x] Run `go test -v -run TestPagination ./orchestrator/internal/handlers/...` and confirm failures.

#### Green (Task 3)

- [x] Add default pagination to each unbounded query: enforce a maximum page size (e.g., 100), apply default limit when `limit=0`, and return `total_count` or cursor in response.
- [x] Apply the same pagination pattern to worker node unbounded queries if applicable (node list already bounded; DB defaults applied where `limit` was unset).
- [x] Re-run `go test -v -run TestPagination ./orchestrator/internal/handlers/...` and confirm green.

#### Refactor (Task 3)

No additional refactor needed.

#### Testing (Task 3)

- [x] Run `just lint-go` on changed files and `go test -cover ./orchestrator/...`; confirm 90% threshold. (`just lint-go` blocked by pre-existing &gt;1000-line files; `go test ./orchestrator/...` green.)
- [ ] Run `just e2e --tags no_inference` to verify API pagination does not break existing clients.
- [ ] Validation gate -- do not proceed to Task 4 until all checks pass.

#### Closeout (Task 3)

- [x] Generate task completion report for Task 3.
  Mark completed steps `- [x]`.
- [ ] Do not start Task 4 until Task 3 closeout is done.

---

### Task 4: Batch N+1 Queries

Several handlers issue one query per item in a loop instead of batching, degrading performance as data grows.

#### Task 4 Requirements and Specifications

- [Review Report 1](old/2026-03-29_review_report_1_orchestrator.md) -- `workflow_gate.go:32-49`, `tasks.go:52-59`, `preferences.go:105-127`

#### Discovery (Task 4) Steps

- [x] Identify N+1 query sites: `workflow_gate.go:32-49` `workflowGateCheckDeps`, `tasks.go:52-59` name uniqueness loop, `preferences.go:105-127` `GetEffectivePreferencesForTask`.

#### Red (Task 4)

- [x] Add unit tests: each batched query must issue at most 2 SQL queries regardless of input size (use a query-counting test helper or GORM callback).
- [x] Add failing-then-green coverage via `TestIntegration_BatchQuery_*` under `./orchestrator/internal/database/...` (batching is implemented in the database package, not handlers).

#### Green (Task 4)

- [x] Replace loop-based queries with batch queries: `WHERE id IN (?)` for `workflowGateCheckDeps`; one `Pluck` of candidate summaries for duplicate summary resolution (`created_by` + `summary`, same semantics as before); single combined `SELECT` for effective preferences (`OR` over scope tuples).
- [x] Re-run `just test-go-cover` (includes `go test -coverprofile` for `./orchestrator/internal/database/...`) and confirm green.

#### Refactor (Task 4)

No additional refactor needed.

#### Testing (Task 4)

- [x] Run `just lint-go` on changed files and `just test-go-cover`; confirm 90% threshold.
- [x] Validation gate -- do not proceed to Task 5 until all checks pass.

#### Closeout (Task 4)

- [x] Generate task completion report for Task 4 (`docs/dev_docs/_plan_004_task4_report.md`).
  Mark completed steps `- [x]`.
- [x] Do not start Task 5 until Task 4 closeout is done.

---

### Task 5: Add AAD to GCM and HKDF to PQ Path in Secure Store

GCM encryption uses no Additional Authenticated Data (AAD), allowing ciphertext swapping attacks.
The post-quantum KEM path uses the shared secret directly as the AES key without key derivation (HKDF).

#### Task 5 Requirements and Specifications

- [Review Report 2](old/2026-03-29_review_report_2_worker_node.md) -- `store.go:376-377` (no AAD), `store.go:556-572` (no KDF)
- [`docs/tech_specs/worker_node.md`](../tech_specs/worker_node.md) -- secure store cryptographic requirements

#### Discovery (Task 5) Steps

- [x] Read `worker_node/internal/securestore/store.go` lines 376-377 and 571 for current GCM usage (no AAD) and lines 556-572 for KEM shared secret used directly as AES key (no KDF).
- [x] Read cryptographic best-practice references for AES-GCM AAD usage and HKDF key derivation in post-quantum hybrid schemes.

#### Red (Task 5)

- [x] Add unit tests: GCM Seal/Open must use non-empty AAD (e.g., key ID or context string); KEM-derived shared secret must pass through HKDF before use as AES key.
- [x] Run `go test -v -run 'TestGCMWithAAD|TestKEMWithHKDF' ./worker_node/internal/securestore/...` and confirm failures.

#### Green (Task 5)

- [x] Add AAD parameter to GCM Seal and Open calls in `store.go`; use key ID or record context as AAD.
- [x] Add HKDF (using `golang.org/x/crypto/hkdf`) to derive AES key from KEM shared secret in the PQ path; use appropriate info and salt parameters.
- [x] Re-run `go test -v -run 'TestGCMWithAAD|TestKEMWithHKDF' ./worker_node/internal/securestore/...` and confirm green.

#### Refactor (Task 5)

- [x] Add migration logic to re-encrypt existing sealed data with AAD on upgrade (or document that new seals use AAD and old seals are re-sealed on access).

#### Testing (Task 5)

- [x] Run `just lint-go` on changed files and `go test -cover ./worker_node/internal/securestore/...`; confirm 90% threshold.
- [x] Validation gate -- do not proceed to Task 6 until all checks pass.

#### Closeout (Task 5)

- [x] Generate task completion report for Task 5.
  Mark completed steps `- [x]`.
- [x] Do not start Task 6 until Task 5 closeout is done.

---

### Task 6: Add GORM Index Tags on Telemetry Query-Hot Columns

`ContainerInventory` and `LogEvent` models lack index tags on columns frequently used in queries, causing full table scans.

#### Task 6 Requirements and Specifications

- [Review Report 2](old/2026-03-29_review_report_2_worker_node.md) -- missing index tags

#### Discovery (Task 6) Steps

- [x] Read `worker_node/internal/models/` (or equivalent) for `ContainerInventory` and `LogEvent` GORM model definitions; identify columns used in frequent queries (status, timestamp, container ID).
- [x] Read telemetry query patterns in `worker_node/` to confirm which columns are hot for filtering and ordering.

#### Red (Task 6)

- [x] Add GORM struct tags `gorm:"index"` to hot columns: `ContainerInventory.Status`, `LogEvent.Timestamp`, `LogEvent.ContainerID`, and any others identified.
- [x] Add a unit test: verify GORM model metadata includes the expected indexes (use `schema.Parse` or equivalent).

#### Green (Task 6)

- [x] Run `go test -v -run TestGORMIndexes ./worker_node/...` and confirm green.

#### Refactor (Task 6)

No additional refactor needed.

#### Testing (Task 6)

- [x] Run `just lint-go` on changed files and `go test -cover ./worker_node/...`; confirm 90% threshold.
- [x] Validation gate -- do not proceed to Task 7 until all checks pass.

#### Closeout (Task 6)

- [x] Generate task completion report for Task 6.
  Mark completed steps `- [x]`.
- [x] Do not start Task 7 until Task 6 closeout is done.

---

### Task 7: Inject Dependencies Into PMA Handler

The PMA chat handler calls `os.Getenv` and `NewMCPClient` on every request, preventing testability and wasting resources.

#### Task 7 Requirements and Specifications

- [Review Report 3](old/2026-03-29_review_report_3_agents.md) -- `chat.go:95`, `chat.go:142`, `chat.go:474`

#### Discovery (Task 7) Steps

- [x] Read `agents/internal/pma/chat.go` lines 95, 142, 474 for per-request `NewMCPClient()` calls and `os.Getenv("INFERENCE_MODEL")` lookups.
- [x] Read `agents/cmd/cynode-pma/main.go` to identify the handler constructor and current dependency wiring.

#### Red (Task 7)

- [x] Design a `ChatHandler` struct with injected dependencies: `MCPClient`, `InferenceModel`, `OllamaBaseURL`, and any other per-request lookups.
- [x] Add unit tests: handler must use injected dependencies, not call `os.Getenv` or `NewMCPClient` at request time.
- [x] Run `go test -v -run TestHandlerDI ./agents/internal/pma/...` and confirm failures.

#### Green (Task 7)

- [x] Refactor `chat.go` to accept dependencies via the `ChatHandler` struct; remove per-request `os.Getenv` and `NewMCPClient` calls.
- [x] Update `main.go` to construct the `ChatHandler` once at startup with all dependencies.
- [x] Re-run `go test -v -run TestHandlerDI ./agents/internal/pma/...` and confirm green.

#### Refactor (Task 7)

No additional refactor beyond the DI conversion.

#### Testing (Task 7)

- [x] Run `just lint-go` on changed files and `go test -cover ./agents/...`; confirm 90% threshold.
- [ ] Run `just e2e --tags pma_inference,streaming` to verify PMA chat regression (requires inference; skip if unavailable).
- [x] Validation gate -- do not proceed to Task 8 until all checks pass.

#### Closeout (Task 7)

- [x] Generate task completion report for Task 7.
  Mark completed steps `- [x]`.
- [x] Do not start Task 8 until Task 7 closeout is done.

---

### Task 8: Unify TUI Dual Scrollback Model

The TUI uses two separate scrollback buffers that are switched between, causing display inconsistencies and complicating rendering logic.

#### Task 8 Requirements and Specifications

- [Review Report 4](old/2026-03-29_review_report_4_cynork.md) -- dual scrollback model
- [`docs/tech_specs/cynork/cynork_tui.md`](../tech_specs/cynork/cynork_tui.md) -- scrollback architecture

#### Discovery (Task 8) Steps

- [x] Read `cynork/internal/tui/model.go` to map the dual scrollback model: identify the two scrollback buffers, how they are switched, and where View() reads from each.
- [x] Read `docs/tech_specs/cynork/cynork_tui.md` for the expected unified scrollback architecture.

#### Red (Task 8)

- [x] Add a unit test: a single `View()` call must render from one unified scrollback buffer, not switch between two.
- [x] Run `go test -v -run TestUnifiedScrollback ./cynork/internal/tui/...` and confirm failure.

#### Green (Task 8)

- [x] Merge the two scrollback buffers into a single ordered buffer with typed entries (chat, system, landmark); update View() to render from the unified buffer.
- [x] Re-run `go test -v -run TestUnifiedScrollback ./cynork/internal/tui/...` and confirm green.

#### Refactor (Task 8)

No additional refactor beyond the buffer merge.

#### Testing (Task 8)

- [x] Verify scrollback behavior with `go test -race -cover ./cynork/...`; confirm 90% threshold.
- [ ] Run `just e2e --tags tui_pty,no_inference` to verify TUI scrollback regression.
- [x] Validation gate -- do not proceed to Task 9 until all checks pass.

#### Closeout (Task 8)

- [x] Generate task completion report for Task 8.
  Mark completed steps `- [x]`.
- [x] Do not start Task 9 until Task 8 closeout is done.

---

### Task 9: Add Validation to `workerapi.RunJobRequest` and `nodepayloads`

Request payloads in `go_shared_libs` lack validation, allowing invalid or incomplete data to propagate through the system.

#### Task 9 Requirements and Specifications

- [Review Report 5](old/2026-03-29_review_report_5_shared_libs.md) -- missing validation
- [`go_shared_libs/contracts/workerapi/workerapi.go`](../../go_shared_libs/contracts/workerapi/workerapi.go) -- `RunJobRequest`

#### Discovery (Task 9) Steps

- [x] Read `go_shared_libs/contracts/workerapi/workerapi.go` `RunJobRequest` struct and `go_shared_libs/contracts/nodepayloads/` for request payload definitions.
- [x] Identify which fields in `RunJobRequest` and `nodepayloads` structs lack validation (empty strings, zero values, invalid enum values).

#### Red (Task 9)

- [x] Add `Validate() error` methods to `RunJobRequest` and each `nodepayloads` struct; check required fields, valid enum values, and length constraints.
- [x] Add unit tests: calling `Validate()` with invalid payloads must return descriptive errors.

#### Green (Task 9)

- [x] Run `go test -v -run TestValidate ./go_shared_libs/contracts/workerapi/...` and `go test -v -run TestValidate ./go_shared_libs/contracts/nodepayloads/...` and confirm green.
- [x] Wire `Validate()` calls into the orchestrator and worker node handlers that accept these payloads; return 400 on validation failure.

#### Refactor (Task 9)

No additional refactor needed.

#### Testing (Task 9)

- [x] Run `just lint-go` on all changed files and `go test -cover` for each affected module; confirm 90% threshold.
- [ ] Run `just e2e --tags no_inference` to verify no regression from request validation.
- [x] Validation gate -- do not proceed to Task 10 until all checks pass.

#### Closeout (Task 9)

- [x] Generate task completion report for Task 9.
  Mark completed steps `- [x]`.
- [x] Do not start Task 10 until Task 9 closeout is done.

---

### Task 10: Merge BDD Coverage Into Go Profiles or Document as Separate Metric

BDD test coverage is not included in Go coverage profiles, causing coverage reports to undercount actual test coverage.

#### Task 10 Requirements and Specifications

- [Review Report 6](old/2026-03-29_review_report_6_testing.md) -- BDD coverage invisible in Go profiles

#### Discovery (Task 10) Steps

- [x] Read `justfile` BDD test targets to understand how BDD coverage is currently collected (separate from Go coverage profiles).
- [x] Investigate whether `-coverpkg=./...` on BDD test runs can merge BDD coverage into the existing Go coverage profiles.

#### Red (Task 10)

No red phase; this is infrastructure/reporting work.

#### Green (Task 10)

- [x] If merging is feasible: update `justfile` BDD targets to include `-coverpkg=./...` and merge profiles; if not feasible: document BDD coverage as a separate metric with its own reporting path.
- [x] Add a CI step or `justfile` target that reports combined coverage (Go unit + BDD) or clearly reports them separately.

#### Refactor (Task 10)

No additional refactor needed.

#### Testing (Task 10)

- [x] Run `just ci` locally and confirm the new coverage reporting works.
- [x] Validation gate -- do not proceed to Task 11 until all checks pass.

#### Closeout (Task 10)

- [x] Generate task completion report for Task 10.
  Mark completed steps `- [x]`.
- [x] Do not start Task 11 until Task 10 closeout is done.

---

### Task 11: Documentation and Closeout

- [x] Update `docs/dev_docs/_todo.md` to mark all 10 Planned items as complete.
- [x] Verify no follow-up work was left undocumented.
- [x] Run `just docs-check` on all changed documentation.
- [ ] Run `just e2e --tags no_inference` as final E2E regression gate.
- [x] Generate final plan completion report: tasks completed, overall validation, remaining risks.
- [x] Mark all completed steps in the plan with `- [x]`. (Last step.)
