# Go REST API Standards (February 2026)

- [Document Overview](#document-overview)
- [Scope](#scope)
- [Language and Toolchain](#language-and-toolchain)
- [HTTP Routing and Handlers](#http-routing-and-handlers)
- [Timeouts and Resource Limits](#timeouts-and-resource-limits)
- [Request and Response Model](#request-and-response-model)
- [Error Format and Status Codes](#error-format-and-status-codes)
- [Authentication, Authorization, and Security](#authentication-authorization-and-security)
- [Observability](#observability)
- [Reliability and Idempotency](#reliability-and-idempotency)
- [API Evolution and Compatibility](#api-evolution-and-compatibility)
- [Testing and Quality Gates](#testing-and-quality-gates)

## Document Overview

This document defines implementation standards for CyNodeAI REST APIs.
These standards reflect Go best practices as of February 2026 and are intended to produce secure, observable, and maintainable HTTP services.

## Scope

Applies to all REST APIs and HTTP services in this system, including:

- User API Gateway and Data REST API
- Worker Node worker API
- API Egress Server
- Secure Browser Service

Normative requirements

- All REST APIs MUST be implemented in Go.
- REST APIs SHOULD prefer the Go standard library for HTTP whenever practical.

## Language and Toolchain

Recommended choices

- Go modules (`go.mod`) with a pinned Go version (current baseline: Go 1.25).
  - Repositories SHOULD use the `toolchain` directive to pin an exact toolchain version when appropriate.
- Standard library packages (`net/http`, `encoding/json`, `context`, `crypto/tls`) as the default.

Recommended quality gates

- `go test ./...` and `go test -race ./...` in CI for server packages.
- `go vet` and a static analysis tool (e.g. Staticcheck) in CI.
- `govulncheck` in CI for dependency vulnerability scanning.

## HTTP Routing and Handlers

Recommended approach

- Prefer `net/http` with `http.ServeMux`.
  - Go 1.22+ `ServeMux` supports method-aware patterns and path wildcards (available with the Go 1.25 baseline).
  - Use method patterns to avoid accidental handling of unsupported methods.
- Use small, composable handlers with explicit dependencies.
  - Avoid global mutable state.
  - Prefer constructor functions for routers and services.

Recommended handler structure

- Decode and validate inputs early.
- Use `context.Context` for all downstream operations.
- Keep business logic out of handlers by calling service-layer interfaces.

## Timeouts and Resource Limits

Normative requirements

- Servers MUST set timeouts on `http.Server`, including:
  - `ReadHeaderTimeout`
  - `ReadTimeout`
  - `WriteTimeout`
  - `IdleTimeout`
- Servers MUST set `MaxHeaderBytes` to a safe limit.
- Endpoints that accept request bodies MUST limit body size (for example using `http.MaxBytesReader`).

Recommended behaviors

- Use per-request deadlines derived from the inbound request context.
- Prefer conservative defaults and allow per-endpoint overrides only when justified.

## Request and Response Model

Normative requirements

- JSON endpoints MUST use `Content-Type: application/json` for requests and responses.
- JSON decoders SHOULD reject unknown fields for request bodies to catch client mistakes early.
- Responses MUST be deterministic and stable for the same request and resource state.

Recommended response conventions

- Use consistent envelope patterns only when they add value.
  - Avoid wrapping every response in `{ "data": ... }` unless the API requires it.
- Prefer pagination for list endpoints and define stable paging semantics.
- Prefer UTC timestamps in RFC 3339 format.

## Error Format and Status Codes

Normative requirements

- APIs MUST return a consistent structured error format across endpoints.
- APIs SHOULD use Problem Details JSON (`application/problem+json`, RFC 9457) for error responses.
- APIs MUST not leak secrets in error messages or error details.

Recommended error fields

- `type`: stable error category URI or identifier
- `title`: short, human-readable summary
- `status`: HTTP status code
- `detail`: safe detail for operators and users
- `instance`: request identifier for correlating logs and traces

## Authentication, Authorization, and Security

Normative requirements

- All endpoints (except explicit health checks) MUST authenticate callers.
- Authorization MUST be checked server-side on every request and MUST fail closed.
- Services MUST validate and constrain outbound requests (for example API egress and web browsing) by policy.

Recommended security practices

- Prefer bearer tokens over cookie auth for API clients unless a browser-only flow is required.
- Apply CORS only where required and avoid wildcard origins for authenticated requests.
- Avoid logging request bodies and headers that may contain secrets.

## Observability

Normative requirements

- Services MUST emit structured logs.
  - Prefer the Go standard library `log/slog` for logging.
- Services SHOULD expose health endpoints (for example `/healthz` and `/readyz`).
- Services SHOULD support distributed tracing and metrics collection.
  - Prefer OpenTelemetry for traces and metrics.
  - Prefer wrapping `net/http` handlers and clients with official instrumentation (for example `otelhttp`).

Recommended log fields

- `request_id`
- `trace_id` and `span_id` when tracing is enabled
- `user_id` and `subject_type` when applicable
- `route` or handler name
- `status`
- `duration_ms`

## Reliability and Idempotency

Normative requirements

- Mutating endpoints SHOULD support idempotency when clients may retry.
- APIs MUST return non-2xx status codes on validation and authorization failures.

Recommended behaviors

- For long-running operations, return an operation resource and allow polling.
- Use request-scoped rate limiting where needed (per user, per project, per task).

## API Evolution and Compatibility

Normative requirements

- REST APIs MUST have a stable versioning scheme (for example a `/v1/` path prefix).
- Backward-incompatible changes MUST require a new major API version.

Recommended change management

- Add new fields in responses without breaking old clients.
- Avoid removing or changing meaning of existing fields in a stable version.

## Testing and Quality Gates

Recommended tests

- Table-driven tests for handlers and service-layer logic.
- `httptest`-based integration tests for routing and middleware.
- Fuzz tests for request decoding and validation logic where appropriate.

Recommended production checks

- Validate that timeouts, body limits, and auth middleware are configured for every server.
- Validate that error format is consistent across endpoints.
