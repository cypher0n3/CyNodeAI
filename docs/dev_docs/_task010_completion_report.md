# Task 10 Completion - Sandbox Pod Network Isolation (REQ-WORKER-0174)

## Summary

Pod workload containers (`buildSandboxRunArgsForPod`, `buildSBARunArgsForPod`) now pass **`--network=none`** so sandboxes have no direct IP egress.
The inference proxy sidecar keeps the pod default network for `OLLAMA_UPSTREAM_URL`.
UDS access to the proxy remains via the shared `/run/cynode` bind mount.

## Files

- `worker_node/internal/executor/executor.go` - append `--network=none` to sandbox/SBA pod `podman run` argv.
- `worker_node/internal/executor/executor_test.go`, `executor_runjob_sba_test.go` - assert flag present.
- `docs/dev_docs/_task010_network_isolation_architecture.md` - architecture note.
- `scripts/test_scripts/e2e_0325_sandbox_network_isolation.py` - E2E: `nc` to 8.8.8.8:53 must fail (`BLOCKED`).

## Validation

- `go test -cover ./worker_node/...` - pass (packages >=90%).
- `go test ./worker_node/_bdd` - pass.
- `just e2e --tags worker,no_inference` - pass (includes `e2e_0325`).
- `just setup-dev restart` + stack E2E as above.

## Deviations

- BDD tag `@req_WORKER_0174` is not defined in `features/`; worker `_bdd` suite used as the automated gate instead.
