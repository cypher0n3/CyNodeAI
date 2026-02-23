# Go Implementation Code Review vs Technical Specs and MVP Plan

- [Summary](#summary)
- [Specification Compliance Issues](#specification-compliance-issues)
- [Architectural Issues](#architectural-issues)
- [Concurrency / Safety Issues](#concurrency--safety-issues)
- [Security Risks](#security-risks)
- [Performance Concerns](#performance-concerns)
- [Maintainability Issues](#maintainability-issues)
- [Recommended Refactor Strategy](#recommended-refactor-strategy)
- [References](#references)

## Summary

The codebase is largely aligned with Phase 1, 1.5, and 1.7 of the MVP plan and with the referenced tech specs.
Implementations for orchestrator health/readyz, node config_version (ULID), Worker API readyz/413/truncation,
task input_mode/prompt interpretation, cynode-pma startup, and MCP tool call auditing (P2-02 foundation)
match or closely follow the specs.
Several gaps remain: **readyz does not gate on Project Manager model warmup** (P1-02),
**agents module is outside standard CI quality gates**, and a few spec/requirement traceability
and behavioral details need tightening.
No critical security or concurrency issues were identified; recommendations focus on spec compliance,
CI parity for agents, and completing Phase 2 allow-path and scoping.

---

## Specification Compliance Issues

Findings below compare implementation to tech specs and MVP task IDs.

### 1. Orchestrator Readyz Does Not Gate on PMA Warmup (P1-02)

**Spec:** `orchestrator.md` (CYNAI.ORCHES.Rule.HealthEndpoints), `orchestrator_bootstrap.md` (Standalone Operation Mode), MVP plan P1-02.

**Requirement:** When a local inference worker is available, the orchestrator MUST NOT enter ready state until the effective Project Manager model is selected and confirmed loaded and available (REQ-BOOTST-0002, REQ-ORCHES-0120, REQ-ORCHES-0129).

**Current behavior:** Control-plane `readyzHandler` in `orchestrator/cmd/control-plane/main.go` only checks `store.ListDispatchableNodes(ctx)`.
It does not verify that the Project Manager model is selected and warmed up.

**Gap:** Ready state can be 200 while PMA is not yet available (e.g. cynode-pma still starting or model not loaded).
Clients may get 503 from chat endpoints despite readyz being 200.

**Recommendation:** Extend readyz to include a PMA readiness check when `PMA_ENABLED=true`: e.g. call a lightweight PMA health/ready endpoint or maintain an in-memory "PMA ready" flag set when the subprocess is healthy and (if applicable) model is loaded.
Document the behavior in code and in the orchestrator bootstrap spec.

### 2. User-Gateway Health Endpoints

**Spec:** `orchestrator.md` defines orchestrator `GET /healthz` and `GET /readyz`.
The spec does not explicitly require the user-gateway to expose these; the control-plane is the component that "refuses to enter ready state."

**Current behavior:** User-gateway has its own server and does not expose `/healthz` or `/readyz` in the same way the control-plane does.
E2E and BDD target the control-plane for readyz.
This is consistent with the architecture (control-plane = readiness authority).

**Recommendation:** No change required unless product explicitly requires the user-gateway to proxy or mirror orchestrator readiness; if so, add to user_api_gateway spec and implement.

### 3. Worker API GET /Readyz Body

**Spec:** `worker_api.md` (CYNAI.WORKER.WorkerApiHealthChecks): `GET /readyz` returns 200 with body `ready` when ready, 503 when not.

**Current behavior:** Implemented in `worker_node/cmd/worker-api/main.go` (`readyzHandler`) and in BDD: 200 with `ready`, 503 with reason text. **Compliant.**

### 4. Config_version_version ULID (P1-03)

**Spec:** `worker_node_payloads.md` (CYNAI.WORKER.Payload.ConfigurationV1): For version=1, orchestrator MUST use a ULID encoded as 26-character Crockford Base32; nodes MUST compare lexicographically for monotonic order.

**Current behavior:** `orchestrator/internal/handlers/nodes.go` uses `ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader).String()` (oklog/ulid/v2), which produces 26-character Crockford Base32.
Persisted via `UpdateNodeConfigVersion`.
Node compares config_version in nodemanager. **Compliant.**

### 5. Worker API 413 and Stdout/stderr Truncation (P1-05)

**Spec:** `worker_api.md` request size limits (413 for oversized body), stdout/stderr capture limits with UTF-8-safe truncation and `truncated.stdout` / `truncated.stderr` flags.

**Current behavior:** Worker API uses `http.MaxBytesReader` (10 MiB) and returns 413 with Problem Details for "request body too large".
Executor uses `truncateUTF8`, 256 KiB default (`MAX_OUTPUT_BYTES`), and sets `Truncated.Stdout`/`Truncated.Stderr`. **Compliant.**

### 6. Task Create Input_mode_mode and Prompt Interpretation (P1.5-01, P1.5-02)

**Spec:** `user_api_gateway.md`, REQ-ORCHES-0126/0127/0128: prompt default; inference by default; script/commands explicit; prompt MUST NOT be executed as literal shell unless raw mode.

**Current behavior:** `orchestrator/internal/handlers/tasks.go`: `InputMode` default `InputModePrompt`; prompt mode uses inference (orchestrator-side or sandbox model-call script); script/commands build literal shell job. **Compliant.**

### 7. OpenAI-Compatible Chat and PMA Routing (Phase 1.7)

**Spec:** `openai_compatible_chat_api.md`: `cynodeai.pm` routes to PMA; other models to direct inference; `GET /v1/models`, `POST /v1/chat/completions`; legacy `POST /v1/chat` removed.

**Current behavior:** User-gateway exposes `/v1/models` and `/v1/chat/completions`; effective model `cynodeai.pm` routes to PMA (pmaclient); other models to direct inference. **Compliant.**

### 8. CyNode-Pma Binary and Orchestrator Integration (P1.7-03, P1.7-04)

**Spec:** `cynode_pma.md`: role flag (`project_manager` | `project_analyst`), separate instructions paths, Containerfile; orchestrator starts/stops cynode-pma when enabled.

**Current behavior:** `agents/` module with `cynode-pma` binary, role flag, Containerfile; control-plane starts subprocess when `PMA_ENABLED=true` and stops on shutdown. **Compliant.**

### 9. MCP Tool Call Auditing (P2-02)

**Spec:** `mcp_tool_call_auditing.md`, `postgres_schema.md`: audit record for every routed tool call; MVP does not store tool args/results.

**Current behavior:** MCP gateway `POST /v1/mcp/tools/call` decodes request, writes `McpToolCallAuditLog` (tool_name, decision=deny, status=error, error_type=not_implemented), returns 501.
Model and schema match postgres_schema (created_at, task_id, project_id, run_id, job_id, subject_type, subject_id, user_id, group_ids, role_names, tool_name, decision, status, duration_ms, error_type). **Compliant for deny path.**
Allow path and full routing not yet implemented (as per MVP plan).

---

## Architectural Issues

Observations on structure and CI.

### 1. Agents Module Not in CI Quality Gates

**Observation:** `go.work` includes `./agents`, but `justfile` defines `go_modules := "go_shared_libs orchestrator worker_node cynork"`.
Lint (lint-go, lint-go-ci), vulncheck-go, test-go-cover, and test-bdd iterate only over `go_modules`.
The agents module is built (build-cynode-pma) and cleaned (clean) but is not part of `just ci`.

**Risk:** Regressions in agents (e.g. cynode-pma) can go undetected by standard CI; coverage and lint rules do not apply uniformly.

**Recommendation:** Add `agents` to `go_modules` and ensure coverage/lint exceptions (if any) are documented (e.g. in justfile comments).
Alternatively, add an explicit `test-agents` (and optionally `lint-agents`) target and run it from `ci` so agents is always validated.

### 2. Readyz and PMA Lifecycle

**Observation:** Control-plane starts cynode-pma as a subprocess; if PMA crashes or is not yet listening, readyz still returns 200 as long as there is a dispatchable node.
Chat requests to `cynodeai.pm` would then fail or timeout.

**Recommendation:** Tie readyz to PMA availability when PMA is enabled (see Specification Compliance item 1).

### 3. Layering and Dependency Direction

**Observation:** Orchestrator handlers call database and inference/pmaclient; no circular dependencies observed.
Worker API is self-contained with executor and inference proxy. **No critical architectural drift identified.**

---

## Concurrency / Safety Issues

- **Control-plane:** HTTP server and dispatcher goroutine; shutdown uses `Server.Shutdown` with timeout.
  No unbounded goroutine spawn observed.
- **Worker API:** Same pattern; `exec.RunJob` is synchronous per request.
  No shared mutable state exposed across requests.
- **MCP gateway:** Single handler, store used per request; no races identified.
- **Recommendation:** Run `just test-go-race` periodically (e.g. in CI or pre-release) if not already; no blocking issues found in reviewed code.

---

## Security Risks

- **Secrets:** Worker API bearer token and node config tokens from env/config; not logged.
  Orchestrator JWT secret and PSK from config.
  No hardcoded secrets observed.
- **Input:** Worker API request body limited (10 MiB); task create and auth handlers use structured decode.
  MCP gateway decodes JSON with bounded body (default server limits).
- **Recommendation:** Ensure API Egress and external provider credentials are never passed to sandboxes (spec already requires this); keep validating in Phase 4 implementation.

---

## Performance Concerns

- **Orchestrator readyz:** Single DB query `ListDispatchableNodes` per readiness probe; acceptable for MVP.
- **Worker executor:** Truncation and UTF-8 handling are O(n) in output size; 256 KiB cap keeps allocation bounded.
- **MCP audit:** One insert per tool call; no N+1. **No major performance gaps for current scope.**

---

## Maintainability Issues

Notes on traceability and Phase 2.

### 1. Spec and Requirement Traceability in Code

**Good:** Many packages reference spec names (e.g. worker_api.md, mcp_tool_call_auditing.md) in comments.
BDD steps and test names reference scenarios (readyz, 413, input_mode).

**Improvement:** Add explicit requirement IDs (e.g. REQ-ORCHES-0120, REQ-WORKER-0140) in comments next to readyz and health logic, and next to config_version ULID generation, so future changes stay traceable.

### 2. BDD vs E2E

**Observation:** Script-driven E2E (`just e2e` / setup-dev.sh full-demo) is the primary E2E path; Godog BDD does not run the full e2e feature set (e2e features are not under a runner that `just test-bdd` runs).
This is documented and acceptable; keep feature files and BDD steps in sync when adding new behaviors.

### 3. MCP Gateway: Allow Path and Scoping

**Observation:** P2-01 (scoping/schema) and P2-03 (preference tools) and the full allow path are not started.
Current implementation correctly denies and audits.
When implementing allow path, add task_id/run_id/job_id validation per `mcp_gateway_enforcement.md` and preserve audit-on-allow.

---

## Recommended Refactor Strategy

1. **P1-02 (readyz + PMA warmup):** Implement a PMA readiness check (e.g. HTTP GET to cynode-pma or shared "ready" state) and integrate it into control-plane `readyzHandler` when `PMA_ENABLED=true`.
   Return 503 with an actionable reason (e.g. "PMA not ready") until PMA is up.
   Update orchestrator_bootstrap/orchestrator spec if wording needs to reflect this.
2. **CI parity for agents:** Include `agents` in `go_modules` (or add a dedicated ci step for agents) so lint, vulncheck, and test-go-cover run for agents.
   Add any justified coverage/lint exceptions in the justfile.
3. **Traceability:** Add requirement IDs in comments for health endpoints, config_version, Worker API limits, and MCP audit.
4. **Phase 2:** Proceed with P2-01 (MCP scoping/schema), P2-03 (preference tools), and then the allow path for tool routing while keeping audit-on-allow and schema compliance.

---

## References

- `docs/mvp_plan.md` - Phase summary, task breakdown, done/remaining
- `docs/tech_specs/_main.md` - Spec index
- `docs/tech_specs/orchestrator.md` - Health endpoints, PM model startup
- `docs/tech_specs/orchestrator_bootstrap.md` - Standalone mode, ready-state
- `docs/tech_specs/worker_api.md` - Health, 413, truncation
- `docs/tech_specs/worker_node_payloads.md` - config_version ULID
- `docs/tech_specs/user_api_gateway.md` - Task create, input_mode
- `docs/tech_specs/openai_compatible_chat_api.md` - Chat routing
- `docs/tech_specs/cynode_pma.md` - PMA binary and integration
- `docs/tech_specs/mcp_tool_call_auditing.md` - Audit fields and storage
- `meta.md` - Repo layout, Go modules
- `.github/copilot-instructions.md` - Tech specs vs implementation, justfile usage
