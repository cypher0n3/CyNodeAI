# Chunk 04 Completion Report

- [Deliverables](#deliverables)
- [Code Changes](#code-changes)
- [Validation](#validation)
- [Dependencies](#dependencies)

## Deliverables

**Date:** 2026-02-19.
**Scope:** Update Node Manager to fetch config and start node services in the spec order (MVP Phase 1).

- **Node manager flow:** register => fetch config => start Worker API => start Ollama => send config ack => capability loop.
- **Config fetch:** `FetchConfig` uses bootstrap `node_config_url` (GET with node JWT).
- **Config ack:** `SendConfigAck` sends `node_config_ack_v1` (POST) after applying config.
- **Worker API:** Started with `worker_api.orchestrator_bearer_token` from config; token not logged.
- **Ollama:** Phase 1 inference container started via `CONTAINER_RUNTIME` / `OLLAMA_IMAGE`; fail-fast on error.
- **Optional skip:** `NODE_MANAGER_SKIP_SERVICES=1` skips starting Worker API and Ollama (e.g. tests).

## Code Changes

- [worker_node/internal/nodemanager/nodemanager.go](../worker_node/internal/nodemanager/nodemanager.go): `FetchConfig`, `SendConfigAck`, `RunOptions`, `RunWithOptions`, `applyConfigAndStartServices`, `runCapabilityLoop`.
- [worker_node/cmd/node-manager/main.go](../worker_node/cmd/node-manager/main.go): `startWorkerAPI`, `startOllama`, `RunWithOptions` with opts; `NODE_MANAGER_SKIP_SERVICES` support.
- [worker_node/internal/nodemanager/nodemanager_test.go](../worker_node/internal/nodemanager/nodemanager_test.go): Config/ack handlers in mock server, `TestFetchConfig`, `TestSendConfigAck`, `TestRunWithOptions_OllamaFailFast`, `TestRunWithOptions_StartWorkerAPICalled`, error-path tests, `mockOrchWithConfig` helper.
- [worker_node/cmd/node-manager/main_test.go](../worker_node/cmd/node-manager/main_test.go): `TestRunMainSuccess` updated for config GET/POST and skip services; `TestStartWorkerAPI_*`, `TestStartOllama_Success`.
- [features/worker_node/node_manager_config_startup.feature](../features/worker_node/node_manager_config_startup.feature): New feature for config fetch via bootstrap URL, bearer token from config, config ack, fail-fast on inference startup.
- [worker_node/_bdd/steps.go](../worker_node/_bdd/steps.go): `RegisterNodeManagerConfigSteps`, mock orchestrator, steps for node manager startup scenarios.

## Validation

- `just validate-feature-files`: OK
- `just test-go`: OK (coverage >= 90% per package)
- `just test-bdd`: OK (orchestrator and worker_node suites)
- `just lint-go-ci`: OK

## Dependencies

- `github.com/cucumber/godog` added to `orchestrator/go.mod` and `worker_node/go.mod` so BDD suites resolve (was missing before).
