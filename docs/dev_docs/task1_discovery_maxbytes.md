# Task 1 discovery: unbounded reads and JSON request bodies (March 2026)

Sites where `io.ReadAll` was used without `io.LimitReader` on **response** bodies, or `json.NewDecoder(r.Body)` without prior `http.MaxBytesReader` on **request** bodies, are listed below by area. Test-only and BDD harness code are noted but were not changed for production hardening.

## `go_shared_libs`

- New package `httplimits` defines `DefaultMaxAPIRequestBodyBytes` (10 MiB), `DefaultMaxArtifactUploadBytes` (100 MiB), and `DefaultMaxHTTPResponseBytes` (100 MiB).

## Orchestrator

- **user-gateway** (`cmd/user-gateway/main.go`): Most routes already used local `limitBody`; artifact POST/PUT now use 100 MiB via `httplimits.DefaultMaxArtifactUploadBytes`.
- **control-plane** (`cmd/control-plane/main.go`): Node register/config/capability and workflow POST routes now use `httplimits.LimitBody`.
- **api-egress** (`cmd/api-egress/main.go`): `POST /v1/call` wraps body with `httplimits.WrapRequestBody`; 413 on `http.ErrBodyTooLarge`.
- **mcpgateway** (`internal/mcpgateway/handlers.go`): `ToolCallHandler` wraps request body and returns 413 when decode hits max.
- **handlers** (artifacts, tasks, etc.): JSON routes are covered by gateway/control-plane middleware; artifact blob reads are bounded by `MaxBytesReader` on the mux.
- **dispatcher** (`internal/dispatcher/run.go`): Worker `RunJob` response decode uses `io.LimitReader`.
- **pmaclient** (`internal/pmaclient/client.go`): Completion and managed-proxy JSON decodes and small non-NDJSON fallback read use `httplimits.DefaultMaxHTTPResponseBytes`.

## Worker node

- **embed_handlers** (`internal/workerapiserver/embed_handlers.go`): Managed-service proxy wraps incoming JSON with `httplimits.WrapRequestBody`; upstream buffered reads use `io.LimitReader`.
- **internal_orchestrator_proxy** (`internal/workerapiserver/internal_orchestrator_proxy.go`): Proxy JSON max aligned to `httplimits.DefaultMaxAPIRequestBodyBytes`; upstream response read uses `io.LimitReader`.

## Agents

- **PMA** (`internal/pma/chat.go`): `ChatCompletionHandler` wraps request body; Ollama response read uses a limited reader.
- **mcpclient** (`internal/mcpclient/client.go`): MCP `Call` and internal proxy paths read responses with `io.LimitReader`.
- **SBA** (`internal/sba/agent.go`): Direct Ollama JSON decode uses `io.LimitReader` on the response body.

## Cynork

- **gateway** (`internal/gateway/client.go`, `client_http.go`): All JSON decodes and `GetBytes`/`PostBytes`/etc. use `decodeResponseJSON` / `io.LimitReader` with `httplimits.DefaultMaxHTTPResponseBytes`. SSE streaming paths still read the live stream without a single full-body cap (by design).

## Tech spec

- `docs/tech_specs/go_rest_api_standards.md` — Timeouts and resource limits (REQ-STANDS-0102–0107): conservative defaults and per-request limits; implementation uses shared constants and `MaxBytesReader` / `LimitReader` as above.
