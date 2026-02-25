# MVP Status Report

<!-- Date: 2026-02-23. Branch: mvp/phase-1. Scope: Full MVP phases 0-2; CI and coverage current -->

- [Summary](#summary)
- [Phase Status](#phase-status)
- [Test Coverage and CI](#test-coverage-and-ci)
- [Key Deliverables](#key-deliverables)
- [Deep Code Review (Implementation vs Tech Specs)](#deep-code-review-implementation-vs-tech-specs)
- [Remaining Work](#remaining-work)
- [References](#references)

## Summary

CyNodeAI MVP implementation is in progress per `docs/mvp_plan.md`.
Phases 0, 1, 1.5, and 1.7 are complete; Phase 2 (MCP in the loop) has P2-02 (audit) and P2-03 (preference tools) implemented; Phase 2 scope includes SBA implementation (P2-09, P2-10) per updated MVP plan.
All packages meet the 90% coverage threshold; `just ci` passes.

## Phase Status

- **Phase:** Phase 0
  - description: Foundations (schema, payloads, MCP gateway spec)
  - status: Complete
- **Phase:** Phase 1
  - description: Single-node happy path (registration, dispatch, sandbox, auth, task APIs)
  - status: Complete
- **Phase:** Phase 1.5
  - description: Inference in sandbox, E2E inference, CLI, prompt interpretation
  - status: Complete
- **Phase:** Phase 1.7
  - description: cynode-pma, PMA in orchestrator stack, OpenAI-compatible chat routing
  - status: Complete
- **Phase:** Phase 2
  - description: MCP in the loop (tool enforcement, auditing, preference tools, SBA implementation)
  - status: In progress
- **Phase:** Phase 3
  - description: Multi-node robustness (scheduling, reliability, telemetry)
  - status: Not started
- **Phase:** Phase 4
  - description: API Egress, external routing, admin console
  - status: Not started

### Phase 2 Detail

- **P2-02 (audit):** MCP tool call audit table, store method, mcp-gateway writes one audit record per `POST /v1/mcp/tools/call`.
  Complete.
- **P2-03 (preference tools):** Schema (`PreferenceEntry`, `PreferenceAuditLog`), store (`GetPreference`, `ListPreferences`, `GetEffectivePreferencesForTask`), MCP gateway allow path for `db.preference.get`, `db.preference.list`, `db.preference.effective`.
  Complete.
- **P2-01 (scoping/schema):** Full MCP protocol and per-tool schema enforcement.
  Not started.
- **P2-09 (SBA binary and image):** Implement cynode-sba binary and SBA runner Containerfile per cynode_sba.md (job spec validation, MVP step types, result contract, non-root).
  Not started.
- **P2-10 (SBA integration):** Worker API and orchestrator integration for SBA runner jobs (job spec to /job/job.json, result from /job/result.json, persist before clear).
  Not started.

## Test Coverage and CI

- **Orchestrator:** All packages at or above 90% (database uses testcontainers when `POSTGRES_TEST_DSN` unset; coverage check uses rounded percentage so 90.0% display passes).
- **mcp-gateway:** 90%+; tests include preference handlers, store error paths, and testcontainers for real-DB path.
- **worker_node, cynork, agents:** 90%+.
- **CI:** `just ci` runs lint (shell, Go, Python, Markdown), vulncheck, doc validation, feature-file validation, `just test-go-cover`, `just test-bdd`, containerfile lint.
  All pass.

## Key Deliverables

- **Orchestrator:** control-plane, user-gateway, mcp-gateway, api-egress (scaffold); auth (JWT, Argon2id); database (GORM, migrations, preferences, MCP audit); handlers (tasks, chat, OpenAI-compatible routing); PMA integration and readyz.
- **Worker node:** node-manager (registration, config fetch/ack), worker-api (jobs, sandbox), inference-proxy.
- **Agents:** cynode-pma binary; instructions and role; Containerfile.
- **CLI (cynork):** auth, task create/status/result, list models, chat; BDD suite.
- **Database:** preference_entries, preference_audit_log, mcp_tool_call_audit_log; store methods for preferences and audit.
- **MCP gateway:** `POST /v1/mcp/tools/call` with audit; routing for db.preference.get/list/effective with argument validation and JSON responses.

## Deep Code Review (Implementation vs Tech Specs)

A deep code review of the current implementation against `docs/mvp_plan.md`, `docs/tech_specs/`, and `docs/requirements/` was performed (spec-first validation per senior Go reviewer practices).

### Spec Compliance Summary

- **Orchestrator health/readyz (P1-01, P1-02):** `GET /healthz` (liveness) and `GET /readyz` (readiness) implemented.
  Readyz returns 503 when no dispatchable nodes or when `PMA_ENABLED=true` and PMA is not reachable (`pmaReady` check).
  Compliant with REQ-ORCHES-0120, REQ-BOOTST-0002, REQ-ORCHES-0129 and `CYNAI.ORCHES.Rule.HealthEndpoints`.
- **Config version ULID (P1-03):** Orchestrator emits 26-char Crockford Base32 ULID for node config.
  Node compares lexicographically.
  Compliant with `CYNAI.WORKER.Payload.ConfigurationV1`.
- **Worker API (P1-04, P1-05):** `GET /readyz` (200 "ready" / 503), 413 for oversized body, UTF-8-safe stdout/stderr truncation with flags.
  Compliant with worker_api.md.
- **Task input_mode and prompt interpretation (P1.5-01, P1.5-02):** Default prompt; inference by default; script/commands explicit; prompt not executed as literal shell.
  Compliant with user_api_gateway and REQ-ORCHES-0126/0127/0128.
- **OpenAI-compatible chat and PMA (Phase 1.7):** `GET /v1/models`, `POST /v1/chat/completions`; `cynodeai.pm` routes to PMA; control-plane starts/stops cynode-pma when enabled.
  Compliant with openai_compatible_chat_api.md and cynode_pma.md.
- **MCP tool call auditing (P2-02):** One audit record per `POST /v1/mcp/tools/call`; schema and store match mcp_tool_call_auditing.md and postgres_schema.
  Deny path and allow path (preference tools) both write audit.
- **Preference tools (P2-03):** `db.preference.get` (scope_type, key; scope_id when not system), `db.preference.list` (scope_type; key_prefix, limit, cursor), `db.preference.effective` (task_id).
  Store and schema match user_preferences.md, mcp_tool_catalog.md, postgres_schema.
  Effective resolution: task > project > user > system (group omitted for MVP).
- **CI and agents:** `go_modules` in justfile includes `agents`.
  Lint, vulncheck, test-go-cover, and test-bdd run over all modules including agents.

### Gaps and Recommendations

- **P2-01 (MCP scoping/schema):** Not started.
  When implementing: enforce task_id/run_id/job_id per mcp_gateway_enforcement.md; preserve audit-on-allow.
- **Traceability:** Add requirement IDs (e.g. REQ-ORCHES-0120, REQ-WORKER-0140) in comments at health/readyz, config_version, and MCP audit code for future spec traceability.
- **Phase 1 clarification:** Orchestrator fail-fast scope and capability report schema alignment with worker_node_payloads.md can be tightened in docs or code comments if product needs stricter wording.

Full findings (spec compliance, architecture, security, performance) are in `dev_docs/2026-02-22_go_implementation_code_review.md`.
Items previously flagged there (readyz not gating on PMA, agents outside CI) are now addressed in code (PMA gating and agents in `go_modules`).

## Remaining Work

- **Phase 2:** P2-01 (MCP scoping/schema enforcement); full allow path beyond preference tools (e.g. artifact, sandbox, project tools); **P2-09 (cynode-sba binary and SBA runner image), P2-10 (Worker API and orchestrator integration for SBA jobs)**; LangGraph workflow (P2-04--P2-08).
- **Phase 1 (minor):** Orchestrator fail-fast scope clarification; capability report schema alignment with worker_node_payloads.md; optional requirement-ID comments in code.
- **MVP backlog:** Sandbox container hardening; WebSocket job updates; node heartbeat; OpenAPI; production deployment guides.

## References

- `docs/mvp_plan.md` - Canonical MVP plan and task breakdown
- `dev_docs/PHASE1_STATUS.md` - Phase 1 status and runbook
- `dev_docs/2026-02-22_go_implementation_code_review.md` - Deep code review vs tech specs and MVP plan
- `dev_docs/2026-02-22_phase_2_progress_report.md` - Phase 2 P2-02 progress
- `dev_docs/2026-02-23_phase_2_p2_03_preference_tools_report.md` - P2-03 preference tools
- `dev_docs/2026-02-23_sba_result_artifact_delivery_gap.md` - SBA result/artifact delivery gap and spec updates
- `meta.md` - Project summary and layout
