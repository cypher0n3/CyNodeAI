# MVP Phase 1 Chunks 06 and 07 Completion Report

## Chunk 06: Node-Aware Dispatch

Dispatch is now node-aware: the dispatcher uses per-node Worker API URL and bearer token from config delivery, and only targets nodes that are active and have acknowledged config with Worker API details.

### Chunk 06 Changes

- **Database:** Added `ListDispatchableNodes(ctx)` (active + config ack applied + worker URL and token set) and `UpdateNodeWorkerAPIConfig(ctx, nodeID, targetURL, bearerToken)`.
  Handler persists delivered URL and token in `GetConfig` so dispatch can read them.
- **Dispatcher:** No longer uses global `WORKER_API_URL` or `WORKER_API_BEARER_TOKEN`.
  Uses `ListDispatchableNodes` and per-node `WorkerAPITargetURL` and `WorkerAPIBearerToken` for the outbound call.
- **Tests:** Control-plane tests use a `makeDispatchableNode` helper; added `TestDispatchOnce_NoDispatchableNodes`; error-path test uses `listDispatchableNodesErrorStore`.
- **BDD:** Step "the node X has worker_api_target_url and bearer token in config" starts a fake worker and sets node worker config and config ack.
  Steps for "orchestrator selects node for dispatch", "orchestrator calls Worker API at configured URL", and "request includes bearer token" assert on the captured request.

### Files Touched

- `orchestrator/internal/database/database.go`, `nodes.go`
- `orchestrator/internal/handlers/nodes.go`
- `orchestrator/cmd/control-plane/dispatcher.go`, `main_test.go`
- `orchestrator/internal/testutil/mock_db.go`
- `orchestrator/_bdd/steps.go`

## Chunk 07: E2E Demo Hardening

E2E happy path no longer relies on manually setting the Worker API token on the node; the token is delivered via config and the node manager starts the worker-api with it.

### Chunk 07 Changes

- **Script:** `scripts/setup-dev.sh` uses `WORKER_API_TARGET_URL` for the control-plane. `start_node` only starts the node-manager; the node manager fetches config and starts the worker-api with the delivered token (no `WORKER_API_BEARER_TOKEN` on the node).
- **Documentation:** `dev_docs/PHASE1_STATUS.md` updated with node-aware dispatch and config-delivered token; this report added.

### Additional Notes

- Orchestrator BDD task-lifecycle scenario "Dispatcher uses per-node worker URL and token" has concrete step definitions.
  E2E feature steps under `features/e2e/` that remain skipped are full end-to-end and are covered by `just e2e`.

## Validation

Run: `just validate-feature-files`, `just test-go`, `just lint-go-ci`, `POSTGRES_TEST_DSN="..." just test-bdd`, `just ci`, `just e2e`.
