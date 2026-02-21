# Data REST API

- [Document Overview](#document-overview)
- [API Purpose](#api-purpose)
- [Scope and Boundaries](#scope-and-boundaries)
- [Core Resources](#core-resources)
- [Authentication and Authorization](#authentication-and-authorization)
- [Future GraphQL Option](#future-graphql-option)

## Document Overview

This document defines the Data REST API, a user-client facing API surface exposed by the User API Gateway.
It provides read and write access to database-backed entities without exposing raw SQL or direct PostgreSQL connectivity.

## API Purpose

- Provide a stable REST interface for user clients and integrations to access orchestrator data.
- Support common consumers such as UIs, dashboards, and automation tools.
- Enforce policy, rate limits, and auditing on all data access.

## Scope and Boundaries

The Data REST API is a user-facing interface.
Agents MUST NOT use this API for internal operations and MUST use MCP tools instead.

### Scope and Boundaries Applicable Requirements

- Spec ID: `CYNAI.DATAPI.ScopeBoundaries` <a id="spec-cynai-datapi-scopebound"></a>

Traces To:

- [REQ-DATAPI-0100](../requirements/datapi.md#req-datapi-0100)
- [REQ-DATAPI-0101](../requirements/datapi.md#req-datapi-0101)
- [REQ-DATAPI-0102](../requirements/datapi.md#req-datapi-0102)
- [REQ-DATAPI-0103](../requirements/datapi.md#req-datapi-0103)

## Database Access and ORM Requirements

The Data REST API is implemented in Go.
It serves database-backed resources without exposing PostgreSQL connectivity to user clients.

### Database Access and ORM Applicable Requirements

- Spec ID: `CYNAI.DATAPI.DatabaseAccessOrm` <a id="spec-cynai-datapi-dbaccessorm"></a>

Traces To:

- [REQ-DATAPI-0104](../requirements/datapi.md#req-datapi-0104)
- [REQ-DATAPI-0105](../requirements/datapi.md#req-datapi-0105)
- [REQ-DATAPI-0106](../requirements/datapi.md#req-datapi-0106)
- [REQ-DATAPI-0107](../requirements/datapi.md#req-datapi-0107)
- [REQ-DATAPI-0108](../requirements/datapi.md#req-datapi-0108)
- [REQ-DATAPI-0109](../requirements/datapi.md#req-datapi-0109)
- [REQ-DATAPI-0110](../requirements/datapi.md#req-datapi-0110)
- [REQ-DATAPI-0111](../requirements/datapi.md#req-datapi-0111)
- [REQ-DATAPI-0112](../requirements/datapi.md#req-datapi-0112)

Recommended behaviors

- Configure database connection pooling (`MaxOpenConns`, `MaxIdleConns`, `ConnMaxLifetime`) based on worker concurrency and expected vector query latency.
- Avoid returning embeddings unless required.
  Use `Select()` (or equivalent) to exclude embedding columns for list and read endpoints by default.
- Enable structured ORM logging.
  Avoid full SQL logging in production, or sample it.

## Core Resources

The Data REST API SHOULD expose resource-oriented endpoints for:

- Users
- Tasks, task state, and task history
- Jobs and job results
- Runs and sessions (execution traces, sub-runs, logs, transcripts, and background process attribution)
  - See [`docs/tech_specs/runs_and_sessions_api.md`](runs_and_sessions_api.md).
- Connectors (catalog, instances, credential metadata, and audit-visible operation history)
  - See [`docs/tech_specs/connector_framework.md`](connector_framework.md).
- Artifacts and artifact metadata
- Nodes, node status, and capability reports
- Preferences and effective preferences resolution results
- API Egress credentials (metadata list, get, create, rotate, disable; secrets write-only on create and rotate)
  - See [API Egress Server - Admin API (Gateway Endpoints)](api_egress_server.md#admin-api-gateway-endpoints).
- Access control rules and audit records, when allowed
- Groups, group memberships, roles, and role bindings (RBAC), when allowed
  - See [`docs/tech_specs/rbac_and_groups.md`](rbac_and_groups.md).
- Model registry and model availability, when allowed
- Sandbox image registry metadata, when allowed

Endpoints SHOULD support:

- pagination
- filtering
- stable identifiers
- minimal partial updates for mutable resources

## Authentication and Authorization

- Clients MUST authenticate to the User API Gateway.
- Authorization MUST be evaluated using policy and (when applicable) system settings.
  User preferences are a separate resource and do not govern API authorization; for the distinction between preferences and system settings, see [User preferences (Terminology)](user_preferences.md#2-terminology).
- Responses MUST be scoped to the authenticated subject.

## Future GraphQL Option

The orchestrator MAY add a read-oriented GraphQL interface in the future.
If added, it SHOULD be layered on top of the same authorization, auditing, and resource model as the Data REST API.

Recommended constraints for a future GraphQL interface

- read-only by default
- strict query cost limits and depth limits
- caching for common queries
