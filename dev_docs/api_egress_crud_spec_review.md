# API Egress CRUD Spec Review

- [Summary](#summary)
- [Findings](#findings)
- [Changes Made](#changes-made)
- [Endpoint Contract](#endpoint-contract)
- [Recommendation](#recommendation)

## Summary

Reviewed specs for API egress and how CRUD of API Egress credentials (and related admin operations) are performed via the Admin Web Console and cynork.
Identified gaps and updated the tech specs so the gateway endpoint contract is defined in one place and both clients reference it.
Review date: 2026-02-20.

## Findings

Below is what was already specified and what was missing.

### What Was Already Specified

- **api_egress_server.md:** Service purpose, agent interaction model, credential storage (Postgres schema), access control, policy and auditing.
  No admin-facing REST API for credential management was defined.
- **admin_web_console.md:** Credential create, list, rotate, disable; metadata only on read; API Surface mentions "Credential metadata CRUD and secret rotation endpoints" without paths or methods.
- **cli_management_app.md:** Same operations via `cynork creds list|get|create|rotate|disable`; same requirements; no gateway path/method contract.
- **data_rest_api.md:** Core Resources list does not include API Egress credentials (or a generic credentials resource).
- **requirements (apiegr.md, client.md):** Credential create/list/rotate/disable and metadata-only read are required; REQ-CLIENT-0004 enforces parity between web console and cynork.

### Gaps Identified

- **Gateway API undefined:** Both clients must call the same gateway endpoints, but path prefix, methods, and request/response shape were not specified anywhere.
- **Data REST API scope:** API Egress credentials were not listed as a core resource, so implementers would not know to expose them there.
- **Parity link missing:** Admin console and CLI specs did not point to a canonical endpoint contract, making it easy for implementations to drift.
- **Delete vs disable:** Specs only require "disable" (immediate deactivation); hard delete is not required for MVP.

## Changes Made

The following spec updates were applied.

- **api_egress_server.md:** Added section [Admin API (Gateway Endpoints)](../docs/tech_specs/api_egress_server.md#admin-api-gateway-endpoints); defines the credential resource and HTTP contract (list, get, create, rotate, disable); states that the Admin Web Console and cynork both use these same gateway endpoints.
- **data_rest_api.md:** Added API Egress credentials to Core Resources with a pointer to the API Egress Server spec for the endpoint contract.
- **admin_web_console.md:** Credential Management now references the gateway endpoint contract in api_egress_server.md.
- **cli_management_app.md:** Credential Management now references the same gateway endpoint contract for parity.

## Endpoint Contract

Canonical definition is in api_egress_server.md.

- **List:** `GET /v1/credentials` (or gateway-chosen prefix) with optional filter by provider, owner_type, owner_id.
- **Get:** `GET /v1/credentials/{id}`; response is metadata only (no secret).
- **Create:** `POST /v1/credentials`; body includes provider, credential_name, owner_type, owner_id, credential_type, and secret (write-only).
- **Rotate:** `POST /v1/credentials/{id}/rotate`; body contains new secret only.
- **Disable:** `PATCH /v1/credentials/{id}` with `is_active: false` (or equivalent).

All endpoints require authentication and authorization; gateway MUST audit and MUST NOT return secret values on read.

## Recommendation

- When implementing the User API Gateway, implement the credential endpoints per [API Egress Server - Admin API (Gateway Endpoints)](../docs/tech_specs/api_egress_server.md#admin-api-gateway-endpoints).
- Expose these in the gateway OpenAPI/Swagger spec so the Admin Web Console Swagger UI and cynork (or any scripted client) can discover and call them consistently.
