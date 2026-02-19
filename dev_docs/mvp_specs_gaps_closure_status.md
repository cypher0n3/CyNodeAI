# MVP Specs Gaps Closure Status

- [1 Summary](#1-summary)
- [2 Gap-by-Gap Status](#2-gap-by-gap-status)
- [3 Conclusion](#3-conclusion)

## 1 Summary

This report checks whether the gaps identified in `dev_docs/mvp_tech_specs_gaps_analysis.md` have been closed.

**Conclusion:** The blocking schema gap is closed; optional gaps (MCP statement, requirement scope mapping, BDD) may remain.

## 2 Gap-by-Gap Status

Status of each gap category from the analysis.

### 2.1 Resolved (Per Gaps Analysis)

- Bootstrap payload shape, Worker API auth, config delivery API shape, inference precondition/fail-fast, Phase 1 scope decisions (Section 1.1 of completion plan).
- Sandbox constraints and Worker API ambiguities for Phase 1 (Section 3.2).

Canonical content lives in permanent locations only (not in dev_docs):

- **Requirements:** [`docs/requirements/bootst.md`](../docs/requirements/bootst.md) (e.g. REQ-BOOTST-0002), [`docs/requirements/worker.md`](../docs/requirements/worker.md).
- **Tech specs:**
  - [`docs/tech_specs/node_payloads.md`](../docs/tech_specs/node_payloads.md) (bootstrap, config payloads, config ack)
  - [`docs/tech_specs/worker_api.md`](../docs/tech_specs/worker_api.md) (auth, Phase 1 constraints)
  - [`docs/tech_specs/sandbox_container.md`](../docs/tech_specs/sandbox_container.md) (workspace, env)
  - [`docs/tech_specs/node.md`](../docs/tech_specs/node.md) (startup flow)
  - [`docs/tech_specs/orchestrator.md`](../docs/tech_specs/orchestrator.md) (config delivery GET/POST, Phase 1 job dispatch)
  - [`docs/tech_specs/postgres_schema.md`](../docs/tech_specs/postgres_schema.md) (Nodes: worker_api_target_url, bearer token, config_ack_*)
  - [`docs/tech_specs/_main.md`](../docs/tech_specs/_main.md) (Phase 1 bullets, job dispatch path)
- **Example configs:** [`docs/examples/orchestrator_bootstrap_example.yaml`](../docs/examples/orchestrator_bootstrap_example.yaml) (orchestrator), [`docs/examples/node_bootstrap_example.yaml`](../docs/examples/node_bootstrap_example.yaml) (worker/node).
- **Features:** [`features/single_node_happy_path.feature`](../features/single_node_happy_path.feature) (config fetch, config ack, dispatcher per-node, inference precondition).
- Temporary plan in `dev_docs/` (e.g. completion plan) is not the source of truth; use the above for implementation and traceability.

### 2.2 Schema and Config Storage (Section 4) - **Closed**

- **4.1 and 4.2:** `docs/tech_specs/postgres_schema.md` Nodes table now defines:
  - `worker_api_target_url`, `worker_api_bearer_token` for config delivery and dispatch.
  - `config_ack_at`, `config_ack_status`, `config_ack_error` for config acknowledgement storage.

### 2.3 Phase 0 / MCP Gateway (Section 2.1) - **Closed**

- Phase 1 job dispatch is now stated in [`docs/tech_specs/_main.md`](../docs/tech_specs/_main.md) and [`docs/tech_specs/orchestrator.md`](../docs/tech_specs/orchestrator.md): direct HTTP to Worker API; MCP gateway not in loop.

### 2.4 Requirements vs MVP Scope (Section 5) - **NOT Closed**

- No single document maps requirement IDs (ORCHES, USRGWY, WORKER, BOOTST, IDENTY, etc.) to Phase 1 vs deferred.
- Marked optional: a short "MVP requirement scope" in the completion plan or `_main.md` would make scope explicit.

### 2.5 BDD and Traceability (Section 6) - **Closed**

- `features/single_node_happy_path.feature` includes scenarios for node fetches config, node sends config ack, dispatcher uses per-node worker URL and token, and inference precondition (fail fast when no inference path), with requirement and spec tags.

## 3 Conclusion

Closure status of each gap category.

| Gap | Status | Blocking? |
|-----|--------|-----------|
| Schema: worker_api_target_url, bearer token, config ack storage | Closed | Was blocking |
| Phase 1 does not use MCP gateway (explicit statement) | Closed (in _main.md and orchestrator.md) | No (optional) |
| MVP requirement scope (REQ-* to phase mapping) | Open | No (optional) |
| BDD: config delivery/ack, dispatcher per-node, inference precondition | Closed (scenarios in feature file) | No (optional) |

**Recommendation:** Schema and Phase 1 dispatch/BDD gaps are closed in permanent specs and features.
The optional MVP requirement scope mapping (REQ-* to phase) can be added to the completion plan or `_main.md` if desired.

---

Generated 2026-02-19.

Do not update tech specs without explicit direction.
