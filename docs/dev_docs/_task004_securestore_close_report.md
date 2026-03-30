# Task 4 Completion: Secure Store Close

- [Changes](#changes)
- [Tests](#tests)
- [Deviations](#deviations)

## Changes

- `worker_node/internal/securestore/store.go`: `Close()` zeros `key` with existing `zeroBytes`, clears `kemKey`, uses `runtime.KeepAlive` after zeroing.
  Idempotent.
- `worker_node/internal/nodeagent/nodemanager.go`: `defer store.Close()` after `securestore.Open` in `syncManagedServiceAgentTokens`.
- `worker_node/internal/workerapiserver/embed.go`: On embedded Worker API shutdown, `Close()` the secure store from proxy config when non-nil (node-manager embedded path).
- `worker_node/internal/securestore/store_test.go`: `TestStoreClose` verifies backing array zeroed.

**Date:** 2026-03-29.

## Tests

- `go test -v -run TestStoreClose ./worker_node/internal/securestore/...`
- `go test -cover ./worker_node/internal/securestore/...`, `go test ./worker_node/...`

## Deviations

- Plan cited `node-manager/main.go`; the long-lived store for the embedded Worker API is owned by `workerapiserver.RunEmbedded` shutdown and token sync uses `nodemanager.syncManagedServiceAgentTokens` - both wired.
