# Traceability Follow-up (2026-02-22)

- [Summary](#summary)
- [Changes](#changes)
- [Other Gaps (No Code Change)](#other-gaps-no-code-change)

## Summary

Addressed the **Maintainability Section 1** traceability gap from
`dev_docs/2026-02-22_go_implementation_code_review.md` by adding requirement/spec
IDs in code comments.

## Changes

- **Orchestrator control-plane** (`orchestrator/cmd/control-plane/main.go`): readyz
  comment now references REQ-ORCHES-0120, REQ-ORCHES-0129 in addition to
  REQ-ORCHES-0119 and CYNAI.
    ORCHES.
    Rule.
    HealthEndpoints.
- **Config version ULID** (`orchestrator/internal/handlers/nodes.go`):
  `resolveConfigVersion` comment now references worker_node_payloads.md and
  CYNAI.
    WORKER.
    Payload.
    ConfigurationV1.
- **Worker API** (`worker_node/cmd/worker-api/main.go`): REQ-WORKER-0140,
  REQ-WORKER-0141 for healthz; REQ-WORKER-0140, REQ-WORKER-0142 for readyz;
  REQ-WORKER-0145 for request body size limit (decodeRunJobRequest and 10 MiB).
- **MCP gateway** (`orchestrator/cmd/mcp-gateway/main.go`): toolCallHandler
  comment now references mcp_tool_call_auditing (P2-02).

## Other Gaps (No Code Change)

- **P1-02 (readyz + PMA warmup):** Already implemented (pmaReady, PMA not ready
  message, PMA_ENABLED default in config).
- **CI parity for agents:** Justfile already includes `agents` in `go_modules`;
  coverage/lint/vulncheck run over all modules.
- **User-gateway health:** No change; spec does not require user-gateway to
  expose health/readyz unless product requests it.
- **Phase 2 (P2-01, P2-03, allow path):** Deferred as future work per review.
