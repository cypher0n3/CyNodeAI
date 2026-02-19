# STANDS Requirements

- [1 Overview](#1-overview)
- [2 Requirements](#2-requirements)

## 1 Overview

This document consolidates requirements for the `STANDS` domain.
It covers cross-cutting standards and conventions that apply across components.

## 2 Requirements

- **REQ-STANDS-0001:** Go REST APIs; timeouts and body limits; JSON and error format; auth and fail-closed; observability; stable versioning and backward compatibility.
  [CYNAI.STANDS.Scope](../tech_specs/go_rest_api_standards.md#spec-cynai-stands-scope)
  [CYNAI.STANDS.Timeouts](../tech_specs/go_rest_api_standards.md#spec-cynai-stands-timeouts)
  [CYNAI.STANDS.ApiEvolution](../tech_specs/go_rest_api_standards.md#spec-cynai-stands-apievolution)
  <a id="req-stands-0001"></a>
- **REQ-STANDS-0100:** All REST APIs MUST be implemented in Go.
  [CYNAI.STANDS.Scope](../tech_specs/go_rest_api_standards.md#spec-cynai-stands-scope)
  <a id="req-stands-0100"></a>
- **REQ-STANDS-0101:** REST APIs SHOULD prefer the Go standard library for HTTP whenever practical.
  [CYNAI.STANDS.Scope](../tech_specs/go_rest_api_standards.md#spec-cynai-stands-scope)
  <a id="req-stands-0101"></a>
- **REQ-STANDS-0102:** Servers MUST set timeouts on `http.Server`.
  [CYNAI.STANDS.Timeouts](../tech_specs/go_rest_api_standards.md#spec-cynai-stands-timeouts)
  <a id="req-stands-0102"></a>
- **REQ-STANDS-0103:** Servers MUST set `MaxHeaderBytes` to a safe limit.
  [CYNAI.STANDS.Timeouts](../tech_specs/go_rest_api_standards.md#spec-cynai-stands-timeouts)
  <a id="req-stands-0103"></a>
- **REQ-STANDS-0104:** Endpoints that accept request bodies MUST limit body size.
  [CYNAI.STANDS.Timeouts](../tech_specs/go_rest_api_standards.md#spec-cynai-stands-timeouts)
  <a id="req-stands-0104"></a>
- **REQ-STANDS-0105:** JSON endpoints MUST use `Content-Type: application/json` for requests and responses.
  [CYNAI.STANDS.RequestResponse](../tech_specs/go_rest_api_standards.md#spec-cynai-stands-reqresp)
  <a id="req-stands-0105"></a>
- **REQ-STANDS-0106:** JSON decoders SHOULD reject unknown fields for request bodies to catch client mistakes early.
  [CYNAI.STANDS.RequestResponse](../tech_specs/go_rest_api_standards.md#spec-cynai-stands-reqresp)
  <a id="req-stands-0106"></a>
- **REQ-STANDS-0107:** Responses MUST be deterministic and stable for the same request and resource state.
  [CYNAI.STANDS.RequestResponse](../tech_specs/go_rest_api_standards.md#spec-cynai-stands-reqresp)
  <a id="req-stands-0107"></a>
- **REQ-STANDS-0108:** APIs MUST return a consistent structured error format across endpoints.
  [CYNAI.STANDS.ErrorFormat](../tech_specs/go_rest_api_standards.md#spec-cynai-stands-errorfmt)
  <a id="req-stands-0108"></a>
- **REQ-STANDS-0109:** APIs SHOULD use Problem Details JSON (`application/problem+json`, RFC 9457) for error responses.
  [CYNAI.STANDS.ErrorFormat](../tech_specs/go_rest_api_standards.md#spec-cynai-stands-errorfmt)
  <a id="req-stands-0109"></a>
- **REQ-STANDS-0110:** APIs MUST NOT leak secrets in error messages or error details.
  [CYNAI.STANDS.ErrorFormat](../tech_specs/go_rest_api_standards.md#spec-cynai-stands-errorfmt)
  <a id="req-stands-0110"></a>
- **REQ-STANDS-0111:** All endpoints (except explicit health checks) MUST authenticate callers.
  [CYNAI.STANDS.AuthSecurity](../tech_specs/go_rest_api_standards.md#spec-cynai-stands-authsec)
  <a id="req-stands-0111"></a>
- **REQ-STANDS-0112:** Authorization MUST be checked server-side on every request and MUST fail closed.
  [CYNAI.STANDS.AuthSecurity](../tech_specs/go_rest_api_standards.md#spec-cynai-stands-authsec)
  <a id="req-stands-0112"></a>
- **REQ-STANDS-0113:** Services MUST validate and constrain outbound requests by policy.
  [CYNAI.STANDS.AuthSecurity](../tech_specs/go_rest_api_standards.md#spec-cynai-stands-authsec)
  <a id="req-stands-0113"></a>
- **REQ-STANDS-0114:** Services MUST emit structured logs.
  [CYNAI.STANDS.Observability](../tech_specs/go_rest_api_standards.md#spec-cynai-stands-observ)
  <a id="req-stands-0114"></a>
- **REQ-STANDS-0115:** Services SHOULD expose health endpoints (for example `/healthz` and `/readyz`).
  [CYNAI.STANDS.Observability](../tech_specs/go_rest_api_standards.md#spec-cynai-stands-observ)
  <a id="req-stands-0115"></a>
- **REQ-STANDS-0116:** Services SHOULD support distributed tracing and metrics collection.
  [CYNAI.STANDS.Observability](../tech_specs/go_rest_api_standards.md#spec-cynai-stands-observ)
  <a id="req-stands-0116"></a>
- **REQ-STANDS-0117:** Services SHOULD use GORM as the default database access layer.
  [CYNAI.STANDS.DatabaseAccess](../tech_specs/go_rest_api_standards.md#spec-cynai-stands-dbaccess)
  <a id="req-stands-0117"></a>
- **REQ-STANDS-0118:** Services MUST use GORM's PostgreSQL driver with `pgx`.
  [CYNAI.STANDS.DatabaseAccess](../tech_specs/go_rest_api_standards.md#spec-cynai-stands-dbaccess)
  <a id="req-stands-0118"></a>
- **REQ-STANDS-0119:** Multi-module implementations SHOULD align `pgx` versions across services to avoid drift and inconsistent behavior.
  [CYNAI.STANDS.DatabaseAccess](../tech_specs/go_rest_api_standards.md#spec-cynai-stands-dbaccess)
  <a id="req-stands-0119"></a>
- **REQ-STANDS-0120:** Services MUST use `WithContext(ctx)` for all database operations.
  [CYNAI.STANDS.DatabaseAccess](../tech_specs/go_rest_api_standards.md#spec-cynai-stands-dbaccess)
  <a id="req-stands-0120"></a>
- **REQ-STANDS-0121:** Services MUST configure connection pooling (`MaxOpenConns`, `MaxIdleConns`, `ConnMaxLifetime`).
  [CYNAI.STANDS.DatabaseAccess](../tech_specs/go_rest_api_standards.md#spec-cynai-stands-dbaccess)
  <a id="req-stands-0121"></a>
- **REQ-STANDS-0122:** Services MUST document pool sizing assumptions based on expected concurrency and query latency.
  [CYNAI.STANDS.DatabaseAccess](../tech_specs/go_rest_api_standards.md#spec-cynai-stands-dbaccess)
  <a id="req-stands-0122"></a>
- **REQ-STANDS-0123:** Services MUST be compatible with an AutoMigrate-first schema workflow (GORM models as the schema definition).
  [CYNAI.STANDS.DatabaseAccess](../tech_specs/go_rest_api_standards.md#spec-cynai-stands-dbaccess)
  <a id="req-stands-0123"></a>
- **REQ-STANDS-0124:** Services MUST NOT execute schema-altering operations at request time.
  [CYNAI.STANDS.DatabaseAccess](../tech_specs/go_rest_api_standards.md#spec-cynai-stands-dbaccess)
  <a id="req-stands-0124"></a>
- **REQ-STANDS-0125:** Schema application MUST be performed via an explicit, supported step (startup or admin command).
  [CYNAI.STANDS.DatabaseAccess](../tech_specs/go_rest_api_standards.md#spec-cynai-stands-dbaccess)
  <a id="req-stands-0125"></a>
- **REQ-STANDS-0126:** The schema workflow MUST include a separate, deterministic DDL bootstrap step for non-ORM DDL.
  [CYNAI.STANDS.DatabaseAccess](../tech_specs/go_rest_api_standards.md#spec-cynai-stands-dbaccess)
  <a id="req-stands-0126"></a>
- **REQ-STANDS-0127:** pgvector model fields SHOULD use `pgvector-go` types and MUST declare an explicit column type.
  [CYNAI.STANDS.DatabaseAccess](../tech_specs/go_rest_api_standards.md#spec-cynai-stands-dbaccess)
  <a id="req-stands-0127"></a>
- **REQ-STANDS-0128:** Similarity search queries SHOULD use raw SQL (`db.Raw()`) for pgvector operators and MUST be isolated behind repository interfaces.
  [CYNAI.STANDS.DatabaseAccess](../tech_specs/go_rest_api_standards.md#spec-cynai-stands-dbaccess)
  <a id="req-stands-0128"></a>
- **REQ-STANDS-0129:** Mutating endpoints SHOULD support idempotency when clients may retry.
  [CYNAI.STANDS.ReliabilityIdempotency](../tech_specs/go_rest_api_standards.md#spec-cynai-stands-idempotency)
  <a id="req-stands-0129"></a>
- **REQ-STANDS-0130:** APIs MUST return non-2xx status codes on validation and authorization failures.
  [CYNAI.STANDS.ReliabilityIdempotency](../tech_specs/go_rest_api_standards.md#spec-cynai-stands-idempotency)
  <a id="req-stands-0130"></a>
- **REQ-STANDS-0131:** REST APIs MUST have a stable versioning scheme (for example a `/v1/` path prefix).
  [CYNAI.STANDS.ApiEvolution](../tech_specs/go_rest_api_standards.md#spec-cynai-stands-apievolution)
  <a id="req-stands-0131"></a>
- **REQ-STANDS-0132:** Backward-incompatible changes MUST require a new major API version.
  [CYNAI.STANDS.ApiEvolution](../tech_specs/go_rest_api_standards.md#spec-cynai-stands-apievolution)
  <a id="req-stands-0132"></a>
