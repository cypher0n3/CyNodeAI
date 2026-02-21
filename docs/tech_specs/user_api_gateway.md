# User API Gateway

- [Document Overview](#document-overview)
- [Gateway Purpose](#gateway-purpose)
- [Core Capabilities](#core-capabilities)
- [Client Compatibility](#client-compatibility)
- [Data REST API](#data-rest-api)
- [Live Updates and Messaging](#live-updates-and-messaging)
  - [Delivery Methods](#delivery-methods)
  - [Event Types](#event-types)
  - [Subscriptions and Destinations](#subscriptions-and-destinations)
- [Authentication and Auditing](#authentication-and-auditing)
- [Admin Web Console](#admin-web-console)

## Document Overview

- Spec ID: `CYNAI.USRGWY.Doc.UserApiGateway` <a id="spec-cynai-usrgwy-doc-userapigateway"></a>

This document defines the User API Gateway, a single user-facing endpoint exposed by the orchestrator.
It provides a stable interface for user clients to submit work, query status, and retrieve artifacts.

## Gateway Purpose

- Expose orchestrator capabilities through one user-facing API surface.
- Support multiple user clients and integration tools without exposing internal worker endpoints.
- Provide consistent authentication, authorization, rate limiting, and auditing for user interactions.

## Core Capabilities

The gateway SHOULD support:

- Task submission and management
  - Create tasks with task input as **plain text or Markdown** (inline or from file), optional **attachments**, **script** (path/file), or **short series of commands**; for script/commands the system runs them in the sandbox; otherwise it interprets the task and may call an AI model and/or dispatch sandbox work.
  Interpretation and inference are the **default** for task text; there is no user-facing "use inference" flag (see REQ-ORCHES-0125, REQ-ORCHES-0126, REQ-ORCHES-0127).
  - Set acceptance criteria and attach artifacts.
  - List tasks, read status, and retrieve results.
- Scheduler and cron
  - Create, list, update, disable, and delete scheduled jobs (cron or one-off).
  - Cancel a schedule or the next run; view run history per schedule (past execution times and outcomes).
  - Time-zone aware schedule evaluation (schedules specify or inherit a time zone).
  - Query queue depth and schedule state for user visibility.
  - Support wakeups and automation triggers via the same scheduler surface.
- Runs and sessions
  - First-class runs and sessions API: create sessions, spawn sub-sessions, create and list runs, attach logs, stream status, store transcripts with retention policies.
  - See [`docs/tech_specs/runs_and_sessions_api.md`](runs_and_sessions_api.md).
- Connector framework
  - Install, enable, disable, and uninstall connector instances.
  - Manage connector credentials (create, rotate, revoke, disable) and view connector audit and operation history.
  - Enforce per-operation policy (read, send, admin) for connector invocations and administration.
  - See [`docs/tech_specs/connector_framework.md`](connector_framework.md).
- Interactive sessions
  - Chat-like interaction that creates or updates tasks and threads.
- Capability discovery
  - Report enabled services and features (e.g. external routing, secure browser, API egress).
- Artifact ingress and egress
  - Upload files for tasks and download produced artifacts.
- Admin operations
  - Manage credentials, user preferences, and basic node lifecycle controls through a single user-facing surface.
- Groups and RBAC
  - Manage groups and membership (create group, add member, remove member), when allowed.
  - Manage role bindings (assign role to user or group), when allowed.
  - See [`docs/tech_specs/rbac_and_groups.md`](rbac_and_groups.md).

## Client Compatibility

- Spec ID: `CYNAI.USRGWY.ClientCompatibility` <a id="spec-cynai-usrgwy-clientcompatibility"></a>

The gateway SHOULD provide compatibility modes to support common external tools.

- Open WebUI compatibility
  - The gateway MAY expose an OpenAI-compatible subset for chat and model listing, backed by orchestrator task workflows.
- Messaging integrations
  - The gateway SHOULD support inbound messages via webhooks and outbound notifications via integration adapters.

Compatibility layers MUST preserve orchestrator policy constraints and MUST not bypass auditing.

Traces To:

- [REQ-USRGWY-0121](../requirements/usrgwy.md#req-usrgwy-0121)

## MCP Tool Interface

The User API Gateway MAY expose MCP-facing capability discovery and tool routing for user clients.
Agents use MCP tools as the standard tool interface, as defined in [`docs/tech_specs/mcp_tooling.md`](mcp_tooling.md).

## Data REST API

- Spec ID: `CYNAI.USRGWY.DataRestApi` <a id="spec-cynai-usrgwy-datarestapi"></a>

The User API Gateway MUST provide a Data REST API for user clients and integrations.
This API provides database-backed resources without exposing raw SQL.

Traces To:

- [REQ-USRGWY-0122](../requirements/usrgwy.md#req-usrgwy-0122)

See [`docs/tech_specs/data_rest_api.md`](data_rest_api.md).

## Live Updates and Messaging

- Spec ID: `CYNAI.USRGWY.MessagingAndEvents` <a id="spec-cynai-usrgwy-messagingevents"></a>

The User API Gateway is the single user-facing integration surface for live updates.
Users can connect destinations and subscribe them to task and agent events.

### Delivery Methods

The gateway SHOULD support multiple delivery methods through adapters.

Examples

- Webhooks
- Chat platform adapters (e.g. Matrix, Slack, Discord, Mattermost)

Secrets required for delivery MUST be stored securely in PostgreSQL and MUST NOT be exposed to agents.

Traces To:

- [REQ-USRGWY-0123](../requirements/usrgwy.md#req-usrgwy-0123)

### Event Types

The orchestrator SHOULD emit structured events that can be subscribed to.

Examples

- Task created, updated, completed, failed
- Job started, progress update, completed, failed
- Agent started, sub-agent spawned, verification pass, verification fail

Events SHOULD include task context, timestamps, and a stable event name.

### Subscriptions and Destinations

Users connect one or more messaging destinations and subscribe them to event types.

#### Messaging Destinations Table

- `id` (uuid, pk)
- `user_id` (uuid)
  - foreign key to [`docs/tech_specs/user_preferences.md`](user_preferences.md) `users.id`
- `destination_type` (text)
  - examples: webhook, matrix, slack, discord
- `destination_config` (jsonb)
  - non-secret configuration (e.g. webhook URL host allowlist, channel ids)
- `secret_ciphertext` (bytea, nullable)
  - encrypted secrets, if required (e.g. tokens)
- `secret_kid` (text, nullable)
- `is_active` (boolean)
- `created_at` (timestamptz)
- `updated_at` (timestamptz)

Constraints

- Index: (`user_id`)
- Index: (`destination_type`)
- Index: (`is_active`)

#### Messaging Subscriptions Table

- `id` (uuid, pk)
- `destination_id` (uuid)
  - foreign key to `messaging_destinations.id`
- `event_pattern` (text)
  - exact or pattern match for event names
- `filter` (jsonb, nullable)
  - optional constraints, such as project_id or task tags
- `created_at` (timestamptz)

Constraints

- Index: (`destination_id`)
- Index: (`event_pattern`)

## Authentication and Auditing

- Spec ID: `CYNAI.USRGWY.AuthAuditing` <a id="spec-cynai-usrgwy-authauditing"></a>

- The gateway MUST authenticate user clients.
- The gateway MUST authorize user actions using policy and (when applicable) user task-execution preferences and constraints.
- The gateway SHOULD emit audit logs for all user actions, including task submission and artifact access.
- The gateway SHOULD support per-user rate limiting and request size limits.
- For the MVP local user account model and secure credential handling requirements, see [`docs/tech_specs/local_user_accounts.md`](local_user_accounts.md).

Traces To:

- [REQ-USRGWY-0124](../requirements/usrgwy.md#req-usrgwy-0124)
- [REQ-USRGWY-0125](../requirements/usrgwy.md#req-usrgwy-0125)

## Admin Web Console

- Spec ID: `CYNAI.USRGWY.AdminConsole` <a id="spec-cynai-usrgwy-adminconsole"></a>

The User API Gateway SHOULD support an admin-focused web console for managing credentials and user preferences.
The web console MUST be a client of the gateway and MUST NOT connect directly to PostgreSQL.

Traces To:

- [REQ-USRGWY-0126](../requirements/usrgwy.md#req-usrgwy-0126)

See [`docs/tech_specs/admin_web_console.md`](admin_web_console.md).
