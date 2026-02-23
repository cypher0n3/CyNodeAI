# P1-02: Orchestrator Readyz Gates on PMA Warmup

## Summary

**Date:** 2026-02-22. **Gap:** Orchestrator readyz did not gate on PMA warmup; ready could be 200 while cynode-pma was still starting, causing chat for cynodeai.pm to 503.

- Extended control-plane `GET /readyz` with a PMA readiness check when `PMA_ENABLED=true`.
- Default for `PMA_ENABLED` is now `true` (was `false`).
- When PMA is enabled, readyz returns 503 with body `PMA not ready (cynode-pma not reachable or not yet started)` until the PMA responds 200 to `GET /healthz` on its listen address.

## Changes

Implementation details by area.

### 1. Config (`orchestrator/internal/config/config.go`)

- `PMAEnabled` default: `getBoolEnv("PMA_ENABLED", true)`.
- Comment updated to "default true".

### 2. Control-Plane Readyz (`orchestrator/cmd/control-plane/main.go`)

- **`pmaReady(ctx, listenAddr string) bool`:** Builds `http://127.0.0.1:<port>/healthz` from `PMAListenAddr` (via `net.SplitHostPort`), performs GET with 2s timeout; returns true only on HTTP 200.
  Closes response body (errcheck-safe).
- **`readyzHandler`** now takes `(store, cfg *config.OrchestratorConfig, logger)`.
  After the existing DB and dispatchable-nodes checks, when `cfg != nil && cfg.PMAEnabled` it calls `pmaReady(ctx, cfg.PMAListenAddr)`; if false, responds 503 with the new reason text.
- Order of checks: (1) DB error, (2) PMA not ready (if enabled), (3) no dispatchable nodes, (4) 200 "ready".

### 3. Tests (`orchestrator/cmd/control-plane/main_test.go`)

- Existing readyz tests pass `cfg` with `PMAEnabled: false` so they do not depend on a running PMA.
- **TestReadyzHandler_PMANotReady:** PMA enabled, listen on `:19999` (nothing listening); dispatchable node present; expect 503 and body containing "PMA not ready".
- **TestReadyzHandler_PMAReady:** Mock HTTP server for `/healthz` (200); PMA enabled with that server's port; dispatchable node; expect 200 "ready".
- Tests that call `run()` or `runMainWithContext()` and do not start a real cynode-pma: either set `cfg.PMAEnabled = false` after `LoadOrchestratorConfig()` or set `PMA_ENABLED=false` in the environment and restore in defer.

### 4. Lint Fixes

- `resp.Body.Close()` wrapped in `defer func() { _ = resp.Body.Close() }()` for errcheck.
- Repeated test URL/token replaced with constants `testWorkerAPIURL` and `testWorkerAPIToken` for goconst.

## Verification

- `just lint-go-ci` (orchestrator): 0 issues.
- `just test-go-cover` (orchestrator): control-plane tests pass; coverage 89.6% (meets 89% minimum for control-plane).
- BDD: Orchestrator suite uses its own inline readyz stub (nodes only); no change required for "no inference path" scenario.

## Notes

- PMA (cynode-pma) already exposes `GET /healthz`; no change in agents.
- To disable PMA and avoid the readiness check (e.g. in tests or minimal deployments), set `PMA_ENABLED=false`.
