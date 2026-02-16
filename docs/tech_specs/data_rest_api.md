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

Normative requirements

- The Data REST API MUST be implemented by the User API Gateway.
- The Data REST API MUST NOT expose raw SQL execution.
- The Data REST API MUST enforce authentication and authorization for all endpoints.
- The Data REST API MUST emit audit logs for reads and writes.

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
- Authorization MUST be evaluated using policy and preferences.
- Responses MUST be scoped to the authenticated subject.

## Future GraphQL Option

The orchestrator MAY add a read-oriented GraphQL interface in the future.
If added, it SHOULD be layered on top of the same authorization, auditing, and resource model as the Data REST API.

Recommended constraints for a future GraphQL interface

- read-only by default
- strict query cost limits and depth limits
- caching for common queries
