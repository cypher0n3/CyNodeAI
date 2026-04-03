# Plan 005a - Task 4 Completion Report (Worker NATS)

<!-- Date: 2026-04-01 (UTC) -->

## Summary

Task 4 wires the worker to NATS after registration: `BootstrapData` carries optional `nats` from
`nodepayloads.BootstrapResponse`; `NatsRuntime` uses `natsutil.Connect`, `EnsureStreams`, session
`activity` relay for PMA rows with `CYNODE_*` env (orchestrator injects those env vars for
session-bound PMA services), JetStream publish via `natsutil.PublishSessionActivity`, subscription to
`cynode.node.config_changed.<tenant>.<node_id>` with non-blocking bump into the capability loop, and
JWT refresh via periodic re-registration before NATS JWT expiry.
`runCapabilityLoop` merges NATS-driven config bumps with the existing capability ticker.
Shutdown closes the NATS connection via defer.

## Deliverables

- `worker_node/internal/nodeagent/nats_runtime.go` - connect, relay, subscriber, refresh loop, `ParseNodeIDFromNodeJWT`.
- `worker_node/internal/nodeagent/nodemanager.go` - `BootstrapData.Nats`, `capabilityLoopState`, `RunWithOptions` / `runCapabilityLoop` integration.
- `orchestrator/internal/handlers/nodes.go` - session env on extra PMA managed-service rows.
- Tests: `nats_runtime_internal_test.go` (`TestWorkerNatsConn_*`, parse-node-JWT); orchestrator `nodes_inference_test` env assertions.

## Validation

- `go test -count=1 -run TestWorkerNatsConn ./worker_node/...`
- `go test ./worker_node/...`, `go test ./orchestrator/internal/handlers/...`
- `golangci-lint run ./worker_node/...`

## Plan Reference

`docs/dev_docs/_plan_005a_nats+pma_session_tracking.md` (Task 4).
