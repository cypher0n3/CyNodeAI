# Review Report 1: Orchestrator

- [1 Summary](#1-summary)
- [2 Specification Compliance](#2-specification-compliance)
- [3 Architectural Issues](#3-architectural-issues)
- [4 Concurrency and Safety](#4-concurrency-and-safety)
- [5 Security Risks](#5-security-risks)
- [6 Performance Concerns](#6-performance-concerns)
- [7 Maintainability Issues](#7-maintainability-issues)
- [8 Recommended Actions](#8-recommended-actions)

## 1 Summary

This report covers the `orchestrator/` module: 177 Go files across 4 binaries (`control-plane`, `user-gateway`, `mcp-gateway`, `api-egress`) and ~17 internal packages including `database`, `handlers`, `mcpgateway`, `artifacts`, `dispatcher`, `auth`, `config`, and `middleware`.

The orchestrator is functional for its current MVP scope with working task lifecycle, node management, chat streaming relay, MCP tool routing, and artifact storage.
However, the review surfaces **8 critical**, **19 high**, **24 medium**, and **13 low** severity findings across specification compliance, security, concurrency, and architecture.

The most impactful gaps are:

- **REQ-ORCHES-0176/0177/0178 (`planning_state`)** is entirely unimplemented -- tasks execute immediately on creation instead of entering `draft` state for PMA review.
- **MCP gateway authorization is fail-open** -- no-token requests bypass all allowlist enforcement, PM agents have unrestricted tool access, and system skills are mutable by any user.
- **Database layer has pervasive TOCTOU races** -- read-then-write patterns without transactions or row locking in task creation, preference upserts, workflow lease acquisition, and checkpoint saves.
- **Bearer token comparisons use `!=`** in api-egress and workflow middleware, enabling timing side-channel attacks.
- **Insecure defaults ship without startup validation** -- JWT secret, admin password, and PSK tokens all have hardcoded dev defaults with no fail-fast outside dev mode.

## 2 Specification Compliance

Gaps identified against requirements and technical specifications.

### 2.1 Critical Gaps

- ❌ **REQ-ORCHES-0176/0177/0178 -- `planning_state` not implemented.**
  `TaskBase` in `orchestrator/internal/models/models.go` has no `PlanningState` field.
  `CreateTask` in `handlers/tasks.go` immediately calls `tryCompleteWithOrchestratorInference` or `createSandboxJob`, executing the task on creation.
  The spec requires: `planning_state=draft` on create (0176), create MUST NOT start workflow (0177), only `planning_state=ready` may start workflow (0178).
  This is the single largest spec divergence.

- ❌ **REQ-ORCHES-0150 -- PMA started as local subprocess, not via worker node.**
  `control-plane/main.go` starts PMA via `pmasubprocess.Start` as a child process.
  The spec requires: "The orchestrator MUST start the Project Manager Agent by instructing a **worker node** to run at least one PMA managed service instance."
  While worker-reported PMA is accepted for readiness, the actual startup path diverges from the worker-instruction model.

- ❌ **MCP gateway fail-open (REQ-MCPGAT-0106, mcp_gateway_enforcement.md).**
  `allowlist.go:129-135`: requests without a Bearer token bypass all allowlist enforcement.
  The spec states: "The orchestrator MUST fail closed.
    If required context is missing for a tool call, the call MUST be rejected."

- ❌ **No PM agent allowlist enforcement (REQ-MCPGAT-0114, access_allowlists_and_scope.md).**
  `allowlist.go:141-144` only restricts sandbox agents.
  `AgentRolePM` falls through with unrestricted tool access.
  The spec defines a PM Agent Allowlist that is not "all tools."

- ❌ **No PA (Project Analyst) agent role (REQ-MCPGAT-0114).**
  Only `AgentRolePM` and `AgentRoleSandbox` exist in `allowlist.go:29-33`.
  The spec defines three agent types: PM, PA, and sandbox.

- ❌ **System skills mutable by any user via `skills.update`/`skills.delete`.**
  `handlers.go:769`: the ownership guard short-circuits when `skill.IsSystem == true`, allowing any caller to mutate system skills.
  The spec states: "Skill tools MUST NOT be invocable with ADMIN-level principals; the gateway MUST reject such invocations."

### 2.2 High-Severity Gaps

- ⚠️ **REQ-ORCHES-0129 -- No continuous PMA health monitoring.**
  No proactive re-startup or state-transition loop; `readyzHandler` only checks PMA health per request.
  The spec requires continuous validation and automatic re-selection if PMA becomes unavailable.

- ⚠️ **REQ-ORCHES-0180 -- Workflow start gate has no `planning_state` check.**
  `workflow_gate.go:55-57` allows start when `task.PlanID == nil` (the common case).
  Without `planning_state`, the gate "deny start for draft" is unenforceable.

- ⚠️ **Sandbox allowlist missing `artifact.put` and `artifact.list` (REQ-MCPGAT-0114).**
  `allowlist.go:87-96` only includes `artifact.get`.
  The Worker Agent Allowlist spec says `artifact.*` (get/put/list).
  SBA cannot upload or list artifacts through the gateway.

- ⚠️ **Agent type not recorded in audit records (REQ-MCPGAT-0107).**
  The resolved `AgentRole` from `tryAgentAllowlist` is never propagated to `McpToolCallAuditLogBase`.

- ⚠️ **PMA invocation class not tracked (REQ-MCPGAT-0107).**
  No concept of invocation class (`user_gateway_session` vs `orchestrator_initiated`) anywhere in gateway code.

- ⚠️ **No admin per-tool enable/disable (REQ-MCPGAT-0113).**
  The gateway has no check against system settings for disabled tools.

- ⚠️ **Workflow handlers have no authentication.**
  `handlers/workflow.go:129-252`: Start, Resume, SaveCheckpoint, and Release perform no auth checks.
  Combined with the `RequireWorkflowRunnerAuth` bypass when token is empty (the default), these endpoints are completely unauthenticated.

### 2.3 Medium-Severity Gaps

- **REQ-ORCHES-0171 -- Heartbeat fallback hardcodes `elapsed_s: 0`.**
  `openai_chat.go:557` always emits `ElapsedS: 0` regardless of actual elapsed time.

- **`project_id` never populated in audit records** except `project.get` (`handlers.go:74`).

- **`subject_type` and `subject_id` never set** in any audit record despite model fields existing.

- **`DurationMs` not set on deny audit records** from `writeDenyAuditAndRespond`.

## 3 Architectural Issues

Structural and design concerns in the orchestrator codebase.

### 3.1 Database Layer

- ❌ **God interface (60+ methods).** `Store` in `database.go:45-169` spans users, auth, tasks, jobs, nodes, chat, preferences, system settings, skills, workflows, access control, artifacts, and API credentials.
  Violates Interface Segregation Principle; any consumer needing one domain must mock the entire interface.
  Should be split into focused sub-interfaces composed via embedding.

- ⚠️ **`DB.GORM()` exposes raw `*gorm.DB`** (`database.go:273-275`), allowing callers to bypass the `Store` abstraction.

- ⚠️ **Interface drift.**
  Several methods (`GrantArtifactRead`, `HasArtifactReadGrant`, `OrchestratorArtifact*`, `DeleteVectorItemsForArtifact`) are not on the `Store` interface, making consumers untestable via the interface.

- ⚠️ **AutoMigrate in production startup** (`migrate.go:28-69`).
  AutoMigrate cannot safely rename, drop, or change column types.
  No migration version tracking; every startup re-runs all AutoMigrate calls.
  Should use versioned migrations (golang-migrate, goose, or atlas).

- ⚠️ **No migration transaction.** `RunSchema` at `migrate.go:17-22` runs AutoMigrate then DDL bootstrap sequentially without a transaction.

### 3.2 Handler Layer

- ⚠️ **Fat handlers with business logic.**
  `TaskHandler.createTaskWithOrchestratorInference` (`tasks.go:206-236`) mixes inference calls, job creation, result marshaling, and task status updates.
  `chatPollUntilTerminal` (`tasks.go:538-565`) is an unbounded DB polling loop inside a request handler (N concurrent requests = N*2 DB queries/second).

- ⚠️ **Environment coupling in request path.**
  `NodeHandler.buildManagedServicesDesiredState` reads 5 environment variables via `os.Getenv` at request time (`nodes.go:338-339,423,432,468`), bypassing constructor injection.

- **DTO/domain mixing.**
  OpenAI chat handlers build responses as `map[string]interface{}` literals instead of typed DTOs (`openai_chat.go:107`, `openai_chat_threads.go:48,277,303,347`).

### 3.3 MCP Gateway

- ❌ **`artifactToolService` is package-level mutable global state with no synchronization** (`artifact_gateway.go:16-21`).
  Written by `SetArtifactToolService` at startup; read concurrently by every handler goroutine.
  No `sync.Once`, `atomic.Pointer`, or mutex.
  Data race if set after HTTP server starts.

- **`handlers.go` is 885 lines** mixing routing, audit, validation, and business logic across all tool domains.

### 3.4 Entry Points and Infrastructure

- ⚠️ **Control-plane does not close DB on shutdown.**
  `run()` receives `database.Store` but never calls `Close()`.
  User-gateway properly does `defer db.Close()`.

- ⚠️ **MCP-gateway has no signal handling.**
  No `signal.Notify` for SIGTERM/SIGINT; with `context.Background()` the process can only be killed ungracefully.

- ⚠️ **Dispatcher discards job/task status update errors.**
  `dispatcher/run.go:38-39,103`: `_ = db.UpdateJobStatus(...)` and `_ = db.UpdateTaskStatus(...)`.
  If these writes fail, the job state machine becomes inconsistent.

- **Dispatcher node selection is always `nodes[0]`** (`run.go:73`) -- no round-robin or load distribution.

## 4 Concurrency and Safety

Race conditions, TOCTOU bugs, and goroutine lifecycle issues.

### 4.1 Critical Race Conditions

- ❌ **Workflow lease acquisition race.** `AcquireTaskWorkflowLease` (`workflow.go:20-65`) performs read-then-conditional-write without a transaction or `SELECT ... FOR UPDATE`.
  Two concurrent callers can both read "no lease" and both attempt `Create`.

- ❌ **Workflow checkpoint upsert race.** `UpsertWorkflowCheckpoint` (`workflow.go:93-132`) does read-then-create/update without transaction.

### 4.2 High-Severity TOCTOU

- **Task name uniqueness check** (`tasks.go:44-59`): COUNT then INSERT without transaction.
- **Preference create** (`preferences.go:131-165`): existence check then Create without transaction.
- **Preference/SystemSetting update** (`preferences.go:168-201`, `system_settings.go:84-116`): read-version-check-update without row locking.

### 4.3 Other Concurrency Issues

- ⚠️ **`runCompletionWithRetry` uses `time.Sleep` ignoring context cancellation** during backoff (`openai_chat.go:702-704`).
  If client disconnects during retry backoff, goroutine continues sleeping.
  Fix: replace `time.Sleep(backoff)` with `select { case <-ctx.Done(): ... case <-time.After(backoff): }`.

- ⚠️ **PMA streaming relay does not check write errors or context cancellation** (`openai_chat.go:487-517`).
  Relay continues accumulating content and writing to a dead connection.

- ⚠️ **`persistStreamedAssistantTurn` uses request context for DB writes** (`openai_chat.go:467-482`).
  If client disconnected, ctx is canceled and `AppendChatMessage` fails -- assistant response is lost.
  Should use a detached context with timeout.

- ⚠️ **Shutdown context derived from parent ctx.**
  `context.WithTimeout(ctx, 30s)` in control-plane and user-gateway: if ctx is already canceled, shutdown context is immediately expired.
  Should use `context.WithTimeout(context.Background(), ...)`.

- **RateLimiter goroutine leak.** `NewRateLimiter` (`auth/ratelimit.go:23-34`) starts a cleanup goroutine with no cancellation mechanism.

- **Global mutable function variables for test injection** (`database.go:230-233,261`) are not safe for `t.Parallel()`.

### 4.4 Silent Error Swallowing

- ❌ **`CreateTask`** (`tasks.go:46,54`): both the count query and uniqueness-check errors are silently discarded with `_ =`.
- ❌ **`UpdateChatThreadTitle`** (`chat.go:196`): second update error silently discarded.
- **`AppendChatMessage`** (`chat.go:71-92`): two independent writes without transaction; thread update failure leaves stale timestamp.

## 5 Security Risks

Vulnerabilities organized by severity level.

### 5.1 Critical Severity

- ❌ **Timing side-channel in api-egress token comparison.**
  `cmd/api-egress/main.go:155`: `strings.TrimPrefix(auth, "Bearer ") != h.token` uses non-constant-time comparison.
  Must use `subtle.ConstantTimeCompare`.

- ❌ **Timing side-channel in workflow middleware.**
  `middleware/auth.go:148`: `got != token` comparison leaks token value via timing.

- ❌ **Insecure defaults without startup validation.**
  `config.go:127-137`: `JWTSecret="change-me-in-production"`, `BootstrapAdminPassword="admin123"`, `NodeRegistrationPSK="default-psk-change-me"`, `WorkerAPIBearerToken="dev-worker-api-token-change-me"`.
  No startup warning or fail-fast outside dev mode.

### 5.2 High Severity

- ⚠️ **Plaintext bearer token in DB.** `UpdateNodeWorkerAPIConfig` (`nodes.go:168-174`) stores `bearerToken` as plaintext in `worker_api_bearer_token` column.
  Unlike `ApiCredentialRecord` which has `CredentialCiphertext`, this token is readable by anyone with DB access.

- ⚠️ **Unbounded request body.**
  Multiple handlers read full request body with no size limit:
  `artifacts.go:58,165` uses `io.ReadAll(r.Body)`.
  All `json.NewDecoder(r.Body).Decode` calls have no `http.MaxBytesReader`.
  MCP gateway `handlers.go:45` has the same issue.

- ⚠️ **IP spoofing for rate-limit bypass.** `getClientIP` (`auth.go:304-316`) trusts `X-Forwarded-For` without validation.

- ⚠️ **JSON injection in MCP gateway error responses.**
  `handlers.go:160` and `task_tools.go:29`: raw string concatenation builds JSON error responses.
  If `errMsg` ever contains quotes, produces malformed JSON or response splitting.

- ⚠️ **Task-scoped MCP tools have no user ownership check.**
  `handleTaskGet`, `handleTaskCancel`, `handleTaskResult`, `handleTaskLogs` -- any caller who knows a `task_id` can access any task.

- ⚠️ **Workflow and api-egress auth bypassed when token is empty** (the default).
  `middleware/auth.go:138-140` and `api-egress/main.go:47-53`: empty token means no auth.

### 5.3 Medium Severity

- **Model mismatch in embedded script.** `promptModeModelCommand` (`tasks.go:244`) hardcodes `qwen3.5:0.8b` instead of using the configured `inferenceModel`.

- **`artifactToolErr` exposes raw Go error messages** (`artifact_gateway.go:79-80`) to callers.

- **Duplicate key detection via string matching.** `CreateOrchestratorArtifact` (`orchestrator_artifacts.go:61`) uses `strings.Contains(err.Error(), "duplicate key")` instead of `pgconn.PgError` code `23505`.

## 6 Performance Concerns

Query efficiency, allocation, and caching issues.

### 6.1 Unbounded Queries

- ⚠️ **`GetJobsByTaskID`** (`tasks.go:251-265`): returns all jobs for a task with no limit.
- **`ListActiveNodes`** (`nodes.go:62-73`) and **`ListDispatchableNodes`** (`nodes.go:101-117`): no limit or pagination.
- **`ListSkillsForUser`** (`skills.go:56-75`): no limit.
- **`ListChatMessages`** with `limit=0` (`chat.go:96-110`): returns all messages.
- **`ListChatThreads`** with `limit=0` (`chat.go:152-172`): returns all threads.

### 6.2 N+1 Query Patterns

- ⚠️ **`workflowGateCheckDeps`** (`workflow_gate.go:32-49`): calls `GetTaskByID` in a loop for each dependency.
  Should be `WHERE id IN (?) AND status != 'completed'`.
- **`CreateTask` name uniqueness** (`tasks.go:52-59`): one COUNT query per loop iteration.
- **`GetEffectivePreferencesForTask`** (`preferences.go:105-127`): one `ListPreferences` per scope level (up to 4).

### 6.3 Other Concerns

- **SSE writer allocation per event.** `writeSSEEvent` (`openai_chat.go:793`) allocates a new `bufio.NewWriter` on every call during streaming.
  Hundreds of delta events = hundreds of 4KB buffer allocations.

- **PMA endpoint resolution queries all nodes per request.** `collectReadyPMACandidates` (`openai_chat.go:628`) calls `ListActiveNodes` then `GetLatestNodeCapabilitySnapshot` for each node.
  Should be cached with a short TTL.

- **Sequential telemetry pull.** `runTelemetryPullLoop` (`control-plane/main.go:366-374`) pulls from each node sequentially within a single ticker iteration.

- **Clock skew in lease expiry.** `AcquireTaskWorkflowLease` (`workflow.go:48`) uses `time.Now().UTC()` instead of DB `NOW()`.

## 7 Maintainability Issues

- **Inconsistent error comparison.** `skills.go:107,194,226,271,284` uses `err == database.ErrNotFound` instead of `errors.Is`.
  Same in `task_tools.go:62,128`.

- **Duplicate conversion code.**
  Most `To*()` methods manually copy every field from the base struct instead of using the embedded base directly.

- **`reason` parameter ignored.** `CreatePreference`, `UpdatePreference`, `DeletePreference`, `CreateSystemSetting`, `UpdateSystemSetting`, `DeleteSystemSetting` all accept a `reason` parameter and discard it with `_ = reason`.

- **Panic risk.** `resolveConfigVersion` (`nodes.go:242`) calls `ulid.MustNew` which panics on entropy failure.

- **`writePreferenceErrToAudit` misnomer.**
  Used across task, job, skill, node, project, and system setting handlers despite its name suggesting preference-specific logic.

- **Fragile string-based error classification.** `isTransientWorkerDispatchError` (`dispatcher/run.go:171-180`) uses `strings.Contains` on error messages instead of `errors.Is`/`errors.As` with typed errors.

- **Silent config parse failures.**
  All `get*Env` helpers (`config.go:190-227`) swallow parse errors and fall back to defaults with no log output.

- **5 package-level test hooks** in `control-plane/main.go:31-43` introduce global mutable state unsafe for concurrent tests.

- **`SystemSettingRecord` breaks record convention** (`system_setting_records.go:11-18`): defines its own fields instead of embedding `GormModelUUID` + domain base struct.

## 8 Recommended Actions

Remediation items organized by priority tier.

### 8.1 P0 -- Immediate (Correctness and Security)

1. **Implement `planning_state`** on `TaskBase`.
   Set `draft` on create, gate workflow execution on `ready`.
   Remove immediate execution from `CreateTask`. (REQ-ORCHES-0176/0177/0178)
2. **Replace all `!=` token comparisons** with `subtle.ConstantTimeCompare` in api-egress and workflow middleware.
3. **Add startup validation** that rejects insecure default secrets outside dev mode (`JWT_SECRET`, `BOOTSTRAP_ADMIN_PASSWORD`, `NODE_REGISTRATION_PSK`, `WORKER_API_BEARER_TOKEN`).
4. **Fix MCP gateway fail-open**: reject no-token requests when auth is expected.
5. **Implement PM and PA allowlists** in `tryAgentAllowlist`.
   Add PA token support.
6. **Fix system skill mutation guard**: reject with 403 when `skill.IsSystem == true`.
7. **Wrap lease acquisition and checkpoint upsert in transactions** with `SELECT ... FOR UPDATE`.

### 8.2 P1 -- Short-Term (High-Severity Issues)

1. **Apply `http.MaxBytesReader`** at router/middleware level for all JSON and artifact endpoints.
2. **Replace `time.Sleep` with context-aware select** in `runCompletionWithRetry`.
3. **Add auth checks to workflow handlers** or ensure routing middleware enforces authentication.
4. **Encrypt `worker_api_bearer_token`** in DB at rest.
5. **Add `artifact.put` and `artifact.list`** to sandbox allowed tools.
6. **Propagate agent role and invocation class** into MCP audit records.
7. **Replace JSON string concatenation** in gateway error responses with `json.Marshal`.
8. **Stop discarding `UpdateJobStatus`/`UpdateTaskStatus` errors** in dispatcher.
9. **Add `signal.Notify`** to mcp-gateway.
   Fix shutdown context to use `context.Background()`.
10. **Add `defer store.Close()`** to control-plane.

### 8.3 P2 -- Planned (Medium-Severity Improvements)

1. **Use detached context** for `persistStreamedAssistantTurn` so client disconnect does not lose assistant responses.
2. **Wrap task name uniqueness, preference create/update, and chat append** in transactions.
3. **Split `Store` interface** into focused sub-interfaces (`TaskStore`, `ChatStore`, `NodeStore`, etc.).
4. **Add pagination** to `GetJobsByTaskID`, `ListActiveNodes`, `ListSkillsForUser`, `ListChatMessages`.
5. **Replace N+1 queries**: batch `workflowGateCheckDeps`, combine `GetEffectivePreferencesForTask` scopes.
6. **Reuse `bufio.Writer` per SSE response**; cache PMA endpoint resolution with short TTL.
7. **Fix `err ==` to `errors.Is`** in `skills.go` and `task_tools.go`.
8. **Extract business logic** from fat handlers into a service layer.
9. **Move env var reads** from `NodeHandler` into constructor parameters.
10. **Implement admin per-tool enable/disable** (REQ-MCPGAT-0113).

### 8.4 P3 -- Longer-Term (Maintenance and Debt)

1. **Adopt versioned migrations** (golang-migrate, goose, or atlas) to replace AutoMigrate.
2. **Align PMA startup** with REQ-ORCHES-0150 worker-instruction model.
3. **Implement continuous PMA monitoring** per REQ-ORCHES-0129.
4. **Add node selection strategy** (round-robin or least-loaded) to dispatcher.
5. **Replace string-based error classification** in dispatcher with typed error matching.
6. **Log config parse failures** instead of silently falling back to defaults.
7. **Replace package-level test hooks** with dependency injection via struct fields.
8. **Extract MCP gateway handlers** from monolithic `handlers.go` into per-domain files.
