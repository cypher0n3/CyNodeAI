# Why `just ci` Can Appear to Hang

## Summary

The hang occurs during **`test-go-cover`** in the **orchestrator** module.
A likely cause is **control-plane**'s `TestRunMain_OpenFails`: it calls `runMain()` with an invalid `DATABASE_URL` (unreachable host/port) and **no connection timeout** in the DSN, so the driver can block for the OS TCP connect timeout (often 75s or more on Linux) before failing.

## Root Cause: `TestRunMain_OpenFails` (Control-Plane)

- The test sets `DATABASE_URL=postgres://...@127.0.0.1:1/...` so that `runMain()` should fail when opening the DB.
- The DSN did not include `connect_timeout`.
- The Postgres driver then attempts a TCP connection to `127.0.0.1:1`; when nothing is listening, the connect can block for the system TCP timeout (e.g. 75-120s) instead of failing quickly.
- **Fix (applied):** Add `connect_timeout=2` to that test DSN so the test fails in a few seconds.

## Other Possible Causes

- **orchestrator/internal/database** when `POSTGRES_TEST_DSN` is unset: `TestMain` uses testcontainers (`postgres.Run`, `ConnectionString`) with `context.Background()`, so if the provider (Podman) or library blocks, there is no upper bound.
  Prefer setting `POSTGRES_TEST_DSN` to a real Postgres so testcontainers is skipped, or ensure the image is pre-pulled and the daemon is responsive.

- **worker_node/cmd/node-manager**'s `TestStartOllama_Success`: runs `podman run ...`; can block if Podman is slow (skipped when `startOllama()` returns an error).

## How to Avoid or Diagnose

- **Skip testcontainers in CI:** Set `SKIP_TESTCONTAINERS=1` when running `just ci` (or `just test-go-cover`) so the database package does not start a container; integration tests will skip and CI will not hang.
- **Use a real Postgres:** Set `POSTGRES_TEST_DSN` so database tests use your DB and skip testcontainers.
- **Bounded wait:** Database TestMain now uses a 90s timeout for container setup; if testcontainers/Podman blocks, the run fails after 90s instead of hanging indefinitely (only if the library respects context).
- **See which package hangs:** Run with one package at a time: `cd orchestrator && go test ./... -coverprofile=... -p 1`.

## References

- `orchestrator/cmd/control-plane/main_test.go`: `TestRunMain_OpenFails` (invalid DSN; now uses `connect_timeout=2`).
- `orchestrator/internal/database/testcontainers_test.go`: `TestMain` uses unbounded context for testcontainers.
