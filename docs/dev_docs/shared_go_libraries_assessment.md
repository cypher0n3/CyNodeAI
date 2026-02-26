# Shared Go Libraries Assessment

- [1. Purpose and Scope](#1-purpose-and-scope)
- [2. Current State](#2-current-state)
- [3. Assessment by Category](#3-assessment-by-category)
- [4. Components not yet written](#4-components-not-yet-written)
- [5. Refactor Recommendations](#5-refactor-recommendations)
- [6. References](#6-references)

## 1. Purpose and Scope

**Date:** 2026-02-26.

Assessment of existing and potential shared Go types and contracts.

This document assesses Go components across the repository (`go_shared_libs/`, `orchestrator/`, `worker_node/`, `cynork/`) to identify what could be moved or introduced as shared libraries.

Focus: structs that map to PostgreSQL or to API contracts and are used (or will be used) across more than one module; separation of GORM-specific types from shared domain/API types; and patterns that future components (e.g. Web Console backend, API Egress) would follow.

No code changes are prescribed here; this is a documentation-only assessment with refactor recommendations.

## 2. Current State

Summary of what lives in each module and what is already shared.

### 2.1. Go Shared Libs Today

The [go_shared_libs/README.md](../../go_shared_libs/README.md) describes the module as holding stable contracts and small cross-cutting utilities used by orchestrator and worker node.

Current packages:

- **contracts/workerapi**: `RunJobRequest`, `RunJobResponse`, `SandboxSpec`, status constants (`StatusCompleted`, `StatusFailed`, `StatusTimeout`).
  Used by orchestrator (dispatcher, task handler) and worker_node (BDD, worker API).
- **contracts/nodepayloads**: `CapabilityReport`, `RegistrationRequest`, `BootstrapResponse`, `NodeConfigurationPayload`, `ConfigAck`, and related config structs.
  Used by orchestrator (node handler, BDD) and worker_node (nodemanager, BDD).
- **contracts/problem**: RFC 9457 Problem Details (`Details`), type constants (`TypeValidation`, etc.), `Validate()`.
  Used by orchestrator (handlers) and worker_node for error responses.
- **contracts/sbajob**: SBA job spec and result (`JobSpec`, `Result`, `StepResult`, etc.), validation.
  Used by orchestrator and worker_node for job payload and result handling.
- **contracts/userapi**: User API Gateway request/response types (auth, users, tasks, chat) and API-facing status constants (`StatusQueued`, `StatusRunning`, etc.).
  Used by orchestrator (handlers) and cynork (gateway client, cmd); single source of truth for the gateway contract per [REQ-CLIENT-0004](../requirements/client.md).

All of the above are JSON (and/or spec) contracts with no dependency on GORM or database drivers.

### 2.2. Types Only in Orchestrator

- **orchestrator/internal/models**: Single file defining all PostgreSQL-facing structs with both `gorm` and `json` tags: `User`, `PasswordCredential`, `RefreshSession`, `AuthAuditLog`, `Node`, `NodeCapability`, `Task`, `Job`, `PreferenceEntry`, `Project`, `Session`, `ChatThread`, `ChatMessage`, and others.
  Includes `JSONBString` (driver.Valuer/sql.Scanner for jsonb), `TableName()` methods, and status constants: `TaskStatus*`, `JobStatus*`, `NodeStatus*`.
  Used only inside the orchestrator (database layer, handlers, dispatcher, tests).
- **orchestrator/internal/handlers**: Handler logic and conversion from `models.*` to DTOs.
  Request/response DTOs for the User API Gateway are now in [contracts/userapi](../../go_shared_libs/contracts/userapi/userapi.go); handlers use those types.
  Error responses use `problem.Details` and `problem.Type*` from [contracts/problem](../../go_shared_libs/contracts/problem/problem.go).

### 2.3. Cynork Module

- **cynork**: Standalone Go module; depends on `go_shared_libs` for `contracts/userapi` and `contracts/problem`.
- **cynork/internal/gateway**: HTTP client for the User API Gateway; uses shared types from `go_shared_libs/contracts/userapi` for all request/response DTOs and from `contracts/problem` for error parsing.
  Single source of truth for the gateway contract; supports [REQ-CLIENT-0004](../requirements/client.md) (CLI/Web Console parity).

### 2.4. Worker Node

- Worker node depends on `go_shared_libs` (nodepayloads, problem, sbajob, workerapi) and does not import orchestrator or orchestrator models.
  It does not need PostgreSQL entity shapes; it uses wire contracts only.

## 3. Assessment by Category

Assessment of structs and contracts by category.

### 3.1. Structs That Map Directly to PostgreSQL

All current PostgreSQL-mapping structs live in `orchestrator/internal/models/models.go` and use GORM (tags, `TableName()`).

**Principle:** Create stable structs in `go_shared_libs` and wrap them in the orchestrator with GORM only for entities that are **reused outside the orchestrator**.
Do not migrate orchestrator models that are consumed solely by the orchestrator; the cost of keeping shared base and GORM wrapper in sync is only justified when another module needs the same shape.

#### 3.1.1. Validation: Which Structs Are Reused Outside Orchestrator?

- **Job (orchestrator model):** The DB row (`id`, `task_id`, `node_id`, `status`, `payload`, `result` jsonb, timestamps) is used only inside the orchestrator (database layer, dispatcher, handlers).
  SBA and the simple step executor need the **job specification and result** as defined in [sbajob](../../go_shared_libs/contracts/sbajob/sbajob.go): `JobSpec` (job_id, task_id, constraints, steps, inference, context) and `Result` (job_id, status, steps, artifacts).
  Those types already live in `go_shared_libs/contracts/sbajob` and are consumed by the orchestrator (payload building, result handling), worker node (when running SBA or step-executor images), and will be used by the step-executor binary (read `/job/job.json`, write `/job/result.json`).
  The orchestrator **Job** model is the persistence wrapper that stores payload/result as opaque jsonb; it is not the same type as JobSpec/Result.
  **Conclusion:** Do not split the Job DB entity into a shared base + GORM wrap; the shared "job" types are already `sbajob.JobSpec` and `sbajob.Result`.
- **Task (orchestrator model):** Used only in the orchestrator (database, handlers, BDD).
  Cynork and Web Console receive task data via gateway API DTOs (e.g. TaskResponse), not the Task entity.
  **Conclusion:** No cross-module reuse; keep Task in orchestrator only.
- **Node (orchestrator model):** Used only in the orchestrator (database, node handler, dispatcher).
  Worker node sends/receives node **payloads** (registration, capability report, config) defined in `go_shared_libs/contracts/nodepayloads`; it does not use the Node DB row.
  **Conclusion:** No cross-module reuse; keep Node in orchestrator only.
- **Other models (User, Project, Session, ChatThread, etc.):** Same pattern; only orchestrator and gateway response DTOs (which should move to shared userapi) expose them to clients.

#### 3.1.2. Recommendation for Shared Base Structs

- **Do not** migrate any current PostgreSQL entity to a shared base struct for GORM wrapping, because none of them are consumed by another module in entity form.
- Keep full GORM models in the orchestrator.
- Rely on existing shared contracts for cross-boundary use: `sbajob` for job spec/result (SBA, step executor, worker), `nodepayloads` for node registration/config, `workerapi` for run-job request/response.
- If a future component (e.g. API Egress) needs a minimal Job or Task **view** (e.g. id, task_id, status) without DB access, introduce a small shared domain or DTO type at that time and have the orchestrator (or gateway) populate it; only then consider a shared base struct that the orchestrator model embeds if the overlap is substantial.

### 3.2. Status Constants (Task, Job, Node)

- **Current:** `TaskStatusPending`, `TaskStatusRunning`, `TaskStatusCompleted`, etc., and analogous `JobStatus*`, `NodeStatus*` live in `orchestrator/internal/models`.
  Used throughout orchestrator code and tests.
- **Worker API:** `contracts/workerapi` already has `StatusCompleted`, `StatusFailed`, `StatusTimeout` for job execution results; string values align with orchestrator job status where applicable.
- **API surface:** The User API Gateway and CLI spec use a different surface enum (e.g. "queued", "running", "completed", "failed", "canceled"); orchestrator maps internal status to this in handlers (e.g. `taskStatusToSpec`).
- **Recommendation:** Move **API-facing** status constants (the set returned in REST responses and used by CLI/Web Console) into `go_shared_libs` (e.g. a small `contracts/userapi` or `contracts/status` package) so that orchestrator responses and cynork (and any future web backend) use the same literals.
  Internal-only constants (e.g. `JobStatusLeaseExpired`) can remain in orchestrator; the shared package should expose only what the gateway contract specifies.

### 3.3. User API Gateway Request/Response Types

- **Current:** Implemented.
  `go_shared_libs/contracts/userapi` contains all User API Gateway request/response types (auth, users, tasks, chat) and API-facing status constants.
  Orchestrator handlers and cynork gateway client both import this package; orchestrator converts from `models.*` to userapi DTOs in handlers; cynork uses the same types for requests and responses.
  Single source of truth for the gateway contract; supports [REQ-CLIENT-0004](../requirements/client.md) (CLI/Web Console parity).
- **Note:** If adding new gateway endpoints or fields, extend `contracts/userapi` and update both orchestrator and cynork in the same change series.

### 3.4. Problem Details (RFC 9457)

- **Current:** Implemented.
  `go_shared_libs/contracts/problem` defines `Details` and type constants.
  Orchestrator handlers use `problem.Details` and `problem.Type*` from go_shared_libs for all Problem Details responses; the duplicate `ProblemDetails` and `ErrType*` have been removed from the handlers package.
  Error responses are aligned with [go_rest_api_standards.md](../tech_specs/go_rest_api_standards.md).

### 3.5. JSONBString and Job Payload/Result

- **Current:** `JSONBString` in `orchestrator/internal/models` is used only for `Job.Payload` and `Job.Result` (GORM jsonb columns).
  It implements `driver.Valuer` and `sql.Scanner` and is tied to DB serialization.
- **Recommendation:** Keep `JSONBString` in the orchestrator unless a shared component (e.g. a shared job payload validator or worker-side result parser) needs to marshal/unmarshal the same shape.
  No current cross-module use.

### 3.6. Node Capability and Handler DTOs

- Orchestrator `internal/handlers/nodes.go` defines `NodeCapabilityReport`, `NodeCapabilityNode`, etc., for the **response** shape of capability/registration.
  Node **input** payloads already use `nodepayloads.CapabilityReport` from go_shared_libs.
- If the gateway exposes node capability or node list responses to clients (cynork, Web Console), those response DTOs are good candidates for the same shared User API contract package (Section 3.3) so all clients see the same shape.

### 3.7. Structs With Only Minor Differences

When two or more structs have the **same logical fields and JSON shape** and differ only in type names, package location, or tags (e.g. one has GORM tags, another has only `json`), treat them as one contract and keep a single shared definition.
Duplicate "almost the same" types create drift risk and unnecessary conversion code.

#### 3.7.1. Unification Guidance

- Prefer a **single shared definition** in `go_shared_libs` (or in the appropriate existing contract package).
  All consumers use that type; convert at the boundary only when necessary (e.g. `models.User` to shared `userapi.UserResponse` in handlers).
- If one consumer needs an **extra optional field**, embed the shared struct and add the field, or add the field to the shared type with `omitempty` and document which consumers use it.
  Do not maintain two separate structs that are identical except for one field.
- If the only difference is **naming** (e.g. orchestrator `NodeCapabilityReport` vs `nodepayloads.CapabilityReport` with identical fields and tags), remove the duplicate and use the shared type everywhere.

#### 3.7.2. Current Examples

- **Problem Details:** Done; orchestrator uses `problem.Details` (Section 3.4).
- **UserResponse, LoginRequest, LoginResponse (orchestrator vs cynork):** Done; shared in `contracts/userapi` (Section 3.3).
- **NodeCapabilityReport and related (orchestrator) vs CapabilityReport (nodepayloads):** Same shape and JSON tags; only type names differ.
  Orchestrator handlers should use `nodepayloads.CapabilityReport`, `nodepayloads.RegistrationRequest`, and the existing `CapabilityNode`, `Platform`, `Compute`, `SandboxSupport` types instead of defining `NodeCapabilityReport`, `NodeCapabilityNode`, etc.
  This removes duplication and keeps registration/capability as a single contract.

## 4. Components Not yet Written

The following would follow similar patterns and benefit from the same shared contracts:

- **Web Console:** Will call the User API Gateway for all operations (no direct DB).
  If the Web Console is a Nuxt/Vue front end only, it consumes the same REST API; shared Go types matter for any Go BFF or for keeping OpenAPI/specs in sync with Go.
  If a Go backend is added for the Web Console, it should use the same gateway request/response types from `go_shared_libs` so parity with the CLI is automatic.
- **API Egress server:** May need task or job context (e.g. for auditing or policy).
  If it is implemented in Go and needs Task/Job/Node views without using GORM, shared domain structs (Section 3.1) would allow reuse; otherwise it can rely on gateway APIs and shared gateway DTOs.
- **Additional orchestrator services:** New handlers or internal services will continue to use `orchestrator/internal/models` and existing handler DTOs; moving DTOs to shared libs (Section 3.3) does not change that, but ensures those DTOs are the same ones used by cynork and Web Console.

## 5. Refactor Recommendations

Recommended order: adopt shared contracts where duplication or parity is already a concern, then consider domain structs only if a second consumer appears.

### 5.1. High Priority - User API Gateway Contract Package (Done)

- Done: `go_shared_libs/contracts/userapi` added with all User API Gateway request/response types (auth, users, tasks, chat) and API-facing status constants.
- Done: Orchestrator handlers use userapi types; cynork depends on go_shared_libs and `cynork/internal/gateway` uses shared userapi types.
- Single source of truth for the gateway API; supports CLI/Web Console parity.

### 5.2. High Priority - Problem Details (Done)

- Done: Orchestrator handlers use `problem.Details` and `problem.Type*` from `go_shared_libs/contracts/problem`.
- Done: Duplicate `ProblemDetails` and `ErrType*` removed from orchestrator handlers.

### 5.3. Medium Priority - API-Facing Status Constants

- Add to the shared User API contract package (or a small `contracts/status` package) the status string constants that the gateway returns (e.g. queued, running, completed, failed, canceled).
- Use these in orchestrator response building and in cynork when displaying or filtering by status.
- Keep internal-only status (e.g. `JobStatusLeaseExpired`) in orchestrator.

### 5.4. Lower Priority - Node and Other Gateway Response DTOs

- If the gateway exposes node list or node capability in a stable form, move those response structs into the shared User API contract package so cynork and Web Console share the same shape.

### 5.5. Unify Structs With Only Minor Differences

- Where structs are identical or differ only by name or tags, use the shared definition and remove the duplicate (Section 3.7).
- **Node registration:** Refactor orchestrator node handler to use `nodepayloads.CapabilityReport` and `nodepayloads.RegistrationRequest` instead of `NodeCapabilityReport`, `NodeRegistrationRequest`, and the duplicate node/platform/compute/sandbox types.
- **Auth/user/task API types:** Once shared userapi exists, remove duplicate handler and cynork gateway types in favour of the shared contract.

### 5.6. Defer - Shared Base Structs for PostgreSQL Entities

- Only introduce shared base structs (with GORM wrap in orchestrator) for entities that are **reused outside the orchestrator** (see Section 3.1.1).
- Current validation: Job spec/result are already shared via `sbajob`; the Job/Task/Node DB rows are orchestrator-only, so do not split them.
- If a future Go service (e.g. API Egress) needs the same entity shape without DB access, introduce minimal shared structs (e.g. id, task_id, status) in `go_shared_libs` and have the orchestrator model embed them and add GORM tags.

### 5.7. Retain in the Orchestrator Codebase

- Full GORM models and migrations.
- `JSONBString` (unless a shared job payload/result type is introduced).
- Internal status constants not exposed in the API.
- Handler-to-DTO conversion logic (using shared DTO types once introduced).

## 6. References

- [go_shared_libs/README.md](../../go_shared_libs/README.md)
- [meta.md](../../meta.md) (repository layout, four Go modules)
- [docs/tech_specs/postgres_schema.md](../tech_specs/postgres_schema.md) (schema and storing in code)
- [docs/tech_specs/go_rest_api_standards.md](../tech_specs/go_rest_api_standards.md) (error format, Problem Details)
- [docs/requirements/client.md](../requirements/client.md) (REQ-CLIENT-0004, capability parity)
- [dev_docs/mvp_remediation_plan.md](mvp_remediation_plan.md) (existing gaps and remediation)
