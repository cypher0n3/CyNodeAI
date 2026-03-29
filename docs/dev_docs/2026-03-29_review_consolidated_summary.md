# Consolidated Review Summary: Implementation vs Requirements and MVP Plan

- [1 Overview](#1-overview)
- [2 Cross-Cutting Critical Findings](#2-cross-cutting-critical-findings)
- [3 Severity Distribution](#3-severity-distribution)
- [4 Prioritized Remediation Plan](#4-prioritized-remediation-plan)
- [5 Component Health Assessment](#5-component-health-assessment)
- [6 Specification Compliance Scorecard](#6-specification-compliance-scorecard)
- [7 Individual Report References](#7-individual-report-references)

## 1 Overview

This document consolidates findings from 6 adversarial code review reports covering the entire CyNodeAI codebase.
The review was conducted as a senior Go developer code review against the project's requirements (`docs/requirements/`), technical specifications (`docs/tech_specs/`), and MVP plan (`docs/mvp_plan.md`).

**Scope:** 375+ Go files across 6 modules (`orchestrator`, `worker_node`, `agents`, `cynork`, `go_shared_libs`, `e2e`), 54 BDD feature files, 48 Python E2E test modules, and the CI pipeline.

**Overall assessment:** The codebase delivers a functional MVP with working task lifecycle, chat streaming, MCP tool routing, sandbox execution, secure storage, and a rich TUI.
Code quality is generally good with 90%+ unit test coverage, disciplined BDD tagging, and comprehensive linting.
However, the review identified **systemic patterns** that must be addressed before production readiness.

## 2 Cross-Cutting Critical Findings

These findings appear across multiple modules and represent systemic risks:

### 2.1 Authorization Fail-Open Pattern

The MCP gateway allows unauthenticated requests through when no Bearer token is present (`allowlist.go:129-135`), PM agents have unrestricted tool access, and system skills are mutable by any user.
The workflow middleware and api-egress both bypass auth when their token config is empty (the default).
**Multiple security boundaries are open by default.**

### 2.2 Non-Constant-Time Token Comparison

Bearer token comparisons use `!=` (timing side-channel vulnerable) in:

- `orchestrator/cmd/api-egress/main.go`
- `orchestrator/internal/middleware/auth.go`
- `worker_node/internal/workerapiserver/embed_handlers.go`

This is a cross-module pattern that must be replaced with `subtle.ConstantTimeCompare` everywhere.

### 2.3 Unbounded Reads

`io.ReadAll` without `io.LimitReader` and `json.NewDecoder(r.Body).Decode` without `http.MaxBytesReader` appear in:

- Orchestrator handlers (artifacts, MCP gateway)
- Worker node (managed proxy, nodeagent HTTP calls)
- PMA (chat handler, Ollama response)
- SBA (stdin, file reads, MCP client)
- Cynork (gateway client methods, password input)

A malicious or buggy peer can OOM any component.
This is the most pervasive vulnerability.

### 2.4 Insecure Defaults Without Validation

Production-critical secrets default to weak values with no startup validation:

- `JWT_SECRET="change-me-in-production"`
- `BOOTSTRAP_ADMIN_PASSWORD="admin123"`
- `NODE_REGISTRATION_PSK="default-psk-change-me"`
- `WORKER_API_BEARER_TOKEN="dev-worker-api-token-change-me"`

No fail-fast or warning outside dev mode.

### 2.5 Missing `context.Context` Propagation

Multiple components perform network I/O or long-running operations without context:

- Worker node: `waitForPMAReadyUDS`, `pullModels`, `detectExistingInference`
- Cynork: all non-streaming gateway client methods
- SBA: `applyUnifiedDiffStep`
- TUI: synchronous network calls in `Update()` freeze the UI

### 2.6 Global Mutable State for Test Injection

Package-level `var` for test hooks appears in orchestrator, worker_node, PMA, and cynork.
This pattern is not goroutine-safe and prevents `t.Parallel()`.

## 3 Severity Distribution

Across all 6 reports:

- **Critical:** 19 findings
- **High:** 44 findings
- **Medium:** 62 findings
- **Low:** 42 findings
- **Total:** 167 findings

### 3.1 Critical Findings by Module

- Orchestrator: 8 (spec gaps, TOCTOU races, timing attacks, insecure defaults)
- Worker Node: 2 (network isolation, container name bug)
- Agents: 5 (3 PMA spec gaps, SBA prompt construction, SBA persona missing)
- Cynork: 1 (data race in thread management)
- Shared Libs: 1 (ExitCode zero-value omission)
- Testing: 2 (no CI pipeline, E2E not in CI)

## 4 Prioritized Remediation Plan

Issues are grouped into four priority tiers based on severity and production impact.

### 4.1 P0 -- Immediate (Security and Correctness)

These must be addressed before any production deployment:

1. **Fix authorization fail-open.**
   MCP gateway must reject no-token requests.
   Implement PM and PA allowlists.
   Fix system skill mutation guard. (Report 1)
2. **Replace all `!=` token comparisons** with `subtle.ConstantTimeCompare` across orchestrator, worker node, and cynork. (Reports 1, 2, 4)
3. **Add startup validation** rejecting insecure default secrets outside dev mode. (Report 1)
4. **Implement `planning_state`** on TaskBase (REQ-ORCHES-0176/0177/0178). (Report 1)
5. **Fix pod network isolation** for sandbox containers (REQ-WORKER-0174). (Report 2)
6. **Fix container name matching** in `startOneManagedService`. (Report 2)
7. **Add `Close()` to securestore** that zeros key material. (Report 2)
8. **Implement PMA keep-warm** (REQ-PMAGNT-0129), **secret scan** (REQ-PMAGNT-0125), and **overwrite events** (REQ-PMAGNT-0124). (Report 3)
9. **Fix SBA prompt construction** (REQ-SBAGNT-0113): add persona, skills, preferences; fix context ordering. (Report 3)
10. **Fix PMA WriteTimeout** (120s < inference timeout 300s). (Report 3)
11. **Fix cynork `runEnsureThread` data race.** (Report 4)
12. **Change `RunJobResponse.ExitCode`** from `int` to `*int`. (Report 5)
13. **Add GitHub Actions CI workflow.** (Report 6)

### 4.2 P1 -- Short-Term (High-Severity Issues)

Address within 1-2 sprints:

1. **Add `http.MaxBytesReader` and `io.LimitReader`** across all modules for unbounded reads. (All reports)
2. **Add `context.Context`** to all functions performing network I/O without it. (Reports 2, 3, 4)
3. **Replace `time.Sleep` with context-aware select** in retry loops. (Report 1)
4. **Move synchronous network I/O to async `tea.Cmd`** in TUI. (Report 4)
5. **Add auth checks to workflow handlers.** (Report 1)
6. **Encrypt `worker_api_bearer_token`** in DB. (Report 1)
7. **Add audit logging to internal orchestrator proxy.** (Report 2)
8. **Close lifecycle response bodies** in SBA. (Report 3)
9. **Set default HTTP client timeout** in cynork.
   Make `Client.Token`/`BaseURL` unexported with synchronized accessors. (Report 4)
10. **Extract SBA result status constants** and create shared status mapping. (Report 5)
11. **Add BDD feature files** for ACCESS, AGENTS, MCPGAT, MCPTOO domains. (Report 6)
12. **Add E2E to CI** (`just e2e --tags no_inference`). (Report 6)

### 4.3 P2 -- Planned (Medium-Severity Improvements)

Address within the next release cycle:

1. **Wrap database operations in transactions** (lease, checkpoint, task create, preference upsert). (Report 1)
2. **Split `Store` interface** into focused sub-interfaces. (Report 1)
3. **Add pagination** to unbounded queries. (Reports 1, 2)
4. **Batch N+1 queries.** (Report 1)
5. **Add AAD to GCM** and HKDF to PQ path in secure store. (Report 2)
6. **Add GORM index tags** on telemetry query-hot columns. (Report 2)
7. **Inject dependencies into PMA handler** (eliminate per-request `os.Getenv`/`NewMCPClient`). (Report 3)
8. **Unify TUI dual scrollback model.** (Report 4)
9. **Add validation** to `workerapi.RunJobRequest` and `nodepayloads`. (Report 5)
10. **Merge BDD coverage into Go profiles** or document as separate metric. (Report 6)

### 4.4 P3 -- Longer-Term (Maintenance and Debt)

1. **Adopt versioned migrations** to replace AutoMigrate. (Reports 1, 2)
2. **Align PMA startup** with worker-instruction model (REQ-ORCHES-0150). (Report 1)
3. **Implement continuous PMA monitoring** (REQ-ORCHES-0129). (Report 1)
4. **Extract centralized config package** for worker node. (Report 2)
5. **Replace global mutable test hooks** with dependency injection across all modules. (All reports)
6. **Add load/performance testing** and chaos/failure E2E scenarios. (Report 6)

## 5 Component Health Assessment

- **Orchestrator:** Functional but has the most findings (51).
  Spec compliance gaps in `planning_state` and MCP authorization are the biggest risks.
  Database layer needs transaction safety.
  Handlers need service-layer extraction.

- **Worker Node:** Solid for MVP.
  Critical fix needed for pod network isolation.
  Secure store cryptography is sound but needs memory hygiene.
  Telemetry needs indexes and retention for `node_boot`.

- **Agents (PMA/SBA):** Working streaming and agent loop.
  Three PMA spec features are entirely missing.
  SBA prompt construction violates context ordering spec.
  Both need body size limits.

- **Cynork CLI/TUI:** Rich, functional TUI.
  Critical data race in thread management.
  Synchronous network calls in Update freeze UI.
  Gateway client needs timeouts and context propagation.

- **Shared Libraries:** Clean contracts with 97.1% coverage.
  Critical ExitCode serialization bug.
  Status constants need consolidation.
  Validation functions missing for several contract types.

- **Testing/CI:** Mature infrastructure with excellent tagging discipline.
  No automated CI pipeline is the biggest operational risk.
  44% of requirement domains lack BDD coverage.

## 6 Specification Compliance Scorecard

Key requirement gaps by domain:

- **ORCHES:** REQ-0150 (PMA via worker), REQ-0176/0177/0178 (planning_state), REQ-0129 (continuous PMA monitoring)
- **MCPGAT:** REQ-0106 (fail-closed), REQ-0113 (per-tool enable/disable), REQ-0114 (PM/PA allowlists), REQ-0107 (audit completeness)
- **WORKER:** REQ-0174 (network restriction), REQ-0163 (proxy audit logging)
- **PMAGNT:** REQ-0124 (overwrite events), REQ-0125 (secret scan), REQ-0129 (keep-warm)
- **SBAGNT:** REQ-0113 (prompt construction and context ordering)
- **CLIENT:** REQ-0171 (model selection), REQ-0216/0217/0218 (thinking/tool/overwrite rendering)

## 7 Individual Report References

- [Report 1: Orchestrator](2026-03-29_review_report_1_orchestrator.md)
- [Report 2: Worker Node](2026-03-29_review_report_2_worker_node.md)
- [Report 3: Agents (PMA and SBA)](2026-03-29_review_report_3_agents.md)
- [Report 4: Cynork CLI/TUI](2026-03-29_review_report_4_cynork.md)
- [Report 5: Shared Libraries and Cross-Module Contracts](2026-03-29_review_report_5_shared_libs.md)
- [Report 6: Testing, BDD, E2E, and CI/CD](2026-03-29_review_report_6_testing.md)
