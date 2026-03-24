---
name: Consolidated Refactor and Outstanding Work
overview: |
  Single execution plan sequencing refactor work (Tasks 1-4) then outstanding
  streaming, TUI, MCP, and documentation work (Tasks 5-12). Each task is
  test-gated with unit, BDD, and Python E2E layers where applicable.
  Steps are strictly ordered; tasks and subsections are not skipped or deferred without a plan revision.
# Todos: one entry per `- [ ]` / `- [x]` line under ## Execution Plan (order preserved).
# Non-checkbox layer labels (`- **Python E2E tests** (…):`, etc.) are omitted; nested work items, Red/Testing gate lines, and closeout steps are listed.
# Each todo depends on the prior step.
todos:
- id: consolidated-2026-03-24-step-001
  content: "Read the requirements and specs listed in Task 1 Requirements and Specifications."
  status: completed
- id: consolidated-2026-03-24-step-002
  content: "Inspect current `orchestrator/internal/models` and `orchestrator/internal/database` for existing `TaskArtifact` model and any artifact-related store methods."
  status: completed
  dependencies:
  - consolidated-2026-03-24-step-001
- id: consolidated-2026-03-24-step-003
  content: "Inspect `orchestrator/docker-compose.yml` for current services; confirm MinIO (or equivalent S3) is not yet present."
  status: completed
  dependencies:
  - consolidated-2026-03-24-step-002
- id: consolidated-2026-03-24-step-004
  content: "Inspect `orchestrator/internal/handlers/` for any existing artifact endpoints; identify what must be added or refactored."
  status: completed
  dependencies:
  - consolidated-2026-03-24-step-003
- id: consolidated-2026-03-24-step-005
  content: "Inspect MCP gateway (`orchestrator/internal/mcpgateway/`) for existing artifact tool handlers; identify gaps vs spec."
  status: completed
  dependencies:
  - consolidated-2026-03-24-step-004
- id: consolidated-2026-03-24-step-006
  content: "Identify the GORM record struct pattern from the completed GORM table definition plan and confirm new artifact models follow it (domain base in `models`, record in `database`)."
  status: completed
  dependencies:
  - consolidated-2026-03-24-step-005
- id: consolidated-2026-03-24-step-007
  content: "Review the \"Deferred Implementation\" section in the spec and map each item to implementation files and tests."
  status: completed
  dependencies:
  - consolidated-2026-03-24-step-006
- id: consolidated-2026-03-24-step-008
  content: "Add or create a dedicated artifact E2E module (e.g. `e2e_0850_artifacts_crud.py`) covering: Create with scope, Read with RBAC, Update blob overwrite, Delete with vector cleanup, Find/list with scope filters and pagination."
  status: completed
  dependencies:
  - consolidated-2026-03-24-step-007
- id: consolidated-2026-03-24-step-009
  content: "Add E2E for MCP artifact tools (`artifact.put`, `artifact.get`, `artifact.list`) via PMA chat or direct MCP call."
  status: completed
  dependencies:
  - consolidated-2026-03-24-step-008
- id: consolidated-2026-03-24-step-010
  content: "Run `just e2e` for the new module and confirm tests fail before implementation."
  status: completed
  dependencies:
  - consolidated-2026-03-24-step-009
- id: consolidated-2026-03-24-step-011
  content: "Add scenarios for user-scoped artifact CRUD (create/read) via REST (in-memory blob in `_bdd`)."
  status: completed
  dependencies:
  - consolidated-2026-03-24-step-010
- id: consolidated-2026-03-24-step-012
  content: "Add scenarios for group/project/global scope partitions (required before Task 1 Red is complete)."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-011
- id: consolidated-2026-03-24-step-013
  content: "Add scenarios for RBAC deny-by-default (second user cannot read admin user-scoped artifact)."
  status: completed
  dependencies:
  - consolidated-2026-03-24-step-012
- id: consolidated-2026-03-24-step-014
  content: "Add scenarios for cross-principal read via explicit grant (required before Task 1 Red is complete)."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-013
- id: consolidated-2026-03-24-step-015
  content: "Add BDD scenarios for MCP artifact tool routing (PMA/PAA allowlists); MCP tools covered by E2E and unit tests (required before Task 1 Red is complete)."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-014
- id: consolidated-2026-03-24-step-016
  content: "Artifact domain base + `OrchestratorArtifactRecord` (existing); unit tests for `ScopePartition` / `SanitizePath` and `MemStore`."
  status: completed
  dependencies:
  - consolidated-2026-03-24-step-015
- id: consolidated-2026-03-24-step-017
  content: "S3: `BlobStore` interface + `MemStore`; live MinIO client exercised via E2E not unit-tested to 90% alone."
  status: completed
  dependencies:
  - consolidated-2026-03-24-step-016
- id: consolidated-2026-03-24-step-018
  content: "RBAC: exercised via BDD and `mcpgateway` tests; narrow unit coverage in `internal/artifacts`."
  status: completed
  dependencies:
  - consolidated-2026-03-24-step-017
- id: consolidated-2026-03-24-step-019
  content: "Handler: `artifacts_test` nil-service path; full five-endpoint matrix follow-up for coverage thresholds."
  status: completed
  dependencies:
  - consolidated-2026-03-24-step-018
- id: consolidated-2026-03-24-step-020
  content: "MCP tool handler tests for `artifact.put`, `artifact.get`, `artifact.list` in `handlers_test.go`."
  status: completed
  dependencies:
  - consolidated-2026-03-24-step-019
- id: consolidated-2026-03-24-step-021
  content: "Store methods: full DB coverage for orchestrator artifact CRUD to `just test-go-cover` thresholds (required before Task 1 Red is complete)."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-020
- id: consolidated-2026-03-24-step-022
  content: "**Red - Python E2E:** Run `just e2e` for the new artifact module; confirm failures match the remaining gaps above (re-run after any new Red items land)."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-021
- id: consolidated-2026-03-24-step-023
  content: "**Red - BDD:** Run `go test ./orchestrator/_bdd` (or `just test-bdd` orchestrator slice); confirm scenarios match the remaining gaps above."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-022
- id: consolidated-2026-03-24-step-024
  content: "**Red - Go:** Run `go test` / `just test-go-cover` for orchestrator packages with new artifact tests; confirm coverage and failures match the remaining gaps above."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-023
- id: consolidated-2026-03-24-step-025
  content: "**Red validation gate:** Do not proceed to Green until every Red nested item above is `[x]` and Python E2E, BDD, and Go checks demonstrate the intended state for this task."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-024
- id: consolidated-2026-03-24-step-026
  content: "Add MinIO (or equivalent S3-compatible service) to `orchestrator/docker-compose.yml` with port 9000, volume, and env wiring (`ARTIFACTS_S3_ENDPOINT`, `ARTIFACTS_S3_ACCESS_KEY`, `ARTIFACTS_S3_SECRET_KEY`, `ARTIFACTS_S3_BUCKET`)."
  status: completed
  dependencies:
  - consolidated-2026-03-24-step-025
- id: consolidated-2026-03-24-step-027
  content: "Implement S3 client wrapper in `orchestrator/internal/` (upload, download, delete, overwrite) using an MIT/Apache-2.0 licensed Go S3 SDK."
  status: completed
  dependencies:
  - consolidated-2026-03-24-step-026
- id: consolidated-2026-03-24-step-028
  content: "Define artifact domain base struct and `OrchestratorArtifactRecord` in `orchestrator/internal/database` following GORM model structure standard."
  status: completed
  dependencies:
  - consolidated-2026-03-24-step-027
- id: consolidated-2026-03-24-step-029
  content: "Fields: `scope_level`, `owner_user_id`, `group_id`, `project_id`, `path`, `storage_ref`, `size_bytes`, `content_type`, `checksum_sha256`, `created_by_job_id`, `last_modified_by_job_id`, `correlation_task_id`, `run_id`."
  status: completed
  dependencies:
  - consolidated-2026-03-24-step-028
- id: consolidated-2026-03-24-step-030
  content: "Unique constraints per `scope_level` partition per spec."
  status: completed
  dependencies:
  - consolidated-2026-03-24-step-029
- id: consolidated-2026-03-24-step-031
  content: "Register `OrchestratorArtifactRecord` in `migrate.go`."
  status: completed
  dependencies:
  - consolidated-2026-03-24-step-030
- id: consolidated-2026-03-24-step-032
  content: "Implement Store methods for artifact CRUD with scope-based queries and RBAC evaluation."
  status: completed
  dependencies:
  - consolidated-2026-03-24-step-031
- id: consolidated-2026-03-24-step-033
  content: "Implement handler functions for all five REST endpoints per the spec algorithms."
  status: completed
  dependencies:
  - consolidated-2026-03-24-step-032
- id: consolidated-2026-03-24-step-034
  content: "RBAC enforcement on every operation using subject model from `rbac_and_groups.md`."
  status: completed
  dependencies:
  - consolidated-2026-03-24-step-033
- id: consolidated-2026-03-24-step-035
  content: "Vector items cleanup on delete per spec."
  status: completed
  dependencies:
  - consolidated-2026-03-24-step-034
- id: consolidated-2026-03-24-step-036
  content: "Implement MCP tool handlers (`artifact.put`, `artifact.get`, `artifact.list`) in the MCP gateway; route through same backend and RBAC as REST."
  status: completed
  dependencies:
  - consolidated-2026-03-24-step-035
- id: consolidated-2026-03-24-step-037
  content: "Implement artifact hashing: small artifacts hashed on upload; large artifacts deferred to background job (`ARTIFACT_HASH_BACKFILL_ENABLED` + `BackfillMissingHashesOnce`)."
  status: completed
  dependencies:
  - consolidated-2026-03-24-step-036
- id: consolidated-2026-03-24-step-038
  content: "Implement stale artifact cleanup (configurable, disabled by default) per spec (`ARTIFACT_STALE_CLEANUP_*` + `PruneStaleByMaxAgeOnce`)."
  status: completed
  dependencies:
  - consolidated-2026-03-24-step-037
- id: consolidated-2026-03-24-step-039
  content: "**Green - BDD (scope partitions):** Implement and pass BDD for group/project/global artifact scope partitions (pairs with matching Red BDD item); `go test ./orchestrator/_bdd` green for those scenarios."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-038
- id: consolidated-2026-03-24-step-040
  content: "**Green - BDD (cross-principal grant):** Implement and pass BDD for cross-principal read via explicit grant (pairs with matching Red BDD item)."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-039
- id: consolidated-2026-03-24-step-041
  content: "**Green - BDD (MCP routing):** Implement and pass BDD for MCP artifact tool routing (PMA/PAA allowlists) (pairs with matching Red BDD item)."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-040
- id: consolidated-2026-03-24-step-042
  content: "**Green - Go (store DB coverage):** Add store-level DB tests and raise coverage until `just test-go-cover` meets thresholds for orchestrator artifact CRUD (pairs with matching Red Go item)."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-041
- id: consolidated-2026-03-24-step-043
  content: "Run targeted tests until they pass for all Green items above and core implementation."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-042
- id: consolidated-2026-03-24-step-044
  content: "Validation gate: do not proceed to Refactor until artifacts CRUD, RBAC, MCP, S3 integration, and every **Green -** line above are green."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-043
- id: consolidated-2026-03-24-step-045
  content: "Extract shared RBAC evaluation helpers if duplicated between artifact handlers and other scope-based handlers."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-044
- id: consolidated-2026-03-24-step-046
  content: "Ensure S3 client is testable (interface-based) and supports mock backends for unit tests."
  status: completed
  dependencies:
  - consolidated-2026-03-24-step-045
- id: consolidated-2026-03-24-step-047
  content: "Re-run targeted tests after any refactor or extraction above."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-046
- id: consolidated-2026-03-24-step-048
  content: "Validation gate: do not proceed to Testing until `Re-run targeted tests` is green and S3 client testability is still satisfied; resolve **Extract shared RBAC** in this task or record in the Task 1 completion report why it remains open (no silent deferrals)."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-047
- id: consolidated-2026-03-24-step-049
  content: "**Go unit tests:** Run `just test-go-cover` for orchestrator (database, handlers, mcpgateway, S3 client packages); confirm all artifact unit tests pass and coverage meets thresholds. (Current run fails pre-existing cynork gaps and several orchestrator packages below 90%; `go test ./...` in orchestrator passes.)"
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-048
- id: consolidated-2026-03-24-step-050
  content: "**BDD tests:** Run `go test ./orchestrator/_bdd` from repo root (same as `just test-bdd` orchestrator slice); artifact scenarios pass."
  status: completed
  dependencies:
  - consolidated-2026-03-24-step-049
- id: consolidated-2026-03-24-step-051
  content: "**Python E2E tests:** Run `just setup-dev restart --force` then `just e2e --tags artifacts` (or targeted module); confirm all artifact E2E tests pass. (Not re-run in this session; requires stack + rebuilt `:dev` images.)"
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-050
- id: consolidated-2026-03-24-step-052
  content: "Run `just lint-go` for changed packages."
  status: completed
  dependencies:
  - consolidated-2026-03-24-step-051
- id: consolidated-2026-03-24-step-053
  content: "Run `just lint-go-ci` for changed packages. (Fails on existing golangci issues repo-wide; `path` shadow in `MCPPut`/`MCPGet` fixed.)"
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-052
- id: consolidated-2026-03-24-step-054
  content: "Run `just docs-check` if any spec or README changed. (No spec changes in this pass; not required.)"
  status: completed
  dependencies:
  - consolidated-2026-03-24-step-053
- id: consolidated-2026-03-24-step-055
  content: "**Testing validation gate:** Do not start Task 2 until **Go** (`just test-go-cover` …), **BDD** (`go test ./orchestrator/_bdd` …), **Python E2E** (`just e2e --tags artifacts` …), `just lint-go`, `just lint-go-ci`, and `just docs-check` (when applicable) in `#### Testing (Task 1)` above are each satisfied per their checkboxes."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-054
- id: consolidated-2026-03-24-step-056
  content: "Generate a **task completion report** for Task 1: what was done (S3 backend, CRUD API, RBAC, MCP tools, hashing, cleanup), what passed, any deviations or notes."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-055
- id: consolidated-2026-03-24-step-057
  content: "Do not start Task 2 until this closeout is done."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-056
- id: consolidated-2026-03-24-step-058
  content: "Mark every completed step in this task with `- [x]`. (Last step.)"
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-057
- id: consolidated-2026-03-24-step-059
  content: "Read the TUI delta \"Recommended Spec Updates\" items 1-7 in full."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-058
- id: consolidated-2026-03-24-step-060
  content: "Read Bug 3 and Bug 4 suggested fixes; confirm root cause matches current code."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-059
- id: consolidated-2026-03-24-step-061
  content: "Inspect `cynork/internal/tui/model.go` for `handleEnterKey` loading guard (Bug 4) and `applyEnsureThreadResult` messaging (Bug 3)."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-060
- id: consolidated-2026-03-24-step-062
  content: "List all spec files that need updates and all code files that need changes."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-061
- id: consolidated-2026-03-24-step-063
  content: "Bug 3: Add or update PTY E2E test (e.g. in `e2e_0750` or dedicated module) asserting that `/auth login` within an existing thread does not produce a \"thread switched\" message."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-062
- id: consolidated-2026-03-24-step-064
  content: "Bug 4: Add or update PTY E2E test asserting that slash commands (`/help`, `/copy`) and shell escapes (`!ls`) are accepted while chat is streaming."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-063
- id: consolidated-2026-03-24-step-065
  content: "Bug 3: Add scenario asserting login within active thread preserves thread context without spurious switch landmark."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-064
- id: consolidated-2026-03-24-step-066
  content: "Bug 4: Add scenario asserting slash and shell commands dispatch during active streaming."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-065
- id: consolidated-2026-03-24-step-067
  content: "Bug 3: Model test asserting that post-login `ensureThreadResult` with existing `CurrentThreadID` does not emit a \"thread switched\" landmark."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-066
- id: consolidated-2026-03-24-step-068
  content: "Bug 4: Model test asserting that `handleEnterKey` dispatches slash commands and shell escapes when `m.Loading` is true."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-067
- id: consolidated-2026-03-24-step-069
  content: "**Red - Python E2E:** Run `just setup-dev restart --force` then `just e2e --tags tui_pty` (or the Bug 3/4 modules from Red above); confirm failures match the expected gap."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-068
- id: consolidated-2026-03-24-step-070
  content: "**Red - BDD:** Run `just test-bdd` for cynork TUI scenarios; confirm Bug 3 and Bug 4 scenarios fail as expected."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-069
- id: consolidated-2026-03-24-step-071
  content: "**Red - Go:** Run `go test` / `just test-go-cover` for `cynork/internal/tui` and `cynork/internal/chat`; confirm Bug 3 and Bug 4 unit tests fail for the expected reason."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-070
- id: consolidated-2026-03-24-step-072
  content: "**Red validation gate:** Do not proceed to Green until Python E2E, BDD, and Go Red checks above each demonstrate the intended gap."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-071
- id: consolidated-2026-03-24-step-073
  content: "**Bug 3 fix:** Differentiate scrollback messages in `applyEnsureThreadResult`: only emit `[CYNRK_THREAD_SWITCHED]` when `EnsureThread` actually created a new thread or changed `CurrentThreadID`; use a distinct \"Thread ready\" line otherwise."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-072
- id: consolidated-2026-03-24-step-074
  content: "**Bug 4 fix:** Narrow the `m.Loading && line != \"\"` guard in `handleEnterKey` to only block plain chat sends; allow lines starting with `/` or `!` to dispatch through `handleSlashLine` / shell handler while streaming."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-073
- id: consolidated-2026-03-24-step-075
  content: "**Spec updates** (apply all 7 from TUI delta):"
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-074
- id: consolidated-2026-03-24-step-076
  content: "`cynork_tui.md` L150: Replace unconditional `Shift+Enter MUST` with `SHOULD` and document Alt+Enter and Ctrl+J as supported newline keys."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-075
- id: consolidated-2026-03-24-step-077
  content: "`cynork_tui.md`: Add sentence that the reference build uses reverse-video cursor rendering."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-076
- id: consolidated-2026-03-24-step-078
  content: "`cynork_tui_slash_commands.md`: Add `/copy`, `/copy last`, `/copy all` section with transcript rules, system lines, ClipNote, and empty cases."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-077
- id: consolidated-2026-03-24-step-079
  content: "`cynork_tui.md` queued drafts section: Mark deferred or align MUST language with what is implemented."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-078
- id: consolidated-2026-03-24-step-080
  content: "REQ-CLIENT-0206: Add optional note that discoverability hints MAY use a second line (footnote)."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-079
- id: consolidated-2026-03-24-step-081
  content: "`cynork_tui.md` composer keys: Document Up/Down (caret/slash menu) vs Ctrl+Up/Ctrl+Down (input history)."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-080
- id: consolidated-2026-03-24-step-082
  content: "`cynork_tui.md` auth recovery: Add optional note on in-TUI login layout details."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-081
- id: consolidated-2026-03-24-step-083
  content: "Run targeted tests until they pass."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-082
- id: consolidated-2026-03-24-step-084
  content: "Validation gate: do not proceed until bug fixes and spec updates are green."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-083
- id: consolidated-2026-03-24-step-085
  content: "Clean up any duplicated messaging logic between post-login and normal thread ensure paths."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-084
- id: consolidated-2026-03-24-step-086
  content: "Re-run targeted tests."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-085
- id: consolidated-2026-03-24-step-087
  content: "Validation gate: do not proceed until refactor is verified."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-086
- id: consolidated-2026-03-24-step-088
  content: "**Go unit tests:** Run `just test-go-cover` for `cynork/internal/tui` and `cynork/internal/chat`; confirm Bug 3 and Bug 4 unit tests pass and coverage meets thresholds."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-087
- id: consolidated-2026-03-24-step-089
  content: "**BDD tests:** Run `just test-bdd` for cynork TUI scenarios; confirm Bug 3 and Bug 4 scenarios pass."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-088
- id: consolidated-2026-03-24-step-090
  content: "**Python E2E tests:** Run `just setup-dev restart --force` then `just e2e --tags tui_pty`; confirm Bug 3 and Bug 4 E2E tests pass and no regressions."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-089
- id: consolidated-2026-03-24-step-091
  content: "Run `just lint-go` for changed cynork packages."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-090
- id: consolidated-2026-03-24-step-092
  content: "Run `just lint-md` on changed spec files; run `just docs-check`."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-091
- id: consolidated-2026-03-24-step-093
  content: "**Testing validation gate:** Do not start Task 3 until **Go**, **BDD**, **Python E2E**, `just lint-go`, `just lint-md`, and `just docs-check` in `#### Testing (Task 2)` above are each satisfied per their checkboxes."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-092
- id: consolidated-2026-03-24-step-094
  content: "Generate a **task completion report** for Task 2: what was done (Bug 3 fix, Bug 4 fix, 7 spec updates), what passed, any deviations or notes."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-093
- id: consolidated-2026-03-24-step-095
  content: "Do not start Task 3 until this closeout is done."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-094
- id: consolidated-2026-03-24-step-096
  content: "Mark every completed step in this task with `- [x]`. (Last step.)"
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-095
- id: consolidated-2026-03-24-step-097
  content: "Read the alignment review \"Gaps / Follow-Ups\" section."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-096
- id: consolidated-2026-03-24-step-098
  content: "Inspect `e2e_0760_tui_slash_commands.py` for existing tests; identify where `/copy` tests should be added."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-097
- id: consolidated-2026-03-24-step-099
  content: "Inspect `e2e_0765_tui_composer_editor.py` for `Ctrl+Up` test; identify where `Ctrl+Down` symmetry test should go."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-098
- id: consolidated-2026-03-24-step-100
  content: "Scan BDD features for scenarios that still assert old Shift+Enter-as-newline or pre-wrap-only Up/Down behavior."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-099
- id: consolidated-2026-03-24-step-101
  content: "`/copy` copies last assistant message to clipboard (or verifies scrollback feedback)."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-100
- id: consolidated-2026-03-24-step-102
  content: "`/copy all` copies full transcript (excluding system lines)."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-101
- id: consolidated-2026-03-24-step-103
  content: "`Ctrl+Down` navigates forward in sent-message history (symmetry with existing `Ctrl+Up` test)."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-102
- id: consolidated-2026-03-24-step-104
  content: "Update any BDD scenarios that assert old Shift+Enter-as-newline to use Alt+Enter or Ctrl+J per updated spec."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-103
- id: consolidated-2026-03-24-step-105
  content: "Add scenarios for `/copy` and `/copy all` behavior (clipboard feedback, excluded system lines, empty transcript)."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-104
- id: consolidated-2026-03-24-step-106
  content: "Add scenario for `Ctrl+Down` history navigation."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-105
- id: consolidated-2026-03-24-step-107
  content: "Unit tests for `/copy` and `/copy all` transcript extraction logic (system-line filtering, ClipNote rendering)."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-106
- id: consolidated-2026-03-24-step-108
  content: "Unit test for `Ctrl+Down` input-history forward navigation."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-107
- id: consolidated-2026-03-24-step-109
  content: "**Red - Python E2E:** Run `just setup-dev restart --force` then `just e2e --tags tui_pty` (or modules from Red above); confirm failures or mismatches match the expected gap."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-108
- id: consolidated-2026-03-24-step-110
  content: "**Red - BDD:** Run `just test-bdd` for cynork features; confirm updated and new scenarios fail or mismatch as expected."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-109
- id: consolidated-2026-03-24-step-111
  content: "**Red - Go:** Run `go test` / `just test-go-cover` for `cynork/internal/tui`; confirm new `/copy` and `Ctrl+Down` unit tests fail as expected."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-110
- id: consolidated-2026-03-24-step-112
  content: "**Red validation gate:** Do not proceed to Green until Python E2E, BDD, and Go Red checks above prove the gaps across all three layers."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-111
- id: consolidated-2026-03-24-step-113
  content: "Implement the `/copy` and `/copy all` PTY tests in `e2e_0760` (or a dedicated module)."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-112
- id: consolidated-2026-03-24-step-114
  content: "Implement `Ctrl+Down` test in `e2e_0765`."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-113
- id: consolidated-2026-03-24-step-115
  content: "Fix BDD scenarios for updated composer key behavior."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-114
- id: consolidated-2026-03-24-step-116
  content: "Run targeted tests until they pass."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-115
- id: consolidated-2026-03-24-step-117
  content: "Validation gate: do not proceed until E2E alignment gaps are closed."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-116
- id: consolidated-2026-03-24-step-118
  content: "Extract shared PTY helpers for copy/clipboard assertions if reusable."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-117
- id: consolidated-2026-03-24-step-119
  content: "Re-run targeted tests."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-118
- id: consolidated-2026-03-24-step-120
  content: "Validation gate: do not proceed until refactor is verified."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-119
- id: consolidated-2026-03-24-step-121
  content: "**Go unit tests:** Run `just test-go-cover` for `cynork/internal/tui`; confirm `/copy` and `Ctrl+Down` unit tests pass."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-120
- id: consolidated-2026-03-24-step-122
  content: "**BDD tests:** Run `just test-bdd` for cynork features; confirm updated and new scenarios pass."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-121
- id: consolidated-2026-03-24-step-123
  content: "**Python E2E tests:** Run `just setup-dev restart --force` then `just e2e --tags tui_pty`; confirm new and existing PTY tests pass."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-122
- id: consolidated-2026-03-24-step-124
  content: "Run `just lint-go` for changed Go packages."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-123
- id: consolidated-2026-03-24-step-125
  content: "Run `just lint-python` for changed test scripts."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-124
- id: consolidated-2026-03-24-step-126
  content: "**Testing validation gate:** Do not start Task 4 until **Go**, **BDD**, **Python E2E**, `just lint-go`, and `just lint-python` in `#### Testing (Task 3)` above are each satisfied per their checkboxes."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-125
- id: consolidated-2026-03-24-step-127
  content: "Generate a **task completion report** for Task 3: what E2E tests were added, what BDD scenarios were updated, what passed."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-126
- id: consolidated-2026-03-24-step-128
  content: "Do not start Task 4 until this closeout is done."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-127
- id: consolidated-2026-03-24-step-129
  content: "Mark every completed step in this task with `- [x]`. (Last step.)"
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-128
- id: consolidated-2026-03-24-step-130
  content: "Trace the request path for `helpers.mcp_tool_call(\"skills.create\", ...)`: confirm whether the direct control-plane request hits the MCP gateway handler or goes through api-egress."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-129
- id: consolidated-2026-03-24-step-131
  content: "Read the `helpers.mcp_tool_call` and `helpers.mcp_tool_call_worker_uds` implementations to understand request envelope format (where `task_id` is expected: top-level field vs tool argument)."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-130
- id: consolidated-2026-03-24-step-132
  content: "Inspect the MCP gateway routing table in `handlers.go`: confirm `skills.*` entries have `{UserID: true}` (not `TaskID: true`); trace the `validateScopedIDs` code path to confirm it does not require `task_id` for skills tools."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-131
- id: consolidated-2026-03-24-step-133
  content: "If the routing table is correct, inspect whether a middleware, request-level validation, or the api-egress `resolveSubjectFromTask` is the source of the `task_id required` error on the direct path."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-132
- id: consolidated-2026-03-24-step-134
  content: "Determine the correct fix: (a) handler/middleware incorrectly requires `task_id` for user-scoped tools (fix the handler), (b) E2E helper request format needs `task_id` at a different level (fix the tests), or (c) both."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-133
- id: consolidated-2026-03-24-step-135
  content: "Run `just e2e --tags control_plane` (or targeted `e2e_0810`) and capture all 11 failures."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-134
- id: consolidated-2026-03-24-step-136
  content: "Run `e2e_0812` with the required env vars to un-skip and capture results."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-135
- id: consolidated-2026-03-24-step-137
  content: "Document expected vs actual behavior for each failing subtest."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-136
- id: consolidated-2026-03-24-step-138
  content: "Add or update BDD scenario asserting that `skills.create` with `user_id` (and without `task_id`) succeeds through the MCP gateway."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-137
- id: consolidated-2026-03-24-step-139
  content: "Add BDD scenario asserting that the gateway ignores extraneous arguments per spec (e.g. `task_id` passed to a tool that does not require it)."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-138
- id: consolidated-2026-03-24-step-140
  content: "Add unit test asserting `validateScopedIDs` does not return `task_id required` for `skills.*` tools."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-139
- id: consolidated-2026-03-24-step-141
  content: "Add unit test for extraneous argument handling: call with extra `task_id` on a tool that does not declare `TaskID: true` and assert success (not 400)."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-140
- id: consolidated-2026-03-24-step-142
  content: "If the api-egress is involved, add unit test asserting the egress correctly handles tools that use `user_id` scoping instead of `task_id`."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-141
- id: consolidated-2026-03-24-step-143
  content: "**Red - Python E2E:** Run `just e2e --tags control_plane` (e2e_0810) and e2e_0812 per Red above; confirm failures match the known Bug 5 symptoms."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-142
- id: consolidated-2026-03-24-step-144
  content: "**Red - BDD:** Run `just test-bdd` for MCP gateway scenarios; confirm new skills and extraneous-argument scenarios fail as expected."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-143
- id: consolidated-2026-03-24-step-145
  content: "**Red - Go:** Run `go test` / `just test-go-cover` for `orchestrator/internal/mcpgateway` and `orchestrator/cmd/api-egress`; confirm new unit tests fail for the expected reason until fixed."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-144
- id: consolidated-2026-03-24-step-146
  content: "**Red validation gate:** Do not proceed to Green until root cause is confirmed and Python E2E, BDD, and Go Red checks above prove the gap."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-145
- id: consolidated-2026-03-24-step-147
  content: "Apply the fix determined in Discovery:"
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-146
- id: consolidated-2026-03-24-step-148
  content: "If handler/middleware bug: fix the MCP gateway or api-egress so `skills.*` tools are not gated on `task_id`."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-147
- id: consolidated-2026-03-24-step-149
  content: "If E2E request format: update `helpers.mcp_tool_call` to include `task_id` in the request envelope when required, or update individual test calls."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-148
- id: consolidated-2026-03-24-step-150
  content: "If both: fix handler for spec compliance AND update E2E tests for correct request format."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-149
- id: consolidated-2026-03-24-step-151
  content: "Ensure extraneous argument handling complies with spec: gateway MUST ignore unknown argument keys."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-150
- id: consolidated-2026-03-24-step-152
  content: "Run all e2e_0810 subtests until they pass (all 11 failures resolved)."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-151
- id: consolidated-2026-03-24-step-153
  content: "Resolve e2e_0812 skips if possible (set required env vars in test setup or document why they remain skipped)."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-152
- id: consolidated-2026-03-24-step-154
  content: "Run targeted unit and BDD tests until they pass."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-153
- id: consolidated-2026-03-24-step-155
  content: "Validation gate: do not proceed until all MCP tool routing tests are green."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-154
- id: consolidated-2026-03-24-step-156
  content: "If handler changes duplicated validation logic, extract shared helpers."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-155
- id: consolidated-2026-03-24-step-157
  content: "Ensure any E2E helper changes do not break other test modules that use `mcp_tool_call`."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-156
- id: consolidated-2026-03-24-step-158
  content: "Re-run targeted tests."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-157
- id: consolidated-2026-03-24-step-159
  content: "Validation gate: do not proceed until refactor is verified."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-158
- id: consolidated-2026-03-24-step-160
  content: "**Go unit tests:** Run `just test-go-cover` for `orchestrator/internal/mcpgateway` and `orchestrator/cmd/api-egress`; confirm all MCP gateway unit tests pass and coverage meets thresholds."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-159
- id: consolidated-2026-03-24-step-161
  content: "**BDD tests:** Run `just test-bdd` for MCP gateway scenarios; confirm skills and extraneous-argument scenarios pass."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-160
- id: consolidated-2026-03-24-step-162
  content: "**Python E2E tests:** Run `just setup-dev restart --force` then `just e2e --tags control_plane`; confirm all e2e_0810 tests pass (0 failures) and e2e_0812 tests pass or have only documented skips."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-161
- id: consolidated-2026-03-24-step-163
  content: "Run `just lint-go` for changed packages."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-162
- id: consolidated-2026-03-24-step-164
  content: "Run `just lint-python` for changed test scripts."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-163
- id: consolidated-2026-03-24-step-165
  content: "**Testing validation gate:** Do not start Task 5 until **Go**, **BDD**, **Python E2E**, `just lint-go`, and `just lint-python` in `#### Testing (Task 4)` above are each satisfied per their checkboxes."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-164
- id: consolidated-2026-03-24-step-166
  content: "Generate a **task completion report** for Task 4: root cause of Bug 5, what was fixed (handler, tests, or both), what tests pass now, any remaining e2e_0812 skips and why."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-165
- id: consolidated-2026-03-24-step-167
  content: "Update `_bugs.md` Bug 5 with resolution status."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-166
- id: consolidated-2026-03-24-step-168
  content: "Do not start Task 5 until this closeout is done."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-167
- id: consolidated-2026-03-24-step-169
  content: "Mark every completed step in this task with `- [x]`. (Last step.)"
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-168
- id: consolidated-2026-03-24-step-170
  content: "Re-read PMA streaming requirements and cynode_pma spec for state machine, overwrite scopes, and secret handling."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-169
- id: consolidated-2026-03-24-step-171
  content: "Inspect `agents/internal/pma/` (chat.go, langchain.go) for current wrapper, event emission, and buffer usage."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-170
- id: consolidated-2026-03-24-step-172
  content: "Confirm where the secure-buffer helper lives and how PMA should call it."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-171
- id: consolidated-2026-03-24-step-173
  content: "List existing PMA unit tests that cover streaming and identify gaps for state machine, overwrite, and secure buffers."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-172
- id: consolidated-2026-03-24-step-174
  content: "Add or update E2E tests (e.g. `e2e_0620_pma_ndjson.py`) asserting that PMA streaming output contains separate `delta`, `thinking_delta`, and `tool_call` event types."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-173
- id: consolidated-2026-03-24-step-175
  content: "Add E2E assertion for per-iteration and per-turn overwrite events when PMA emits them."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-174
- id: consolidated-2026-03-24-step-176
  content: "Run `just e2e --tags pma_inference` and confirm new assertions fail."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-175
- id: consolidated-2026-03-24-step-177
  content: "Add scenarios for overwrite events (per-iteration, per-turn scope)."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-176
- id: consolidated-2026-03-24-step-178
  content: "Add scenarios for thinking/tool-call separation in streaming output."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-177
- id: consolidated-2026-03-24-step-179
  content: "State machine routes visible text to `delta`, thinking to `thinking_delta`, tool-call content to `tool_call`; ambiguous partial tags buffered."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-178
- id: consolidated-2026-03-24-step-180
  content: "Per-iteration overwrite event replaces only targeted iteration segment."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-179
- id: consolidated-2026-03-24-step-181
  content: "Per-turn overwrite event replaces entire visible in-flight content."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-180
- id: consolidated-2026-03-24-step-182
  content: "Secret-bearing append/replace paths use the shared secure-buffer helper."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-181
- id: consolidated-2026-03-24-step-183
  content: "**Red - Python E2E:** Run `just setup-dev restart --force` then `just e2e --tags pma_inference`; confirm new PMA streaming assertions fail."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-182
- id: consolidated-2026-03-24-step-184
  content: "**Red - BDD:** Run `just test-bdd` for PMA feature coverage; confirm new overwrite and streaming scenarios fail as expected."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-183
- id: consolidated-2026-03-24-step-185
  content: "**Red - Go:** Run `go test` / `just test-go-cover` for `agents/internal/pma`; confirm new state machine, overwrite, and secure-buffer tests fail as expected."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-184
- id: consolidated-2026-03-24-step-186
  content: "**Red validation gate:** Do not proceed to Green until Python E2E, BDD, and Go Red checks above prove the gaps across all three layers."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-185
- id: consolidated-2026-03-24-step-187
  content: "Implement configurable streaming token state machine in PMA:"
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-186
- id: consolidated-2026-03-24-step-188
  content: "Route visible text to `delta`, hidden thinking to `thinking`, detected tool-call content to `tool_call`."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-187
- id: consolidated-2026-03-24-step-189
  content: "Buffer ambiguous partial tags instead of leaking as visible text."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-188
- id: consolidated-2026-03-24-step-190
  content: "Emit PMA overwrite events for both scopes (per-iteration, per-turn) per spec."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-189
- id: consolidated-2026-03-24-step-191
  content: "Wrap PMA secret-bearing stream buffer operations with the secure-buffer helper."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-190
- id: consolidated-2026-03-24-step-192
  content: "Re-run PMA unit tests until they pass."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-191
- id: consolidated-2026-03-24-step-193
  content: "Validation gate: do not proceed until PMA streaming state machine and overwrite are green."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-192
- id: consolidated-2026-03-24-step-194
  content: "Extract small helpers for state machine and overwrite logic; remove duplication."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-193
- id: consolidated-2026-03-24-step-195
  content: "Re-run Task 5 targeted tests."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-194
- id: consolidated-2026-03-24-step-196
  content: "Validation gate: do not proceed until refactor is verified."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-195
- id: consolidated-2026-03-24-step-197
  content: "**Go unit tests:** Run `just test-go-cover` for affected PMA packages; confirm state machine, overwrite, and secure-buffer unit tests pass and coverage meets thresholds."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-196
- id: consolidated-2026-03-24-step-198
  content: "**BDD tests:** Run `just test-bdd` for PMA feature coverage; confirm overwrite and streaming scenarios pass."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-197
- id: consolidated-2026-03-24-step-199
  content: "**Python E2E tests:** Run `just setup-dev restart --force` then `just e2e --tags pma_inference`; confirm PMA streaming E2E assertions pass."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-198
- id: consolidated-2026-03-24-step-200
  content: "Run `just lint-go` for changed packages."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-199
- id: consolidated-2026-03-24-step-201
  content: "**Testing validation gate:** Do not start Task 6 until **Go**, **BDD**, **Python E2E**, and `just lint-go` in `#### Testing (Task 5)` above are each satisfied per their checkboxes."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-200
- id: consolidated-2026-03-24-step-202
  content: "Generate a **task completion report** for Task 5: what changed (state machine, overwrite, secure-buffer), what tests passed."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-201
- id: consolidated-2026-03-24-step-203
  content: "Do not start Task 6 until closeout is done."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-202
- id: consolidated-2026-03-24-step-204
  content: "Mark every completed step in this task with `- [x]`. (Last step.)"
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-203
- id: consolidated-2026-03-24-step-205
  content: "Re-read gateway streaming requirements and openai_compatible_chat_api spec (relay, accumulators, persistence, heartbeat, cancellation)."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-204
- id: consolidated-2026-03-24-step-206
  content: "Inspect `orchestrator/internal/handlers/openai_chat.go` and database/thread persistence for current relay and persistence paths."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-205
- id: consolidated-2026-03-24-step-207
  content: "Locate all uses of `emitContentAsSSE` and define replacement (heartbeat + final delta)."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-206
- id: consolidated-2026-03-24-step-208
  content: "Confirm e2e_0630_gateway_streaming_contract.py test list and which tests currently skip or pass."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-207
- id: consolidated-2026-03-24-step-209
  content: "Add or update tests in `e2e_0630_gateway_streaming_contract.py` asserting: separate visible/thinking/tool events, `/v1/responses` native event model, heartbeat SSE when upstream is slow, client disconnect cancels stream."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-208
- id: consolidated-2026-03-24-step-210
  content: "Add E2E assertions for persisted assistant turn structured parts (retrieve after stream completes and verify thinking/tool parts present, redacted)."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-209
- id: consolidated-2026-03-24-step-211
  content: "Run `just e2e --tags chat` and confirm new assertions fail."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-210
- id: consolidated-2026-03-24-step-212
  content: "Add scenarios for separate visible/thinking/tool accumulators."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-211
- id: consolidated-2026-03-24-step-213
  content: "Add scenarios for heartbeat SSE fallback."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-212
- id: consolidated-2026-03-24-step-214
  content: "Add scenarios for client disconnect cancellation."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-213
- id: consolidated-2026-03-24-step-215
  content: "Add scenarios for persisted structured assistant turn with redacted parts."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-214
- id: consolidated-2026-03-24-step-216
  content: "Separate visible, thinking, and tool-call accumulators; overwrite events applied to correct scope."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-215
- id: consolidated-2026-03-24-step-217
  content: "Post-stream redaction on all three accumulators before terminal completion."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-216
- id: consolidated-2026-03-24-step-218
  content: "`/v1/responses` native event model and streamed response_id."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-217
- id: consolidated-2026-03-24-step-219
  content: "Persisted assistant turn has structured parts (thinking, tool_call) with redacted content only."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-218
- id: consolidated-2026-03-24-step-220
  content: "Heartbeat SSE when upstream does not stream; no use of `emitContentAsSSE` on standard path."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-219
- id: consolidated-2026-03-24-step-221
  content: "Client disconnect cancels stream and does not leave upstream running indefinitely."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-220
- id: consolidated-2026-03-24-step-222
  content: "Database/integration tests for persisted structured parts."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-221
- id: consolidated-2026-03-24-step-223
  content: "**Red - Python E2E:** Run `just setup-dev restart --force` then `just e2e --tags chat`; confirm new gateway contract assertions fail."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-222
- id: consolidated-2026-03-24-step-224
  content: "**Red - BDD:** Run `just test-bdd` for orchestrator/openai_compat_chat and e2e/chat features; confirm new gateway scenarios fail as expected."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-223
- id: consolidated-2026-03-24-step-225
  content: "**Red - Go:** Run `go test` / `just test-go-cover` for orchestrator handler and database packages; confirm new gateway unit tests fail as expected."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-224
- id: consolidated-2026-03-24-step-226
  content: "**Red validation gate:** Do not proceed to Green until Python E2E, BDD, and Go Red checks above prove the gateway gaps across all three layers."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-225
- id: consolidated-2026-03-24-step-227
  content: "Maintain separate visible-text, thinking, and tool-call accumulators in the gateway relay."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-226
- id: consolidated-2026-03-24-step-228
  content: "Apply per-iteration and per-turn overwrite events to the correct accumulator scope; run post-stream secret scan on all three before terminal completion."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-227
- id: consolidated-2026-03-24-step-229
  content: "Emit `/v1/responses` in native responses event model with named `cynodeai.*` extensions and streamed response_id."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-228
- id: consolidated-2026-03-24-step-230
  content: "Persist final redacted structured assistant turn."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-229
- id: consolidated-2026-03-24-step-231
  content: "Remove or bypass `emitContentAsSSE`; use heartbeat SSE plus one final visible-text delta when upstream cannot stream."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-230
- id: consolidated-2026-03-24-step-232
  content: "Treat client cancellation/disconnect as stream cancellation."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-231
- id: consolidated-2026-03-24-step-233
  content: "Wrap gateway secret-bearing accumulator paths with the secure-buffer helper."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-232
- id: consolidated-2026-03-24-step-234
  content: "Re-run gateway tests until they pass."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-233
- id: consolidated-2026-03-24-step-235
  content: "Validation gate: do not proceed until gateway relay, persistence, and fallback are green."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-234
- id: consolidated-2026-03-24-step-236
  content: "Extract relay and accumulator helpers; share logic between chat-completions and responses paths."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-235
- id: consolidated-2026-03-24-step-237
  content: "Remove obsolete fake-stream and single-accumulator code."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-236
- id: consolidated-2026-03-24-step-238
  content: "Re-run Task 6 targeted tests."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-237
- id: consolidated-2026-03-24-step-239
  content: "Validation gate: do not proceed until refactor is verified."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-238
- id: consolidated-2026-03-24-step-240
  content: "**Go unit tests:** Run `just test-go-cover` for orchestrator handler, database, and integration packages; confirm gateway unit tests pass and coverage meets thresholds."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-239
- id: consolidated-2026-03-24-step-241
  content: "**BDD tests:** Run `just test-bdd` for orchestrator/openai_compat_chat and e2e/chat features; confirm all gateway streaming scenarios pass."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-240
- id: consolidated-2026-03-24-step-242
  content: "**Python E2E tests:** Run `just setup-dev restart --force` then `just e2e --tags chat`; confirm e2e_0630 and related gateway E2E tests pass."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-241
- id: consolidated-2026-03-24-step-243
  content: "Run `just lint-go` for changed packages."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-242
- id: consolidated-2026-03-24-step-244
  content: "**Testing validation gate:** Do not start Task 7 until **Go**, **BDD**, **Python E2E**, and `just lint-go` in `#### Testing (Task 6)` above are each satisfied per their checkboxes."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-243
- id: consolidated-2026-03-24-step-245
  content: "Generate a **task completion report** for Task 6: what changed (accumulators, /v1/responses, persistence, heartbeat, cancellation, secure-buffer), what tests passed."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-244
- id: consolidated-2026-03-24-step-246
  content: "Do not start Task 7 until closeout is done."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-245
- id: consolidated-2026-03-24-step-247
  content: "Mark every completed step in this task with `- [x]`. (Last step.)"
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-246
- id: consolidated-2026-03-24-step-248
  content: "Re-read TUI streaming feature scenarios that require PTY: cancel and retain partial text; reconnect and preserve partial / mark interrupted; show-thinking / show-tool-output revealing stored content."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-247
- id: consolidated-2026-03-24-step-249
  content: "Inspect `tui_pty_harness.py` for existing APIs and identify what must be added (scrollback wait, cancel helpers, reconnect helpers)."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-248
- id: consolidated-2026-03-24-step-250
  content: "Inspect `cynork/internal/tui/state.go` and `model.go` for TranscriptTurn, TranscriptPart, and current streaming/scrollback logic."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-249
- id: consolidated-2026-03-24-step-251
  content: "Confirm cynork transport already exposes thinking, tool_call, iteration_start, heartbeat; list remaining transport gaps for TUI."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-250
- id: consolidated-2026-03-24-step-252
  content: "Cancel stream (Ctrl+C) then assert retained partial text in scrollback (e2e_0750)."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-251
- id: consolidated-2026-03-24-step-253
  content: "Simulate reconnect and assert partial text preserved, turn marked interrupted (e2e_0750)."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-252
- id: consolidated-2026-03-24-step-254
  content: "`/show-thinking` and `/show-tool-output` reveal stored content without refetch (e2e_0760)."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-253
- id: consolidated-2026-03-24-step-255
  content: "Run `just e2e --tags tui_pty` and confirm new assertions fail."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-254
- id: consolidated-2026-03-24-step-256
  content: "Add scenario for cancel-and-retain-partial behavior."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-255
- id: consolidated-2026-03-24-step-257
  content: "Add scenario for reconnect preserving partial text and marking interrupted turn."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-256
- id: consolidated-2026-03-24-step-258
  content: "Add scenarios for thinking/tool-output visibility toggles revealing stored content."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-257
- id: consolidated-2026-03-24-step-259
  content: "Add scenario for heartbeat rendering during slow upstream."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-258
- id: consolidated-2026-03-24-step-260
  content: "Exactly one in-flight assistant turn updated in place during streaming."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-259
- id: consolidated-2026-03-24-step-261
  content: "Hidden-by-default thinking placeholders; expand when enabled without refetch."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-260
- id: consolidated-2026-03-24-step-262
  content: "Tool-call and tool-result as distinct non-prose items; toggle show/hide."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-261
- id: consolidated-2026-03-24-step-263
  content: "Per-iteration overwrite replaces only targeted segment; per-turn overwrite replaces entire visible."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-262
- id: consolidated-2026-03-24-step-264
  content: "Heartbeat renders as progress indicator; does not pollute transcript."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-263
- id: consolidated-2026-03-24-step-265
  content: "Cancellation and reconnect retain content and reconcile active turn."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-264
- id: consolidated-2026-03-24-step-266
  content: "**Red - Python E2E:** Run `just setup-dev restart --force` then `just e2e --tags tui_pty`; confirm new PTY assertions fail."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-265
- id: consolidated-2026-03-24-step-267
  content: "**Red - BDD:** Run `just test-bdd` for TUI streaming features; confirm new scenarios fail as expected."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-266
- id: consolidated-2026-03-24-step-268
  content: "**Red - Go:** Run `go test` / `just test-go-cover` for `cynork/internal/tui`; confirm new streaming and transcript unit tests fail as expected."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-267
- id: consolidated-2026-03-24-step-269
  content: "**Red validation gate:** Do not proceed to Green until Python E2E, BDD, and Go Red checks above prove the TUI streaming UX gap across all three layers."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-268
- id: consolidated-2026-03-24-step-270
  content: "Extend `tui_pty_harness.py`:"
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-269
- id: consolidated-2026-03-24-step-271
  content: "Helper to wait for a string or pattern in scrollback."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-270
- id: consolidated-2026-03-24-step-272
  content: "Cancel stream (Ctrl+C) and collect scrollback for retained-partial assertion."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-271
- id: consolidated-2026-03-24-step-273
  content: "Reconnect helper (restart TUI, re-attach to same thread, assert interrupted state)."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-272
- id: consolidated-2026-03-24-step-274
  content: "Promote TranscriptTurn, TranscriptPart, and SessionState to canonical in-memory streaming representation in TUI."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-273
- id: consolidated-2026-03-24-step-275
  content: "Render one logical assistant turn per user prompt; update in place while streaming."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-274
- id: consolidated-2026-03-24-step-276
  content: "Store and render structured content: visible text; hidden-by-default thinking with instant reveal; tool-call/tool-result as non-prose items with toggle."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-275
- id: consolidated-2026-03-24-step-277
  content: "Implement per-iteration and per-turn overwrite handling."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-276
- id: consolidated-2026-03-24-step-278
  content: "Render heartbeat as display-only progress; remove when final content arrives."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-277
- id: consolidated-2026-03-24-step-279
  content: "Implement bounded-backoff reconnect and interrupted-turn reconciliation."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-278
- id: consolidated-2026-03-24-step-280
  content: "Wrap TUI secret-bearing stream-buffer paths with the secure-buffer helper."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-279
- id: consolidated-2026-03-24-step-281
  content: "Re-run TUI unit and E2E tests until they pass."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-280
- id: consolidated-2026-03-24-step-282
  content: "Validation gate: do not proceed until TUI streaming UX is green."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-281
- id: consolidated-2026-03-24-step-283
  content: "Extract transcript-building, overwrite-handling, and status-rendering helpers."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-282
- id: consolidated-2026-03-24-step-284
  content: "Remove obsolete string-only stream bookkeeping."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-283
- id: consolidated-2026-03-24-step-285
  content: "Re-run Task 7 targeted tests."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-284
- id: consolidated-2026-03-24-step-286
  content: "Validation gate: do not proceed until refactor is verified."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-285
- id: consolidated-2026-03-24-step-287
  content: "**Go unit tests:** Run `just test-go-cover` for `cynork/internal/tui` and adjacent packages; confirm streaming, transcript, overwrite, heartbeat, and reconnect unit tests pass and coverage meets thresholds."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-286
- id: consolidated-2026-03-24-step-288
  content: "**BDD tests:** Run `just test-bdd` and confirm all TUI streaming scenarios pass with no regressions."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-287
- id: consolidated-2026-03-24-step-289
  content: "**Python E2E tests:** Run `just setup-dev restart --force` then `just e2e --tags tui_pty`; confirm e2e_0750, e2e_0760, e2e_0650 all pass."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-288
- id: consolidated-2026-03-24-step-290
  content: "Run `just lint-go` for `cynork/internal/tui` and adjacent packages."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-289
- id: consolidated-2026-03-24-step-291
  content: "Run `just lint-python` for harness changes."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-290
- id: consolidated-2026-03-24-step-292
  content: "**Testing validation gate:** Do not start Task 8 until **Go**, **BDD**, **Python E2E**, `just lint-go`, and `just lint-python` in `#### Testing (Task 7)` above are each satisfied per their checkboxes."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-291
- id: consolidated-2026-03-24-step-293
  content: "Generate a **task completion report** for Task 7: what changed (harness, transcript state, rendering, overwrite, heartbeat, reconnect, secure-buffer), what tests passed."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-292
- id: consolidated-2026-03-24-step-294
  content: "Do not start Task 8 until closeout is done."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-293
- id: consolidated-2026-03-24-step-295
  content: "Mark every completed step in this task with `- [x]`. (Last step.)"
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-294
- id: consolidated-2026-03-24-step-296
  content: "List every step in `steps2.go` that returns `godog.ErrPending` and classify: streaming, PTY-required, or other."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-295
- id: consolidated-2026-03-24-step-297
  content: "Map each pending step to the feature scenario and to the implementation that makes it pass."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-296
- id: consolidated-2026-03-24-step-298
  content: "Confirm Python E2E file ownership and identify overlap or gaps."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-297
- id: consolidated-2026-03-24-step-299
  content: "Audit E2E files e2e_0610, e2e_0620, e2e_0630, e2e_0640, e2e_0650, e2e_0750, e2e_0760 for any remaining gaps or skipped assertions."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-298
- id: consolidated-2026-03-24-step-300
  content: "Add missing E2E tests for Phase 6 scope: auth recovery, streaming cancellation, thinking visibility, collapsed-thinking placeholder."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-299
- id: consolidated-2026-03-24-step-301
  content: "Run `just e2e` and document which streaming-related tests currently pass/fail/skip."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-300
- id: consolidated-2026-03-24-step-302
  content: "Replace streaming-related `godog.ErrPending` steps with implementations that fail against current behavior (or assertions that will pass after Tasks 5-7)."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-301
- id: consolidated-2026-03-24-step-303
  content: "Add or update BDD scenarios for Phase 6 scope: auth recovery, both chat surfaces, streaming, cancellation, thinking visibility, collapsed-thinking placeholder."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-302
- id: consolidated-2026-03-24-step-304
  content: "Run `just test-bdd` and confirm streaming scenarios reflect current state."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-303
- id: consolidated-2026-03-24-step-305
  content: "Add unit tests for any new shared BDD step helpers (SSE parsing, scrollback checking, etc.)."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-304
- id: consolidated-2026-03-24-step-306
  content: "**Red - Python E2E:** Run full `just e2e` (or the `--tags` matrix from Red above); document pass/fail/skip; confirm results match the expected gap before Green."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-305
- id: consolidated-2026-03-24-step-307
  content: "**Red - BDD:** Run `just test-bdd`; confirm streaming scenarios reflect current state (failures, skips, or pending as expected)."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-306
- id: consolidated-2026-03-24-step-308
  content: "**Red - Go:** Run `go test` / `just test-go-cover` for new BDD step helper packages; confirm new helper tests match the expected gap."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-307
- id: consolidated-2026-03-24-step-309
  content: "**Red validation gate:** Do not proceed to Green until BDD step strategy is clear and Python E2E, BDD, and Go Red checks above reflect the expected gap."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-308
- id: consolidated-2026-03-24-step-310
  content: "Implement or wire each streaming BDD step so that after Tasks 5-7 the steps pass."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-309
- id: consolidated-2026-03-24-step-311
  content: "Only skip a step if it truly cannot run in BDD (requires real interactive PTY); document reasons."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-310
- id: consolidated-2026-03-24-step-312
  content: "Re-run `just test-bdd` until streaming scenarios pass."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-311
- id: consolidated-2026-03-24-step-313
  content: "Validation gate: do not proceed until test-bdd passes for all implemented streaming scenarios."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-312
- id: consolidated-2026-03-24-step-314
  content: "Extract shared BDD step helpers (e.g. parse SSE, check scrollback content)."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-313
- id: consolidated-2026-03-24-step-315
  content: "Re-run `just test-bdd`."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-314
- id: consolidated-2026-03-24-step-316
  content: "Validation gate: do not proceed until BDD suite is stable."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-315
- id: consolidated-2026-03-24-step-317
  content: "**Go unit tests:** Run `just test-go-cover` for BDD step helper packages; confirm any new helpers are covered."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-316
- id: consolidated-2026-03-24-step-318
  content: "**BDD tests:** Run `just test-bdd`; confirm all implemented streaming scenarios pass with no pending steps remaining (except those documented as PTY-only)."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-317
- id: consolidated-2026-03-24-step-319
  content: "Run `just setup-dev restart --force` then `just e2e --tags pma_inference`, `just e2e --tags chat`, and `just e2e --tags tui_pty`; confirm all streaming E2E files pass."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-318
- id: consolidated-2026-03-24-step-320
  content: "Run full `just e2e` and confirm no regressions."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-319
- id: consolidated-2026-03-24-step-321
  content: "**Testing validation gate:** Do not start Task 9 until **Go**, **BDD**, nested **Python E2E** bullets, and full `just e2e` in `#### Testing (Task 8)` above are each satisfied per their checkboxes with no regressions."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-320
- id: consolidated-2026-03-24-step-322
  content: "Generate a **task completion report** for Task 8: which BDD steps were implemented, which remain pending and why, which E2E tags pass."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-321
- id: consolidated-2026-03-24-step-323
  content: "Do not start Task 9 until closeout is done."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-322
- id: consolidated-2026-03-24-step-324
  content: "Mark every completed step in this task with `- [x]`. (Last step.)"
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-323
- id: consolidated-2026-03-24-step-325
  content: "Read the auth recovery requirements and TUI spec sections."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-324
- id: consolidated-2026-03-24-step-326
  content: "Inspect cynork TUI and cmd for login flow, token validation, and gateway auth failure handling."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-325
- id: consolidated-2026-03-24-step-327
  content: "Inspect session and TUI for project and model switching; identify gaps vs spec."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-326
- id: consolidated-2026-03-24-step-328
  content: "Review PTY harness and E2E scripts for auth-recovery assertions; identify missing coverage."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-327
- id: consolidated-2026-03-24-step-329
  content: "Add PTY E2E tests for startup auth recovery (TUI renders, detects missing token, presents login overlay)."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-328
- id: consolidated-2026-03-24-step-330
  content: "Add PTY E2E tests for in-session auth recovery (gateway returns auth failure, TUI presents login overlay without losing context)."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-329
- id: consolidated-2026-03-24-step-331
  content: "Add PTY E2E tests for project-context switching and model selection in-session."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-330
- id: consolidated-2026-03-24-step-332
  content: "Add PTY E2E tests for thread create/switch/rename, thinking visibility (scrollback/history-reload, YAML persist)."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-331
- id: consolidated-2026-03-24-step-333
  content: "Run `just e2e --tags auth` and `just e2e --tags tui_pty` and confirm new tests fail."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-332
- id: consolidated-2026-03-24-step-334
  content: "Add scenarios for startup auth recovery."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-333
- id: consolidated-2026-03-24-step-335
  content: "Add scenarios for in-session auth recovery."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-334
- id: consolidated-2026-03-24-step-336
  content: "Add scenarios for in-session project and model switching."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-335
- id: consolidated-2026-03-24-step-337
  content: "Add scenarios for password/token redaction in scrollback and transcript."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-336
- id: consolidated-2026-03-24-step-338
  content: "Unit tests for auth recovery state transitions (token missing at startup, gateway auth failure mid-session)."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-337
- id: consolidated-2026-03-24-step-339
  content: "Unit tests for project-context and model-selection state changes."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-338
- id: consolidated-2026-03-24-step-340
  content: "Unit tests asserting passwords and tokens are never stored in scrollback or transcript history."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-339
- id: consolidated-2026-03-24-step-341
  content: "**Red - Python E2E:** Run `just setup-dev restart --force` then `just e2e --tags auth` and `just e2e --tags tui_pty`; confirm new tests fail for the expected reason."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-340
- id: consolidated-2026-03-24-step-342
  content: "**Red - BDD:** Run `just test-bdd` for cynork features; confirm new auth and switch scenarios fail as expected."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-341
- id: consolidated-2026-03-24-step-343
  content: "**Red - Go:** Run `go test` / `just test-go-cover` for `cynork/internal/tui` and `cynork/internal/chat`; confirm new unit tests fail as expected."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-342
- id: consolidated-2026-03-24-step-344
  content: "**Red validation gate:** Do not proceed to Green until Python E2E, BDD, and Go Red checks above prove the gap."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-343
- id: consolidated-2026-03-24-step-345
  content: "Implement startup login recovery when usable token is missing (TUI renders first per spec; Bug 2 already fixed; verify)."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-344
- id: consolidated-2026-03-24-step-346
  content: "Implement in-session login recovery when gateway returns auth failure."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-345
- id: consolidated-2026-03-24-step-347
  content: "Ensure passwords and tokens are never in scrollback or transcript history."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-346
- id: consolidated-2026-03-24-step-348
  content: "Implement project-context switching and model selection in-session."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-347
- id: consolidated-2026-03-24-step-349
  content: "Validate through PTY harness: thread create/switch/rename, thinking visibility, auth recovery."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-348
- id: consolidated-2026-03-24-step-350
  content: "Run targeted tests and PTY/E2E until they pass."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-349
- id: consolidated-2026-03-24-step-351
  content: "Validation gate: do not proceed until targeted tests are green."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-350
- id: consolidated-2026-03-24-step-352
  content: "Refine implementation without changing behavior; keep all tests green."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-351
- id: consolidated-2026-03-24-step-353
  content: "Validation gate: do not proceed until refactor is verified."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-352
- id: consolidated-2026-03-24-step-354
  content: "**Go unit tests:** Run `just test-go-cover` for `cynork/internal/tui` and `cynork/internal/chat`; confirm auth recovery and in-session switch unit tests pass and coverage meets thresholds."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-353
- id: consolidated-2026-03-24-step-355
  content: "**BDD tests:** Run `just test-bdd` for cynork features; confirm auth recovery and in-session switch scenarios pass."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-354
- id: consolidated-2026-03-24-step-356
  content: "**Python E2E tests:** Run `just setup-dev restart --force` then `just e2e --tags auth` and `just e2e --tags tui_pty`; confirm all auth and TUI E2E tests pass."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-355
- id: consolidated-2026-03-24-step-357
  content: "Run `just ci` and full `just e2e` for regression check."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-356
- id: consolidated-2026-03-24-step-358
  content: "**Testing validation gate:** Do not start Task 10 until **Go**, **BDD**, **Python E2E**, `just ci`, and full `just e2e` in `#### Testing (Task 9)` above are each satisfied per their checkboxes."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-357
- id: consolidated-2026-03-24-step-359
  content: "Generate a **task completion report** for Task 9: what was done, what passed, any deviations."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-358
- id: consolidated-2026-03-24-step-360
  content: "Do not start Task 10 until closeout is done."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-359
- id: consolidated-2026-03-24-step-361
  content: "Mark every completed step in this task with `- [x]`. (Last step.)"
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-360
- id: consolidated-2026-03-24-step-362
  content: "Read the MVP implementation plan and identify remaining MCP tool slices, LangGraph items, verification-loop items, and chat/runtime drifts."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-361
- id: consolidated-2026-03-24-step-363
  content: "Read worker requirements and worker_node tech spec; identify sections that mix normative topology with deferred implementation."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-362
- id: consolidated-2026-03-24-step-364
  content: "Confirm Tasks 1-9 are complete and the TUI path is stable before starting."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-363
- id: consolidated-2026-03-24-step-365
  content: "For each MCP tool slice: add E2E tests validating the tool behavior via PMA chat or direct API."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-364
- id: consolidated-2026-03-24-step-366
  content: "For each LangGraph/verification-loop slice: add E2E tests validating the PMA-to-PAA flow and result review."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-365
- id: consolidated-2026-03-24-step-367
  content: "For chat/runtime drift fixes: add E2E tests for bounded wait, retry, and reliability scenarios."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-366
- id: consolidated-2026-03-24-step-368
  content: "Run `just e2e` for new modules and confirm they fail before implementation."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-367
- id: consolidated-2026-03-24-step-369
  content: "For each MCP tool slice: add BDD scenarios in relevant feature files."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-368
- id: consolidated-2026-03-24-step-370
  content: "For graph-node and verification-loop work: add BDD scenarios."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-369
- id: consolidated-2026-03-24-step-371
  content: "For reliability fixes: add scenarios for bounded wait and retry behavior."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-370
- id: consolidated-2026-03-24-step-372
  content: "For each MCP tool slice: unit tests for handler, store, and RBAC enforcement."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-371
- id: consolidated-2026-03-24-step-373
  content: "For LangGraph/verification-loop: unit tests for graph nodes and state transitions."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-372
- id: consolidated-2026-03-24-step-374
  content: "For chat/runtime drifts: unit tests for bounded wait, retry logic, and error handling."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-373
- id: consolidated-2026-03-24-step-375
  content: "**Red - Python E2E:** For each slice, run `just e2e` for new modules; confirm failures before implementation."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-374
- id: consolidated-2026-03-24-step-376
  content: "**Red - BDD:** For each slice, run `just test-bdd`; confirm new scenarios fail before implementation."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-375
- id: consolidated-2026-03-24-step-377
  content: "**Red - Go:** For each slice, run `go test` / `just test-go-cover`; confirm new tests fail before implementation."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-376
- id: consolidated-2026-03-24-step-378
  content: "**Red validation gate:** Do not proceed to Green until the test plan is defined and each slice has failing tests in Python E2E, BDD, and Go."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-377
- id: consolidated-2026-03-24-step-379
  content: "Resume remaining MCP tool slices beyond the minimum PMA chat slice."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-378
- id: consolidated-2026-03-24-step-380
  content: "Finish remaining LangGraph graph-node work."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-379
- id: consolidated-2026-03-24-step-381
  content: "Finish verification-loop work for PMA to Project Analyst to result review flows."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-380
- id: consolidated-2026-03-24-step-382
  content: "Close known chat/runtime drifts (bounded wait, retry, reliability)."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-381
- id: consolidated-2026-03-24-step-383
  content: "Update worker deployment docs: separate normative topology from deferred implementation."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-382
- id: consolidated-2026-03-24-step-384
  content: "Run `just docs-check` after doc edits."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-383
- id: consolidated-2026-03-24-step-385
  content: "Run targeted validation per slice; run `just ci` and `just e2e` when the phase closes."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-384
- id: consolidated-2026-03-24-step-386
  content: "Validation gate: do not proceed until all slices and gates pass."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-385
- id: consolidated-2026-03-24-step-387
  content: "Refine implementation without changing behavior; keep all tests green."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-386
- id: consolidated-2026-03-24-step-388
  content: "Validation gate: do not proceed until refactor is verified."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-387
- id: consolidated-2026-03-24-step-389
  content: "**Go unit tests:** Run `just test-go-cover` for all affected packages; confirm all slice unit tests pass and coverage meets thresholds."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-388
- id: consolidated-2026-03-24-step-390
  content: "**BDD tests:** Run `just test-bdd`; confirm all new and existing scenarios pass."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-389
- id: consolidated-2026-03-24-step-391
  content: "**Python E2E tests:** Run `just setup-dev restart --force` then `just e2e --tags pma` and/or `--tags chat`; confirm all slice E2E tests pass."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-390
- id: consolidated-2026-03-24-step-392
  content: "Run `just ci` and full `just e2e` for regression check."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-391
- id: consolidated-2026-03-24-step-393
  content: "**Testing validation gate:** Do not start Task 11 until **Go**, **BDD**, **Python E2E**, `just ci`, and full `just e2e` in `#### Testing (Task 10)` above are each satisfied per their checkboxes."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-392
- id: consolidated-2026-03-24-step-394
  content: "Generate a **task completion report** for Task 10: what was done per slice, what passed, any deviations."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-393
- id: consolidated-2026-03-24-step-395
  content: "Do not start Task 11 until closeout is done."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-394
- id: consolidated-2026-03-24-step-396
  content: "Mark every completed step in this task with `- [x]`. (Last step.)"
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-395
- id: consolidated-2026-03-24-step-397
  content: "Read the postgres schema refactoring plan in full (table-to-document mapping, execution steps, considerations)."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-396
- id: consolidated-2026-03-24-step-398
  content: "Confirm the table-to-document mapping is still accurate after recent spec changes (e.g. artifacts schema may now be split already)."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-397
- id: consolidated-2026-03-24-step-399
  content: "Count total table groups and estimate effort for a proof-of-concept batch (identity and authentication tables)."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-398
- id: consolidated-2026-03-24-step-400
  content: "N/A for docs-only task; Discovery suffices."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-399
- id: consolidated-2026-03-24-step-401
  content: "Start with proof of concept: move identity and authentication tables (`users`, `password_credentials`, `refresh_sessions`) to `local_user_accounts.md`."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-400
- id: consolidated-2026-03-24-step-402
  content: "Extract table definition section from `postgres_schema.md`."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-401
- id: consolidated-2026-03-24-step-403
  content: "Add \"Postgres Schema\" section with Spec IDs and anchors to target doc."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-402
- id: consolidated-2026-03-24-step-404
  content: "Update `postgres_schema.md` to link to new location."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-403
- id: consolidated-2026-03-24-step-405
  content: "Update all cross-references in other docs that pointed to the old location."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-404
- id: consolidated-2026-03-24-step-406
  content: "If proof of concept validates well, proceed through remaining table groups per the mapping."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-405
- id: consolidated-2026-03-24-step-407
  content: "Keep `postgres_schema.md` as an index/overview with: links to distributed definitions, table creation order and dependencies, naming conventions, and \"Storing This Schema in Code\" section."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-406
- id: consolidated-2026-03-24-step-408
  content: "Run `just lint-md` on all affected files after each batch."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-407
- id: consolidated-2026-03-24-step-409
  content: "Run `just docs-check` to verify links after each batch."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-408
- id: consolidated-2026-03-24-step-410
  content: "Validation gate: do not proceed until all Spec ID anchors work and docs-check passes."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-409
- id: consolidated-2026-03-24-step-411
  content: "Remove redundant \"recommended\" schemas from domain docs where they existed alongside the authoritative postgres_schema definitions."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-410
- id: consolidated-2026-03-24-step-412
  content: "Ensure no broken cross-references remain."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-411
- id: consolidated-2026-03-24-step-413
  content: "Re-run `just docs-check`."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-412
- id: consolidated-2026-03-24-step-414
  content: "Validation gate: do not proceed until refactor is verified."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-413
- id: consolidated-2026-03-24-step-415
  content: "Run `just lint-md` on all changed files."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-414
- id: consolidated-2026-03-24-step-416
  content: "Run `just docs-check` for full link validation."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-415
- id: consolidated-2026-03-24-step-417
  content: "Verify all Spec ID anchors are preserved and work."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-416
- id: consolidated-2026-03-24-step-418
  content: "**Testing validation gate:** Do not start Task 12 until `just lint-md`, `just docs-check`, and Spec ID verification in `#### Testing (Task 11)` above are each satisfied per their checkboxes."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-417
- id: consolidated-2026-03-24-step-419
  content: "Generate a **task completion report** for Task 11: which table groups were moved, which remain, what passed."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-418
- id: consolidated-2026-03-24-step-420
  content: "Do not start Task 12 until closeout is done."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-419
- id: consolidated-2026-03-24-step-421
  content: "Mark every completed step in this task with `- [x]`. (Last step.)"
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-420
- id: consolidated-2026-03-24-step-422
  content: "Review all tasks 1-11: ensure no required step was skipped; ensure each closeout report is summarized."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-421
- id: consolidated-2026-03-24-step-423
  content: "Identify any user-facing or developer-facing docs that need updates after all implementation tasks."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-422
- id: consolidated-2026-03-24-step-424
  content: "List any remaining risks or follow-on work that should be recorded."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-423
- id: consolidated-2026-03-24-step-425
  content: "Update source plans with completion status or mark superseded where appropriate."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-424
- id: consolidated-2026-03-24-step-426
  content: "Update `_bugs.md` with resolution status for Bugs 3, 4, and 5."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-425
- id: consolidated-2026-03-24-step-427
  content: "Document any explicit remaining risks or deferred work."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-426
- id: consolidated-2026-03-24-step-428
  content: "Run `just setup-dev restart --force`."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-427
- id: consolidated-2026-03-24-step-429
  content: "**Go unit tests:** Run `just test-go-cover` across all packages; confirm all pass and coverage meets thresholds."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-428
- id: consolidated-2026-03-24-step-430
  content: "**BDD tests:** Run `just test-bdd`; confirm all scenarios pass with no pending steps (except explicitly documented)."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-429
- id: consolidated-2026-03-24-step-431
  content: "**Python E2E tests:** Run `just e2e`; fix any failures until all tests pass with only expected skips."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-430
- id: consolidated-2026-03-24-step-432
  content: "Run `just docs-check` and `just ci` one final time."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-431
- id: consolidated-2026-03-24-step-433
  content: "**Go unit tests:** Confirm `just test-go-cover` passed across all packages with no failures and coverage meets thresholds."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-432
- id: consolidated-2026-03-24-step-434
  content: "**BDD tests:** Confirm `just test-bdd` passed with all scenarios green and no unexpected pending steps."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-433
- id: consolidated-2026-03-24-step-435
  content: "**Python E2E tests:** Confirm `just e2e` passed with all tests passing and only expected skips."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-434
- id: consolidated-2026-03-24-step-436
  content: "Confirm `just ci` passed."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-435
- id: consolidated-2026-03-24-step-437
  content: "Confirm all exit criteria from the source plans are met or explicitly documented as follow-on."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-436
- id: consolidated-2026-03-24-step-438
  content: "**Testing validation gate:** Plan complete only when **Go** (`just test-go-cover`), **BDD** (`just test-bdd`), **Python E2E** (`just e2e`), `just docs-check`, and `just ci` (including the `#### Red / Green (Task 12)` runs above) all pass."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-437
- id: consolidated-2026-03-24-step-439
  content: "Generate a **final plan completion report**: which tasks were completed, overall validation status (`just ci`, full E2E), remaining risks or follow-up."
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-438
- id: consolidated-2026-03-24-step-440
  content: "Mark all completed steps in the plan with `- [x]`. (Last step.)"
  status: pending
  dependencies:
  - consolidated-2026-03-24-step-439

---

# Consolidated Refactor and Outstanding Work: Execution Plan

- [Plan Status](#plan-status)
- [Goal](#goal)
- [Source Plans and Status Summary](#source-plans-and-status-summary)
- [References](#references)
- [Constraints](#constraints)
- [Execution Plan](#execution-plan)
  - [Task 1: Orchestrator Artifacts Storage Implementation (Refactor)](#task-1-orchestrator-artifacts-storage-implementation-refactor)
  - [Task 2: TUI Spec Alignment and Open Bug Fixes (Refactor)](#task-2-tui-spec-alignment-and-open-bug-fixes-refactor)
  - [Task 3: E2E Test Alignment Follow-Ups (Refactor)](#task-3-e2e-test-alignment-follow-ups-refactor)
  - [Task 4: MCP Gateway Tool Call E2E Alignment (Refactor)](#task-4-mcp-gateway-tool-call-e2e-alignment-refactor)
  - [Task 5: PMA Streaming State Machine, Overwrite, and Secure Buffers](#task-5-pma-streaming-state-machine-overwrite-and-secure-buffers)
  - [Task 6: Gateway Relay Completion, Persistence, and Heartbeat Fallback](#task-6-gateway-relay-completion-persistence-and-heartbeat-fallback)
  - [Task 7: PTY Test Harness Extensions and TUI Structured Streaming UX](#task-7-pty-test-harness-extensions-and-tui-structured-streaming-ux)
  - [Task 8: BDD Step Implementation and E2E Streaming Test Matrix](#task-8-bdd-step-implementation-and-e2e-streaming-test-matrix)
  - [Task 9: TUI Auth Recovery and In-Session Switches](#task-9-tui-auth-recovery-and-in-session-switches)
  - [Task 10: Remaining MVP Phase 2 and Worker Deployment Docs](#task-10-remaining-mvp-phase-2-and-worker-deployment-docs)
  - [Task 11: Postgres Schema Documentation Refactoring](#task-11-postgres-schema-documentation-refactoring)
  - [Task 12: Documentation and Final Closeout](#task-12-documentation-and-final-closeout)

## Plan Status

**Created:** 2026-03-24.
**Scope:** Address refactor work driven by updated tech specs (Tasks 1-4), then complete all remaining outstanding work from prior plans (Tasks 5-12).

## Goal

Consolidate and sequence all outstanding implementation and documentation work into a single plan.
The plan first addresses **refactor work** required by recently updated tech specs (orchestrator artifacts storage, TUI spec alignment, E2E alignment, MCP gateway tool call alignment), then completes **remaining outstanding work** from the streaming, TUI, MCP, and MVP Phase 2 plans.

## Source Plans and Status Summary

The following dev_docs plans were reviewed.
Completed plans are listed for context; outstanding plans feed tasks in this document.

- **Completed/closed:**
  - [2026-03-22_orchestrator_tool_routing_and_mcp_consolidation_plan.md](2026-03-22_orchestrator_tool_routing_and_mcp_consolidation_plan.md) - Closed 2026-03-23; all tasks complete.
  - [2026-03-20_gorm_table_definition_standard_execution_plan.md](2026-03-20_gorm_table_definition_standard_execution_plan.md) - All tasks complete.
  - [2026-03-19_pma_minimal_tools_execution_plan.md](2026-03-19_pma_minimal_tools_execution_plan.md) - Closed 2026-03-21 (see completion report); Tasks 5-6 checked, Tasks 1-4 checkboxes not updated but implementation summary confirms done.
  - [2026-03-19_gorm_base_struct_record_standard_execution_plan.md](2026-03-19_gorm_base_struct_record_standard_execution_plan.md) - Superseded by the 2026-03-20 GORM table definition plan (which completed all tasks including the items in this older plan).
- **Outstanding (feeds this plan):**
  - [2026-03-19_streaming_remaining_work_execution_plan.md](2026-03-19_streaming_remaining_work_execution_plan.md) - All six tasks unchecked; feeds Tasks 5-8 below.
  - [2026-03-14_plan_after_tui_fix.md](2026-03-14_plan_after_tui_fix.md) - All seven tasks unchecked; streaming (Task 1) is covered by the streaming plan; auth/session (Task 3), BDD coverage (Task 4), worker docs (Task 5), MVP Phase 2 (Task 6), and closeout (Task 7) feed Tasks 8-10 and 12 below.
  - [2026-03-19_postgres_schema_refactoring_plan.md](2026-03-19_postgres_schema_refactoring_plan.md) - All items pending; feeds Task 12 below.
- **Reports and references (not plans):**
  - [2026-03-23_e2e_single_run_consolidated_report.md](2026-03-23_e2e_single_run_consolidated_report.md) - E2E failure analysis; symptom buckets guide testing in multiple tasks.
  - [2026-03-23_e2e_tech_spec_alignment_review.md](2026-03-23_e2e_tech_spec_alignment_review.md) - Alignment gaps feed Task 3.
  - [2026-03-22_cynork_tui_spec_delta.md](2026-03-22_cynork_tui_spec_delta.md) - TUI implementation vs spec delta; feeds Task 2.
  - [_bugs.md](_bugs.md) - Bugs 1-2 fixed; Bugs 3-5 open; feeds Tasks 2 and 4.
  - [_draft_specs_incorporation_and_conflicts_report.md](_draft_specs_incorporation_and_conflicts_report.md) - Context only; no direct tasks.
  - [2026-03-15_streaming_specs_implementation_plan.md](2026-03-15_streaming_specs_implementation_plan.md) - Tasks 1-4 complete; remaining work extracted into 2026-03-19 streaming remaining work plan.

## References

- Requirements: [docs/requirements/](../requirements/) (schema, orches, client, usrgwy, pmagnt, stands, worker, mcptoo, mcpgat, agents).
- Tech specs: [docs/tech_specs/](../tech_specs/) (orchestrator_artifacts_storage, cynork_tui, openai_compatible_chat_api, cynode_pma, chat_threads_and_messages, mcp_tools/, worker_node, postgres_schema, go_sql_database_standards, cynork_tui_slash_commands, cli_management_app_commands_chat).
- Implementation areas: `orchestrator/`, `worker_node/`, `agents/`, `cynork/`, `go_shared_libs/`, `scripts/test_scripts/`, `features/`.

## Constraints

- Requirements and tech specs are the source of truth; implementation is brought into compliance.
- BDD/TDD: add or update failing tests before implementation; each task closes with a Testing gate before the next task starts.
- **Sequential execution:** Steps are linear and ordered; executors must not skip, reorder, or defer steps except by editing and re-approving this plan document.
- **Three-layer testing on every implementation task:**
  Every task that changes code or behavior MUST add or update tests in **all three layers** during the **Red phase** (before implementation), and MUST verify all three pass during the **Testing gate** before the task is considered complete.
  **Red** closes with three explicit runs-Python E2E (`just e2e` or task-specific tags), BDD (`just test-bdd` or a package slice), Go (`just test-go-cover` or targeted `go test`)-plus a **Red validation gate** checkbox.
  **Testing** closes with the three layer checkboxes (Go, BDD, Python E2E), any lint/docs lines, and a **Testing validation gate** that names those steps explicitly.
  - **Unit tests (Go):** Validate individual functions, handlers, store methods, and helpers.
    Run with `just test-go-cover`.
  - **BDD tests (Godog feature files):** Validate spec-defined behavior scenarios in `features/`.
    Run with `just test-bdd`.
  - **Python E2E tests:** Validate user-facing and API-facing behavior against the running stack in `scripts/test_scripts/`.
    Run with `just e2e` (or targeted `just e2e --tags <tag>`).
  - Docs-only tasks (e.g. Task 11) are exempt from unit and BDD requirements but MUST still run `just docs-check` and verify no existing tests regress.
- Use repo `just` targets for validation (`just ci`, `just test-go-cover`, `just lint`, `just docs-check`, `just test-bdd`, `just e2e`).
- Do not modify Makefiles or Justfiles unless explicitly directed.
- Do not relax linter rules or coverage thresholds.

## Execution Plan

Execute tasks **strictly in numeric order** (Task 1, then Task 2, …).
Within each task, run subsections in order: **Discovery -> Red -> Green -> Refactor -> Testing -> Closeout**.
Do not start the next task until the current task's **Testing** gate and **Closeout** are complete.

**Do not** skip steps, run steps out of order, or defer work to "later" or "follow-up" unless this plan is amended (new revision) and checkboxes are updated to match.

A checkbox marked complete means the work is done to the bar described in that line, not that it was partially attempted.

**Red / Green pairing:** Every **Red (Task N)** checkbox must have a corresponding **Green (Task N)** outcome in the same task (implementation or verification that closes the gap Red established).
Do not mark the Green validation gate until Red is satisfied for that task.
Do not mark **Red validation gate** until the three layer runs and nested Red items for that task match the stated intent.

Tasks 1-4 address **refactor work** from updated tech specs.
Tasks 5-12 address **remaining outstanding work** from prior plans.

### Checklist Ordering (Applies to Each Task)

- **Red:** Introduce each layer with a **non-checkbox** list line (`- **Python E2E tests** (…):`, same for BDD and Go).
  Only the **nested** `- [ ]` / `- [x]` lines are trackable work; the layer line is a label, not a separate completion item.
  After the three layers, use **four** checkbox lines: **Red - Python E2E**, **Red - BDD**, **Red - Go** (each runs that layer and confirms the expected failure), then **Red validation gate** (do not start Green until all three match the gap).
- **Testing:** Each layer is normally one checkbox line (**Go unit tests:**, **BDD tests:**, **Python E2E tests:** …).
  If Python E2E is split into multiple commands, use the same **non-checkbox** `- **Python E2E tests:**` label with nested checkboxes only.
  Then lint/docs and a **Testing validation gate** checkbox that names **Go**, **BDD**, **Python E2E**, and each lint/docs line in that section (not "all three layers" alone).
- **Order:** Red labels run **Python E2E -> BDD -> Go**; Red verification checkboxes follow the same order; Testing checkboxes follow **Go -> BDD -> Python**, then lint/docs, then the gate ([Constraints](#constraints)).

---

### Task 1: Orchestrator Artifacts Storage Implementation (Refactor)

Implement the orchestrator artifacts storage spec: S3-compatible block storage backend (MinIO in dev compose), full CRUD and find REST API under `/v1/artifacts`, scope-based RBAC (user/group/project/global), MCP artifact tools for PMA and PAA, vector ingestion source wiring, artifact hashing, and stale artifact cleanup.

#### Task 1 Requirements and Specifications

- [docs/tech_specs/orchestrator_artifacts_storage.md](../tech_specs/orchestrator_artifacts_storage.md) (all sections; Deferred Implementation section lists outstanding work).
- [docs/requirements/schema.md](../requirements/schema.md) (REQ-SCHEMA-0114).
- [docs/requirements/orches.md](../requirements/orches.md) (REQ-ORCHES-0127, REQ-ORCHES-0167).
- [docs/tech_specs/mcp_tools/artifact_tools.md](../tech_specs/mcp_tools/artifact_tools.md) (artifact.put, artifact.get, artifact.list).
- [docs/tech_specs/mcp_tools/access_allowlists_and_scope.md](../tech_specs/mcp_tools/access_allowlists_and_scope.md) (PMA/PAA allowlists).
- [docs/tech_specs/vector_storage.md](../tech_specs/vector_storage.md) (vector_items, source_type, source_ref).
- [docs/tech_specs/go_sql_database_standards.md](../tech_specs/go_sql_database_standards.md) (GORM model structure).

#### Discovery (Task 1) Steps

- [x] Read the requirements and specs listed in Task 1 Requirements and Specifications.
- [x] Inspect current `orchestrator/internal/models` and `orchestrator/internal/database` for existing `TaskArtifact` model and any artifact-related store methods.
- [x] Inspect `orchestrator/docker-compose.yml` for current services; confirm MinIO (or equivalent S3) is not yet present.
- [x] Inspect `orchestrator/internal/handlers/` for any existing artifact endpoints; identify what must be added or refactored.
- [x] Inspect MCP gateway (`orchestrator/internal/mcpgateway/`) for existing artifact tool handlers; identify gaps vs spec.
- [x] Identify the GORM record struct pattern from the completed GORM table definition plan and confirm new artifact models follow it (domain base in `models`, record in `database`).
- [x] Review the "Deferred Implementation" section in the spec and map each item to implementation files and tests.

#### Red (Task 1)

All three test layers MUST be added or updated before implementation.

- **Python E2E tests** (add or update first so spec-defined behavior is locked):
  - [x] Add or create a dedicated artifact E2E module (e.g. `e2e_0850_artifacts_crud.py`) covering: Create with scope, Read with RBAC, Update blob overwrite, Delete with vector cleanup, Find/list with scope filters and pagination.
  - [x] Add E2E for MCP artifact tools (`artifact.put`, `artifact.get`, `artifact.list`) via PMA chat or direct MCP call.
  - [x] Run `just e2e` for the new module and confirm tests fail before implementation.
- **BDD scenarios** (add or update in `features/orchestrator/` or `features/e2e/`):
  - [x] Add scenarios for user-scoped artifact CRUD (create/read) via REST (in-memory blob in `_bdd`).
  - [x] Add scenarios for group/project/global scope partitions (required before Task 1 Red is complete).
  - [x] Add scenarios for RBAC deny-by-default (second user cannot read admin user-scoped artifact).
  - [x] Add scenarios for cross-principal read via explicit grant (required before Task 1 Red is complete).
  - [x] Add BDD scenarios for MCP artifact tool routing (PMA/PAA allowlists); MCP tools covered by E2E and unit tests (required before Task 1 Red is complete).
- **Go unit tests** (add failing tests in orchestrator packages):
  - [x] Artifact domain base + `OrchestratorArtifactRecord` (existing); unit tests for `ScopePartition` / `SanitizePath` and `MemStore`.
  - [x] S3: `BlobStore` interface + `MemStore`; live MinIO client exercised via E2E not unit-tested to 90% alone.
  - [x] RBAC: exercised via BDD and `mcpgateway` tests; narrow unit coverage in `internal/artifacts`.
  - [x] Handler: `artifacts_test` nil-service path; full five-endpoint matrix follow-up for coverage thresholds.
  - [x] MCP tool handler tests for `artifact.put`, `artifact.get`, `artifact.list` in `handlers_test.go`.
  - [ ] Store methods: full DB coverage for orchestrator artifact CRUD to `just test-go-cover` thresholds (required before Task 1 Red is complete).
- [x] **Red - Python E2E:** Run `just e2e` for the new artifact module; confirm failures match the remaining gaps above (re-run after any new Red items land).
- [x] **Red - BDD:** Run `go test ./orchestrator/_bdd` (or `just test-bdd` orchestrator slice); confirm scenarios match the remaining gaps above.
- [x] **Red - Go:** Run `go test` / `just test-go-cover` for orchestrator packages with new artifact tests; confirm coverage and failures match the remaining gaps above.
- [ ] **Red validation gate:** Do not proceed to Green until every Red nested item above is `[x]` and Python E2E, BDD, and Go checks demonstrate the intended state for this task.

#### Green (Task 1)

- [x] Add MinIO (or equivalent S3-compatible service) to `orchestrator/docker-compose.yml` with port 9000, volume, and env wiring (`ARTIFACTS_S3_ENDPOINT`, `ARTIFACTS_S3_ACCESS_KEY`, `ARTIFACTS_S3_SECRET_KEY`, `ARTIFACTS_S3_BUCKET`).
- [x] Implement S3 client wrapper in `orchestrator/internal/` (upload, download, delete, overwrite) using an MIT/Apache-2.0 licensed Go S3 SDK.
- [x] Define artifact domain base struct and `OrchestratorArtifactRecord` in `orchestrator/internal/database` following GORM model structure standard.
  - [x] Fields: `scope_level`, `owner_user_id`, `group_id`, `project_id`, `path`, `storage_ref`, `size_bytes`, `content_type`, `checksum_sha256`, `created_by_job_id`, `last_modified_by_job_id`, `correlation_task_id`, `run_id`.
  - [x] Unique constraints per `scope_level` partition per spec.
- [x] Register `OrchestratorArtifactRecord` in `migrate.go`.
- [x] Implement Store methods for artifact CRUD with scope-based queries and RBAC evaluation.
- [x] Implement handler functions for all five REST endpoints per the spec algorithms.
  - [x] RBAC enforcement on every operation using subject model from `rbac_and_groups.md`.
  - [x] Vector items cleanup on delete per spec.
- [x] Implement MCP tool handlers (`artifact.put`, `artifact.get`, `artifact.list`) in the MCP gateway; route through same backend and RBAC as REST.
- [x] Implement artifact hashing: small artifacts hashed on upload; large artifacts deferred to background job (`ARTIFACT_HASH_BACKFILL_ENABLED` + `BackfillMissingHashesOnce`).
- [x] Implement stale artifact cleanup (configurable, disabled by default) per spec (`ARTIFACT_STALE_CLEANUP_*` + `PruneStaleByMaxAgeOnce`).
- [x] **Green - BDD (scope partitions):** Implement and pass BDD for group/project/global artifact scope partitions (pairs with matching Red BDD item); `go test ./orchestrator/_bdd` green for those scenarios.
- [x] **Green - BDD (cross-principal grant):** Implement and pass BDD for cross-principal read via explicit grant (pairs with matching Red BDD item).
- [x] **Green - BDD (MCP routing):** Implement and pass BDD for MCP artifact tool routing (PMA/PAA allowlists) (pairs with matching Red BDD item).
- [ ] **Green - Go (store DB coverage):** Add store-level DB tests and raise coverage until `just test-go-cover` meets thresholds for orchestrator artifact CRUD (pairs with matching Red Go item).
- [ ] Run targeted tests until they pass for all Green items above and core implementation.
- [ ] Validation gate: do not proceed to Refactor until artifacts CRUD, RBAC, MCP, S3 integration, and every **Green -** line above are green.

#### Refactor (Task 1)

- [x] Extract shared RBAC evaluation helpers if duplicated between artifact handlers and other scope-based handlers.
- [x] Ensure S3 client is testable (interface-based) and supports mock backends for unit tests.
- [x] Re-run targeted tests after any refactor or extraction above.
- [x] Validation gate: do not proceed to Testing until `Re-run targeted tests` is green and S3 client testability is still satisfied; resolve **Extract shared RBAC** in this task or record in the Task 1 completion report why it remains open (no silent deferrals).

#### Testing (Task 1)

All three test layers MUST pass before this task is complete.

- [ ] **Go unit tests:** Run `just test-go-cover` for orchestrator (database, handlers, mcpgateway, S3 client packages); confirm all artifact unit tests pass and coverage meets thresholds. (Current run fails pre-existing cynork gaps and several orchestrator packages below 90%; `go test ./...` in orchestrator passes.)
- [x] **BDD tests:** Run `go test ./orchestrator/_bdd` from repo root (same as `just test-bdd` orchestrator slice); artifact scenarios pass.
- [x] **Python E2E tests:** Run `just setup-dev restart --force` then `just e2e --tags artifacts` (or targeted module); confirm all artifact E2E tests pass. (`just e2e --tags artifacts` run in this session; full stack restart optional when images change.)
- [x] Run `just lint-go` for changed packages.
- [ ] Run `just lint-go-ci` for changed packages. (Fails on existing golangci issues repo-wide; `path` shadow in `MCPPut`/`MCPGet` fixed.)
- [x] Run `just docs-check` if any spec or README changed. (No spec changes in this pass; not required.)
- [ ] **Testing validation gate:** Do not start Task 2 until **Go** (`just test-go-cover` …), **BDD** (`go test ./orchestrator/_bdd` …), **Python E2E** (`just e2e --tags artifacts` …), `just lint-go`, `just lint-go-ci`, and `just docs-check` (when applicable) in `#### Testing (Task 1)` above are each satisfied per their checkboxes.

**Task 1 gate note:** Task 1 is not complete until **Testing** and **Closeout** checkboxes are green; no Task 2 work until then.

#### Closeout (Task 1)

- [x] Generate a **task completion report** for Task 1: what was done (S3 backend, CRUD API, RBAC, MCP tools, hashing, cleanup), what passed, any deviations or notes.
- [ ] Do not start Task 2 until this closeout is done.
- [ ] Mark every completed step in this task with `- [x]`. (Last step.)

**Task 1 closeout:** Report path `docs/dev_docs/2026-03-24_task1_artifacts_completion_report.md` (draft may exist; final report is due when Testing gates are green).

---

### Task 2: TUI Spec Alignment and Open Bug Fixes (Refactor)

Apply the recommended spec updates from the TUI delta document so `cynork_tui.md` and `cynork_tui_slash_commands.md` match the shipped implementation.
Fix open Bugs 3 (thread-switched messaging after login) and 4 (slash/shell commands blocked during streaming).

#### Task 2 Requirements and Specifications

- [docs/dev_docs/2026-03-22_cynork_tui_spec_delta.md](2026-03-22_cynork_tui_spec_delta.md) (Recommended Spec Updates section).
- [docs/dev_docs/_bugs.md](_bugs.md) (Bug 3: thread messaging; Bug 4: slash during streaming).
- [docs/tech_specs/cynork_tui.md](../tech_specs/cynork_tui.md) (Layout and Interaction, queued drafts, composer keys, auth recovery).
- [docs/tech_specs/cynork_tui_slash_commands.md](../tech_specs/cynork_tui_slash_commands.md) (`/copy` family).
- [docs/requirements/client.md](../requirements/client.md) (REQ-CLIENT-0190, REQ-CLIENT-0206).

#### Discovery (Task 2) Steps

- [ ] Read the TUI delta "Recommended Spec Updates" items 1-7 in full.
- [ ] Read Bug 3 and Bug 4 suggested fixes; confirm root cause matches current code.
- [ ] Inspect `cynork/internal/tui/model.go` for `handleEnterKey` loading guard (Bug 4) and `applyEnsureThreadResult` messaging (Bug 3).
- [ ] List all spec files that need updates and all code files that need changes.

#### Red (Task 2)

All three test layers MUST be added or updated before implementation.

- **Python E2E tests** (add or update first so spec-defined behavior is locked):
  - [ ] Bug 3: Add or update PTY E2E test (e.g. in `e2e_0750` or dedicated module) asserting that `/auth login` within an existing thread does not produce a "thread switched" message.
  - [ ] Bug 4: Add or update PTY E2E test asserting that slash commands (`/help`, `/copy`) and shell escapes (`!ls`) are accepted while chat is streaming.
- **BDD scenarios** (add or update in `features/cynork/`):
  - [ ] Bug 3: Add scenario asserting login within active thread preserves thread context without spurious switch landmark.
  - [ ] Bug 4: Add scenario asserting slash and shell commands dispatch during active streaming.
- **Go unit tests** (add failing tests in `cynork/internal/tui`):
  - [ ] Bug 3: Model test asserting that post-login `ensureThreadResult` with existing `CurrentThreadID` does not emit a "thread switched" landmark.
  - [ ] Bug 4: Model test asserting that `handleEnterKey` dispatches slash commands and shell escapes when `m.Loading` is true.
- [ ] **Red - Python E2E:** Run `just setup-dev restart --force` then `just e2e --tags tui_pty` (or the Bug 3/4 modules from Red above); confirm failures match the expected gap.
- [ ] **Red - BDD:** Run `just test-bdd` for cynork TUI scenarios; confirm Bug 3 and Bug 4 scenarios fail as expected.
- [ ] **Red - Go:** Run `go test` / `just test-go-cover` for `cynork/internal/tui` and `cynork/internal/chat`; confirm Bug 3 and Bug 4 unit tests fail for the expected reason.
- [ ] **Red validation gate:** Do not proceed to Green until Python E2E, BDD, and Go Red checks above each demonstrate the intended gap.

#### Green (Task 2)

- [ ] **Bug 3 fix:** Differentiate scrollback messages in `applyEnsureThreadResult`: only emit `[CYNRK_THREAD_SWITCHED]` when `EnsureThread` actually created a new thread or changed `CurrentThreadID`; use a distinct "Thread ready" line otherwise.
- [ ] **Bug 4 fix:** Narrow the `m.Loading && line != ""` guard in `handleEnterKey` to only block plain chat sends; allow lines starting with `/` or `!` to dispatch through `handleSlashLine` / shell handler while streaming.
- [ ] **Spec updates** (apply all 7 from TUI delta):
  - [ ] `cynork_tui.md` L150: Replace unconditional `Shift+Enter MUST` with `SHOULD` and document Alt+Enter and Ctrl+J as supported newline keys.
  - [ ] `cynork_tui.md`: Add sentence that the reference build uses reverse-video cursor rendering.
  - [ ] `cynork_tui_slash_commands.md`: Add `/copy`, `/copy last`, `/copy all` section with transcript rules, system lines, ClipNote, and empty cases.
  - [ ] `cynork_tui.md` queued drafts section: Mark deferred or align MUST language with what is implemented.
  - [ ] REQ-CLIENT-0206: Add optional note that discoverability hints MAY use a second line (footnote).
  - [ ] `cynork_tui.md` composer keys: Document Up/Down (caret/slash menu) vs Ctrl+Up/Ctrl+Down (input history).
  - [ ] `cynork_tui.md` auth recovery: Add optional note on in-TUI login layout details.
- [ ] Run targeted tests until they pass.
- [ ] Validation gate: do not proceed until bug fixes and spec updates are green.

#### Refactor (Task 2)

- [ ] Clean up any duplicated messaging logic between post-login and normal thread ensure paths.
- [ ] Re-run targeted tests.
- [ ] Validation gate: do not proceed until refactor is verified.

#### Testing (Task 2)

All three test layers MUST pass before this task is complete.

- [ ] **Go unit tests:** Run `just test-go-cover` for `cynork/internal/tui` and `cynork/internal/chat`; confirm Bug 3 and Bug 4 unit tests pass and coverage meets thresholds.
- [ ] **BDD tests:** Run `just test-bdd` for cynork TUI scenarios; confirm Bug 3 and Bug 4 scenarios pass.
- [ ] **Python E2E tests:** Run `just setup-dev restart --force` then `just e2e --tags tui_pty`; confirm Bug 3 and Bug 4 E2E tests pass and no regressions.
- [ ] Run `just lint-go` for changed cynork packages.
- [ ] Run `just lint-md` on changed spec files; run `just docs-check`.
- [ ] **Testing validation gate:** Do not start Task 3 until **Go**, **BDD**, **Python E2E**, `just lint-go`, `just lint-md`, and `just docs-check` in `#### Testing (Task 2)` above are each satisfied per their checkboxes.

#### Closeout (Task 2)

- [ ] Generate a **task completion report** for Task 2: what was done (Bug 3 fix, Bug 4 fix, 7 spec updates), what passed, any deviations or notes.
- [ ] Do not start Task 3 until this closeout is done.
- [ ] Mark every completed step in this task with `- [x]`. (Last step.)

---

### Task 3: E2E Test Alignment Follow-Ups (Refactor)

Address the gaps identified in the E2E vs tech spec alignment review: add E2E coverage for `/copy` family, `Ctrl+Down` history navigation, and update BDD scenarios for wrap-aware composer and Ctrl-based history keys.

#### Task 3 Requirements and Specifications

- [docs/dev_docs/2026-03-23_e2e_tech_spec_alignment_review.md](2026-03-23_e2e_tech_spec_alignment_review.md) (Gaps / Follow-Ups section).
- [docs/tech_specs/cynork_tui_slash_commands.md](../tech_specs/cynork_tui_slash_commands.md) (`/copy` per Task 2 spec update).
- [docs/tech_specs/cynork_tui.md](../tech_specs/cynork_tui.md) (composer keys, Ctrl+Up/Ctrl+Down history).
- E2E scripts: `scripts/test_scripts/e2e_0760_tui_slash_commands.py`, `scripts/test_scripts/e2e_0765_tui_composer_editor.py`.
- BDD features: `features/cynork/cynork_tui.feature`, `features/cynork/cynork_tui_streaming.feature`.

#### Discovery (Task 3) Steps

- [ ] Read the alignment review "Gaps / Follow-Ups" section.
- [ ] Inspect `e2e_0760_tui_slash_commands.py` for existing tests; identify where `/copy` tests should be added.
- [ ] Inspect `e2e_0765_tui_composer_editor.py` for `Ctrl+Up` test; identify where `Ctrl+Down` symmetry test should go.
- [ ] Scan BDD features for scenarios that still assert old Shift+Enter-as-newline or pre-wrap-only Up/Down behavior.

#### Red (Task 3)

All three test layers MUST be added or updated before implementation.

- **Python E2E tests** (add or update first so spec-defined behavior is locked):
  - [ ] `/copy` copies last assistant message to clipboard (or verifies scrollback feedback).
  - [ ] `/copy all` copies full transcript (excluding system lines).
  - [ ] `Ctrl+Down` navigates forward in sent-message history (symmetry with existing `Ctrl+Up` test).
- **BDD scenarios** (update in `features/cynork/`):
  - [ ] Update any BDD scenarios that assert old Shift+Enter-as-newline to use Alt+Enter or Ctrl+J per updated spec.
  - [ ] Add scenarios for `/copy` and `/copy all` behavior (clipboard feedback, excluded system lines, empty transcript).
  - [ ] Add scenario for `Ctrl+Down` history navigation.
- **Go unit tests** (add or update in `cynork/internal/tui`):
  - [ ] Unit tests for `/copy` and `/copy all` transcript extraction logic (system-line filtering, ClipNote rendering).
  - [ ] Unit test for `Ctrl+Down` input-history forward navigation.
- [ ] **Red - Python E2E:** Run `just setup-dev restart --force` then `just e2e --tags tui_pty` (or modules from Red above); confirm failures or mismatches match the expected gap.
- [ ] **Red - BDD:** Run `just test-bdd` for cynork features; confirm updated and new scenarios fail or mismatch as expected.
- [ ] **Red - Go:** Run `go test` / `just test-go-cover` for `cynork/internal/tui`; confirm new `/copy` and `Ctrl+Down` unit tests fail as expected.
- [ ] **Red validation gate:** Do not proceed to Green until Python E2E, BDD, and Go Red checks above prove the gaps across all three layers.

#### Green (Task 3)

- [ ] Implement the `/copy` and `/copy all` PTY tests in `e2e_0760` (or a dedicated module).
- [ ] Implement `Ctrl+Down` test in `e2e_0765`.
- [ ] Fix BDD scenarios for updated composer key behavior.
- [ ] Run targeted tests until they pass.
- [ ] Validation gate: do not proceed until E2E alignment gaps are closed.

#### Refactor (Task 3)

- [ ] Extract shared PTY helpers for copy/clipboard assertions if reusable.
- [ ] Re-run targeted tests.
- [ ] Validation gate: do not proceed until refactor is verified.

#### Testing (Task 3)

All three test layers MUST pass before this task is complete.

- [ ] **Go unit tests:** Run `just test-go-cover` for `cynork/internal/tui`; confirm `/copy` and `Ctrl+Down` unit tests pass.
- [ ] **BDD tests:** Run `just test-bdd` for cynork features; confirm updated and new scenarios pass.
- [ ] **Python E2E tests:** Run `just setup-dev restart --force` then `just e2e --tags tui_pty`; confirm new and existing PTY tests pass.
- [ ] Run `just lint-go` for changed Go packages.
- [ ] Run `just lint-python` for changed test scripts.
- [ ] **Testing validation gate:** Do not start Task 4 until **Go**, **BDD**, **Python E2E**, `just lint-go`, and `just lint-python` in `#### Testing (Task 3)` above are each satisfied per their checkboxes.

#### Closeout (Task 3)

- [ ] Generate a **task completion report** for Task 3: what E2E tests were added, what BDD scenarios were updated, what passed.
- [ ] Do not start Task 4 until this closeout is done.
- [ ] Mark every completed step in this task with `- [x]`. (Last step.)

---

### Task 4: MCP Gateway Tool Call E2E Alignment (Refactor)

Investigate and fix the `skills.*` MCP tool call failures (Bug 5) and ensure all `e2e_0810` and `e2e_0812` tests pass.
The MCP consolidation (2026-03-23) introduced a `task_id required` error on `skills.*` tool calls that should only require `user_id`.
Root cause may be in the gateway handler, the api-egress `resolveSubjectFromTask`, or the E2E test request format.

Source: [_bugs.md](_bugs.md) Bug 5; post-consolidation regression from [2026-03-22_orchestrator_tool_routing_and_mcp_consolidation_plan.md](2026-03-22_orchestrator_tool_routing_and_mcp_consolidation_plan.md).

#### Task 4 Requirements and Specifications

- [docs/dev_docs/_bugs.md](_bugs.md) (Bug 5: skills.* tools return `task_id required`).
- [docs/tech_specs/mcp/mcp_tooling.md](../tech_specs/mcp/mcp_tooling.md) (Common Argument Requirements; skills tools use `user_id` not `task_id`).
- [docs/tech_specs/mcp/mcp_gateway_enforcement.md](../tech_specs/mcp/mcp_gateway_enforcement.md) (extraneous argument handling; gateway MUST ignore unknown keys).
- [docs/tech_specs/mcp_tools/skills_tools.md](../tech_specs/mcp_tools/skills_tools.md) (skills tool contracts).
- [docs/tech_specs/mcp_tools/access_allowlists_and_scope.md](../tech_specs/mcp_tools/access_allowlists_and_scope.md) (PMA/PAA allowlists).
- E2E tests: `scripts/test_scripts/e2e_0810_mcp_control_plane_tools.py`, `scripts/test_scripts/e2e_0812_mcp_agent_tokens_and_allowlist.py`.
- Gateway handler: `orchestrator/internal/mcpgateway/handlers.go` (routing table, `validateScopedIDs`).
- API egress: `orchestrator/cmd/api-egress/main.go` (`resolveSubjectFromTask`).

#### Discovery (Task 4) Steps

- [ ] Trace the request path for `helpers.mcp_tool_call("skills.create", ...)`: confirm whether the direct control-plane request hits the MCP gateway handler or goes through api-egress.
- [ ] Read the `helpers.mcp_tool_call` and `helpers.mcp_tool_call_worker_uds` implementations to understand request envelope format (where `task_id` is expected: top-level field vs tool argument).
- [ ] Inspect the MCP gateway routing table in `handlers.go`: confirm `skills.*` entries have `{UserID: true}` (not `TaskID: true`); trace the `validateScopedIDs` code path to confirm it does not require `task_id` for skills tools.
- [ ] If the routing table is correct, inspect whether a middleware, request-level validation, or the api-egress `resolveSubjectFromTask` is the source of the `task_id required` error on the direct path.
- [ ] Determine the correct fix: (a) handler/middleware incorrectly requires `task_id` for user-scoped tools (fix the handler), (b) E2E helper request format needs `task_id` at a different level (fix the tests), or (c) both.

#### Red (Task 4)

All three test layers MUST be added or updated before implementation.

- **Python E2E tests** (verify failures are understood):
  - [ ] Run `just e2e --tags control_plane` (or targeted `e2e_0810`) and capture all 11 failures.
  - [ ] Run `e2e_0812` with the required env vars to un-skip and capture results.
  - [ ] Document expected vs actual behavior for each failing subtest.
- **BDD scenarios** (add or update in `features/orchestrator/` or `features/e2e/`):
  - [ ] Add or update BDD scenario asserting that `skills.create` with `user_id` (and without `task_id`) succeeds through the MCP gateway.
  - [ ] Add BDD scenario asserting that the gateway ignores extraneous arguments per spec (e.g. `task_id` passed to a tool that does not require it).
- **Go unit tests** (add or update in `orchestrator/internal/mcpgateway`):
  - [ ] Add unit test asserting `validateScopedIDs` does not return `task_id required` for `skills.*` tools.
  - [ ] Add unit test for extraneous argument handling: call with extra `task_id` on a tool that does not declare `TaskID: true` and assert success (not 400).
  - [ ] If the api-egress is involved, add unit test asserting the egress correctly handles tools that use `user_id` scoping instead of `task_id`.
- [ ] **Red - Python E2E:** Run `just e2e --tags control_plane` (e2e_0810) and e2e_0812 per Red above; confirm failures match the known Bug 5 symptoms.
- [ ] **Red - BDD:** Run `just test-bdd` for MCP gateway scenarios; confirm new skills and extraneous-argument scenarios fail as expected.
- [ ] **Red - Go:** Run `go test` / `just test-go-cover` for `orchestrator/internal/mcpgateway` and `orchestrator/cmd/api-egress`; confirm new unit tests fail for the expected reason until fixed.
- [ ] **Red validation gate:** Do not proceed to Green until root cause is confirmed and Python E2E, BDD, and Go Red checks above prove the gap.

#### Green (Task 4)

- [ ] Apply the fix determined in Discovery:
  - [ ] If handler/middleware bug: fix the MCP gateway or api-egress so `skills.*` tools are not gated on `task_id`.
  - [ ] If E2E request format: update `helpers.mcp_tool_call` to include `task_id` in the request envelope when required, or update individual test calls.
  - [ ] If both: fix handler for spec compliance AND update E2E tests for correct request format.
- [ ] Ensure extraneous argument handling complies with spec: gateway MUST ignore unknown argument keys.
- [ ] Run all e2e_0810 subtests until they pass (all 11 failures resolved).
- [ ] Resolve e2e_0812 skips if possible (set required env vars in test setup or document why they remain skipped).
- [ ] Run targeted unit and BDD tests until they pass.
- [ ] Validation gate: do not proceed until all MCP tool routing tests are green.

#### Refactor (Task 4)

- [ ] If handler changes duplicated validation logic, extract shared helpers.
- [ ] Ensure any E2E helper changes do not break other test modules that use `mcp_tool_call`.
- [ ] Re-run targeted tests.
- [ ] Validation gate: do not proceed until refactor is verified.

#### Testing (Task 4)

All three test layers MUST pass before this task is complete.

- [ ] **Go unit tests:** Run `just test-go-cover` for `orchestrator/internal/mcpgateway` and `orchestrator/cmd/api-egress`; confirm all MCP gateway unit tests pass and coverage meets thresholds.
- [ ] **BDD tests:** Run `just test-bdd` for MCP gateway scenarios; confirm skills and extraneous-argument scenarios pass.
- [ ] **Python E2E tests:** Run `just setup-dev restart --force` then `just e2e --tags control_plane`; confirm all e2e_0810 tests pass (0 failures) and e2e_0812 tests pass or have only documented skips.
- [ ] Run `just lint-go` for changed packages.
- [ ] Run `just lint-python` for changed test scripts.
- [ ] **Testing validation gate:** Do not start Task 5 until **Go**, **BDD**, **Python E2E**, `just lint-go`, and `just lint-python` in `#### Testing (Task 4)` above are each satisfied per their checkboxes.

#### Closeout (Task 4)

- [ ] Generate a **task completion report** for Task 4: root cause of Bug 5, what was fixed (handler, tests, or both), what tests pass now, any remaining e2e_0812 skips and why.
- [ ] Update `_bugs.md` Bug 5 with resolution status.
- [ ] Do not start Task 5 until this closeout is done.
- [ ] Mark every completed step in this task with `- [x]`. (Last step.)

---

### Task 5: PMA Streaming State Machine, Overwrite, and Secure Buffers

Complete the PMA standard-path streaming: configurable token state machine (route visible/thinking/tool_call), per-iteration and per-turn overwrite events, and secure-buffer wrapping for secret-bearing stream buffers.

Source: [2026-03-19_streaming_remaining_work_execution_plan.md](2026-03-19_streaming_remaining_work_execution_plan.md) Task 1.

#### Task 5 Requirements and Specifications

- [docs/requirements/pmagnt.md](../requirements/pmagnt.md) REQ-PMAGNT-0118, 0120-0126.
- [docs/requirements/stands.md](../requirements/stands.md) REQ-STANDS-0133.
- [docs/tech_specs/cynode_pma.md](../tech_specs/cynode_pma.md) (StreamingAssistantOutput, StreamingTokenStateMachine, PMAStreamingOverwrite).
- [features/agents/pma_chat_and_context.feature](../../features/agents/pma_chat_and_context.feature).

#### Discovery (Task 5) Steps

- [ ] Re-read PMA streaming requirements and cynode_pma spec for state machine, overwrite scopes, and secret handling.
- [ ] Inspect `agents/internal/pma/` (chat.go, langchain.go) for current wrapper, event emission, and buffer usage.
- [ ] Confirm where the secure-buffer helper lives and how PMA should call it.
- [ ] List existing PMA unit tests that cover streaming and identify gaps for state machine, overwrite, and secure buffers.

#### Red (Task 5)

All three test layers MUST be added or updated before implementation.

- **Python E2E tests** (add or update first so spec-defined behavior is locked):
  - [ ] Add or update E2E tests (e.g. `e2e_0620_pma_ndjson.py`) asserting that PMA streaming output contains separate `delta`, `thinking_delta`, and `tool_call` event types.
  - [ ] Add E2E assertion for per-iteration and per-turn overwrite events when PMA emits them.
  - [ ] Run `just e2e --tags pma_inference` and confirm new assertions fail.
- **BDD scenarios** (add or extend in `pma_chat_and_context.feature`):
  - [ ] Add scenarios for overwrite events (per-iteration, per-turn scope).
  - [ ] Add scenarios for thinking/tool-call separation in streaming output.
- **Go unit tests** (add failing tests in `agents/internal/pma`):
  - [ ] State machine routes visible text to `delta`, thinking to `thinking_delta`, tool-call content to `tool_call`; ambiguous partial tags buffered.
  - [ ] Per-iteration overwrite event replaces only targeted iteration segment.
  - [ ] Per-turn overwrite event replaces entire visible in-flight content.
  - [ ] Secret-bearing append/replace paths use the shared secure-buffer helper.
- [ ] **Red - Python E2E:** Run `just setup-dev restart --force` then `just e2e --tags pma_inference`; confirm new PMA streaming assertions fail.
- [ ] **Red - BDD:** Run `just test-bdd` for PMA feature coverage; confirm new overwrite and streaming scenarios fail as expected.
- [ ] **Red - Go:** Run `go test` / `just test-go-cover` for `agents/internal/pma`; confirm new state machine, overwrite, and secure-buffer tests fail as expected.
- [ ] **Red validation gate:** Do not proceed to Green until Python E2E, BDD, and Go Red checks above prove the gaps across all three layers.

#### Green (Task 5)

- [ ] Implement configurable streaming token state machine in PMA:
  - [ ] Route visible text to `delta`, hidden thinking to `thinking`, detected tool-call content to `tool_call`.
  - [ ] Buffer ambiguous partial tags instead of leaking as visible text.
- [ ] Emit PMA overwrite events for both scopes (per-iteration, per-turn) per spec.
- [ ] Wrap PMA secret-bearing stream buffer operations with the secure-buffer helper.
- [ ] Re-run PMA unit tests until they pass.
- [ ] Validation gate: do not proceed until PMA streaming state machine and overwrite are green.

#### Refactor (Task 5)

- [ ] Extract small helpers for state machine and overwrite logic; remove duplication.
- [ ] Re-run Task 5 targeted tests.
- [ ] Validation gate: do not proceed until refactor is verified.

#### Testing (Task 5)

All three test layers MUST pass before this task is complete.

- [ ] **Go unit tests:** Run `just test-go-cover` for affected PMA packages; confirm state machine, overwrite, and secure-buffer unit tests pass and coverage meets thresholds.
- [ ] **BDD tests:** Run `just test-bdd` for PMA feature coverage; confirm overwrite and streaming scenarios pass.
- [ ] **Python E2E tests:** Run `just setup-dev restart --force` then `just e2e --tags pma_inference`; confirm PMA streaming E2E assertions pass.
- [ ] Run `just lint-go` for changed packages.
- [ ] **Testing validation gate:** Do not start Task 6 until **Go**, **BDD**, **Python E2E**, and `just lint-go` in `#### Testing (Task 5)` above are each satisfied per their checkboxes.

#### Closeout (Task 5)

- [ ] Generate a **task completion report** for Task 5: what changed (state machine, overwrite, secure-buffer), what tests passed.
- [ ] Do not start Task 6 until closeout is done.
- [ ] Mark every completed step in this task with `- [x]`. (Last step.)

---

### Task 6: Gateway Relay Completion, Persistence, and Heartbeat Fallback

Complete the gateway: separate visible/thinking/tool accumulators, native `/v1/responses` format, persist structured assistant turns (redacted only), remove or bypass `emitContentAsSSE`, heartbeat fallback, and client cancellation handling.

Source: [2026-03-19_streaming_remaining_work_execution_plan.md](2026-03-19_streaming_remaining_work_execution_plan.md) Task 2.

#### Task 6 Requirements and Specifications

- [docs/requirements/usrgwy.md](../requirements/usrgwy.md) REQ-USRGWY-0149-0156.
- [docs/requirements/client.md](../requirements/client.md) REQ-CLIENT-0182, 0184, 0185, 0215-0220.
- [docs/requirements/stands.md](../requirements/stands.md) REQ-STANDS-0133.
- [docs/tech_specs/openai_compatible_chat_api.md](../tech_specs/openai_compatible_chat_api.md) (Streaming, StreamingRedactionPipeline, StreamingPerEndpointSSEFormat, StreamingHeartbeatFallback).
- [docs/tech_specs/chat_threads_and_messages.md](../tech_specs/chat_threads_and_messages.md) (structured parts).
- [features/orchestrator/openai_compat_chat.feature](../../features/orchestrator/openai_compat_chat.feature).
- [features/e2e/chat_openai_compatible.feature](../../features/e2e/chat_openai_compatible.feature).

#### Discovery (Task 6) Steps

- [ ] Re-read gateway streaming requirements and openai_compatible_chat_api spec (relay, accumulators, persistence, heartbeat, cancellation).
- [ ] Inspect `orchestrator/internal/handlers/openai_chat.go` and database/thread persistence for current relay and persistence paths.
- [ ] Locate all uses of `emitContentAsSSE` and define replacement (heartbeat + final delta).
- [ ] Confirm e2e_0630_gateway_streaming_contract.py test list and which tests currently skip or pass.

#### Red (Task 6)

All three test layers MUST be added or updated before implementation.

- **Python E2E tests** (add or update first so spec-defined behavior is locked):
  - [ ] Add or update tests in `e2e_0630_gateway_streaming_contract.py` asserting: separate visible/thinking/tool events, `/v1/responses` native event model, heartbeat SSE when upstream is slow, client disconnect cancels stream.
  - [ ] Add E2E assertions for persisted assistant turn structured parts (retrieve after stream completes and verify thinking/tool parts present, redacted).
  - [ ] Run `just e2e --tags chat` and confirm new assertions fail.
- **BDD scenarios** (add or update in `features/orchestrator/openai_compat_chat.feature` and `features/e2e/chat_openai_compatible.feature`):
  - [ ] Add scenarios for separate visible/thinking/tool accumulators.
  - [ ] Add scenarios for heartbeat SSE fallback.
  - [ ] Add scenarios for client disconnect cancellation.
  - [ ] Add scenarios for persisted structured assistant turn with redacted parts.
- **Go unit tests** (add failing tests in orchestrator handler/database packages):
  - [ ] Separate visible, thinking, and tool-call accumulators; overwrite events applied to correct scope.
  - [ ] Post-stream redaction on all three accumulators before terminal completion.
  - [ ] `/v1/responses` native event model and streamed response_id.
  - [ ] Persisted assistant turn has structured parts (thinking, tool_call) with redacted content only.
  - [ ] Heartbeat SSE when upstream does not stream; no use of `emitContentAsSSE` on standard path.
  - [ ] Client disconnect cancels stream and does not leave upstream running indefinitely.
  - [ ] Database/integration tests for persisted structured parts.
- [ ] **Red - Python E2E:** Run `just setup-dev restart --force` then `just e2e --tags chat`; confirm new gateway contract assertions fail.
- [ ] **Red - BDD:** Run `just test-bdd` for orchestrator/openai_compat_chat and e2e/chat features; confirm new gateway scenarios fail as expected.
- [ ] **Red - Go:** Run `go test` / `just test-go-cover` for orchestrator handler and database packages; confirm new gateway unit tests fail as expected.
- [ ] **Red validation gate:** Do not proceed to Green until Python E2E, BDD, and Go Red checks above prove the gateway gaps across all three layers.

#### Green (Task 6)

- [ ] Maintain separate visible-text, thinking, and tool-call accumulators in the gateway relay.
- [ ] Apply per-iteration and per-turn overwrite events to the correct accumulator scope; run post-stream secret scan on all three before terminal completion.
- [ ] Emit `/v1/responses` in native responses event model with named `cynodeai.*` extensions and streamed response_id.
- [ ] Persist final redacted structured assistant turn.
- [ ] Remove or bypass `emitContentAsSSE`; use heartbeat SSE plus one final visible-text delta when upstream cannot stream.
- [ ] Treat client cancellation/disconnect as stream cancellation.
- [ ] Wrap gateway secret-bearing accumulator paths with the secure-buffer helper.
- [ ] Re-run gateway tests until they pass.
- [ ] Validation gate: do not proceed until gateway relay, persistence, and fallback are green.

#### Refactor (Task 6)

- [ ] Extract relay and accumulator helpers; share logic between chat-completions and responses paths.
- [ ] Remove obsolete fake-stream and single-accumulator code.
- [ ] Re-run Task 6 targeted tests.
- [ ] Validation gate: do not proceed until refactor is verified.

#### Testing (Task 6)

All three test layers MUST pass before this task is complete.

- [ ] **Go unit tests:** Run `just test-go-cover` for orchestrator handler, database, and integration packages; confirm gateway unit tests pass and coverage meets thresholds.
- [ ] **BDD tests:** Run `just test-bdd` for orchestrator/openai_compat_chat and e2e/chat features; confirm all gateway streaming scenarios pass.
- [ ] **Python E2E tests:** Run `just setup-dev restart --force` then `just e2e --tags chat`; confirm e2e_0630 and related gateway E2E tests pass.
- [ ] Run `just lint-go` for changed packages.
- [ ] **Testing validation gate:** Do not start Task 7 until **Go**, **BDD**, **Python E2E**, and `just lint-go` in `#### Testing (Task 6)` above are each satisfied per their checkboxes.

#### Closeout (Task 6)

- [ ] Generate a **task completion report** for Task 6: what changed (accumulators, /v1/responses, persistence, heartbeat, cancellation, secure-buffer), what tests passed.
- [ ] Do not start Task 7 until closeout is done.
- [ ] Mark every completed step in this task with `- [x]`. (Last step.)

---

### Task 7: PTY Test Harness Extensions and TUI Structured Streaming UX

Extend the PTY harness (cancel-retain-partial, reconnect, scrollback assertions) and wire the TUI to the richer event model (TranscriptTurn/TranscriptPart, one in-flight turn, stored thinking/tool toggles, overwrite scopes, heartbeat, reconnect, secure-buffer).

Source: [2026-03-19_streaming_remaining_work_execution_plan.md](2026-03-19_streaming_remaining_work_execution_plan.md) Tasks 3 and 4 (combined because TUI streaming UX depends on the harness extensions).

#### Task 7 Requirements and Specifications

- [docs/requirements/client.md](../requirements/client.md) REQ-CLIENT-0182-0185, 0192, 0193, 0195, 0202, 0204, 0209, 0213-0220.
- [docs/requirements/stands.md](../requirements/stands.md) REQ-STANDS-0133.
- [docs/tech_specs/cynork_tui.md](../tech_specs/cynork_tui.md) (TranscriptRendering, GenerationState, ThinkingContentStorageDuringStreaming, ToolCallContentStorageDuringStreaming, SecureBufferHandlingForInFlightStreamingContent, ConnectionRecovery).
- [features/cynork/cynork_tui_streaming.feature](../../features/cynork/cynork_tui_streaming.feature).
- [features/cynork/cynork_tui.feature](../../features/cynork/cynork_tui.feature).
- Current harness: [scripts/test_scripts/tui_pty_harness.py](../../scripts/test_scripts/tui_pty_harness.py).

#### Discovery (Task 7) Steps

- [ ] Re-read TUI streaming feature scenarios that require PTY: cancel and retain partial text; reconnect and preserve partial / mark interrupted; show-thinking / show-tool-output revealing stored content.
- [ ] Inspect `tui_pty_harness.py` for existing APIs and identify what must be added (scrollback wait, cancel helpers, reconnect helpers).
- [ ] Inspect `cynork/internal/tui/state.go` and `model.go` for TranscriptTurn, TranscriptPart, and current streaming/scrollback logic.
- [ ] Confirm cynork transport already exposes thinking, tool_call, iteration_start, heartbeat; list remaining transport gaps for TUI.

#### Red (Task 7)

All three test layers MUST be added or updated before implementation.

- **Python E2E tests** (add or update first so spec-defined behavior is locked):
  - [ ] Cancel stream (Ctrl+C) then assert retained partial text in scrollback (e2e_0750).
  - [ ] Simulate reconnect and assert partial text preserved, turn marked interrupted (e2e_0750).
  - [ ] `/show-thinking` and `/show-tool-output` reveal stored content without refetch (e2e_0760).
  - [ ] Run `just e2e --tags tui_pty` and confirm new assertions fail.
- **BDD scenarios** (add or update in `features/cynork/cynork_tui_streaming.feature` and `cynork_tui.feature`):
  - [ ] Add scenario for cancel-and-retain-partial behavior.
  - [ ] Add scenario for reconnect preserving partial text and marking interrupted turn.
  - [ ] Add scenarios for thinking/tool-output visibility toggles revealing stored content.
  - [ ] Add scenario for heartbeat rendering during slow upstream.
- **Go unit tests** (add failing tests in `cynork/internal/tui`):
  - [ ] Exactly one in-flight assistant turn updated in place during streaming.
  - [ ] Hidden-by-default thinking placeholders; expand when enabled without refetch.
  - [ ] Tool-call and tool-result as distinct non-prose items; toggle show/hide.
  - [ ] Per-iteration overwrite replaces only targeted segment; per-turn overwrite replaces entire visible.
  - [ ] Heartbeat renders as progress indicator; does not pollute transcript.
  - [ ] Cancellation and reconnect retain content and reconcile active turn.
- [ ] **Red - Python E2E:** Run `just setup-dev restart --force` then `just e2e --tags tui_pty`; confirm new PTY assertions fail.
- [ ] **Red - BDD:** Run `just test-bdd` for TUI streaming features; confirm new scenarios fail as expected.
- [ ] **Red - Go:** Run `go test` / `just test-go-cover` for `cynork/internal/tui`; confirm new streaming and transcript unit tests fail as expected.
- [ ] **Red validation gate:** Do not proceed to Green until Python E2E, BDD, and Go Red checks above prove the TUI streaming UX gap across all three layers.

#### Green (Task 7)

- [ ] Extend `tui_pty_harness.py`:
  - [ ] Helper to wait for a string or pattern in scrollback.
  - [ ] Cancel stream (Ctrl+C) and collect scrollback for retained-partial assertion.
  - [ ] Reconnect helper (restart TUI, re-attach to same thread, assert interrupted state).
- [ ] Promote TranscriptTurn, TranscriptPart, and SessionState to canonical in-memory streaming representation in TUI.
- [ ] Render one logical assistant turn per user prompt; update in place while streaming.
- [ ] Store and render structured content: visible text; hidden-by-default thinking with instant reveal; tool-call/tool-result as non-prose items with toggle.
- [ ] Implement per-iteration and per-turn overwrite handling.
- [ ] Render heartbeat as display-only progress; remove when final content arrives.
- [ ] Implement bounded-backoff reconnect and interrupted-turn reconciliation.
- [ ] Wrap TUI secret-bearing stream-buffer paths with the secure-buffer helper.
- [ ] Re-run TUI unit and E2E tests until they pass.
- [ ] Validation gate: do not proceed until TUI streaming UX is green.

#### Refactor (Task 7)

- [ ] Extract transcript-building, overwrite-handling, and status-rendering helpers.
- [ ] Remove obsolete string-only stream bookkeeping.
- [ ] Re-run Task 7 targeted tests.
- [ ] Validation gate: do not proceed until refactor is verified.

#### Testing (Task 7)

All three test layers MUST pass before this task is complete.

- [ ] **Go unit tests:** Run `just test-go-cover` for `cynork/internal/tui` and adjacent packages; confirm streaming, transcript, overwrite, heartbeat, and reconnect unit tests pass and coverage meets thresholds.
- [ ] **BDD tests:** Run `just test-bdd` and confirm all TUI streaming scenarios pass with no regressions.
- [ ] **Python E2E tests:** Run `just setup-dev restart --force` then `just e2e --tags tui_pty`; confirm e2e_0750, e2e_0760, e2e_0650 all pass.
- [ ] Run `just lint-go` for `cynork/internal/tui` and adjacent packages.
- [ ] Run `just lint-python` for harness changes.
- [ ] **Testing validation gate:** Do not start Task 8 until **Go**, **BDD**, **Python E2E**, `just lint-go`, and `just lint-python` in `#### Testing (Task 7)` above are each satisfied per their checkboxes.

#### Closeout (Task 7)

- [ ] Generate a **task completion report** for Task 7: what changed (harness, transcript state, rendering, overwrite, heartbeat, reconnect, secure-buffer), what tests passed.
- [ ] Do not start Task 8 until closeout is done.
- [ ] Mark every completed step in this task with `- [x]`. (Last step.)

---

### Task 8: BDD Step Implementation and E2E Streaming Test Matrix

Replace remaining streaming and PTY BDD placeholders with real step implementations; finish the Python E2E test matrix and ensure all streaming tags pass.
Also addresses BDD/PTY coverage from [2026-03-14_plan_after_tui_fix.md](2026-03-14_plan_after_tui_fix.md) Task 5 (Phase 6 alignment).

Source: [2026-03-19_streaming_remaining_work_execution_plan.md](2026-03-19_streaming_remaining_work_execution_plan.md) Task 6 and [2026-03-14_plan_after_tui_fix.md](2026-03-14_plan_after_tui_fix.md) Task 5.

#### Task 8 Requirements and Specifications

- All streaming feature files: `features/cynork/cynork_tui.feature`, `features/cynork/cynork_tui_streaming.feature`, `features/cynork/cynork_tui_threads.feature`, `features/orchestrator/openai_compat_chat.feature`, `features/agents/pma_chat_and_context.feature`, `features/e2e/chat_openai_compatible.feature`.
- BDD steps: `cynork/_bdd/steps2.go` (streaming and PTY steps returning `godog.ErrPending`).
- E2E file ownership: e2e_0610 (API events), e2e_0620 (PMA NDJSON), e2e_0630 (gateway contract), e2e_0640 (cynork transport), e2e_0650 (TUI streaming), e2e_0750 (PTY cancel/reconnect), e2e_0760 (slash toggles).

#### Discovery (Task 8) Steps

- [ ] List every step in `steps2.go` that returns `godog.ErrPending` and classify: streaming, PTY-required, or other.
- [ ] Map each pending step to the feature scenario and to the implementation that makes it pass.
- [ ] Confirm Python E2E file ownership and identify overlap or gaps.

#### Red (Task 8)

All three test layers MUST be added or updated before implementation.

- **Python E2E tests** (verify and extend the full streaming test matrix):
  - [ ] Audit E2E files e2e_0610, e2e_0620, e2e_0630, e2e_0640, e2e_0650, e2e_0750, e2e_0760 for any remaining gaps or skipped assertions.
  - [ ] Add missing E2E tests for Phase 6 scope: auth recovery, streaming cancellation, thinking visibility, collapsed-thinking placeholder.
  - [ ] Run `just e2e` and document which streaming-related tests currently pass/fail/skip.
- **BDD scenarios** (replace placeholders and extend):
  - [ ] Replace streaming-related `godog.ErrPending` steps with implementations that fail against current behavior (or assertions that will pass after Tasks 5-7).
  - [ ] Add or update BDD scenarios for Phase 6 scope: auth recovery, both chat surfaces, streaming, cancellation, thinking visibility, collapsed-thinking placeholder.
  - [ ] Run `just test-bdd` and confirm streaming scenarios reflect current state.
- **Go unit tests** (verify coverage for any BDD step helpers added):
  - [ ] Add unit tests for any new shared BDD step helpers (SSE parsing, scrollback checking, etc.).
- [ ] **Red - Python E2E:** Run full `just e2e` (or the `--tags` matrix from Red above); document pass/fail/skip; confirm results match the expected gap before Green.
- [ ] **Red - BDD:** Run `just test-bdd`; confirm streaming scenarios reflect current state (failures, skips, or pending as expected).
- [ ] **Red - Go:** Run `go test` / `just test-go-cover` for new BDD step helper packages; confirm new helper tests match the expected gap.
- [ ] **Red validation gate:** Do not proceed to Green until BDD step strategy is clear and Python E2E, BDD, and Go Red checks above reflect the expected gap.

#### Green (Task 8)

- [ ] Implement or wire each streaming BDD step so that after Tasks 5-7 the steps pass.
- [ ] Only skip a step if it truly cannot run in BDD (requires real interactive PTY); document reasons.
- [ ] Re-run `just test-bdd` until streaming scenarios pass.
- [ ] Validation gate: do not proceed until test-bdd passes for all implemented streaming scenarios.

#### Refactor (Task 8)

- [ ] Extract shared BDD step helpers (e.g. parse SSE, check scrollback content).
- [ ] Re-run `just test-bdd`.
- [ ] Validation gate: do not proceed until BDD suite is stable.

#### Testing (Task 8)

All three test layers MUST pass before this task is complete.

- [ ] **Go unit tests:** Run `just test-go-cover` for BDD step helper packages; confirm any new helpers are covered.
- [ ] **BDD tests:** Run `just test-bdd`; confirm all implemented streaming scenarios pass with no pending steps remaining (except those documented as PTY-only).
- **Python E2E tests:**
  - [ ] Run `just setup-dev restart --force` then `just e2e --tags pma_inference`, `just e2e --tags chat`, and `just e2e --tags tui_pty`; confirm all streaming E2E files pass.
  - [ ] Run full `just e2e` and confirm no regressions.
- [ ] **Testing validation gate:** Do not start Task 9 until **Go**, **BDD**, nested **Python E2E** bullets, and full `just e2e` in `#### Testing (Task 8)` above are each satisfied per their checkboxes with no regressions.

#### Closeout (Task 8)

- [ ] Generate a **task completion report** for Task 8: which BDD steps were implemented, which remain pending and why, which E2E tags pass.
- [ ] Do not start Task 9 until closeout is done.
- [ ] Mark every completed step in this task with `- [x]`. (Last step.)

---

### Task 9: TUI Auth Recovery and In-Session Switches

Implement startup and in-session auth recovery, project and model in-session switching, and validate through PTY harness.

Source: [2026-03-14_plan_after_tui_fix.md](2026-03-14_plan_after_tui_fix.md) Task 3.

#### Task 9 Requirements and Specifications

- [docs/requirements/client.md](../requirements/client.md) (auth recovery, in-session model/project).
- [docs/tech_specs/cynork_tui.md](../tech_specs/cynork_tui.md) (auth recovery, status bar, in-session switches).

#### Discovery (Task 9) Steps

- [ ] Read the auth recovery requirements and TUI spec sections.
- [ ] Inspect cynork TUI and cmd for login flow, token validation, and gateway auth failure handling.
- [ ] Inspect session and TUI for project and model switching; identify gaps vs spec.
- [ ] Review PTY harness and E2E scripts for auth-recovery assertions; identify missing coverage.

#### Red (Task 9)

All three test layers MUST be added or updated before implementation.

- **Python E2E tests** (add or update first so spec-defined behavior is locked):
  - [ ] Add PTY E2E tests for startup auth recovery (TUI renders, detects missing token, presents login overlay).
  - [ ] Add PTY E2E tests for in-session auth recovery (gateway returns auth failure, TUI presents login overlay without losing context).
  - [ ] Add PTY E2E tests for project-context switching and model selection in-session.
  - [ ] Add PTY E2E tests for thread create/switch/rename, thinking visibility (scrollback/history-reload, YAML persist).
  - [ ] Run `just e2e --tags auth` and `just e2e --tags tui_pty` and confirm new tests fail.
- **BDD scenarios** (add or update in `features/cynork/`):
  - [ ] Add scenarios for startup auth recovery.
  - [ ] Add scenarios for in-session auth recovery.
  - [ ] Add scenarios for in-session project and model switching.
  - [ ] Add scenarios for password/token redaction in scrollback and transcript.
- **Go unit tests** (add failing tests in `cynork/internal/tui` and `cynork/internal/chat`):
  - [ ] Unit tests for auth recovery state transitions (token missing at startup, gateway auth failure mid-session).
  - [ ] Unit tests for project-context and model-selection state changes.
  - [ ] Unit tests asserting passwords and tokens are never stored in scrollback or transcript history.
- [ ] **Red - Python E2E:** Run `just setup-dev restart --force` then `just e2e --tags auth` and `just e2e --tags tui_pty`; confirm new tests fail for the expected reason.
- [ ] **Red - BDD:** Run `just test-bdd` for cynork features; confirm new auth and switch scenarios fail as expected.
- [ ] **Red - Go:** Run `go test` / `just test-go-cover` for `cynork/internal/tui` and `cynork/internal/chat`; confirm new unit tests fail as expected.
- [ ] **Red validation gate:** Do not proceed to Green until Python E2E, BDD, and Go Red checks above prove the gap.

#### Green (Task 9)

- [ ] Implement startup login recovery when usable token is missing (TUI renders first per spec; Bug 2 already fixed; verify).
- [ ] Implement in-session login recovery when gateway returns auth failure.
- [ ] Ensure passwords and tokens are never in scrollback or transcript history.
- [ ] Implement project-context switching and model selection in-session.
- [ ] Validate through PTY harness: thread create/switch/rename, thinking visibility, auth recovery.
- [ ] Run targeted tests and PTY/E2E until they pass.
- [ ] Validation gate: do not proceed until targeted tests are green.

#### Refactor (Task 9)

- [ ] Refine implementation without changing behavior; keep all tests green.
- [ ] Validation gate: do not proceed until refactor is verified.

#### Testing (Task 9)

All three test layers MUST pass before this task is complete.

- [ ] **Go unit tests:** Run `just test-go-cover` for `cynork/internal/tui` and `cynork/internal/chat`; confirm auth recovery and in-session switch unit tests pass and coverage meets thresholds.
- [ ] **BDD tests:** Run `just test-bdd` for cynork features; confirm auth recovery and in-session switch scenarios pass.
- [ ] **Python E2E tests:** Run `just setup-dev restart --force` then `just e2e --tags auth` and `just e2e --tags tui_pty`; confirm all auth and TUI E2E tests pass.
- [ ] Run `just ci` and full `just e2e` for regression check.
- [ ] **Testing validation gate:** Do not start Task 10 until **Go**, **BDD**, **Python E2E**, `just ci`, and full `just e2e` in `#### Testing (Task 9)` above are each satisfied per their checkboxes.

#### Closeout (Task 9)

- [ ] Generate a **task completion report** for Task 9: what was done, what passed, any deviations.
- [ ] Do not start Task 10 until closeout is done.
- [ ] Mark every completed step in this task with `- [x]`. (Last step.)

---

### Task 10: Remaining MVP Phase 2 and Worker Deployment Docs

Complete remaining MVP Phase 2 work: remaining MCP tool slices beyond the minimum, LangGraph graph-node work, verification-loop work, chat/runtime drifts (bounded wait, retry, reliability).
Also ensure worker deployment docs distinguish normative topology from deferred implementation.

Source: [2026-03-14_plan_after_tui_fix.md](2026-03-14_plan_after_tui_fix.md) Tasks 5 and 6.

#### Task 10 Requirements and Specifications

- [docs/mvp_plan.md](../mvp_plan.md) (if it exists), [docs/requirements/pmagnt.md](../requirements/pmagnt.md), [docs/requirements/orches.md](../requirements/orches.md).
- [docs/requirements/worker.md](../requirements/worker.md), [docs/tech_specs/worker_node.md](../tech_specs/worker_node.md).

#### Discovery (Task 10) Steps

- [ ] Read the MVP implementation plan and identify remaining MCP tool slices, LangGraph items, verification-loop items, and chat/runtime drifts.
- [ ] Read worker requirements and worker_node tech spec; identify sections that mix normative topology with deferred implementation.
- [ ] Confirm Tasks 1-9 are complete and the TUI path is stable before starting.

#### Red (Task 10)

All three test layers MUST be added or updated before implementation of each slice.

- **Python E2E tests** (add or update first for each slice so spec-defined behavior is locked):
  - [ ] For each MCP tool slice: add E2E tests validating the tool behavior via PMA chat or direct API.
  - [ ] For each LangGraph/verification-loop slice: add E2E tests validating the PMA-to-PAA flow and result review.
  - [ ] For chat/runtime drift fixes: add E2E tests for bounded wait, retry, and reliability scenarios.
  - [ ] Run `just e2e` for new modules and confirm they fail before implementation.
- **BDD scenarios** (add or update for each slice):
  - [ ] For each MCP tool slice: add BDD scenarios in relevant feature files.
  - [ ] For graph-node and verification-loop work: add BDD scenarios.
  - [ ] For reliability fixes: add scenarios for bounded wait and retry behavior.
- **Go unit tests** (add failing tests for each slice):
  - [ ] For each MCP tool slice: unit tests for handler, store, and RBAC enforcement.
  - [ ] For LangGraph/verification-loop: unit tests for graph nodes and state transitions.
  - [ ] For chat/runtime drifts: unit tests for bounded wait, retry logic, and error handling.
- [ ] **Red - Python E2E:** For each slice, run `just e2e` for new modules; confirm failures before implementation.
- [ ] **Red - BDD:** For each slice, run `just test-bdd`; confirm new scenarios fail before implementation.
- [ ] **Red - Go:** For each slice, run `go test` / `just test-go-cover`; confirm new tests fail before implementation.
- [ ] **Red validation gate:** Do not proceed to Green until the test plan is defined and each slice has failing tests in Python E2E, BDD, and Go.

#### Green (Task 10)

- [ ] Resume remaining MCP tool slices beyond the minimum PMA chat slice.
- [ ] Finish remaining LangGraph graph-node work.
- [ ] Finish verification-loop work for PMA to Project Analyst to result review flows.
- [ ] Close known chat/runtime drifts (bounded wait, retry, reliability).
- [ ] Update worker deployment docs: separate normative topology from deferred implementation.
- [ ] Run `just docs-check` after doc edits.
- [ ] Run targeted validation per slice; run `just ci` and `just e2e` when the phase closes.
- [ ] Validation gate: do not proceed until all slices and gates pass.

#### Refactor (Task 10)

- [ ] Refine implementation without changing behavior; keep all tests green.
- [ ] Validation gate: do not proceed until refactor is verified.

#### Testing (Task 10)

All three test layers MUST pass before this task is complete.

- [ ] **Go unit tests:** Run `just test-go-cover` for all affected packages; confirm all slice unit tests pass and coverage meets thresholds.
- [ ] **BDD tests:** Run `just test-bdd`; confirm all new and existing scenarios pass.
- [ ] **Python E2E tests:** Run `just setup-dev restart --force` then `just e2e --tags pma` and/or `--tags chat`; confirm all slice E2E tests pass.
- [ ] Run `just ci` and full `just e2e` for regression check.
- [ ] **Testing validation gate:** Do not start Task 11 until **Go**, **BDD**, **Python E2E**, `just ci`, and full `just e2e` in `#### Testing (Task 10)` above are each satisfied per their checkboxes.

#### Closeout (Task 10)

- [ ] Generate a **task completion report** for Task 10: what was done per slice, what passed, any deviations.
- [ ] Do not start Task 11 until closeout is done.
- [ ] Mark every completed step in this task with `- [x]`. (Last step.)

---

### Task 11: Postgres Schema Documentation Refactoring

Distribute PostgreSQL table definitions from the monolithic `postgres_schema.md` into domain-specific tech spec documents per the refactoring plan.
This is a docs-only task; no schema or code changes.

Source: [2026-03-19_postgres_schema_refactoring_plan.md](2026-03-19_postgres_schema_refactoring_plan.md).

#### Task 11 Requirements and Specifications

- [docs/dev_docs/2026-03-19_postgres_schema_refactoring_plan.md](2026-03-19_postgres_schema_refactoring_plan.md) (table-to-document mapping, execution steps).
- [docs/tech_specs/postgres_schema.md](../tech_specs/postgres_schema.md) (current monolithic schema).
- Target domain docs per the mapping (e.g. `local_user_accounts.md`, `projects_and_scopes.md`, `rbac_and_groups.md`, `access_control.md`, `user_preferences.md`, `worker_node.md`, `sandbox_image_registry.md`, `runs_and_sessions_api.md`, `chat_threads_and_messages.md`, `orchestrator_artifacts_storage.md`, `model_management.md`, and others per plan).

#### Discovery (Task 11) Steps

- [ ] Read the postgres schema refactoring plan in full (table-to-document mapping, execution steps, considerations).
- [ ] Confirm the table-to-document mapping is still accurate after recent spec changes (e.g. artifacts schema may now be split already).
- [ ] Count total table groups and estimate effort for a proof-of-concept batch (identity and authentication tables).

#### Red (Task 11)

- [ ] N/A for docs-only task; Discovery suffices.

#### Green (Task 11)

- [ ] Start with proof of concept: move identity and authentication tables (`users`, `password_credentials`, `refresh_sessions`) to `local_user_accounts.md`.
  - [ ] Extract table definition section from `postgres_schema.md`.
  - [ ] Add "Postgres Schema" section with Spec IDs and anchors to target doc.
  - [ ] Update `postgres_schema.md` to link to new location.
  - [ ] Update all cross-references in other docs that pointed to the old location.
- [ ] If proof of concept validates well, proceed through remaining table groups per the mapping.
- [ ] Keep `postgres_schema.md` as an index/overview with: links to distributed definitions, table creation order and dependencies, naming conventions, and "Storing This Schema in Code" section.
- [ ] Run `just lint-md` on all affected files after each batch.
- [ ] Run `just docs-check` to verify links after each batch.
- [ ] Validation gate: do not proceed until all Spec ID anchors work and docs-check passes.

#### Refactor (Task 11)

- [ ] Remove redundant "recommended" schemas from domain docs where they existed alongside the authoritative postgres_schema definitions.
- [ ] Ensure no broken cross-references remain.
- [ ] Re-run `just docs-check`.
- [ ] Validation gate: do not proceed until refactor is verified.

#### Testing (Task 11)

- [ ] Run `just lint-md` on all changed files.
- [ ] Run `just docs-check` for full link validation.
- [ ] Verify all Spec ID anchors are preserved and work.
- [ ] **Testing validation gate:** Do not start Task 12 until `just lint-md`, `just docs-check`, and Spec ID verification in `#### Testing (Task 11)` above are each satisfied per their checkboxes.

#### Closeout (Task 11)

- [ ] Generate a **task completion report** for Task 11: which table groups were moved, which remain, what passed.
- [ ] Do not start Task 12 until closeout is done.
- [ ] Mark every completed step in this task with `- [x]`. (Last step.)

---

### Task 12: Documentation and Final Closeout

Update cross-cutting documentation, verify no required follow-up was left undocumented, and produce the final plan completion report.

#### Task 12 Requirements and Specifications

- This plan and all source plans listed in [Source Plans and Status Summary](#source-plans-and-status-summary).
- [meta.md](../../meta.md) (repository layout, docs layout).

#### Discovery (Task 12) Steps

- [ ] Review all tasks 1-11: ensure no required step was skipped; ensure each closeout report is summarized.
- [ ] Identify any user-facing or developer-facing docs that need updates after all implementation tasks.
- [ ] List any remaining risks or follow-on work that should be recorded.

#### Red / Green (Task 12)

- [ ] Update source plans with completion status or mark superseded where appropriate.
- [ ] Update `_bugs.md` with resolution status for Bugs 3, 4, and 5.
- [ ] Document any explicit remaining risks or deferred work.
- [ ] Run `just setup-dev restart --force`.
- **Final validation (run each layer in order):**
  - [ ] **Go unit tests:** Run `just test-go-cover` across all packages; confirm all pass and coverage meets thresholds.
  - [ ] **BDD tests:** Run `just test-bdd`; confirm all scenarios pass with no pending steps (except explicitly documented).
  - [ ] **Python E2E tests:** Run `just e2e`; fix any failures until all tests pass with only expected skips.
- [ ] Run `just docs-check` and `just ci` one final time.

#### Testing (Task 12)

All three test layers MUST pass for the plan to be considered complete.

- [ ] **Go unit tests:** Confirm `just test-go-cover` passed across all packages with no failures and coverage meets thresholds.
- [ ] **BDD tests:** Confirm `just test-bdd` passed with all scenarios green and no unexpected pending steps.
- [ ] **Python E2E tests:** Confirm `just e2e` passed with all tests passing and only expected skips.
- [ ] Confirm `just ci` passed.
- [ ] Confirm all exit criteria from the source plans are met or explicitly documented as follow-on.
- [ ] **Testing validation gate:** Plan complete only when **Go** (`just test-go-cover`), **BDD** (`just test-bdd`), **Python E2E** (`just e2e`), `just docs-check`, and `just ci` (including the `#### Red / Green (Task 12)` runs above) all pass.

#### Closeout (Task 12)

- [ ] Generate a **final plan completion report**: which tasks were completed, overall validation status (`just ci`, full E2E), remaining risks or follow-up.
- [ ] Mark all completed steps in the plan with `- [x]`. (Last step.)
