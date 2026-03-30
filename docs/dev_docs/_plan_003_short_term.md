---
name: Short-Term High-Severity Issues
overview: |
  Address 12 high-severity issues from review reports 1-6 within 1-2 sprints.
  Tasks are ordered by cross-cutting impact (unbounded reads, context propagation)
  first, then by module (orchestrator, worker, agents, cynork, shared libs),
  finishing with test infrastructure (BDD domains, E2E in CI).
  Each task follows BDD/TDD with per-task validation gates.
todos:
  - id: st-001
    content: "Search all Go modules for `io.ReadAll` without `io.LimitReader` and `json.NewDecoder(r.Body).Decode` without `http.MaxBytesReader`; list every site by file and line."
    status: pending
  - id: st-002
    content: "Categorize sites by module: orchestrator handlers (artifacts, MCP gateway), worker node (managed proxy, nodeagent), PMA (chat handler, Ollama response), SBA (stdin, file reads, MCP client), cynork (gateway client, password input)."
    status: pending
    dependencies:
      - st-001
  - id: st-003
    content: "Read `docs/tech_specs/go_rest_api_standards.md` for body-size-limit requirements."
    status: pending
    dependencies:
      - st-002
  - id: st-004
    content: "Add a unit test per module: send a request body exceeding 10 MB and assert rejection (413 or reader error)."
    status: pending
    dependencies:
      - st-003
  - id: st-005
    content: "Run `go test -v -run TestMaxBytes ./orchestrator/...`, `go test -v -run TestMaxBytes ./worker_node/...`, `go test -v -run TestMaxBytes ./agents/...`, `go test -v -run TestMaxBytes ./cynork/...` and confirm failures."
    status: pending
    dependencies:
      - st-004
  - id: st-006
    content: "Wrap every `json.NewDecoder(r.Body)` with `http.MaxBytesReader(w, r.Body, maxBodySize)` in orchestrator, worker node, PMA, and SBA handlers."
    status: pending
    dependencies:
      - st-005
  - id: st-007
    content: "Wrap every `io.ReadAll(resp.Body)` with `io.LimitReader(resp.Body, maxResponseSize)` in cynork gateway client, SBA MCP client, and worker node nodeagent."
    status: pending
    dependencies:
      - st-006
  - id: st-008
    content: "Define `maxBodySize` and `maxResponseSize` constants in `go_shared_libs` or per-module config; use consistent defaults (e.g., 10 MB for API bodies, 100 MB for artifact uploads)."
    status: pending
    dependencies:
      - st-007
  - id: st-009
    content: "Re-run `go test -v -run TestMaxBytes ./orchestrator/...`, `go test -v -run TestMaxBytes ./worker_node/...`, `go test -v -run TestMaxBytes ./agents/...`, `go test -v -run TestMaxBytes ./cynork/...` and confirm green."
    status: pending
    dependencies:
      - st-008
  - id: st-010
    content: "Run `just lint-go` on all changed files and `go test -cover` for each module; confirm 90% threshold."
    status: pending
    dependencies:
      - st-009
  - id: st-011
    content: "Run `just e2e --tags no_inference` to verify no regression from body-size limits."
    status: pending
    dependencies:
      - st-010
  - id: st-012
    content: "Validation gate -- do not proceed to Task 2 until all checks pass."
    status: pending
    dependencies:
      - st-011
  - id: st-013
    content: "Generate task completion report for Task 1 listing every file changed and the limit applied. Mark completed steps `- [x]`."
    status: pending
    dependencies:
      - st-012
  - id: st-014
    content: "Do not start Task 2 until Task 1 closeout is done."
    status: pending
    dependencies:
      - st-013
  - id: st-015
    content: "Search worker node for functions performing network I/O without `context.Context`: `waitForPMAReadyUDS`, `pullModels`, `detectExistingInference` in `worker_node/cmd/node-manager/main.go`."
    status: pending
    dependencies:
      - st-014
  - id: st-016
    content: "Search cynork for gateway client methods missing context: all non-streaming methods in `cynork/internal/gateway/client.go`."
    status: pending
    dependencies:
      - st-015
  - id: st-017
    content: "Search SBA for `applyUnifiedDiffStep` and other network calls missing context in `agents/internal/sba/`."
    status: pending
    dependencies:
      - st-016
  - id: st-018
    content: "Add a unit test per function: pass a pre-cancelled context and assert the function returns `context.Canceled` (not hangs)."
    status: pending
    dependencies:
      - st-017
  - id: st-019
    content: "Run `go test -v -run TestContextCancel ./worker_node/...`, `go test -v -run TestContextCancel ./cynork/...`, `go test -v -run TestContextCancel ./agents/...` and confirm failures."
    status: pending
    dependencies:
      - st-018
  - id: st-020
    content: "Add `ctx context.Context` as the first parameter to each identified function; propagate to downstream HTTP calls via `req.WithContext(ctx)` or `http.NewRequestWithContext`."
    status: pending
    dependencies:
      - st-019
  - id: st-021
    content: "Update all callers to pass the appropriate context (request context, shutdown context, or `context.Background()` with documented justification)."
    status: pending
    dependencies:
      - st-020
  - id: st-022
    content: "Re-run context cancellation tests and confirm green."
    status: pending
    dependencies:
      - st-021
  - id: st-023
    content: "Run `just lint-go` on all changed files and `go test -cover` for each module; confirm 90% threshold."
    status: pending
    dependencies:
      - st-022
  - id: st-024
    content: "Validation gate -- do not proceed to Task 3 until all checks pass."
    status: pending
    dependencies:
      - st-023
  - id: st-025
    content: "Generate task completion report for Task 2. Mark completed steps `- [x]`."
    status: pending
    dependencies:
      - st-024
  - id: st-026
    content: "Do not start Task 3 until Task 2 closeout is done."
    status: pending
    dependencies:
      - st-025
  - id: st-027
    content: "Search orchestrator for `time.Sleep` in retry loops: `openai_chat.go:702-704` and any other sites."
    status: pending
    dependencies:
      - st-026
  - id: st-028
    content: "Add a unit test: retry loop must respect context cancellation during backoff (cancelled context causes immediate return, not sleep)."
    status: pending
    dependencies:
      - st-027
  - id: st-029
    content: "Run `go test -v -run TestRetryContextCancel ./orchestrator/...` and confirm failure."
    status: pending
    dependencies:
      - st-028
  - id: st-030
    content: "Replace `time.Sleep(backoff)` with `select { case <-ctx.Done(): return ctx.Err() case <-time.After(backoff): }` in each retry loop."
    status: pending
    dependencies:
      - st-029
  - id: st-031
    content: "Re-run `go test -v -run TestRetryContextCancel ./orchestrator/...` and confirm green."
    status: pending
    dependencies:
      - st-030
  - id: st-032
    content: "Run `just lint-go` on changed files and `go test -cover ./orchestrator/...`; confirm 90% threshold."
    status: pending
    dependencies:
      - st-031
  - id: st-033
    content: "Validation gate -- do not proceed to Task 4 until all checks pass."
    status: pending
    dependencies:
      - st-032
  - id: st-034
    content: "Generate task completion report for Task 3. Mark completed steps `- [x]`."
    status: pending
    dependencies:
      - st-033
  - id: st-035
    content: "Do not start Task 4 until Task 3 closeout is done."
    status: pending
    dependencies:
      - st-034
  - id: st-036
    content: "Read `cynork/internal/tui/model.go` and identify all synchronous network I/O calls in `Update()`: `/thread new`, `/thread switch`, stream recovery `Health()`, and any other blocking calls."
    status: pending
    dependencies:
      - st-035
  - id: st-037
    content: "Read `docs/tech_specs/cynork/cynork_tui.md` for the Bubble Tea `Cmd` pattern and async I/O requirements."
    status: pending
    dependencies:
      - st-036
  - id: st-038
    content: "Add a test: `Update()` must return in under 50ms for each identified network call (assert no blocking I/O)."
    status: pending
    dependencies:
      - st-037
  - id: st-039
    content: "Run `go test -v -run TestUpdateNoBlock ./cynork/internal/tui/...` and confirm failure (Update blocks on network I/O)."
    status: pending
    dependencies:
      - st-038
  - id: st-040
    content: "Refactor each synchronous network call into a `tea.Cmd` that returns a message; handle the result in `Update()` via the returned message type."
    status: pending
    dependencies:
      - st-039
  - id: st-041
    content: "Re-run `go test -v -run TestUpdateNoBlock ./cynork/internal/tui/...` and confirm green."
    status: pending
    dependencies:
      - st-040
  - id: st-042
    content: "Run `just lint-go` on changed files and `go test -race -cover ./cynork/...`; confirm 90% threshold."
    status: pending
    dependencies:
      - st-041
  - id: st-043
    content: "Run `just e2e --tags tui_pty,no_inference` to verify TUI responsiveness."
    status: pending
    dependencies:
      - st-042
  - id: st-044
    content: "Validation gate -- do not proceed to Task 5 until all checks pass."
    status: pending
    dependencies:
      - st-043
  - id: st-045
    content: "Generate task completion report for Task 4. Mark completed steps `- [x]`."
    status: pending
    dependencies:
      - st-044
  - id: st-046
    content: "Do not start Task 5 until Task 4 closeout is done."
    status: pending
    dependencies:
      - st-045
  - id: st-047
    content: "Read `orchestrator/internal/handlers/workflow.go` lines 129-252 and identify which workflow handlers lack auth checks."
    status: pending
    dependencies:
      - st-046
  - id: st-048
    content: "Read `orchestrator/internal/middleware/auth.go` to understand the existing auth middleware and how to apply it to workflow routes."
    status: pending
    dependencies:
      - st-047
  - id: st-049
    content: "Add unit tests: unauthenticated requests to each workflow handler endpoint must return 401."
    status: pending
    dependencies:
      - st-048
  - id: st-050
    content: "Run `go test -v -run TestWorkflowAuth ./orchestrator/internal/handlers/...` and confirm failures."
    status: pending
    dependencies:
      - st-049
  - id: st-051
    content: "Apply auth middleware to all workflow handler routes that currently lack it."
    status: pending
    dependencies:
      - st-050
  - id: st-052
    content: "Re-run `go test -v -run TestWorkflowAuth ./orchestrator/internal/handlers/...` and confirm green."
    status: pending
    dependencies:
      - st-051
  - id: st-053
    content: "Run `just lint-go` on changed files and `go test -cover ./orchestrator/...`; confirm 90% threshold."
    status: pending
    dependencies:
      - st-052
  - id: st-054
    content: "Validation gate -- do not proceed to Task 6 until all checks pass."
    status: pending
    dependencies:
      - st-053
  - id: st-055
    content: "Generate task completion report for Task 5. Mark completed steps `- [x]`."
    status: pending
    dependencies:
      - st-054
  - id: st-056
    content: "Do not start Task 6 until Task 5 closeout is done."
    status: pending
    dependencies:
      - st-055
  - id: st-057
    content: "Read `orchestrator/internal/handlers/nodes.go` lines 168-174 to confirm `worker_api_bearer_token` is stored in plaintext."
    status: pending
    dependencies:
      - st-056
  - id: st-058
    content: "Read `orchestrator/internal/models/models.go` for the node model and identify the column storing the token."
    status: pending
    dependencies:
      - st-057
  - id: st-059
    content: "Add a unit test: stored `worker_api_bearer_token` must not equal the plaintext input (assert encryption at rest)."
    status: pending
    dependencies:
      - st-058
  - id: st-060
    content: "Run `go test -v -run TestTokenEncryption ./orchestrator/internal/handlers/...` and confirm failure."
    status: pending
    dependencies:
      - st-059
  - id: st-061
    content: "Implement symmetric encryption (AES-GCM with a server-side key derived from `JWTSecret` or a dedicated encryption key) for `worker_api_bearer_token` before DB write; decrypt on read."
    status: pending
    dependencies:
      - st-060
  - id: st-062
    content: "Re-run `go test -v -run TestTokenEncryption ./orchestrator/internal/handlers/...` and confirm green."
    status: pending
    dependencies:
      - st-061
  - id: st-063
    content: "Add a migration or startup path to re-encrypt existing plaintext tokens on first run after upgrade."
    status: pending
    dependencies:
      - st-062
  - id: st-064
    content: "Run `just lint-go` on changed files and `go test -cover ./orchestrator/...`; confirm 90% threshold."
    status: pending
    dependencies:
      - st-063
  - id: st-065
    content: "Run `just e2e --tags worker,no_inference` to verify node registration and worker communication."
    status: pending
    dependencies:
      - st-064
  - id: st-066
    content: "Validation gate -- do not proceed to Task 7 until all checks pass."
    status: pending
    dependencies:
      - st-065
  - id: st-067
    content: "Generate task completion report for Task 6. Mark completed steps `- [x]`."
    status: pending
    dependencies:
      - st-066
  - id: st-068
    content: "Do not start Task 7 until Task 6 closeout is done."
    status: pending
    dependencies:
      - st-067
  - id: st-069
    content: "Read `worker_node/internal/proxy/internal_orchestrator_proxy.go` lines 126-175 to map current proxy request/response flow."
    status: pending
    dependencies:
      - st-068
  - id: st-070
    content: "Read `docs/requirements/worker.md` REQ-WORKER-0163 for audit logging requirements on the internal proxy."
    status: pending
    dependencies:
      - st-069
  - id: st-071
    content: "Add a unit test: proxied requests must produce a structured audit log entry containing timestamp, source, destination, method, path, status code, and duration."
    status: pending
    dependencies:
      - st-070
  - id: st-072
    content: "Run `go test -v -run TestProxyAuditLog ./worker_node/internal/proxy/...` and confirm failure."
    status: pending
    dependencies:
      - st-071
  - id: st-073
    content: "Implement audit logging middleware in the internal orchestrator proxy: log each request/response pair as structured JSON."
    status: pending
    dependencies:
      - st-072
  - id: st-074
    content: "Re-run `go test -v -run TestProxyAuditLog ./worker_node/internal/proxy/...` and confirm green."
    status: pending
    dependencies:
      - st-073
  - id: st-075
    content: "Run `just lint-go` on changed files and `go test -cover ./worker_node/...`; confirm 90% threshold."
    status: pending
    dependencies:
      - st-074
  - id: st-076
    content: "Validation gate -- do not proceed to Task 8 until all checks pass."
    status: pending
    dependencies:
      - st-075
  - id: st-077
    content: "Generate task completion report for Task 7. Mark completed steps `- [x]`."
    status: pending
    dependencies:
      - st-076
  - id: st-078
    content: "Do not start Task 8 until Task 7 closeout is done."
    status: pending
    dependencies:
      - st-077
  - id: st-079
    content: "Read `agents/internal/sba/lifecycle.go` lines 53 (`NotifyInProgress`) and 71 (`NotifyCompletion`) to confirm response bodies are not closed."
    status: pending
    dependencies:
      - st-078
  - id: st-080
    content: "Add a unit test: lifecycle HTTP calls must close response bodies (use a mock server that tracks body close)."
    status: pending
    dependencies:
      - st-079
  - id: st-081
    content: "Run `go test -v -run TestLifecycleBodyClose ./agents/internal/sba/...` and confirm failure."
    status: pending
    dependencies:
      - st-080
  - id: st-082
    content: "Add `defer resp.Body.Close()` to `NotifyInProgress` and `NotifyCompletion` in `lifecycle.go`."
    status: pending
    dependencies:
      - st-081
  - id: st-083
    content: "Re-run `go test -v -run TestLifecycleBodyClose ./agents/internal/sba/...` and confirm green."
    status: pending
    dependencies:
      - st-082
  - id: st-084
    content: "Run `just lint-go` on changed files and `go test -cover ./agents/...`; confirm 90% threshold."
    status: pending
    dependencies:
      - st-083
  - id: st-085
    content: "Validation gate -- do not proceed to Task 9 until all checks pass."
    status: pending
    dependencies:
      - st-084
  - id: st-086
    content: "Generate task completion report for Task 8. Mark completed steps `- [x]`."
    status: pending
    dependencies:
      - st-085
  - id: st-087
    content: "Do not start Task 9 until Task 8 closeout is done."
    status: pending
    dependencies:
      - st-086
  - id: st-088
    content: "Read `cynork/internal/gateway/client.go` and identify the `http.Client` configuration (timeout, transport settings)."
    status: pending
    dependencies:
      - st-087
  - id: st-089
    content: "Read `cynork/internal/gateway/client.go` and identify exported fields `Token` and `BaseURL` that are accessed without synchronization."
    status: pending
    dependencies:
      - st-088
  - id: st-090
    content: "Add a unit test: `http.Client` must have a non-zero `Timeout` (e.g., 30s default)."
    status: pending
    dependencies:
      - st-089
  - id: st-091
    content: "Add a test using `go test -race`: concurrent reads and writes of `Client.Token` and `Client.BaseURL` must not race."
    status: pending
    dependencies:
      - st-090
  - id: st-092
    content: "Run `go test -v -run 'TestClientTimeout|TestClientRace' -race ./cynork/internal/gateway/...` and confirm failures."
    status: pending
    dependencies:
      - st-091
  - id: st-093
    content: "Set a default `http.Client.Timeout` of 30s in the client constructor."
    status: pending
    dependencies:
      - st-092
  - id: st-094
    content: "Make `Token` and `BaseURL` unexported; add synchronized getter/setter methods using `sync.RWMutex`."
    status: pending
    dependencies:
      - st-093
  - id: st-095
    content: "Update all callers in cynork to use the new accessor methods."
    status: pending
    dependencies:
      - st-094
  - id: st-096
    content: "Re-run `go test -v -run 'TestClientTimeout|TestClientRace' -race ./cynork/internal/gateway/...` and confirm green."
    status: pending
    dependencies:
      - st-095
  - id: st-097
    content: "Run `just lint-go` on changed files and `go test -race -cover ./cynork/...`; confirm 90% threshold."
    status: pending
    dependencies:
      - st-096
  - id: st-098
    content: "Validation gate -- do not proceed to Task 10 until all checks pass."
    status: pending
    dependencies:
      - st-097
  - id: st-099
    content: "Generate task completion report for Task 9. Mark completed steps `- [x]`."
    status: pending
    dependencies:
      - st-098
  - id: st-100
    content: "Do not start Task 10 until Task 9 closeout is done."
    status: pending
    dependencies:
      - st-099
  - id: st-101
    content: "Read `agents/cmd/cynode-sba/main.go` line 140 (`failureResult` with hardcoded `ProtocolVersion: \"1.0\"`) and lines 167-172 (dead `writeResultFailure`)."
    status: pending
    dependencies:
      - st-100
  - id: st-102
    content: "Read `go_shared_libs/contracts/sbajob/sbajob.go` for existing result status types and protocol version definitions."
    status: pending
    dependencies:
      - st-101
  - id: st-103
    content: "Add a unit test: SBA result status values and protocol version must come from shared constants in `go_shared_libs`, not hardcoded strings."
    status: pending
    dependencies:
      - st-102
  - id: st-104
    content: "Run `go test -v -run TestResultConstants ./agents/cmd/cynode-sba/...` and confirm failure."
    status: pending
    dependencies:
      - st-103
  - id: st-105
    content: "Define `SBAProtocolVersion`, `StatusSuccess`, `StatusFailure`, and other result status constants in `go_shared_libs/contracts/sbajob/`."
    status: pending
    dependencies:
      - st-104
  - id: st-106
    content: "Replace hardcoded strings in `agents/cmd/cynode-sba/main.go` with the shared constants; remove dead `writeResultFailure` code."
    status: pending
    dependencies:
      - st-105
  - id: st-107
    content: "Re-run `go test -v -run TestResultConstants ./agents/cmd/cynode-sba/...` and confirm green."
    status: pending
    dependencies:
      - st-106
  - id: st-108
    content: "Run `just lint-go` on changed files and `go test -cover ./agents/...` and `go test -cover ./go_shared_libs/...`; confirm 90% threshold."
    status: pending
    dependencies:
      - st-107
  - id: st-109
    content: "Validation gate -- do not proceed to Task 11 until all checks pass."
    status: pending
    dependencies:
      - st-108
  - id: st-110
    content: "Generate task completion report for Task 10. Mark completed steps `- [x]`."
    status: pending
    dependencies:
      - st-109
  - id: st-111
    content: "Do not start Task 11 until Task 10 closeout is done."
    status: pending
    dependencies:
      - st-110
  - id: st-112
    content: "Read `docs/requirements/` index to identify the 11 domains with zero BDD coverage: ACCESS, DATAPI, PROJCT, WEBPRX, MCPTOO, AGENTS, WEBCON, CONNEC, BROWSR, STANDS, STEPEX."
    status: pending
    dependencies:
      - st-111
  - id: st-113
    content: "For each domain, read the corresponding requirements file and identify 2-3 key scenarios suitable for BDD feature files."
    status: pending
    dependencies:
      - st-112
  - id: st-114
    content: "Create BDD feature files for the four domains called out in the todo: ACCESS, AGENTS, MCPGAT, MCPTOO; follow the project feature-file conventions in `features/`."
    status: pending
    dependencies:
      - st-113
  - id: st-115
    content: "Tag each scenario with the appropriate `@req_DOMAIN_NNNN` and `@wip` tags."
    status: pending
    dependencies:
      - st-114
  - id: st-116
    content: "Run `just test-bdd` and confirm the new scenarios are discovered (expected to be pending or fail if step definitions are missing)."
    status: pending
    dependencies:
      - st-115
  - id: st-117
    content: "Add step definitions for the new scenarios where existing Go test infrastructure supports them; leave `@wip` on scenarios that require infrastructure not yet available."
    status: pending
    dependencies:
      - st-116
  - id: st-118
    content: "Run `just lint-md` on all new feature files."
    status: pending
    dependencies:
      - st-117
  - id: st-119
    content: "Validation gate -- do not proceed to Task 12 until all checks pass."
    status: pending
    dependencies:
      - st-118
  - id: st-120
    content: "Generate task completion report for Task 11. Mark completed steps `- [x]`."
    status: pending
    dependencies:
      - st-119
  - id: st-121
    content: "Do not start Task 12 until Task 11 closeout is done."
    status: pending
    dependencies:
      - st-120
  - id: st-122
    content: "Read `.github/workflows/ci.yml` to understand current CI job structure."
    status: pending
    dependencies:
      - st-121
  - id: st-123
    content: "Read `justfile` to identify the `just e2e --tags no_inference` target and its prerequisites (dev stack, config)."
    status: pending
    dependencies:
      - st-122
  - id: st-124
    content: "Add a CI job that starts the dev stack and runs `just e2e --tags no_inference`; use service containers or `just setup-dev start` in the workflow."
    status: pending
    dependencies:
      - st-123
  - id: st-125
    content: "Ensure the E2E job runs after the existing lint/test/build jobs and only on branches that have the dev stack available."
    status: pending
    dependencies:
      - st-124
  - id: st-126
    content: "Run `just ci` locally and confirm all targets pass including the new E2E integration."
    status: pending
    dependencies:
      - st-125
  - id: st-127
    content: "Validation gate -- do not proceed to Task 13 until all checks pass."
    status: pending
    dependencies:
      - st-126
  - id: st-128
    content: "Generate task completion report for Task 12. Mark completed steps `- [x]`."
    status: pending
    dependencies:
      - st-127
  - id: st-129
    content: "Do not start Task 13 until Task 12 closeout is done."
    status: pending
    dependencies:
      - st-128
  - id: st-130
    content: "Update `docs/dev_docs/_todo.md` to mark all 12 Short-Term items as complete."
    status: pending
    dependencies:
      - st-129
  - id: st-131
    content: "Verify no follow-up work was left undocumented."
    status: pending
    dependencies:
      - st-130
  - id: st-132
    content: "Run `just docs-check` on all changed documentation."
    status: pending
    dependencies:
      - st-131
  - id: st-133
    content: "Run `just e2e --tags no_inference` as final E2E regression gate."
    status: pending
    dependencies:
      - st-132
  - id: st-134
    content: "Generate final plan completion report: tasks completed, overall validation, remaining risks."
    status: pending
    dependencies:
      - st-133
  - id: st-135
    content: "Mark all completed steps in the plan with `- [x]`. (Last step.)"
    status: pending
    dependencies:
      - st-134
---

# Short-Term High-Severity Issues Plan

## Goal

Address 12 high-severity issues identified in review reports 1-6.
These should be completed within 1-2 sprints.
The issues span all modules and cover unbounded reads, missing context propagation, auth gaps, resource leaks, and test infrastructure.

## References

- Requirements: [`docs/requirements/orches.md`](../requirements/orches.md), [`docs/requirements/worker.md`](../requirements/worker.md), [`docs/requirements/client.md`](../requirements/client.md), [`docs/requirements/sbagnt.md`](../requirements/sbagnt.md), [`docs/requirements/mcpgat.md`](../requirements/mcpgat.md)
- Tech specs: [`docs/tech_specs/orchestrator.md`](../tech_specs/orchestrator.md), [`docs/tech_specs/worker_node.md`](../tech_specs/worker_node.md), [`docs/tech_specs/cynode_sba.md`](../tech_specs/cynode_sba.md), [`docs/tech_specs/cynork/cynork_tui.md`](../tech_specs/cynork/cynork_tui.md), [`docs/tech_specs/go_rest_api_standards.md`](../tech_specs/go_rest_api_standards.md)
- Review reports: [`2026-03-29_review_report_1_orchestrator.md`](2026-03-29_review_report_1_orchestrator.md) through [`2026-03-29_review_report_6_testing.md`](2026-03-29_review_report_6_testing.md)
- Consolidated summary: [`2026-03-29_review_consolidated_summary.md`](2026-03-29_review_consolidated_summary.md) sections 2.3, 2.5
- Implementation: `orchestrator/`, `worker_node/`, `agents/`, `cynork/`, `go_shared_libs/`

## Constraints

- Requirements take precedence over tech specs; tech specs take precedence over current code.
- Follow BDD/TDD: failing tests before implementation.
- All changes must pass `just ci` before the task is considered complete.
- No changes that reduce test coverage below the 90% per-package threshold.
- No modifications to linter rules or suppression comments.
- Each task's validation gate must pass before starting the next task.

## Execution Plan

Tasks are ordered by cross-cutting impact first (unbounded reads, context propagation), then by module isolation, finishing with test infrastructure.

### Task 1: Add `http.MaxBytesReader` and `io.LimitReader` Across All Modules

`io.ReadAll` without `io.LimitReader` and `json.NewDecoder(r.Body).Decode` without `http.MaxBytesReader` appear across all modules.
A malicious or buggy peer can OOM any component.

#### Task 1 Requirements and Specifications

- [`docs/tech_specs/go_rest_api_standards.md`](../tech_specs/go_rest_api_standards.md) -- body size limits
- [Consolidated summary section 2.3](2026-03-29_review_consolidated_summary.md#23-unbounded-reads)

#### Discovery (Task 1) Steps

- [ ] Search all Go modules for `io.ReadAll` without `io.LimitReader` and `json.NewDecoder(r.Body).Decode` without `http.MaxBytesReader`; list every site by file and line.
- [ ] Categorize sites by module: orchestrator handlers (artifacts, MCP gateway), worker node (managed proxy, nodeagent), PMA (chat handler, Ollama response), SBA (stdin, file reads, MCP client), cynork (gateway client, password input).
- [ ] Read `docs/tech_specs/go_rest_api_standards.md` for body-size-limit requirements.

#### Red (Task 1)

- [ ] Add a unit test per module: send a request body exceeding 10 MB and assert rejection (413 or reader error).
- [ ] Run `go test -v -run TestMaxBytes ./orchestrator/...`, `go test -v -run TestMaxBytes ./worker_node/...`, `go test -v -run TestMaxBytes ./agents/...`, `go test -v -run TestMaxBytes ./cynork/...` and confirm failures.

#### Green (Task 1)

- [ ] Wrap every `json.NewDecoder(r.Body)` with `http.MaxBytesReader(w, r.Body, maxBodySize)` in orchestrator, worker node, PMA, and SBA handlers.
- [ ] Wrap every `io.ReadAll(resp.Body)` with `io.LimitReader(resp.Body, maxResponseSize)` in cynork gateway client, SBA MCP client, and worker node nodeagent.
- [ ] Define `maxBodySize` and `maxResponseSize` constants in `go_shared_libs` or per-module config; use consistent defaults (e.g., 10 MB for API bodies, 100 MB for artifact uploads).
- [ ] Re-run `go test -v -run TestMaxBytes ./orchestrator/...`, `go test -v -run TestMaxBytes ./worker_node/...`, `go test -v -run TestMaxBytes ./agents/...`, `go test -v -run TestMaxBytes ./cynork/...` and confirm green.

#### Refactor (Task 1)

No additional refactor needed; the constants and wrapping are the implementation.

#### Testing (Task 1)

- [ ] Run `just lint-go` on all changed files and `go test -cover` for each module; confirm 90% threshold.
- [ ] Run `just e2e --tags no_inference` to verify no regression from body-size limits.
- [ ] Validation gate -- do not proceed to Task 2 until all checks pass.

#### Closeout (Task 1)

- [ ] Generate task completion report for Task 1 listing every file changed and the limit applied.
  Mark completed steps `- [x]`.
- [ ] Do not start Task 2 until Task 1 closeout is done.

---

### Task 2: Add `context.Context` to All Functions Performing Network I/O Without It

Multiple components perform network I/O or long-running operations without context, preventing cancellation and timeout propagation.

#### Task 2 Requirements and Specifications

- [Review Report 2](2026-03-29_review_report_2_worker_node.md) -- worker node context gaps
- [Review Report 3](2026-03-29_review_report_3_agents.md) -- SBA context gaps
- [Review Report 4](2026-03-29_review_report_4_cynork.md) -- cynork context gaps
- [Consolidated summary section 2.5](2026-03-29_review_consolidated_summary.md#25-missing-contextcontext-propagation)

#### Discovery (Task 2) Steps

- [ ] Search worker node for functions performing network I/O without `context.Context`: `waitForPMAReadyUDS`, `pullModels`, `detectExistingInference` in `worker_node/cmd/node-manager/main.go`.
- [ ] Search cynork for gateway client methods missing context: all non-streaming methods in `cynork/internal/gateway/client.go`.
- [ ] Search SBA for `applyUnifiedDiffStep` and other network calls missing context in `agents/internal/sba/`.

#### Red (Task 2)

- [ ] Add a unit test per function: pass a pre-cancelled context and assert the function returns `context.Canceled` (not hangs).
- [ ] Run `go test -v -run TestContextCancel ./worker_node/...`, `go test -v -run TestContextCancel ./cynork/...`, `go test -v -run TestContextCancel ./agents/...` and confirm failures.

#### Green (Task 2)

- [ ] Add `ctx context.Context` as the first parameter to each identified function; propagate to downstream HTTP calls via `req.WithContext(ctx)` or `http.NewRequestWithContext`.
- [ ] Update all callers to pass the appropriate context (request context, shutdown context, or `context.Background()` with documented justification).
- [ ] Re-run context cancellation tests and confirm green.

#### Refactor (Task 2)

No additional refactor beyond the signature changes.

#### Testing (Task 2)

- [ ] Run `just lint-go` on all changed files and `go test -cover` for each module; confirm 90% threshold.
- [ ] Validation gate -- do not proceed to Task 3 until all checks pass.

#### Closeout (Task 2)

- [ ] Generate task completion report for Task 2.
  Mark completed steps `- [x]`.
- [ ] Do not start Task 3 until Task 2 closeout is done.

---

### Task 3: Replace `time.Sleep` With Context-Aware Select in Retry Loops

Retry loops in the orchestrator use `time.Sleep` which ignores context cancellation during backoff, causing slow shutdown and test flakiness.

#### Task 3 Requirements and Specifications

- [Review Report 1](2026-03-29_review_report_1_orchestrator.md) -- `openai_chat.go:702-704`

#### Discovery (Task 3) Steps

- [ ] Search orchestrator for `time.Sleep` in retry loops: `openai_chat.go:702-704` and any other sites.

#### Red (Task 3)

- [ ] Add a unit test: retry loop must respect context cancellation during backoff (cancelled context causes immediate return, not sleep).
- [ ] Run `go test -v -run TestRetryContextCancel ./orchestrator/...` and confirm failure.

#### Green (Task 3)

- [ ] Replace `time.Sleep(backoff)` with `select { case <-ctx.Done(): return ctx.Err() case <-time.After(backoff): }` in each retry loop.
- [ ] Re-run `go test -v -run TestRetryContextCancel ./orchestrator/...` and confirm green.

#### Refactor (Task 3)

No additional refactor needed.

#### Testing (Task 3)

- [ ] Run `just lint-go` on changed files and `go test -cover ./orchestrator/...`; confirm 90% threshold.
- [ ] Validation gate -- do not proceed to Task 4 until all checks pass.

#### Closeout (Task 3)

- [ ] Generate task completion report for Task 3.
  Mark completed steps `- [x]`.
- [ ] Do not start Task 4 until Task 3 closeout is done.

---

### Task 4: Move Synchronous Network I/O to Async `tea.Cmd` in TUI

Synchronous network calls in `Update()` for `/thread new`, `/thread switch`, and stream recovery `Health()` freeze the TUI while waiting for responses.

#### Task 4 Requirements and Specifications

- [`docs/tech_specs/cynork/cynork_tui.md`](../tech_specs/cynork/cynork_tui.md) -- Bubble Tea async pattern
- [Review Report 4](2026-03-29_review_report_4_cynork.md) -- synchronous I/O in Update()

#### Discovery (Task 4) Steps

- [ ] Read `cynork/internal/tui/model.go` and identify all synchronous network I/O calls in `Update()`: `/thread new`, `/thread switch`, stream recovery `Health()`, and any other blocking calls.
- [ ] Read `docs/tech_specs/cynork/cynork_tui.md` for the Bubble Tea `Cmd` pattern and async I/O requirements.

#### Red (Task 4)

- [ ] Add a test: `Update()` must return in under 50ms for each identified network call (assert no blocking I/O).
- [ ] Run `go test -v -run TestUpdateNoBlock ./cynork/internal/tui/...` and confirm failure (Update blocks on network I/O).

#### Green (Task 4)

- [ ] Refactor each synchronous network call into a `tea.Cmd` that returns a message; handle the result in `Update()` via the returned message type.
- [ ] Re-run `go test -v -run TestUpdateNoBlock ./cynork/internal/tui/...` and confirm green.

#### Refactor (Task 4)

No additional refactor beyond the async conversion.

#### Testing (Task 4)

- [ ] Run `just lint-go` on changed files and `go test -race -cover ./cynork/...`; confirm 90% threshold.
- [ ] Run `just e2e --tags tui_pty,no_inference` to verify TUI responsiveness.
- [ ] Validation gate -- do not proceed to Task 5 until all checks pass.

#### Closeout (Task 4)

- [ ] Generate task completion report for Task 4.
  Mark completed steps `- [x]`.
- [ ] Do not start Task 5 until Task 4 closeout is done.

---

### Task 5: Add Auth Checks to Workflow Handlers

Workflow handlers in the orchestrator lack authentication middleware, allowing unauthenticated access to workflow operations.

#### Task 5 Requirements and Specifications

- [Review Report 1](2026-03-29_review_report_1_orchestrator.md) -- `handlers/workflow.go:129-252`
- [`docs/tech_specs/orchestrator.md`](../tech_specs/orchestrator.md) -- API authentication

#### Discovery (Task 5) Steps

- [ ] Read `orchestrator/internal/handlers/workflow.go` lines 129-252 and identify which workflow handlers lack auth checks.
- [ ] Read `orchestrator/internal/middleware/auth.go` to understand the existing auth middleware and how to apply it to workflow routes.

#### Red (Task 5)

- [ ] Add unit tests: unauthenticated requests to each workflow handler endpoint must return 401.
- [ ] Run `go test -v -run TestWorkflowAuth ./orchestrator/internal/handlers/...` and confirm failures.

#### Green (Task 5)

- [ ] Apply auth middleware to all workflow handler routes that currently lack it.
- [ ] Re-run `go test -v -run TestWorkflowAuth ./orchestrator/internal/handlers/...` and confirm green.

#### Refactor (Task 5)

No additional refactor needed.

#### Testing (Task 5)

- [ ] Run `just lint-go` on changed files and `go test -cover ./orchestrator/...`; confirm 90% threshold.
- [ ] Validation gate -- do not proceed to Task 6 until all checks pass.

#### Closeout (Task 5)

- [ ] Generate task completion report for Task 5.
  Mark completed steps `- [x]`.
- [ ] Do not start Task 6 until Task 5 closeout is done.

---

### Task 6: Encrypt `worker_api_bearer_token` in DB

The `worker_api_bearer_token` is stored in plaintext in the orchestrator database, exposing it to anyone with DB read access.

#### Task 6 Requirements and Specifications

- [Review Report 1](2026-03-29_review_report_1_orchestrator.md) -- `nodes.go:168-174`
- [`docs/tech_specs/orchestrator.md`](../tech_specs/orchestrator.md) -- node registration and security

#### Discovery (Task 6) Steps

- [ ] Read `orchestrator/internal/handlers/nodes.go` lines 168-174 to confirm `worker_api_bearer_token` is stored in plaintext.
- [ ] Read `orchestrator/internal/models/models.go` for the node model and identify the column storing the token.

#### Red (Task 6)

- [ ] Add a unit test: stored `worker_api_bearer_token` must not equal the plaintext input (assert encryption at rest).
- [ ] Run `go test -v -run TestTokenEncryption ./orchestrator/internal/handlers/...` and confirm failure.

#### Green (Task 6)

- [ ] Implement symmetric encryption (AES-GCM with a server-side key derived from `JWTSecret` or a dedicated encryption key) for `worker_api_bearer_token` before DB write; decrypt on read.
- [ ] Re-run `go test -v -run TestTokenEncryption ./orchestrator/internal/handlers/...` and confirm green.

#### Refactor (Task 6)

- [ ] Add a migration or startup path to re-encrypt existing plaintext tokens on first run after upgrade.

#### Testing (Task 6)

- [ ] Run `just lint-go` on changed files and `go test -cover ./orchestrator/...`; confirm 90% threshold.
- [ ] Run `just e2e --tags worker,no_inference` to verify node registration and worker communication.
- [ ] Validation gate -- do not proceed to Task 7 until all checks pass.

#### Closeout (Task 6)

- [ ] Generate task completion report for Task 6.
  Mark completed steps `- [x]`.
- [ ] Do not start Task 7 until Task 6 closeout is done.

---

### Task 7: Add Audit Logging to Internal Orchestrator Proxy

The internal orchestrator proxy in the worker node does not log proxied requests, making it impossible to audit cross-component communication.

#### Task 7 Requirements and Specifications

- [REQ-WORKER-0163](../requirements/worker.md#req-worker-0163) -- proxy audit logging
- [Review Report 2](2026-03-29_review_report_2_worker_node.md) -- `internal_orchestrator_proxy.go:126-175`

#### Discovery (Task 7) Steps

- [ ] Read `worker_node/internal/proxy/internal_orchestrator_proxy.go` lines 126-175 to map current proxy request/response flow.
- [ ] Read `docs/requirements/worker.md` REQ-WORKER-0163 for audit logging requirements on the internal proxy.

#### Red (Task 7)

- [ ] Add a unit test: proxied requests must produce a structured audit log entry containing timestamp, source, destination, method, path, status code, and duration.
- [ ] Run `go test -v -run TestProxyAuditLog ./worker_node/internal/proxy/...` and confirm failure.

#### Green (Task 7)

- [ ] Implement audit logging middleware in the internal orchestrator proxy: log each request/response pair as structured JSON.
- [ ] Re-run `go test -v -run TestProxyAuditLog ./worker_node/internal/proxy/...` and confirm green.

#### Refactor (Task 7)

No additional refactor needed.

#### Testing (Task 7)

- [ ] Run `just lint-go` on changed files and `go test -cover ./worker_node/...`; confirm 90% threshold.
- [ ] Validation gate -- do not proceed to Task 8 until all checks pass.

#### Closeout (Task 7)

- [ ] Generate task completion report for Task 7.
  Mark completed steps `- [x]`.
- [ ] Do not start Task 8 until Task 7 closeout is done.

---

### Task 8: Close Lifecycle Response Bodies in SBA

SBA lifecycle HTTP calls (`NotifyInProgress`, `NotifyCompletion`) do not close response bodies, causing resource leaks and potential connection exhaustion.

#### Task 8 Requirements and Specifications

- [Review Report 3](2026-03-29_review_report_3_agents.md) -- `lifecycle.go:53`, `lifecycle.go:71`

#### Discovery (Task 8) Steps

- [ ] Read `agents/internal/sba/lifecycle.go` lines 53 (`NotifyInProgress`) and 71 (`NotifyCompletion`) to confirm response bodies are not closed.

#### Red (Task 8)

- [ ] Add a unit test: lifecycle HTTP calls must close response bodies (use a mock server that tracks body close).
- [ ] Run `go test -v -run TestLifecycleBodyClose ./agents/internal/sba/...` and confirm failure.

#### Green (Task 8)

- [ ] Add `defer resp.Body.Close()` to `NotifyInProgress` and `NotifyCompletion` in `lifecycle.go`.
- [ ] Re-run `go test -v -run TestLifecycleBodyClose ./agents/internal/sba/...` and confirm green.

#### Refactor (Task 8)

No additional refactor needed.

#### Testing (Task 8)

- [ ] Run `just lint-go` on changed files and `go test -cover ./agents/...`; confirm 90% threshold.
- [ ] Validation gate -- do not proceed to Task 9 until all checks pass.

#### Closeout (Task 8)

- [ ] Generate task completion report for Task 8.
  Mark completed steps `- [x]`.
- [ ] Do not start Task 9 until Task 8 closeout is done.

---

### Task 9: Set Default HTTP Client Timeout in Cynork and Synchronize Accessors

The cynork gateway client has no default HTTP timeout and exposes `Token`/`BaseURL` as exported fields without synchronization, allowing data races.

#### Task 9 Requirements and Specifications

- [Review Report 4](2026-03-29_review_report_4_cynork.md) -- gateway client timeout and data race

#### Discovery (Task 9) Steps

- [ ] Read `cynork/internal/gateway/client.go` and identify the `http.Client` configuration (timeout, transport settings).
- [ ] Read `cynork/internal/gateway/client.go` and identify exported fields `Token` and `BaseURL` that are accessed without synchronization.

#### Red (Task 9)

- [ ] Add a unit test: `http.Client` must have a non-zero `Timeout` (e.g., 30s default).
- [ ] Add a test using `go test -race`: concurrent reads and writes of `Client.Token` and `Client.BaseURL` must not race.
- [ ] Run `go test -v -run 'TestClientTimeout|TestClientRace' -race ./cynork/internal/gateway/...` and confirm failures.

#### Green (Task 9)

- [ ] Set a default `http.Client.Timeout` of 30s in the client constructor.
- [ ] Make `Token` and `BaseURL` unexported; add synchronized getter/setter methods using `sync.RWMutex`.
- [ ] Update all callers in cynork to use the new accessor methods.
- [ ] Re-run `go test -v -run 'TestClientTimeout|TestClientRace' -race ./cynork/internal/gateway/...` and confirm green.

#### Refactor (Task 9)

No additional refactor beyond the encapsulation.

#### Testing (Task 9)

- [ ] Run `just lint-go` on changed files and `go test -race -cover ./cynork/...`; confirm 90% threshold.
- [ ] Validation gate -- do not proceed to Task 10 until all checks pass.

#### Closeout (Task 9)

- [ ] Generate task completion report for Task 9.
  Mark completed steps `- [x]`.
- [ ] Do not start Task 10 until Task 9 closeout is done.

---

### Task 10: Extract SBA Result Status Constants and Create Shared Status Mapping

SBA result statuses and protocol version are hardcoded strings; dead code exists in the failure path.

#### Task 10 Requirements and Specifications

- [Review Report 3](2026-03-29_review_report_3_agents.md) -- `main.go:140` (hardcoded `ProtocolVersion`), `main.go:167-172` (dead code)
- [Review Report 5](2026-03-29_review_report_5_shared_libs.md) -- status constant consolidation

#### Discovery (Task 10) Steps

- [ ] Read `agents/cmd/cynode-sba/main.go` line 140 (`failureResult` with hardcoded `ProtocolVersion: "1.0"`) and lines 167-172 (dead `writeResultFailure`).
- [ ] Read `go_shared_libs/contracts/sbajob/sbajob.go` for existing result status types and protocol version definitions.

#### Red (Task 10)

- [ ] Add a unit test: SBA result status values and protocol version must come from shared constants in `go_shared_libs`, not hardcoded strings.
- [ ] Run `go test -v -run TestResultConstants ./agents/cmd/cynode-sba/...` and confirm failure.

#### Green (Task 10)

- [ ] Define `SBAProtocolVersion`, `StatusSuccess`, `StatusFailure`, and other result status constants in `go_shared_libs/contracts/sbajob/`.
- [ ] Replace hardcoded strings in `agents/cmd/cynode-sba/main.go` with the shared constants; remove dead `writeResultFailure` code.
- [ ] Re-run `go test -v -run TestResultConstants ./agents/cmd/cynode-sba/...` and confirm green.

#### Refactor (Task 10)

No additional refactor needed.

#### Testing (Task 10)

- [ ] Run `just lint-go` on changed files and `go test -cover ./agents/...` and `go test -cover ./go_shared_libs/...`; confirm 90% threshold.
- [ ] Validation gate -- do not proceed to Task 11 until all checks pass.

#### Closeout (Task 10)

- [ ] Generate task completion report for Task 10.
  Mark completed steps `- [x]`.
- [ ] Do not start Task 11 until Task 10 closeout is done.

---

### Task 11: Add BDD Feature Files for ACCESS, AGENTS, MCPGAT, MCPTOO Domains

44% of requirement domains have zero BDD coverage.
This task adds initial feature files for the four domains called out in the todo.

#### Task 11 Requirements and Specifications

- [Review Report 6](2026-03-29_review_report_6_testing.md) -- BDD domain coverage gaps
- [`docs/requirements/`](../requirements/) -- ACCESS, AGENTS, MCPGAT, MCPTOO domain requirements

#### Discovery (Task 11) Steps

- [ ] Read `docs/requirements/` index to identify the 11 domains with zero BDD coverage: ACCESS, DATAPI, PROJCT, WEBPRX, MCPTOO, AGENTS, WEBCON, CONNEC, BROWSR, STANDS, STEPEX.
- [ ] For each domain, read the corresponding requirements file and identify 2-3 key scenarios suitable for BDD feature files.

#### Red (Task 11)

- [ ] Create BDD feature files for the four domains called out in the todo: ACCESS, AGENTS, MCPGAT, MCPTOO; follow the project feature-file conventions in `features/`.
- [ ] Tag each scenario with the appropriate `@req_DOMAIN_NNNN` and `@wip` tags.
- [ ] Run `just test-bdd` and confirm the new scenarios are discovered (expected to be pending or fail if step definitions are missing).

#### Green (Task 11)

- [ ] Add step definitions for the new scenarios where existing Go test infrastructure supports them; leave `@wip` on scenarios that require infrastructure not yet available.

#### Refactor (Task 11)

No additional refactor needed.

#### Testing (Task 11)

- [ ] Run `just lint-md` on all new feature files.
- [ ] Validation gate -- do not proceed to Task 12 until all checks pass.

#### Closeout (Task 11)

- [ ] Generate task completion report for Task 11.
  Mark completed steps `- [x]`.
- [ ] Do not start Task 12 until Task 11 closeout is done.

---

### Task 12: Add E2E to CI (`just e2e --tags no_inference`)

E2E tests exist but are not run in CI, making regressions invisible until manual testing.

#### Task 12 Requirements and Specifications

- [Review Report 6](2026-03-29_review_report_6_testing.md) -- E2E not in CI
- Existing CI workflow: [`.github/workflows/ci.yml`](../../.github/workflows/ci.yml)

#### Discovery (Task 12) Steps

- [ ] Read `.github/workflows/ci.yml` to understand current CI job structure.
- [ ] Read `justfile` to identify the `just e2e --tags no_inference` target and its prerequisites (dev stack, config).

#### Red (Task 12)

No red phase; this is infrastructure work.

#### Green (Task 12)

- [ ] Add a CI job that starts the dev stack and runs `just e2e --tags no_inference`; use service containers or `just setup-dev start` in the workflow.
- [ ] Ensure the E2E job runs after the existing lint/test/build jobs and only on branches that have the dev stack available.

#### Refactor (Task 12)

No refactor needed.

#### Testing (Task 12)

- [ ] Run `just ci` locally and confirm all targets pass including the new E2E integration.
- [ ] Validation gate -- do not proceed to Task 13 until all checks pass.

#### Closeout (Task 12)

- [ ] Generate task completion report for Task 12.
  Mark completed steps `- [x]`.
- [ ] Do not start Task 13 until Task 12 closeout is done.

---

### Task 13: Documentation and Closeout

- [ ] Update `docs/dev_docs/_todo.md` to mark all 12 Short-Term items as complete.
- [ ] Verify no follow-up work was left undocumented.
- [ ] Run `just docs-check` on all changed documentation.
- [ ] Run `just e2e --tags no_inference` as final E2E regression gate.
- [ ] Generate final plan completion report: tasks completed, overall validation, remaining risks.
- [ ] Mark all completed steps in the plan with `- [x]`. (Last step.)
