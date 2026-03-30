# Task 1 Completion Report - HTTP Body and Response Limits

## Summary

Introduced shared limits in `go_shared_libs/httplimits` and applied `http.MaxBytesReader` / `io.LimitReader` across orchestrator routes, worker proxy paths, PMA/SBA/MCP clients, cynork gateway client, and nodeagent HTTP decodes.
User-gateway artifact routes use **100 MiB**; default API bodies use **10 MiB**; outbound response reads use **100 MiB** unless already more tightly bounded (e.g. error snippets in `parseError`).

## Constants (`go_shared_libs/httplimits`)

- **Constant:** `DefaultMaxAPIRequestBodyBytes`
  - value: 10 MiB
- **Constant:** `DefaultMaxArtifactUploadBytes`
  - value: 100 MiB
- **Constant:** `DefaultMaxHTTPResponseBytes`
  - value: 100 MiB

## Files Changed (Limits Applied)

- `go_shared_libs/httplimits/httplimits.go`, `httplimits_test.go` - new package
- `orchestrator/internal/mcpgateway/handlers.go` - wrap + 413 on oversize MCP body
- `orchestrator/cmd/control-plane/main.go` - `LimitBody` on node + workflow POST routes
- `orchestrator/cmd/user-gateway/main.go` - `httplimits.LimitBody`; artifacts use `DefaultMaxArtifactUploadBytes`
- `orchestrator/cmd/user-gateway/main_test.go` - `httplimits.LimitBody` in test
- `orchestrator/cmd/api-egress/main.go`, `main_test.go` - wrap POST `/v1/call`; 413 test
- `orchestrator/cmd/control-plane/main_test.go` - MCP oversize test
- `orchestrator/internal/dispatcher/run.go` - worker response decode limited
- `orchestrator/internal/pmaclient/client.go` - response decodes / fallback read limited
- `agents/internal/pma/chat.go` - chat handler request wrap; Ollama read limited
- `agents/internal/mcpclient/client.go`, `client_test.go` - response reads + error-path test
- `agents/internal/sba/agent.go` - Ollama response decode limited
- `cynork/internal/gateway/client.go`, `client_http.go` - `decodeResponseJSON` + byte helpers
- `cynork/internal/gateway/maxbytes_test.go` - response cap test
- `worker_node/internal/workerapiserver/embed_handlers.go` - managed proxy request + upstream read
- `worker_node/internal/workerapiserver/internal_orchestrator_proxy.go` - max body + upstream read
- `worker_node/internal/workerapiserver/maxbytes_test.go` - jobs:run oversize test
- `worker_node/internal/nodeagent/nodemanager.go`, `nodemanager_config.go` - JSON response decodes limited
- `orchestrator/internal/mcpgateway/maxbytes_test.go`, `agents/internal/pma/maxbytes_test.go` - `TestMaxBytes` per module
- `docs/dev_docs/task1_discovery_maxbytes.md` - discovery notes

## Validation

- `just lint-go` - pass
- `just test-go-cover` - pass (>=90% per package)
- `just e2e --tags no_inference` - pass (118 tests, 3 skipped)

## Plan Checklist

Task 1 steps in `docs/dev_docs/_plan_003_short_term.md` are marked complete in the plan document.
