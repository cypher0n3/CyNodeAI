---
name: Immediate Security and Correctness
overview: |
  Address 13 critical security and correctness issues from review reports 1-6
  before any production deployment.
  Tasks are ordered from simplest/most independent to most complex, with
  cross-cutting changes in the middle and CI validation at the end.
  Each task follows BDD/TDD with per-task validation gates.
todos:
  - id: imm-001
    content: "Read `agents/cmd/cynode-pma/main.go` (WriteTimeout ~line 93) and `agents/internal/pma/chat.go` (`pmaLangchainCompletionTimeout` ~line 30); confirm the mismatch."
    status: pending
  - id: imm-002
    content: "Read PMA tech spec (`docs/tech_specs/cynode_pma.md`) for timeout requirements."
    status: pending
  - id: imm-003
    content: "Add a test asserting HTTP server WriteTimeout is zero or >= inference timeout + 10s margin."
    status: pending
  - id: imm-004
    content: "Run `go test -v -run TestWriteTimeout ./agents/cmd/cynode-pma/...` and confirm the test fails."
    status: pending
  - id: imm-005
    content: "Set WriteTimeout to `0` (disabled for streaming) in `agents/cmd/cynode-pma/main.go`."
    status: pending
  - id: imm-006
    content: "Re-run `go test -v -run TestWriteTimeout ./agents/cmd/cynode-pma/...` and confirm green."
    status: pending
  - id: imm-007
    content: "Extract the timeout into a named constant if not already defined; keep tests green."
    status: pending
  - id: imm-008
    content: "Run `just lint-go` on changed files and `go test -cover ./agents/...`; confirm 90% threshold."
    status: pending
  - id: imm-200
    content: "Run `just e2e --tags streaming,pma_inference` to verify PMA streaming regression (requires inference; skip if unavailable)."
    status: pending
  - id: imm-009
    content: "Validation gate -- do not proceed to Task 2 until all checks pass."
    status: pending
  - id: imm-010
    content: "Generate task completion report (changes, tests passed, deviations). Mark completed steps `- [x]`."
    status: pending
  - id: imm-011
    content: "Do not start Task 2 until Task 1 closeout is done."
    status: pending
  - id: imm-012
    content: "Read `worker_node/cmd/node-manager/main.go` ~line 599 (`startOneManagedService` with `strings.Contains`) and ~line 408 (`containerNameExact`)."
    status: pending
  - id: imm-013
    content: "Identify all callers of `startOneManagedService` to assess impact of the fix."
    status: pending
  - id: imm-014
    content: "Add a unit test exercising container name matching with a prefix collision (e.g., `cynodeai-managed-pma` vs `cynodeai-managed-pma-test`)."
    status: pending
  - id: imm-015
    content: "Run `go test -v -run TestContainerNameMatch ./worker_node/cmd/node-manager/...` and confirm the test fails."
    status: pending
  - id: imm-016
    content: "Replace `strings.Contains(string(out), name)` in `startOneManagedService` with `containerNameExact`."
    status: pending
  - id: imm-017
    content: "Re-run the test and confirm green."
    status: pending
  - id: imm-018
    content: "Consolidate `containerNameExact` and `containerNameMatches` into a single shared function."
    status: pending
  - id: imm-019
    content: "Run `just lint-go` on changed files and `go test -cover ./worker_node/...`; confirm 90% threshold."
    status: pending
  - id: imm-201
    content: "Run `just e2e --tags worker,no_inference` to verify managed service startup regression."
    status: pending
  - id: imm-020
    content: "Validation gate -- do not proceed to Task 3 until all checks pass."
    status: pending
  - id: imm-021
    content: "Generate task completion report. Mark completed steps `- [x]`."
    status: pending
  - id: imm-022
    content: "Do not start Task 3 until Task 2 closeout is done."
    status: pending
  - id: imm-023
    content: "Read `cynork/internal/tui/model.go` ~lines 774-791 (`runEnsureThread`) and the `applyEnsureThreadResult` handler."
    status: pending
  - id: imm-024
    content: "Identify all reads of `Session.CurrentThreadID` in `View()` and goroutine-accessed paths."
    status: pending
  - id: imm-025
    content: "Add a test using `go test -race` on the TUI thread-initialization path to detect the race."
    status: pending
  - id: imm-026
    content: "Run `go test -race -v -run TestEnsureThread ./cynork/internal/tui/...` and confirm the race is detected."
    status: pending
  - id: imm-027
    content: "Move all `Session.CurrentThreadID` mutations into `applyEnsureThreadResult`; have the goroutine return the resolved thread ID as data without writing model fields."
    status: pending
  - id: imm-028
    content: "Re-run `go test -race -v -run TestEnsureThread ./cynork/internal/tui/...` and confirm no race."
    status: pending
  - id: imm-029
    content: "Verify no other `tea.Cmd` closures directly mutate model fields; document the pattern if recurring."
    status: pending
  - id: imm-030
    content: "Run `go test -race ./cynork/...` and `go test -cover ./cynork/...`; confirm 90% threshold."
    status: pending
  - id: imm-202
    content: "Run `just e2e --tags tui_pty,no_inference` to verify TUI thread-initialization regression."
    status: pending
  - id: imm-031
    content: "Validation gate -- do not proceed to Task 4 until all checks pass."
    status: pending
  - id: imm-032
    content: "Generate task completion report. Mark completed steps `- [x]`."
    status: pending
  - id: imm-033
    content: "Do not start Task 4 until Task 3 closeout is done."
    status: pending
  - id: imm-034
    content: "Read `worker_node/internal/securestore/store.go` struct definition (~lines 75-80); identify `key` and `kemKey` fields."
    status: pending
  - id: imm-035
    content: "Identify all call sites that create a `securestore.Store` to wire `Close()` into shutdown paths."
    status: pending
  - id: imm-036
    content: "Add a unit test: create a Store, call `Close()`, verify key material is zeroed (all bytes == 0)."
    status: pending
  - id: imm-037
    content: "Run `go test -v -run TestStoreClose ./worker_node/internal/securestore/...` and confirm failure (method missing)."
    status: pending
  - id: imm-038
    content: "Implement `func (s *Store) Close()` that zeros `s.key` and `s.kemKey`."
    status: pending
  - id: imm-039
    content: "Wire `defer store.Close()` into `worker_node/cmd/node-manager/main.go` shutdown path."
    status: pending
  - id: imm-040
    content: "Re-run the test and confirm green."
    status: pending
  - id: imm-041
    content: "Ensure zeroing is not optimizable away by the compiler (use `runtime.KeepAlive` or similar)."
    status: pending
  - id: imm-042
    content: "Run `just lint-go` on changed files and `go test -cover ./worker_node/internal/securestore/...`; confirm 90% threshold."
    status: pending
  - id: imm-043
    content: "Validation gate -- do not proceed to Task 5 until all checks pass."
    status: pending
  - id: imm-044
    content: "Generate task completion report. Mark completed steps `- [x]`."
    status: pending
  - id: imm-045
    content: "Do not start Task 5 until Task 4 closeout is done."
    status: pending
  - id: imm-046
    content: "Search all Go modules for `!=` token/bearer comparisons: `orchestrator/cmd/api-egress/main.go:155`, `orchestrator/internal/middleware/auth.go:148`, `worker_node/internal/workerapiserver/embed_handlers.go:280,405`. Identify any additional sites."
    status: pending
  - id: imm-047
    content: "Add or update unit tests at each site verifying token comparison rejects incorrect tokens and accepts correct ones."
    status: pending
  - id: imm-048
    content: "Run targeted tests: `go test -v -run TestTokenAuth ./orchestrator/cmd/api-egress/...`, `go test -v -run TestWorkflowAuth ./orchestrator/internal/middleware/...`, `go test -v -run TestBearerAuth ./worker_node/internal/workerapiserver/...`."
    status: pending
  - id: imm-049
    content: "Replace each `got != token` with `subtle.ConstantTimeCompare([]byte(got), []byte(expected)) != 1`; add `import \"crypto/subtle\"` where missing."
    status: pending
  - id: imm-050
    content: "Re-run all targeted tests and confirm green."
    status: pending
  - id: imm-051
    content: "If multiple modules duplicate the pattern, extract a shared helper (e.g., `secretutil.TokenEquals`) into `go_shared_libs`."
    status: pending
  - id: imm-052
    content: "Run `go test -cover ./orchestrator/...` and `go test -cover ./worker_node/...`; confirm 90% threshold."
    status: pending
  - id: imm-053
    content: "Run `just lint-go` on all changed files."
    status: pending
  - id: imm-203
    content: "Run `just e2e --tags auth,no_inference` and `just e2e --tags worker,no_inference` to verify bearer auth regression."
    status: pending
  - id: imm-054
    content: "Validation gate -- do not proceed to Task 6 until all checks pass."
    status: pending
  - id: imm-055
    content: "Generate task completion report listing each file changed and old vs new pattern. Mark completed steps `- [x]`."
    status: pending
  - id: imm-056
    content: "Do not start Task 6 until Task 5 closeout is done."
    status: pending
  - id: imm-057
    content: "Read `go_shared_libs/contracts/workerapi/workerapi.go` ~line 69 (`ExitCode int`)."
    status: pending
  - id: imm-058
    content: "Search `orchestrator/` and `worker_node/` for all consumers of `RunJobResponse.ExitCode` that must handle `*int`."
    status: pending
  - id: imm-059
    content: "Add a unit test: marshal `RunJobResponse` with ExitCode=0, unmarshal, assert `exit_code` field is present and equals 0."
    status: pending
  - id: imm-060
    content: "Run `go test -v -run TestExitCodeZero ./go_shared_libs/contracts/workerapi/...` and confirm failure."
    status: pending
  - id: imm-061
    content: "Change `ExitCode int` to `ExitCode *int` in `go_shared_libs/contracts/workerapi/workerapi.go`; remove `omitempty` or keep it (nil omits, non-nil emits)."
    status: pending
  - id: imm-062
    content: "Update all consumers in `orchestrator/` and `worker_node/` to dereference the pointer safely (nil check)."
    status: pending
  - id: imm-063
    content: "Re-run the test and confirm green."
    status: pending
  - id: imm-064
    content: "Verify all JSON serialization/deserialization paths handle `*int` correctly; confirm no compile errors across modules."
    status: pending
  - id: imm-065
    content: "Run `go test ./go_shared_libs/...`, `go test ./orchestrator/...`, `go test ./worker_node/...`; confirm 90% threshold."
    status: pending
  - id: imm-066
    content: "Run `just lint-go` on changed files."
    status: pending
  - id: imm-204
    content: "Run `just e2e --tags task,no_inference` and `just e2e --tags sba,no_inference` to verify task result and SBA contract regression."
    status: pending
  - id: imm-067
    content: "Validation gate -- do not proceed to Task 7 until all checks pass."
    status: pending
  - id: imm-068
    content: "Generate task completion report. Mark completed steps `- [x]`."
    status: pending
  - id: imm-069
    content: "Do not start Task 7 until Task 6 closeout is done."
    status: pending
  - id: imm-070
    content: "Read `orchestrator/internal/config/config.go` ~lines 127-137 for hardcoded defaults (`JWTSecret`, `BootstrapAdminPassword`, `NodeRegistrationPSK`, `WorkerAPIBearerToken`)."
    status: pending
  - id: imm-071
    content: "Determine how dev mode is signaled (env var, config flag) to gate the validation."
    status: pending
  - id: imm-072
    content: "Add unit tests: call validation with each insecure default and `dev_mode=false` => error; `dev_mode=true` => allowed."
    status: pending
  - id: imm-073
    content: "Run `go test -v -run TestInsecureDefaults ./orchestrator/internal/config/...` and confirm failure (function missing)."
    status: pending
  - id: imm-074
    content: "Implement `ValidateSecrets(cfg *Config) error` checking each secret against its hardcoded default; error if any match while `DevMode` is false."
    status: pending
  - id: imm-075
    content: "Call `ValidateSecrets` at startup in each orchestrator binary (control-plane, user-gateway, mcp-gateway, api-egress) before serving."
    status: pending
  - id: imm-076
    content: "Re-run the test and confirm green."
    status: pending
  - id: imm-077
    content: "Extract default values into named constants for clarity; keep tests green."
    status: pending
  - id: imm-078
    content: "Run `just lint-go` on changed files and `go test -cover ./orchestrator/internal/config/...`; confirm 90% threshold."
    status: pending
  - id: imm-205
    content: "Run `just e2e --tags gateway,no_inference` to confirm dev-mode stack starts normally with default secrets."
    status: pending
  - id: imm-079
    content: "Validation gate -- do not proceed to Task 8 until all checks pass."
    status: pending
  - id: imm-080
    content: "Generate task completion report. Mark completed steps `- [x]`."
    status: pending
  - id: imm-081
    content: "Do not start Task 8 until Task 7 closeout is done."
    status: pending
  - id: imm-082
    content: "Read `orchestrator/internal/mcpgateway/allowlist.go` lines 29-33 (agent roles), 87-96 (sandbox allowlist), 129-144 (no-token bypass and PM fallthrough)."
    status: pending
  - id: imm-083
    content: "Read `orchestrator/internal/mcpgateway/handlers.go` ~line 769 (system skill mutation guard)."
    status: pending
  - id: imm-084
    content: "Read MCP gateway enforcement spec and access allowlists spec for required PM and PA tool allowlists."
    status: pending
  - id: imm-085
    content: "Add unit tests: (a) no-token request => 401, (b) PM agent restricted to its allowlist, (c) PA agent role recognized with its allowlist, (d) system skill mutation => 403."
    status: pending
  - id: imm-086
    content: "Run `go test -v -run TestAllowlist ./orchestrator/internal/mcpgateway/...` and confirm failures."
    status: pending
  - id: imm-206
    content: "Add or extend `scripts/test_scripts/e2e_0812_mcp_agent_tokens_and_allowlist.py` with E2E cases: no-token rejection, PM allowlist, PA role, system skill mutation rejection."
    status: pending
  - id: imm-087
    content: "In `allowlist.go`: remove no-token bypass; add `AgentRolePA` with its allowlist; implement PM allowlist enforcement."
    status: pending
  - id: imm-088
    content: "In `handlers.go`: fix system skill mutation guard to reject 403 when `skill.IsSystem == true`."
    status: pending
  - id: imm-089
    content: "Re-run `go test -v -run TestAllowlist ./orchestrator/internal/mcpgateway/...` and confirm green."
    status: pending
  - id: imm-090
    content: "Extract allowlist definitions into a separate config/constant block for maintainability; keep tests green."
    status: pending
  - id: imm-091
    content: "Run `go test -cover ./orchestrator/internal/mcpgateway/...`; confirm 90% threshold."
    status: pending
  - id: imm-092
    content: "Run `just lint-go` on changed files."
    status: pending
  - id: imm-093
    content: "Run BDD scenarios tagged `@req_MCPGAT` via `just test-bdd` to confirm no regressions."
    status: pending
  - id: imm-207
    content: "Run `just e2e --tags control_plane,no_inference` to verify MCP gateway and tool-call regression."
    status: pending
  - id: imm-094
    content: "Validation gate -- do not proceed to Task 9 until all checks pass."
    status: pending
  - id: imm-095
    content: "Generate task completion report detailing each sub-issue fixed. Mark completed steps `- [x]`."
    status: pending
  - id: imm-096
    content: "Do not start Task 9 until Task 8 closeout is done."
    status: pending
  - id: imm-097
    content: "Read `orchestrator/internal/models/models.go` (`TaskBase` ~line 226) and confirm `PlanningState` field is missing."
    status: pending
  - id: imm-098
    content: "Read `orchestrator/internal/handlers/tasks.go` `CreateTask` to identify immediate-execution code path."
    status: pending
  - id: imm-099
    content: "Read `orchestrator/internal/handlers/workflow_gate.go` ~lines 55-57 for current start gate logic."
    status: pending
  - id: imm-100
    content: "Add `PlanningState string` field to `TaskBase` in `models.go` with JSON tag and GORM column."
    status: pending
  - id: imm-101
    content: "Add unit tests: (a) `CreateTask` sets `planning_state=draft` and does NOT call immediate execution; (b) workflow gate rejects `planning_state != ready`; (c) transition endpoint moves `draft` to `ready`."
    status: pending
  - id: imm-102
    content: "Run `go test -v -run TestPlanningState ./orchestrator/internal/handlers/...` and confirm failures."
    status: pending
  - id: imm-208
    content: "Add `scripts/test_scripts/e2e_0425_task_planning_state.py` with tags `[suite_cynork, full_demo, task, no_inference]` and prereqs `[gateway, config, auth, node_register]`: create => draft, blocked until approved, approval => ready."
    status: pending
  - id: imm-103
    content: "In `CreateTask`: set `PlanningState = \"draft\"`; guard execution behind `PlanningState == \"ready\"`."
    status: pending
  - id: imm-104
    content: "In `workflow_gate.go`: reject workflow start when `task.PlanningState != \"ready\"`."
    status: pending
  - id: imm-105
    content: "Implement handler to transition `PlanningState` from `draft` to `ready` (the spec-required approval step)."
    status: pending
  - id: imm-106
    content: "Re-run tests and confirm green."
    status: pending
  - id: imm-107
    content: "Define `PlanningStateDraft` and `PlanningStateReady` as typed constants."
    status: pending
  - id: imm-108
    content: "Update existing tests expecting immediate execution on creation to reflect draft-first behavior."
    status: pending
  - id: imm-109
    content: "Run `go test -cover ./orchestrator/...`; confirm 90% threshold."
    status: pending
  - id: imm-110
    content: "Run `just test-bdd` for orchestrator BDD scenarios to check regressions."
    status: pending
  - id: imm-111
    content: "Run `just lint-go` on all changed files."
    status: pending
  - id: imm-209
    content: "Update existing task E2E tests (`e2e_0420_task_create.py` and downstream) to account for draft-first behavior."
    status: pending
  - id: imm-210
    content: "Run `just e2e --tags task,no_inference` to verify task lifecycle regression with `planning_state`."
    status: pending
  - id: imm-112
    content: "Validation gate -- do not proceed to Task 10 until all checks pass."
    status: pending
  - id: imm-113
    content: "Generate task completion report covering model, handler, and gate changes. Mark completed steps `- [x]`."
    status: pending
  - id: imm-114
    content: "Do not start Task 10 until Task 9 closeout is done."
    status: pending
  - id: imm-115
    content: "Read `worker_node/internal/executor/executor.go`: `buildSandboxRunArgsForPod` (~lines 753-771), `buildSBARunArgsForPod` (~lines 660-695), pod creation (~line 190)."
    status: pending
  - id: imm-116
    content: "Determine target architecture: (a) pods with `--network=none` + restructured proxy, or (b) proxy outside pod while sandbox pod is isolated. Document choice before implementing."
    status: pending
  - id: imm-117
    content: "Add an integration test verifying a sandbox container cannot reach external hosts directly (e.g., TCP connect to external IP fails)."
    status: pending
  - id: imm-118
    content: "Run the test and confirm failure (sandbox can currently reach external hosts)."
    status: pending
  - id: imm-211
    content: "Add `scripts/test_scripts/e2e_0325_sandbox_network_isolation.py` with tags `[suite_worker_node, full_demo, worker, no_inference]` and prereqs `[gateway, config, auth, node_register]`: sandbox cannot reach external hosts."
    status: pending
  - id: imm-119
    content: "Implement chosen network isolation: modify pod creation args and proxy sidecar config so sandbox has no direct external network path."
    status: pending
  - id: imm-120
    content: "Re-run the isolation test and confirm green."
    status: pending
  - id: imm-121
    content: "Ensure UDS-based proxy communication between sandbox and sidecar is not broken by the change."
    status: pending
  - id: imm-122
    content: "Run `go test -cover ./worker_node/...`; confirm 90% threshold."
    status: pending
  - id: imm-123
    content: "Run BDD scenarios tagged `@req_WORKER_0174` via `just test-bdd`."
    status: pending
  - id: imm-124
    content: "Run `just lint-go` on changed files."
    status: pending
  - id: imm-212
    content: "Run `just e2e --tags worker,no_inference` to verify worker node regression."
    status: pending
  - id: imm-125
    content: "Validation gate -- do not proceed to Task 11 until all checks pass."
    status: pending
  - id: imm-126
    content: "Generate task completion report documenting chosen isolation architecture. Mark completed steps `- [x]`."
    status: pending
  - id: imm-127
    content: "Do not start Task 11 until Task 10 closeout is done."
    status: pending
  - id: imm-128
    content: "Read `agents/internal/sba/agent.go` `buildUserPrompt` (~lines 396-460) and map current context ordering."
    status: pending
  - id: imm-129
    content: "Read `go_shared_libs/contracts/sbajob/sbajob.go` `ContextSpec` (~lines 57-69) and confirm missing persona fields."
    status: pending
  - id: imm-130
    content: "Read REQ-SBAGNT-0113 for required ordering: persona, baseline, project, task, requirements, preferences, additional context, skills, runtime."
    status: pending
  - id: imm-131
    content: "Add `PersonaTitle` and `PersonaDescription` fields to `ContextSpec` in `go_shared_libs/contracts/sbajob/sbajob.go`."
    status: pending
  - id: imm-132
    content: "Add a unit test providing full `ContextSpec` (persona, skills, preferences) and asserting prompt contains all blocks in required order."
    status: pending
  - id: imm-133
    content: "Run `go test -v -run TestBuildUserPrompt ./agents/internal/sba/...` and confirm failure (wrong order, missing blocks)."
    status: pending
  - id: imm-134
    content: "Rewrite `buildUserPrompt` to emit context blocks in spec order: persona => baseline => project => task => requirements => preferences => additional context => skills => runtime."
    status: pending
  - id: imm-135
    content: "Add rendering for `Preferences` and `Skills` fields from `ContextSpec`."
    status: pending
  - id: imm-136
    content: "Re-run the test and confirm green."
    status: pending
  - id: imm-137
    content: "Extract each context block renderer into a named helper for testability."
    status: pending
  - id: imm-138
    content: "Run `go test -cover ./agents/internal/sba/...` and `go test -cover ./go_shared_libs/...`; confirm 90% threshold."
    status: pending
  - id: imm-139
    content: "Run `just lint-go` on changed files."
    status: pending
  - id: imm-213
    content: "Run `just e2e --tags sba,no_inference` to verify SBA task result contract regression."
    status: pending
  - id: imm-140
    content: "Validation gate -- do not proceed to Task 12 until all checks pass."
    status: pending
  - id: imm-141
    content: "Generate task completion report mapping each REQ-SBAGNT-0113 block to its implementation. Mark completed steps `- [x]`."
    status: pending
  - id: imm-142
    content: "Do not start Task 12 until Task 11 closeout is done."
    status: pending
  - id: imm-143
    content: "Read `agents/internal/pma/chat.go` and `agents/cmd/cynode-pma/main.go` to identify where keep-warm goroutine wires in."
    status: pending
  - id: imm-144
    content: "Read `agents/internal/pma/streaming_fsm.go` ~lines 203-216 (dead overwrite helpers)."
    status: pending
  - id: imm-145
    content: "Read REQ-PMAGNT-0125, REQ-PMAGNT-0129, REQ-PMAGNT-0124 for exact behavioral requirements."
    status: pending
  - id: imm-146
    content: "Add unit tests for keep-warm: background goroutine sends minimal inference requests at configurable interval (default 300s); stops on context cancellation."
    status: pending
  - id: imm-147
    content: "Add unit tests for secret scan: after each langchaingo iteration, accumulated buffers are scanned; overwrite event queued if secrets detected."
    status: pending
  - id: imm-148
    content: "Add unit tests for overwrite events: `{\"overwrite\": {...}}` NDJSON events are emitted on the stream."
    status: pending
  - id: imm-149
    content: "Run `go test -v -run 'TestKeepWarm|TestSecretScan|TestOverwrite' ./agents/internal/pma/...` and confirm failures."
    status: pending
  - id: imm-150
    content: "Implement keep-warm: background goroutine in `run()` with `time.Ticker` sending minimal Ollama requests; wire lifecycle shutdown via context."
    status: pending
  - id: imm-151
    content: "Implement secret scan: after each langchaingo iteration, scan visible text, thinking, and tool-call buffers for secret patterns."
    status: pending
  - id: imm-152
    content: "Wire overwrite events: connect `iterationOverwriteReplace` and `turnOverwriteReplace` to the streaming pipeline; emit `{\"overwrite\": {...}}` NDJSON."
    status: pending
  - id: imm-153
    content: "Re-run all tests and confirm green."
    status: pending
  - id: imm-154
    content: "Remove dead code now that overwrite helpers are wired; ensure FSM transitions are tested."
    status: pending
  - id: imm-155
    content: "Run `go test -cover ./agents/...`; confirm 90% threshold."
    status: pending
  - id: imm-156
    content: "Run BDD scenarios tagged `@req_PMAGNT_0124`, `@req_PMAGNT_0125`, `@req_PMAGNT_0129` via `just test-bdd`."
    status: pending
  - id: imm-157
    content: "Run `just lint-go` on all changed files."
    status: pending
  - id: imm-214
    content: "Run `just e2e --tags pma_inference,streaming` to verify PMA streaming regression (requires inference; skip if unavailable)."
    status: pending
  - id: imm-158
    content: "Validation gate -- do not proceed to Task 13 until all checks pass."
    status: pending
  - id: imm-159
    content: "Generate task completion report covering all three features. Mark completed steps `- [x]`."
    status: pending
  - id: imm-160
    content: "Do not start Task 13 until Task 12 closeout is done."
    status: pending
  - id: imm-161
    content: "Read `.github/workflows/ci.yml` and compare jobs against `just ci` targets; identify gaps."
    status: pending
  - id: imm-162
    content: "Run `just ci` locally and confirm all targets pass with changes from Tasks 1-12."
    status: pending
  - id: imm-163
    content: "If any `just ci` target is missing from the workflow, note it as a gap to fix."
    status: pending
  - id: imm-164
    content: "Add any missing CI jobs; ensure branch triggers include the working branch (not just `main`/`master`)."
    status: pending
  - id: imm-165
    content: "Verify workflow triggers on current working branch."
    status: pending
  - id: imm-166
    content: "Review job parallelism and caching; ensure Go module cache is shared where possible."
    status: pending
  - id: imm-167
    content: "Run `just ci` locally and confirm all targets pass."
    status: pending
  - id: imm-215
    content: "Run `just e2e --tags no_inference` for full fast E2E regression across all non-inference tests."
    status: pending
  - id: imm-168
    content: "Run `just docs-check` to validate all documentation changes."
    status: pending
  - id: imm-169
    content: "Validation gate -- do not proceed to Task 14 until all checks pass."
    status: pending
  - id: imm-170
    content: "Generate task completion report. Mark completed steps `- [x]`."
    status: pending
  - id: imm-171
    content: "Do not start Task 14 until Task 13 closeout is done."
    status: pending
  - id: imm-172
    content: "Update `docs/dev_docs/_todo.md` to mark all 13 Immediate items as complete."
    status: pending
  - id: imm-173
    content: "Verify no follow-up work was left undocumented."
    status: pending
  - id: imm-174
    content: "Run `just docs-check` on all changed documentation."
    status: pending
  - id: imm-216
    content: "Run `just e2e --tags no_inference` as final E2E regression gate."
    status: pending
  - id: imm-175
    content: "Generate final plan completion report: tasks completed, overall validation, remaining risks."
    status: pending
  - id: imm-176
    content: "Mark all completed steps in the plan with `- [x]`. (Last step.)"
    status: pending
---

# Immediate Security and Correctness Plan

## Goal

Address 13 critical security and correctness issues from review reports 1-6 before any production deployment.
Tasks are ordered from simplest/most independent to most complex, finishing with CI validation and documentation closeout.

## References

- Requirements: [`docs/requirements/orches.md`](../requirements/orches.md), [`docs/requirements/worker.md`](../requirements/worker.md), [`docs/requirements/pmagnt.md`](../requirements/pmagnt.md), [`docs/requirements/sbagnt.md`](../requirements/sbagnt.md), [`docs/requirements/mcpgat.md`](../requirements/mcpgat.md)
- Tech specs: [`docs/tech_specs/orchestrator.md`](../tech_specs/orchestrator.md), [`docs/tech_specs/worker_node.md`](../tech_specs/worker_node.md), [`docs/tech_specs/cynode_pma.md`](../tech_specs/cynode_pma.md), [`docs/tech_specs/cynode_sba.md`](../tech_specs/cynode_sba.md), [`docs/tech_specs/mcp/mcp_gateway_enforcement.md`](../tech_specs/mcp/mcp_gateway_enforcement.md)
- Review reports: [`2026-03-29_review_report_1_orchestrator.md`](2026-03-29_review_report_1_orchestrator.md) through [`2026-03-29_review_report_6_testing.md`](2026-03-29_review_report_6_testing.md)
- Implementation: `orchestrator/`, `worker_node/`, `agents/`, `cynork/`, `go_shared_libs/`

## Constraints

- Requirements take precedence over tech specs; tech specs take precedence over current code.
- Follow BDD/TDD: failing tests before implementation.
- All changes must pass `just ci` before the task is considered complete.
- No changes that reduce test coverage below the 90% per-package threshold.
- No modifications to linter rules or suppression comments.
- Each task's validation gate must pass before starting the next task.

## Execution Plan

Tasks are ordered from simplest/most independent to most complex.
Earlier tasks have no dependency on later ones.

### Task 1: Fix PMA WriteTimeout

PMA `WriteTimeout` (120s) is less than the inference timeout (300s), causing streaming responses to be silently killed mid-stream.

#### Task 1 Requirements and Specifications

- [REQ-PMAGNT-0106](../requirements/pmagnt.md#req-pmagnt-0106) -- PMA streaming
- [`docs/tech_specs/cynode_pma.md`](../tech_specs/cynode_pma.md)
- [Review Report 3 section 2.2](2026-03-29_review_report_3_agents.md#22-high-severity-gaps)

#### Discovery (Task 1) Steps

- [ ] Read `agents/cmd/cynode-pma/main.go` (WriteTimeout ~line 93) and `agents/internal/pma/chat.go` (`pmaLangchainCompletionTimeout` ~line 30); confirm the mismatch.
- [ ] Read PMA tech spec ([`docs/tech_specs/cynode_pma.md`](../tech_specs/cynode_pma.md)) for timeout requirements.

#### Red (Task 1)

- [ ] Add a test asserting HTTP server WriteTimeout is zero or >= inference timeout + 10s margin.
- [ ] Run `go test -v -run TestWriteTimeout ./agents/cmd/cynode-pma/...` and confirm the test fails.

#### Green (Task 1)

- [ ] Set WriteTimeout to `0` (disabled for streaming) in `agents/cmd/cynode-pma/main.go`.
- [ ] Re-run `go test -v -run TestWriteTimeout ./agents/cmd/cynode-pma/...` and confirm green.

#### Refactor (Task 1)

- [ ] Extract the timeout into a named constant if not already defined; keep tests green.

#### Testing (Task 1)

- [ ] Run `just lint-go` on changed files and `go test -cover ./agents/...`; confirm 90% threshold.
- [ ] Run `just e2e --tags streaming,pma_inference` to verify PMA streaming is not truncated after timeout change (requires inference; skip if unavailable).
- [ ] Validation gate -- do not proceed to Task 2 until all checks pass.

#### Closeout (Task 1)

- [ ] Generate task completion report (changes, tests passed, deviations).
  Mark completed steps `- [x]`.
- [ ] Do not start Task 2 until Task 1 closeout is done.

---

### Task 2: Fix Container Name Matching in Worker Node

`startOneManagedService` uses `strings.Contains` for container detection, causing false positives when one container name is a prefix of another.

#### Task 2 Requirements and Specifications

- [`docs/tech_specs/worker_node.md`](../tech_specs/worker_node.md) -- managed service lifecycle
- [Review Report 2 section 3.1](2026-03-29_review_report_2_worker_node.md#31-node-agent)

#### Discovery (Task 2) Steps

- [ ] Read `worker_node/cmd/node-manager/main.go` ~line 599 (`startOneManagedService` with `strings.Contains`) and ~line 408 (`containerNameExact`).
- [ ] Identify all callers of `startOneManagedService` to assess impact of the fix.

#### Red (Task 2)

- [ ] Add a unit test exercising container name matching with a prefix collision (e.g., `cynodeai-managed-pma` vs `cynodeai-managed-pma-test`).
- [ ] Run `go test -v -run TestContainerNameMatch ./worker_node/cmd/node-manager/...` and confirm the test fails.

#### Green (Task 2)

- [ ] Replace `strings.Contains(string(out), name)` in `startOneManagedService` with `containerNameExact`.
- [ ] Re-run the test and confirm green.

#### Refactor (Task 2)

- [ ] Consolidate `containerNameExact` and `containerNameMatches` into a single shared function.

#### Testing (Task 2)

- [ ] Run `just lint-go` on changed files and `go test -cover ./worker_node/...`; confirm 90% threshold.
- [ ] Run `just e2e --tags worker,no_inference` to verify managed service startup regression.
- [ ] Validation gate -- do not proceed to Task 3 until all checks pass.

#### Closeout (Task 2)

- [ ] Generate task completion report.
  Mark completed steps `- [x]`.
- [ ] Do not start Task 3 until Task 2 closeout is done.

---

### Task 3: Fix Cynork `runEnsureThread` Data Race

`runEnsureThread` writes `Session.CurrentThreadID` from a `tea.Cmd` goroutine while `View()` reads it concurrently.

#### Task 3 Requirements and Specifications

- [`docs/tech_specs/cynork_cli.md`](../tech_specs/cynork/cynork_cli.md) -- thread management
- [Review Report 4 section 4.1](2026-03-29_review_report_4_cynork.md#41-critical-severity)

#### Discovery (Task 3) Steps

- [ ] Read `cynork/internal/tui/model.go` ~lines 774-791 (`runEnsureThread`) and the `applyEnsureThreadResult` handler.
- [ ] Identify all reads of `Session.CurrentThreadID` in `View()` and goroutine-accessed paths.

#### Red (Task 3)

- [ ] Add a test using `go test -race` on the TUI thread-initialization path to detect the race.
- [ ] Run `go test -race -v -run TestEnsureThread ./cynork/internal/tui/...` and confirm the race is detected.

#### Green (Task 3)

- [ ] Move all `Session.CurrentThreadID` mutations into `applyEnsureThreadResult`; have the goroutine return the resolved thread ID as data without writing model fields.
- [ ] Re-run `go test -race -v -run TestEnsureThread ./cynork/internal/tui/...` and confirm no race.

#### Refactor (Task 3)

- [ ] Verify no other `tea.Cmd` closures directly mutate model fields; document the pattern if recurring.

#### Testing (Task 3)

- [ ] Run `go test -race ./cynork/...` and `go test -cover ./cynork/...`; confirm 90% threshold.
- [ ] Run `just e2e --tags tui_pty,no_inference` to verify TUI thread-initialization regression.
- [ ] Validation gate -- do not proceed to Task 4 until all checks pass.

#### Closeout (Task 3)

- [ ] Generate task completion report.
  Mark completed steps `- [x]`.
- [ ] Do not start Task 4 until Task 3 closeout is done.

---

### Task 4: Add `Close()` to Secure Store

The secure store has no `Close()` method; AES master key and ML-KEM decapsulation key persist in heap memory for the process lifetime.

#### Task 4 Requirements and Specifications

- [`docs/tech_specs/worker_node.md`](../tech_specs/worker_node.md) -- secure store
- [Review Report 2 section 5.1](2026-03-29_review_report_2_worker_node.md#51-critical-severity)

#### Discovery (Task 4) Steps

- [ ] Read `worker_node/internal/securestore/store.go` struct definition (~lines 75-80); identify `key` and `kemKey` fields.
- [ ] Identify all call sites that create a `securestore.Store` to wire `Close()` into shutdown paths.

#### Red (Task 4)

- [ ] Add a unit test: create a Store, call `Close()`, verify key material is zeroed (all bytes == 0).
- [ ] Run `go test -v -run TestStoreClose ./worker_node/internal/securestore/...` and confirm failure (method missing).

#### Green (Task 4)

- [ ] Implement `func (s *Store) Close()` that zeros `s.key` and `s.kemKey`.
- [ ] Wire `defer store.Close()` into `worker_node/cmd/node-manager/main.go` shutdown path.
- [ ] Re-run the test and confirm green.

#### Refactor (Task 4)

- [ ] Ensure zeroing is not optimizable away by the compiler (use `runtime.KeepAlive` or similar).

#### Testing (Task 4)

- [ ] Run `just lint-go` on changed files and `go test -cover ./worker_node/internal/securestore/...`; confirm 90% threshold.
- [ ] Validation gate -- do not proceed to Task 5 until all checks pass.

#### Closeout (Task 4)

- [ ] Generate task completion report.
  Mark completed steps `- [x]`.
- [ ] Do not start Task 5 until Task 4 closeout is done.

---

### Task 5: Replace All `!=` Token Comparisons With `subtle.ConstantTimeCompare`

Token comparisons using `!=` in api-egress, workflow middleware, and worker API embed handlers are vulnerable to timing side-channel attacks.

#### Task 5 Requirements and Specifications

- [Review Report 1 section 5.1](2026-03-29_review_report_1_orchestrator.md#51-critical-severity) -- orchestrator
- [Review Report 2 section 5.2](2026-03-29_review_report_2_worker_node.md#52-high-severity) -- worker node

#### Discovery (Task 5) Steps

- [ ] Search all Go modules for `!=` token/bearer comparisons: `orchestrator/cmd/api-egress/main.go:155`, `orchestrator/internal/middleware/auth.go:148`, `worker_node/internal/workerapiserver/embed_handlers.go:280,405`.
  Identify any additional sites.

#### Red (Task 5)

- [ ] Add or update unit tests at each site verifying token comparison rejects incorrect tokens and accepts correct ones.
- [ ] Run targeted tests: `go test -v -run TestTokenAuth ./orchestrator/cmd/api-egress/...`, `go test -v -run TestWorkflowAuth ./orchestrator/internal/middleware/...`, `go test -v -run TestBearerAuth ./worker_node/internal/workerapiserver/...`.

#### Green (Task 5)

- [ ] Replace each `got != token` with `subtle.ConstantTimeCompare([]byte(got), []byte(expected)) != 1`; add `import "crypto/subtle"` where missing.
- [ ] Re-run all targeted tests and confirm green.

#### Refactor (Task 5)

- [ ] If multiple modules duplicate the pattern, extract a shared helper (e.g., `secretutil.TokenEquals`) into `go_shared_libs`.

#### Testing (Task 5)

- [ ] Run `go test -cover ./orchestrator/...` and `go test -cover ./worker_node/...`; confirm 90% threshold.
- [ ] Run `just lint-go` on all changed files.
- [ ] Run `just e2e --tags auth,no_inference` and `just e2e --tags worker,no_inference` to verify bearer auth regression across orchestrator and worker.
- [ ] Validation gate -- do not proceed to Task 6 until all checks pass.

#### Closeout (Task 5)

- [ ] Generate task completion report listing each file changed and old vs new pattern.
  Mark completed steps `- [x]`.
- [ ] Do not start Task 6 until Task 5 closeout is done.

---

### Task 6: Change `RunJobResponse.ExitCode` From `int` to `*int`

`RunJobResponse.ExitCode` uses `int` with `omitempty`, causing exit code 0 (success) to be silently dropped from JSON responses.

#### Task 6 Requirements and Specifications

- [`go_shared_libs/contracts/workerapi/workerapi.go`](../../go_shared_libs/contracts/workerapi/workerapi.go) -- `RunJobResponse`
- [Review Report 5 section 2.1](2026-03-29_review_report_5_shared_libs.md#21-critical-severity)

#### Discovery (Task 6) Steps

- [ ] Read `go_shared_libs/contracts/workerapi/workerapi.go` ~line 69 (`ExitCode int`).
- [ ] Search `orchestrator/` and `worker_node/` for all consumers of `RunJobResponse.ExitCode` that must handle `*int`.

#### Red (Task 6)

- [ ] Add a unit test: marshal `RunJobResponse` with ExitCode=0, unmarshal, assert `exit_code` field is present and equals 0.
- [ ] Run `go test -v -run TestExitCodeZero ./go_shared_libs/contracts/workerapi/...` and confirm failure.

#### Green (Task 6)

- [ ] Change `ExitCode int` to `ExitCode *int` in `go_shared_libs/contracts/workerapi/workerapi.go`; remove `omitempty` or keep it (nil omits, non-nil emits).
- [ ] Update all consumers in `orchestrator/` and `worker_node/` to dereference the pointer safely (nil check).
- [ ] Re-run the test and confirm green.

#### Refactor (Task 6)

- [ ] Verify all JSON serialization/deserialization paths handle `*int` correctly; confirm no compile errors across modules.

#### Testing (Task 6)

- [ ] Run `go test ./go_shared_libs/...`, `go test ./orchestrator/...`, `go test ./worker_node/...`; confirm 90% threshold.
- [ ] Run `just lint-go` on changed files.
- [ ] Run `just e2e --tags task,no_inference` and `just e2e --tags sba,no_inference` to verify task result and SBA contract regression.
- [ ] Validation gate -- do not proceed to Task 7 until all checks pass.

#### Closeout (Task 6)

- [ ] Generate task completion report.
  Mark completed steps `- [x]`.
- [ ] Do not start Task 7 until Task 6 closeout is done.

---

### Task 7: Add Startup Validation Rejecting Insecure Defaults

Orchestrator ships hardcoded dev defaults for JWT secret, admin password, and PSK tokens with no fail-fast outside dev mode.

#### Task 7 Requirements and Specifications

- [`docs/tech_specs/orchestrator.md`](../tech_specs/orchestrator.md) -- configuration and security
- [Review Report 1 section 5.1](2026-03-29_review_report_1_orchestrator.md#51-critical-severity)

#### Discovery (Task 7) Steps

- [ ] Read `orchestrator/internal/config/config.go` ~lines 127-137 for hardcoded defaults (`JWTSecret`, `BootstrapAdminPassword`, `NodeRegistrationPSK`, `WorkerAPIBearerToken`).
- [ ] Determine how dev mode is signaled (env var, config flag) to gate the validation.

#### Red (Task 7)

- [ ] Add unit tests: call validation with each insecure default and `dev_mode=false` => error; `dev_mode=true` => allowed.
- [ ] Run `go test -v -run TestInsecureDefaults ./orchestrator/internal/config/...` and confirm failure (function missing).

#### Green (Task 7)

- [ ] Implement `ValidateSecrets(cfg *Config) error` checking each secret against its hardcoded default; error if any match while `DevMode` is false.
- [ ] Call `ValidateSecrets` at startup in each orchestrator binary (control-plane, user-gateway, mcp-gateway, api-egress) before serving.
- [ ] Re-run the test and confirm green.

#### Refactor (Task 7)

- [ ] Extract default values into named constants for clarity; keep tests green.

#### Testing (Task 7)

- [ ] Run `just lint-go` on changed files and `go test -cover ./orchestrator/internal/config/...`; confirm 90% threshold.
- [ ] Run `just e2e --tags gateway,no_inference` to confirm dev-mode stack starts normally with default secrets.
- [ ] Validation gate -- do not proceed to Task 8 until all checks pass.

#### Closeout (Task 7)

- [ ] Generate task completion report.
  Mark completed steps `- [x]`.
- [ ] Do not start Task 8 until Task 7 closeout is done.

---

### Task 8: Fix Authorization Fail-Open in MCP Gateway

Three sub-issues: (a) no-token requests bypass allowlist enforcement, (b) PM agents have unrestricted tool access, (c) system skills are mutable by any user.

#### Task 8 Requirements and Specifications

- [REQ-MCPGAT-0106](../requirements/mcpgat.md#req-mcpgat-0106) -- fail closed
- [REQ-MCPGAT-0114](../requirements/mcpgat.md#req-mcpgat-0114) -- agent allowlists (PM, PA, sandbox)
- [`docs/tech_specs/mcp/mcp_gateway_enforcement.md`](../tech_specs/mcp/mcp_gateway_enforcement.md)
- [Review Report 1 section 2.1](2026-03-29_review_report_1_orchestrator.md#21-critical-gaps)

#### Discovery (Task 8) Steps

- [ ] Read `orchestrator/internal/mcpgateway/allowlist.go` lines 29-33 (agent roles), 87-96 (sandbox allowlist), 129-144 (no-token bypass and PM fallthrough).
- [ ] Read `orchestrator/internal/mcpgateway/handlers.go` ~line 769 (system skill mutation guard).
- [ ] Read MCP gateway enforcement spec and access allowlists spec for required PM and PA tool allowlists.

#### Red (Task 8)

- [ ] Add unit tests: (a) no-token request => 401, (b) PM agent restricted to its allowlist, (c) PA agent role recognized with its allowlist, (d) system skill mutation => 403.
- [ ] Run `go test -v -run TestAllowlist ./orchestrator/internal/mcpgateway/...` and confirm failures.
- [ ] Add or extend `scripts/test_scripts/e2e_0812_mcp_agent_tokens_and_allowlist.py` with E2E cases: no-token rejection, PM allowlist enforcement, PA role recognition, system skill mutation rejection.

#### Green (Task 8)

- [ ] In `allowlist.go`: remove no-token bypass; add `AgentRolePA` with its allowlist; implement PM allowlist enforcement.
- [ ] In `handlers.go`: fix system skill mutation guard to reject 403 when `skill.IsSystem == true`.
- [ ] Re-run `go test -v -run TestAllowlist ./orchestrator/internal/mcpgateway/...` and confirm green.

#### Refactor (Task 8)

- [ ] Extract allowlist definitions into a separate config/constant block for maintainability; keep tests green.

#### Testing (Task 8)

- [ ] Run `go test -cover ./orchestrator/internal/mcpgateway/...`; confirm 90% threshold.
- [ ] Run `just lint-go` on changed files.
- [ ] Run BDD scenarios tagged `@req_MCPGAT` via `just test-bdd` to confirm no regressions.
- [ ] Run `just e2e --tags control_plane,no_inference` to verify MCP gateway and tool-call regression.
- [ ] Validation gate -- do not proceed to Task 9 until all checks pass.

#### Closeout (Task 8)

- [ ] Generate task completion report detailing each sub-issue fixed.
  Mark completed steps `- [x]`.
- [ ] Do not start Task 9 until Task 8 closeout is done.

---

### Task 9: Implement `planning_state` on TaskBase

Tasks execute immediately on creation instead of entering `draft` state for PMA review, violating REQ-ORCHES-0176/0177/0178.

#### Task 9 Requirements and Specifications

- [REQ-ORCHES-0176](../requirements/orches.md#req-orches-0176) -- `planning_state=draft` on create
- [REQ-ORCHES-0177](../requirements/orches.md#req-orches-0177) -- create MUST NOT start workflow
- [REQ-ORCHES-0178](../requirements/orches.md#req-orches-0178) -- only `planning_state=ready` may start workflow
- [`docs/tech_specs/orchestrator.md`](../tech_specs/orchestrator.md) -- task lifecycle
- [Review Report 1 section 2.1](2026-03-29_review_report_1_orchestrator.md#21-critical-gaps)

#### Discovery (Task 9) Steps

- [ ] Read `orchestrator/internal/models/models.go` (`TaskBase` ~line 226) and confirm `PlanningState` field is missing.
- [ ] Read `orchestrator/internal/handlers/tasks.go` `CreateTask` to identify immediate-execution code path.
- [ ] Read `orchestrator/internal/handlers/workflow_gate.go` ~lines 55-57 for current start gate logic.

#### Red (Task 9)

- [ ] Add `PlanningState string` field to `TaskBase` in `models.go` with JSON tag and GORM column.
- [ ] Add unit tests: (a) `CreateTask` sets `planning_state=draft` and does NOT call immediate execution; (b) workflow gate rejects `planning_state != ready`; (c) transition endpoint moves `draft` to `ready`.
- [ ] Run `go test -v -run TestPlanningState ./orchestrator/internal/handlers/...` and confirm failures.
- [ ] Add `scripts/test_scripts/e2e_0425_task_planning_state.py` with tags `[suite_cynork, full_demo, task, no_inference]` and prereqs `[gateway, config, auth, node_register]`: create returns `planning_state=draft`, task blocked until approved, approval transitions to `ready`.

#### Green (Task 9)

- [ ] In `CreateTask`: set `PlanningState = "draft"`; guard execution behind `PlanningState == "ready"`.
- [ ] In `workflow_gate.go`: reject workflow start when `task.PlanningState != "ready"`.
- [ ] Implement handler to transition `PlanningState` from `draft` to `ready` (the spec-required approval step).
- [ ] Re-run tests and confirm green.

#### Refactor (Task 9)

- [ ] Define `PlanningStateDraft` and `PlanningStateReady` as typed constants.
- [ ] Update existing tests expecting immediate execution on creation to reflect draft-first behavior.

#### Testing (Task 9)

- [ ] Run `go test -cover ./orchestrator/...`; confirm 90% threshold.
- [ ] Run `just test-bdd` for orchestrator BDD scenarios to check regressions.
- [ ] Run `just lint-go` on all changed files.
- [ ] Update existing task E2E tests (`e2e_0420_task_create.py` and downstream) to account for draft-first behavior where they assume immediate execution.
- [ ] Run `just e2e --tags task,no_inference` to verify task lifecycle regression with `planning_state`.
- [ ] Validation gate -- do not proceed to Task 10 until all checks pass.

#### Closeout (Task 9)

- [ ] Generate task completion report covering model, handler, and gate changes.
  Mark completed steps `- [x]`.
- [ ] Do not start Task 10 until Task 9 closeout is done.

---

### Task 10: Fix Pod Network Isolation for Sandbox Containers

Pod-based sandbox containers share the pod's network namespace, enabling direct internet access that bypasses the worker proxy.

#### Task 10 Requirements and Specifications

- [REQ-WORKER-0174](../requirements/worker.md#req-worker-0174) -- all traffic must route through worker proxies
- [`docs/tech_specs/worker_node.md`](../tech_specs/worker_node.md) -- sandbox isolation
- [Review Report 2 section 2.1](2026-03-29_review_report_2_worker_node.md#21-critical-gaps)

#### Discovery (Task 10) Steps

- [ ] Read `worker_node/internal/executor/executor.go`: `buildSandboxRunArgsForPod` (~lines 753-771), `buildSBARunArgsForPod` (~lines 660-695), pod creation (~line 190).
- [ ] Determine target architecture: (a) pods with `--network=none` + restructured proxy, or (b) proxy outside pod while sandbox pod is isolated.
  Document choice before implementing.

#### Red (Task 10)

- [ ] Add an integration test verifying a sandbox container cannot reach external hosts directly (e.g., TCP connect to external IP fails).
- [ ] Run the test and confirm failure (sandbox can currently reach external hosts).
- [ ] Add `scripts/test_scripts/e2e_0325_sandbox_network_isolation.py` with tags `[suite_worker_node, full_demo, worker, no_inference]` and prereqs `[gateway, config, auth, node_register]`: verify sandbox container cannot reach external hosts bypassing proxy.

#### Green (Task 10)

- [ ] Implement chosen network isolation: modify pod creation args and proxy sidecar config so sandbox has no direct external network path.
- [ ] Re-run the isolation test and confirm green.

#### Refactor (Task 10)

- [ ] Ensure UDS-based proxy communication between sandbox and sidecar is not broken by the change.

#### Testing (Task 10)

- [ ] Run `go test -cover ./worker_node/...`; confirm 90% threshold.
- [ ] Run BDD scenarios tagged `@req_WORKER_0174` via `just test-bdd`.
- [ ] Run `just lint-go` on changed files.
- [ ] Run `just e2e --tags worker,no_inference` to verify worker node regression.
- [ ] Validation gate -- do not proceed to Task 11 until all checks pass.

#### Closeout (Task 10)

- [ ] Generate task completion report documenting chosen isolation architecture.
  Mark completed steps `- [x]`.
- [ ] Do not start Task 11 until Task 10 closeout is done.

---

### Task 11: Fix SBA Prompt Construction

SBA prompt construction violates REQ-SBAGNT-0113: persona, skills, and preferences are omitted; context ordering is wrong.

#### Task 11 Requirements and Specifications

- [REQ-SBAGNT-0113](../requirements/sbagnt.md#req-sbagnt-0113) -- context ordering
- [REQ-SBAGNT-0111](../requirements/sbagnt.md#req-sbagnt-0111) -- preferences rendering
- [`docs/tech_specs/cynode_sba.md`](../tech_specs/cynode_sba.md)
- [Review Report 3 section 2.1](2026-03-29_review_report_3_agents.md#21-critical-gaps)

#### Discovery (Task 11) Steps

- [ ] Read `agents/internal/sba/agent.go` `buildUserPrompt` (~lines 396-460) and map current context ordering.
- [ ] Read `go_shared_libs/contracts/sbajob/sbajob.go` `ContextSpec` (~lines 57-69) and confirm missing persona fields.
- [ ] Read REQ-SBAGNT-0113 for required ordering: persona, baseline, project, task, requirements, preferences, additional context, skills, runtime.

#### Red (Task 11)

- [ ] Add `PersonaTitle` and `PersonaDescription` fields to `ContextSpec` in `go_shared_libs/contracts/sbajob/sbajob.go`.
- [ ] Add a unit test providing full `ContextSpec` (persona, skills, preferences) and asserting prompt contains all blocks in required order.
- [ ] Run `go test -v -run TestBuildUserPrompt ./agents/internal/sba/...` and confirm failure (wrong order, missing blocks).

#### Green (Task 11)

- [ ] Rewrite `buildUserPrompt` to emit context blocks in spec order: persona => baseline => project => task => requirements => preferences => additional context => skills => runtime.
- [ ] Add rendering for `Preferences` and `Skills` fields from `ContextSpec`.
- [ ] Re-run the test and confirm green.

#### Refactor (Task 11)

- [ ] Extract each context block renderer into a named helper for testability.

#### Testing (Task 11)

- [ ] Run `go test -cover ./agents/internal/sba/...` and `go test -cover ./go_shared_libs/...`; confirm 90% threshold.
- [ ] Run `just lint-go` on changed files.
- [ ] Run `just e2e --tags sba,no_inference` to verify SBA task result contract regression.
- [ ] Validation gate -- do not proceed to Task 12 until all checks pass.

#### Closeout (Task 11)

- [ ] Generate task completion report mapping each REQ-SBAGNT-0113 block to its implementation.
  Mark completed steps `- [x]`.
- [ ] Do not start Task 12 until Task 11 closeout is done.

---

### Task 12: Implement PMA Keep-Warm, Secret Scan, and Overwrite Events

Three unimplemented PMA streaming features: model keep-warm (REQ-PMAGNT-0129), opportunistic secret scan (REQ-PMAGNT-0125), and overwrite NDJSON events (REQ-PMAGNT-0124).

#### Task 12 Requirements and Specifications

- [REQ-PMAGNT-0129](../requirements/pmagnt.md#req-pmagnt-0129) -- keep-warm
- [REQ-PMAGNT-0125](../requirements/pmagnt.md#req-pmagnt-0125) -- secret scan
- [REQ-PMAGNT-0124](../requirements/pmagnt.md#req-pmagnt-0124) -- overwrite events
- [`docs/tech_specs/cynode_pma.md`](../tech_specs/cynode_pma.md)
- [Review Report 3 section 2.1](2026-03-29_review_report_3_agents.md#21-critical-gaps)

#### Discovery (Task 12) Steps

- [ ] Read `agents/internal/pma/chat.go` and `agents/cmd/cynode-pma/main.go` to identify where keep-warm goroutine wires in.
- [ ] Read `agents/internal/pma/streaming_fsm.go` ~lines 203-216 (dead overwrite helpers).
- [ ] Read REQ-PMAGNT-0125, REQ-PMAGNT-0129, REQ-PMAGNT-0124 for exact behavioral requirements.

#### Red (Task 12)

- [ ] Add unit tests for keep-warm: background goroutine sends minimal inference requests at configurable interval (default 300s); stops on context cancellation.
- [ ] Add unit tests for secret scan: after each langchaingo iteration, accumulated buffers are scanned; overwrite event queued if secrets detected.
- [ ] Add unit tests for overwrite events: `{"overwrite": {...}}` NDJSON events are emitted on the stream.
- [ ] Run `go test -v -run 'TestKeepWarm|TestSecretScan|TestOverwrite' ./agents/internal/pma/...` and confirm failures.

#### Green (Task 12)

- [ ] Implement keep-warm: background goroutine in `run()` with `time.Ticker` sending minimal Ollama requests; wire lifecycle shutdown via context.
- [ ] Implement secret scan: after each langchaingo iteration, scan visible text, thinking, and tool-call buffers for secret patterns.
- [ ] Wire overwrite events: connect `iterationOverwriteReplace` and `turnOverwriteReplace` to the streaming pipeline; emit `{"overwrite": {...}}` NDJSON.
- [ ] Re-run all tests and confirm green.

#### Refactor (Task 12)

- [ ] Remove dead code now that overwrite helpers are wired; ensure FSM transitions are tested.

#### Testing (Task 12)

- [ ] Run `go test -cover ./agents/...`; confirm 90% threshold.
- [ ] Run BDD scenarios tagged `@req_PMAGNT_0124`, `@req_PMAGNT_0125`, `@req_PMAGNT_0129` via `just test-bdd`.
- [ ] Run `just lint-go` on all changed files.
- [ ] Run `just e2e --tags pma_inference,streaming` to verify PMA streaming regression (requires inference; skip if unavailable).
- [ ] Validation gate -- do not proceed to Task 13 until all checks pass.

#### Closeout (Task 12)

- [ ] Generate task completion report covering all three features.
  Mark completed steps `- [x]`.
- [ ] Do not start Task 13 until Task 12 closeout is done.

---

### Task 13: Verify and Extend GitHub Actions CI Workflow

CI workflow exists at `.github/workflows/ci.yml` but needs verification that it covers all `just ci` targets, passes with plan changes, and triggers on the working branch.

#### Task 13 Requirements and Specifications

- [Review Report 6 section 4](2026-03-29_review_report_6_testing.md#4-ci-pipeline-analysis)
- Existing workflow: [`.github/workflows/ci.yml`](../../.github/workflows/ci.yml)

#### Discovery (Task 13) Steps

- [ ] Read `.github/workflows/ci.yml` and compare jobs against `just ci` targets; identify gaps.
- [ ] Run `just ci` locally and confirm all targets pass with changes from Tasks 1-12.

#### Red (Task 13)

- [ ] If any `just ci` target is missing from the workflow, note it as a gap to fix.

#### Green (Task 13)

- [ ] Add any missing CI jobs; ensure branch triggers include the working branch (not just `main`/`master`).
- [ ] Verify workflow triggers on current working branch.

#### Refactor (Task 13)

- [ ] Review job parallelism and caching; ensure Go module cache is shared where possible.

#### Testing (Task 13)

- [ ] Run `just ci` locally and confirm all targets pass.
- [ ] Run `just e2e --tags no_inference` for full fast E2E regression across all non-inference tests.
- [ ] Run `just docs-check` to validate all documentation changes.
- [ ] Validation gate -- do not proceed to Task 14 until all checks pass.

#### Closeout (Task 13)

- [ ] Generate task completion report.
  Mark completed steps `- [x]`.
- [ ] Do not start Task 14 until Task 13 closeout is done.

---

### Task 14: Documentation and Closeout

- [ ] Update `docs/dev_docs/_todo.md` to mark all 13 Immediate items as complete.
- [ ] Verify no follow-up work was left undocumented.
- [ ] Run `just docs-check` on all changed documentation.
- [ ] Run `just e2e --tags no_inference` as final E2E regression gate confirming all non-inference tests pass.
- [ ] Generate final plan completion report: tasks completed, overall validation, remaining risks.
- [ ] Mark all completed steps in the plan with `- [x]`. (Last step.)
