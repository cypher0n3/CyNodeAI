# Task 1 completion: PMA WriteTimeout

**Date:** 2026-03-29 (local)

## Changes

- `agents/internal/pma/chat.go`: Renamed `pmaLangchainCompletionTimeout` to exported `LangchainCompletionTimeout` (300s) for a single source of truth aligned with orchestrator/gateway timeouts.
- `agents/internal/pma/langchain.go`: Comment reference updated.
- `agents/cmd/cynode-pma/main.go`: Introduced `pmaHTTPWriteTimeout` set to `0` so `http.Server` does not apply a write deadline shorter than streaming inference. `WriteTimeout` uses this constant.
- `agents/cmd/cynode-pma/main_test.go`: Added `TestWriteTimeout` asserting write timeout is either disabled (`0`) or at least `LangchainCompletionTimeout + 10s`.

## Tests run

- `go test -v -run TestWriteTimeout ./agents/cmd/cynode-pma/...` (pass)
- `go test -cover ./agents/...` (all packages >= 90%)
- `just lint-go` (pass)
- `just e2e --tags streaming,pma_inference` (pass; 6 skips for optional stream shapes)

## Deviations

- None.
