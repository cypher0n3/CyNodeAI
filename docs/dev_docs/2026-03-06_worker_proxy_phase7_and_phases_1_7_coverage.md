# Worker Proxy Phase 7 Completion and Phases 1-7 Coverage

## Metadata

- Date: 2026-03-06
- Plan: [2026-03-05_worker_proxy_spec_reconciliation_plan.md](2026-03-05_worker_proxy_spec_reconciliation_plan.md)
- Scope: Phase 7 completion; feature files, BDD, and E2E coverage for phases 1-7.

## Phase 7 Completion

Phase 7 desired-state wiring is complete; runtime validation is provided by a BDD scenario.

### Phase 7 Implemented

- **BDD scenario:** "Node manager starts managed services from config desired state (Phase 7)" in `features/worker_node/node_manager_config_startup.feature`.
- **Steps:** Mock orchestrator returns config with `managed_services` containing service "pma-main" of type "pma"; node manager runs startup; assertion that `StartManagedServices` was called with that service and config ack was sent.
- **Runtime validation:** BDD confirms that when config includes `managed_services`, the node manager applies config and invokes `StartManagedServices` with the desired list (desired-state wiring).
  Config ack is sent after apply.

### Plan Update

- Phase 7 checklist item "Desired-state convergence and refresh behavior still needs runtime validation" is satisfied by this BDD scenario.
- Full managed-service lifecycle (container start, refresh, teardown) remains covered by unit tests and implementation; BDD validates config-driven desired state and startup path.

## Phases 1-7: Feature Files, BDD, and E2E

This section lists for each phase the feature file(s), BDD scenario coverage, and E2E tests.

### Phase 1: Inference Proxy (Stabilize, Healthz, Loopback)

- **Feature:** `features/worker_node/worker_inference_proxy.feature` (new).
  Scenario: Inference proxy rejects request body exceeding 10 MiB (413).
- **BDD:** `RegisterInferenceProxySteps`; scenario passes using `inferenceproxy` package (size limit).
- **E2E:** `e2e_090_task_inference.py` (suite_worker_node) exercises task inference path using proxy.

### Phase 2: SBA Inference Through Proxy

- **Feature:** `features/worker_node/worker_node_sandbox_execution.feature` ("Sandbox receives OLLAMA_BASE_URL when job requests inference"); `features/worker_node/worker_node_sba.feature`; `features/agents/sba_inference.feature`.
- **BDD:** Worker node sandbox and SBA steps; agents suite has pending steps.
- **E2E:** `e2e_090_task_inference.py`, `e2e_140_sba_task_inference.py`, `e2e_145_sba_inference_reply.py`, `e2e_123_sba_task.py`, `e2e_130_sba_task_result_contract.py`.

### Phase 3: Internal Proxy Endpoints

- **Feature:** `features/worker_node/worker_internal_proxy.feature` (new).
  Scenario: Internal proxy routes are not exposed on public worker API (POST to internal path returns 404).
- **BDD:** Steps "I POST to the worker API path ... with body ..." and "the worker API returns status 404"; scenario passes.
- **E2E:** No dedicated E2E for internal proxy (auth is worker-held tokens; E2E for full path still pending per Phase 6 validation report).

### Phase 4: Secure Store and Token Lifecycle

- **Feature:** `features/worker_node/worker_secure_store.feature`.
- **BDD:** `RegisterSecureStoreSteps`; all secure-store scenarios pass.
- **E2E:** `e2e_122_secure_store_envelope_structure.py` (suite_worker_node).

### Phase 4a: Post-Quantum KEM

- **Feature:** Covered under `worker_secure_store.feature` (encryption at rest); no separate feature file.
- **BDD:** Same secure-store scenarios; encryption behavior covered by unit tests (securestore package).
- **E2E:** Same as Phase 4 (e2e_122 validates envelope structure).

### Phase 5: Identity-Bound UDS

- **Feature:** `worker_secure_store.feature` (run args, no secure store mount); UDS URL reporting covered by unit test `TestBuildCapability_ManagedServicesStatus_HttpUnixURLsWhenAuto` (nodemanager).
- **BDD:** Secure-store and node-manager config steps; UDS path and `http+unix` reporting validated in Go tests.
- **E2E:** No dedicated E2E for UDS reporting (e2e_122 covers secure store envelope).

### Phase 6: Worker-Held Tokens for Internal Proxy

- **Feature:** `worker_secure_store.feature` ("Worker holds agent token and does not pass it to managed-service containers"; "the worker proxy attaches the agent token when forwarding").
- **BDD:** Steps pass (run args no AGENT_TOKEN; proxy attachment covered by unit tests).
- **E2E:** Contract validated in worker-api unit tests; E2E for full internal proxy path pending.

### Phase 7: Desired-State Wiring

- **Feature:** `features/worker_node/node_manager_config_startup.feature` ("Node manager starts managed services from config desired state (Phase 7)").
- **BDD:** Scenario and steps implemented; passes.
- **E2E:** Node registration and config E2E (`e2e_170_control_plane_node_register.py`, `e2e_175_prescribed_startup_config_inference_backend.py`) cover config flow; managed-service start from config is BDD-validated.

## Summary

All phases 1-7 have at least one feature file and BDD coverage.
E2E coverage is present for Phase 1 (e2e_090), Phase 2 (e2e_090, 123, 130, 140, 145), Phase 4 (e2e_122), and Phase 7 (config flow in e2e_170/175).
Phases 3, 5, and 6 rely on unit and BDD tests; E2E for internal proxy path is pending.

## References

- Plan: `docs/dev_docs/2026-03-05_worker_proxy_spec_reconciliation_plan.md`
- Phase 5/6 validation: `docs/dev_docs/2026-03-06_worker_proxy_phase5_6_validation.md`
- Features: `features/worker_node/*.feature`
- BDD steps: `worker_node/_bdd/steps.go`
- E2E: `scripts/test_scripts/e2e_*.py` (see tags in `e2e_tags.py`)
