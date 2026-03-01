# MVP E2E: SBA Job Test 5E (2026-02-28)

## Summary

Implemented optional E2E scenario for SBA job per [mvp_plan.md](../mvp_plan.md) Suggested Next Work item 1.

## Changes

Edits to setup script and MVP plan.

### Setup Script (`scripts/setup-dev.sh`)

- Added `ensure_sba_runner_build_if_delta()` to build `cynodeai-cynode-sba:dev` when context (agents, go_shared_libs, go.work) changes.
- full-demo now calls `ensure_sba_runner_build_if_delta` after `ensure_inference_proxy_build_if_delta` so the node can run SBA jobs.
- Added **Test 5e**: create task with `cynork task create -p "echo from SBA" --use-sba`, poll task result up to 90s, assert `jobs[0].result` parses to JSON containing `sba_result`.

### MVP Plan (`docs/mvp_plan.md`)

- P2-10-orchestrator: noted optional E2E implemented (Test 5e, ensure_sba_runner_build_if_delta).
- Suggested Next Work item 1 marked done.

## Verification

- `just ci` passed (lint, test-go-cover, test-bdd, lint-containerfiles, etc.).
- E2E run via `just e2e --stop-on-success` was started; full run exercises Test 5e after stack and node are up.
