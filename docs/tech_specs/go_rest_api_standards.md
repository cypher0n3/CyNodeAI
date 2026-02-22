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
- [Database Access](#database-access)
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

### Scope Applicable Requirements

- Spec ID: `CYNAI.STANDS.Scope` <a id="spec-cynai-stands-scope"></a>

Traces To:

- [REQ-STANDS-0100](../requirements/stands.md#req-stands-0100)
- [REQ-STANDS-0101](../requirements/stands.md#req-stands-0101)

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

The following requirements apply.

### Timeouts and Resource Limits Applicable Requirements

- Spec ID: `CYNAI.STANDS.Timeouts` <a id="spec-cynai-stands-timeouts"></a>

Traces To:

- [REQ-STANDS-0102](../requirements/stands.md#req-stands-0102)
- [REQ-STANDS-0103](../requirements/stands.md#req-stands-0103)
- [REQ-STANDS-0104](../requirements/stands.md#req-stands-0104)

Recommended behaviors

- Use per-request deadlines derived from the inbound request context.
- Prefer conservative defaults and allow per-endpoint overrides only when justified.

## Request and Response Model

The following requirements apply.

### Request and Response Model Applicable Requirements

- Spec ID: `CYNAI.STANDS.RequestResponse` <a id="spec-cynai-stands-reqresp"></a>

Traces To:

- [REQ-STANDS-0105](../requirements/stands.md#req-stands-0105)
- [REQ-STANDS-0106](../requirements/stands.md#req-stands-0106)
- [REQ-STANDS-0107](../requirements/stands.md#req-stands-0107)

Recommended response conventions

- Use consistent envelope patterns only when they add value.
  - Avoid wrapping every response in `{ "data": ... }` unless the API requires it.
- Prefer pagination for list endpoints and define stable paging semantics.
- Prefer UTC timestamps in RFC 3339 format.

## Error Format and Status Codes

The following requirements apply.

### Error Format and Status Codes Applicable Requirements

- Spec ID: `CYNAI.STANDS.ErrorFormat` <a id="spec-cynai-stands-errorfmt"></a>

Traces To:

- [REQ-STANDS-0108](../requirements/stands.md#req-stands-0108)
- [REQ-STANDS-0109](../requirements/stands.md#req-stands-0109)
- [REQ-STANDS-0110](../requirements/stands.md#req-stands-0110)

Recommended error fields

- `type`: stable error category URI or identifier
- `title`: short, human-readable summary
- `status`: HTTP status code
- `detail`: safe detail for operators and users
- `instance`: request identifier for correlating logs and traces

Protocol compatibility exceptions

Some gateway endpoints exist specifically to emulate an external protocol (for example OpenAI-compatible endpoints).
In those cases, the endpoint MAY return the external protocol's error shape instead of the standard CyNodeAI error envelope.
When an exception is used:

- The endpoint's tech spec MUST define the exact error payload shape.
- Implementations MUST not mix multiple error formats for the same endpoint.
- Error payloads MUST remain safe (no secrets) and observable (include request identifiers in logs).

## Authentication, Authorization, and Security

The following requirements apply.

### Authentication, Authorization, and Security Applicable Requirements

- Spec ID: `CYNAI.STANDS.AuthSecurity` <a id="spec-cynai-stands-authsec"></a>

Traces To:

- [REQ-STANDS-0111](../requirements/stands.md#req-stands-0111)
- [REQ-STANDS-0112](../requirements/stands.md#req-stands-0112)
- [REQ-STANDS-0113](../requirements/stands.md#req-stands-0113)

Recommended security practices

- Prefer bearer tokens over cookie auth for API clients unless a browser-only flow is required.
- Apply CORS only where required and avoid wildcard origins for authenticated requests.
- Avoid logging request bodies and headers that may contain secrets.

## Observability

The following requirements apply.

### Observability Applicable Requirements

- Spec ID: `CYNAI.STANDS.Observability` <a id="spec-cynai-stands-observ"></a>

Traces To:

- [REQ-STANDS-0114](../requirements/stands.md#req-stands-0114)
- [REQ-STANDS-0115](../requirements/stands.md#req-stands-0115)
- [REQ-STANDS-0116](../requirements/stands.md#req-stands-0116)
  - Prefer wrapping `net/http` handlers and clients with official instrumentation (for example `otelhttp`).

Recommended log fields

- `request_id`
- `trace_id` and `span_id` when tracing is enabled
- `user_id` and `subject_type` when applicable
- `route` or handler name
- `status`
- `duration_ms`

## Database Access

This section defines standards for services that access PostgreSQL.
It applies to the User API Gateway, worker services that persist state, and any internal orchestrator services that own database credentials.

### Database Access Applicable Requirements

- Spec ID: `CYNAI.STANDS.DatabaseAccess` <a id="spec-cynai-stands-dbaccess"></a>

Traces To:

- [REQ-STANDS-0117](../requirements/stands.md#req-stands-0117)
- [REQ-STANDS-0118](../requirements/stands.md#req-stands-0118)
- [REQ-STANDS-0119](../requirements/stands.md#req-stands-0119)
- [REQ-STANDS-0120](../requirements/stands.md#req-stands-0120)
- [REQ-STANDS-0121](../requirements/stands.md#req-stands-0121)
- [REQ-STANDS-0122](../requirements/stands.md#req-stands-0122)
- [REQ-STANDS-0123](../requirements/stands.md#req-stands-0123)
- [REQ-STANDS-0124](../requirements/stands.md#req-stands-0124)
- [REQ-STANDS-0125](../requirements/stands.md#req-stands-0125)
- [REQ-STANDS-0126](../requirements/stands.md#req-stands-0126)
- [REQ-STANDS-0127](../requirements/stands.md#req-stands-0127)
- [REQ-STANDS-0128](../requirements/stands.md#req-stands-0128)

Recommended behaviors

- Use explicit transactions for multi-table writes and for upserts involving embeddings.
- Avoid selecting embeddings unless required.
  Use `Select()` (or equivalent) to exclude embedding columns for list and read paths by default.
- Enable structured ORM logging.
  Disable full SQL logging in production, or sample it.
- Track slow query thresholds and collect vector query latency separately from general query latency.

## Reliability and Idempotency

The following requirements apply.

### Reliability and Idempotency Applicable Requirements

- Spec ID: `CYNAI.STANDS.ReliabilityIdempotency` <a id="spec-cynai-stands-idempotency"></a>

Traces To:

- [REQ-STANDS-0129](../requirements/stands.md#req-stands-0129)
- [REQ-STANDS-0130](../requirements/stands.md#req-stands-0130)

Recommended behaviors

- For long-running operations, return an operation resource and allow polling.
- Use request-scoped rate limiting where needed (per user, per project, per task).

## API Evolution and Compatibility

The following requirements apply.

### API Evolution and Compatibility Applicable Requirements

- Spec ID: `CYNAI.STANDS.ApiEvolution` <a id="spec-cynai-stands-apievolution"></a>

Traces To:

- [REQ-STANDS-0131](../requirements/stands.md#req-stands-0131)
- [REQ-STANDS-0132](../requirements/stands.md#req-stands-0132)

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
