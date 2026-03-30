# Task 2 completion: container name matching

**Date:** 2026-03-29

## Changes

- `worker_node/cmd/node-manager/main.go`: `startOneManagedService` uses `containerps.NameListed` instead of `strings.Contains` for podman `ps` output.
- `worker_node/internal/containerps/`: New package with `NameListed` (exact line match for `{{.Names}}` output).
- `worker_node/internal/nodeagent/nodemanager_config.go`: Replaced duplicate `containerNameMatches` with `containerps.NameListed`.
- `worker_node/cmd/node-manager/container_match_test.go`: `TestContainerNameMatch` (table tests); split from `main_test.go` to stay under the 1000-line lint cap.

## Tests

- Red: `TestContainerNameMatch` failed with substring `Contains` semantics.
- Green: `go test ./worker_node/...`, `go test -cover ./worker_node/...` (packages >= 90%).
- `just lint-go`, `just e2e --tags worker,no_inference` (114 tests, 3 skipped).

## Deviations

- None.
