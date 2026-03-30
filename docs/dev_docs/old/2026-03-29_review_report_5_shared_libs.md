# Review Report 5: Shared Libraries and Cross-Module Contracts

- [1 Summary](#1-summary)
- [2 Specification Compliance](#2-specification-compliance)
- [3 Architectural Issues](#3-architectural-issues)
- [4 Security Risks](#4-security-risks)
- [5 Performance Concerns](#5-performance-concerns)
- [6 Maintainability Issues](#6-maintainability-issues)
- [7 Recommended Actions](#7-recommended-actions)

## 1 Summary

This report covers the `go_shared_libs/` module: 14 Go files providing shared contracts consumed by all 4 main modules (`orchestrator`, `worker_node`, `agents`, `cynork`).

The shared library is well-structured for a contract-only module with 97.1% test coverage, stateless packages, and no concurrency concerns.
However, the review surfaces **1 critical**, **3 high**, **7 medium**, and **4 low** severity findings.

The most impactful gaps are:

- **`RunJobResponse.ExitCode` uses `int` with `omitempty`** -- exit code 0 is silently dropped from JSON, making successful completions indistinguishable from "exit code not set."
- **Status constant sets are duplicated and diverge** between `userapi`, `workerapi`, and `orchestrator/internal/models` with no shared mapping.
- **SBA Result status values are undeclared string literals** -- no constants for "success", "failure", "timeout."
- **`ContextSpec.Skills` is `interface{}`** -- completely untyped at a wire boundary.

## 2 Specification Compliance

Gaps identified against requirements and technical specifications.

### 2.1 Critical Severity

- ❌ **`RunJobResponse.ExitCode` uses `int` with `omitempty` -- exit code 0 dropped.**
  `workerapi.go:69`: `ExitCode int json:"exit_code,omitempty"`.
  The zero value of `int` is `0`.
    With `omitempty`, a successful container exit (code 0) is indistinguishable from "exit code not set."
  Consumers receiving `status: "completed"` with no `exit_code` field cannot distinguish success from unset.
  Fix: change to `*int` or remove `omitempty`.

### 2.2 High-Severity Gaps

- ⚠️ **Status constant sets duplicated and diverge.**
  `userapi` defines: `queued, running, completed, failed, canceled, superseded`.
  `workerapi` defines: `completed, failed, timeout`.
  `orchestrator/internal/models` defines task: `pending, running, completed, failed, canceled, superseded` and job: `queued, running, completed, failed, canceled, lease_expired`.
  The shared lib has `StatusQueued` but no `StatusPending`; orchestrator uses `TaskStatusPending` internally.
  Worker `StatusTimeout` maps to `StatusFailed` at orchestrator layer with no shared mapping function.

- ⚠️ **SBA Result status values undeclared.**
  `sbajob.go:76-77`: `Status string` documents three values ("success", "failure", "timeout") but no constants are defined.
  Test code uses raw strings.
    Without constants, typos are undetectable at compile time.

- ⚠️ **`ContextSpec.Skills` is `interface{}`** (`sbajob.go:68`).
  Can hold any JSON value with zero compile-time or runtime validation.
  At minimum, use `json.RawMessage` or define the expected shape.

## 3 Architectural Issues

Structural and design concerns in the shared libraries.

### 3.1 Type Safety

- **`ChatCompletionsChoice.Message` is an anonymous struct** (`userapi.go:145-151`).
  Cannot be referenced by name; duplicates `ChatMessage` shape.
  Extract a named type or reuse `ChatMessage`.

- **Mixed `interface{}` and `any`** in `nodepayloads.go` (line 62 vs line 328).
  Module targets Go 1.26; should use `any` consistently.

- **`GPUDevice.Features`** (`nodepayloads.go:62`) and `ConfigAckError.Details` (`nodepayloads.go:328`) are untyped maps with no schema definition.

### 3.2 API Design

- **`ListTasksResponse` exposes two pagination strategies simultaneously** (`userapi.go:92-97`).
  Both `NextOffset *int` and `NextCursor string` in the same response complicates client implementation.

- **`ResponsesCreateRequest.Input` is `json.RawMessage`** (`userapi.go:168`) with only comment-level schema.
  No validation, no helper to parse, no type for "message-like items."

### 3.3 Contract Consistency

- **`workerapi.ValidateRequest` does not check `TaskID`, `JobID`, or `Version`** (`workerapi.go:100-108`).
  Empty required fields pass validation; downstream processing risks panics or data corruption.

- **`nodepayloads` has zero validation functions.**
  `RegistrationRequest` should validate non-empty `PSK` and `Capability.Node.NodeSlug`.

- **`workerapi.RunJobRequest.Version` present but never validated.**
  Version 2 payload with different semantics would be silently accepted.

## 4 Security Risks

Vulnerabilities identified in the shared contracts.

### 4.1 Medium Severity

- **`RegistrationRequest.PSK` and `ConfigManagedServiceOrchestrator.AgentToken` are plaintext strings with no redaction.**
  `nodepayloads.go:131-134` and `nodepayloads.go:254`.
  No `fmt.Stringer` override, no `MarshalJSON` redaction, no `secretutil` integration.
  If any consumer logs marshaled payloads, secrets are exposed.

- **`LoginRequest` carries password with no length/format constraints** (`userapi.go:22-25`).
  Unlike `sbajob` and `workerapi` which provide `Validate*` functions, `userapi` request types have no validation.
  Unbounded `Password` or `Prompt` strings can cause memory issues.

## 5 Performance Concerns

- **`ParseAndValidateJobSpec` converts `[]byte` to `string` unnecessarily** (`sbajob.go:119`).
  `string(data)` copies the entire byte slice.
  `bytes.NewReader(data)` achieves the same `io.Reader` without the copy.

## 6 Maintainability Issues

- **`gormmodel` package has no test files.**
  The only package without tests.
    A roundtrip test ensuring JSON/GORM tags produce expected output would lock the contract.

- **Timestamps use `string` in API contracts but `time.Time` in `gormmodel`.**
  No shared formatting function or `time.RFC3339` reference in contract packages.

- **`GormModelUUID.ID` has no `BeforeCreate` hook** (`gormmodel.go:23-28`).
  If any consumer forgets to set `ID` before `Create()`, a nil UUID is inserted.
  Either add a hook or document the caller contract.

- **No shared time-formatting helpers** referenced by contract types.

## 7 Recommended Actions

Remediation items organized by priority tier.

### 7.1 P0 -- Immediate (Correctness)

1. **Change `RunJobResponse.ExitCode` from `int` to `*int`** to fix zero-value omission.
   Coordinate with worker_node and orchestrator consumers.

### 7.2 P1 -- Short-Term (High-Severity)

1. **Extract SBA result status constants** (`ResultStatusSuccess`, `ResultStatusFailure`, `ResultStatusTimeout`) into `sbajob`.
2. **Create shared status mapping** or single source of truth for task/job lifecycle states.
3. **Replace `ContextSpec.Skills interface{}`** with `json.RawMessage` or a defined type.

### 7.3 P2 -- Planned (Medium-Severity)

1. **Add `Validate()` to `workerapi.RunJobRequest`** for `TaskID`, `JobID`, `Version`.
2. **Add `Validate()` to `nodepayloads.RegistrationRequest`** and `NodeConfigurationPayload`.
3. **Add `BeforeCreate` hook** to `GormModelUUID` or document caller contract.
4. **Add test file for `gormmodel`.**
5. **Replace `string(data)` with `bytes.NewReader(data)`** in `ParseAndValidateJobSpec`.
6. **Extract anonymous struct** from `ChatCompletionsChoice.Message` into named type.
7. **Add `MarshalJSON`/`String()` redaction** on types carrying secrets.
8. **Normalize `interface{}` to `any`** across the module.

### 7.4 P3 -- Longer-Term

1. Add shared time-formatting helpers.
2. Resolve dual pagination strategy in `ListTasksResponse`.
3. Add input validation helpers for `userapi` request types.
4. Add `Validate()` to `nodepayloads` complex structures.
