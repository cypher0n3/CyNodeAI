# Worker Proxy Phase 5 and Phase 6 Validation Report

## Metadata

- Date: 2026-03-06
- Plan: [2026-03-05_worker_proxy_spec_reconciliation_plan.md](2026-03-05_worker_proxy_spec_reconciliation_plan.md)
- Scope: Phase 5 (identity-bound UDS URL reporting) and Phase 6 (internal proxy auth to worker-held tokens) completion and validation.

## Phase 5 Validation

Phase 5 required tests for `http+unix://...` URL reporting when `binding=per_service_uds`.

### Completed Work

- **Tests for `http+unix://...` URL reporting when `binding=per_service_uds`**
  - Added `TestBuildCapability_ManagedServicesStatus_HttpUnixURLsWhenAuto` in `worker_node/internal/nodemanager/nodemanager_test.go`.
  - Asserts that when node config sets `orchestrator.mcp_gateway_proxy_url` and `orchestrator.ready_callback_proxy_url` to `"auto"`, the capability report's `managed_services_status.services[].agent_to_orchestrator_proxy` has:
    - `binding == "per_service_uds"`
    - `mcp_gateway_proxy_url` and `ready_callback_proxy_url` with prefix `http+unix://` and the correct path suffixes (`/v1/worker/internal/orchestrator/mcp:call`, `/v1/worker/internal/orchestrator/agent:ready`).
  - Run with: `cd worker_node && go test ./internal/nodemanager/... -run TestBuildCapability_ManagedServicesStatus_HttpUnixURLsWhenAuto -v`

### Phase 5 Result

- Phase 5 checklist item "Tests for http+unix URL reporting when binding=per_service_uds" is satisfied.
- UDS identity-resolution tests for token/service mismatch were already in place (worker-api).

## Phase 6 Validation

Phase 6 required contract and (optionally) E2E validation for the internal agent-to-orchestrator proxy path.

### Contract Validation (Unit Tests)

Internal agent-to-orchestrator proxy auth is validated by existing unit tests in `worker_node/cmd/worker-api/main_test.go`:

- **No token in agent request; token from secure store**
  - `TestInternalOrchestratorProxy_MCP`: Request has no `Authorization` header; caller identity is set via `withCallerServiceID` (simulating identity-bound transport).
    Upstream receives `Bearer agent-token` from secure store.
    Asserts 200 when store has token for that service.
- **Unknown identity fails closed**
  - `TestInternalOrchestratorProxy_AuthAndLoopback`: Request from loopback without caller identity yields 401 (missing identity binding).
- **Missing secure store fails closed**
  - `TestInternalOrchestratorProxy_SecureStoreUnavailable`: When `SecureStore` is nil, request returns 502.
- **Token missing for identity fails closed**
  - `TestInternalOrchestratorProxy_TokenMissingForIdentity`: Store has no token for the resolved `service_id`; response 401.
- **Public mux does not expose internal routes**
  - `TestPublicMux_DoesNotExposeInternalProxyRoutes`: POST to internal proxy path on public mux returns 404.

Run with: `cd worker_node && go test ./cmd/worker-api/... -run TestInternalOrchestratorProxy -v`

### Phase 6 Result

- Phase 6 checklist item "Contract and end-to-end validation for internal agent-to-orchestrator proxy path" is satisfied for **contract** (unit) validation.
- **E2E validation** for the internal agent-to-orchestrator proxy path remains pending (plan line 112: "E2E still pending").
  E2E will require full stack (node manager, worker API, UDS listeners, secure store, mock orchestrator) and is out of scope for this validation.

## Summary

- **Phase 5**  
  Tests for http+unix URL reporting when binding=per_service_uds: Done (new nodemanager test).
- **Phase 6**  
  Contract validation (no token in request; identity + secure store): Done (existing worker-api tests).
- **Phase 6**  
  E2E validation for internal proxy path: Pending.

## References

- Plan: `docs/dev_docs/2026-03-05_worker_proxy_spec_reconciliation_plan.md`
- Spec: `docs/tech_specs/worker_node.md` (Agent-To-Orchestrator UDS Binding), `docs/tech_specs/worker_api.md` (CYNAI.WORKER.ManagedAgentProxyBidirectional)
- Code: `worker_node/internal/nodemanager/nodemanager.go` (`buildAgentToOrchestratorProxyStatus`, `buildManagedServicesStatus`), `worker_node/cmd/worker-api/main.go` (`handleInternalOrchestratorProxy`, `validateInternalProxyRequest`)
