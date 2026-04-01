# NATS Messaging Specification

- [Document Overview](#document-overview)
- [Design Principles](#design-principles)
- [Naming Conventions](#naming-conventions)
- [Subject Taxonomy](#subject-taxonomy)
- [NATS Authentication and Credentials](#nats-authentication-and-credentials)
- [NATS WebSocket Listener](#nats-websocket-listener)
- [JetStream Streams](#jetstream-streams)
- [Consumer Patterns](#consumer-patterns)
- [Message Envelope Specification](#message-envelope-specification)
- [Payload Schemas](#payload-schemas)
- [RBAC and Multi-Tenancy Controls](#rbac-and-multi-tenancy-controls)
- [Idempotency and Deduplication](#idempotency-and-deduplication)
- [Ordering and Consistency](#ordering-and-consistency)
- [Payload Size Limits](#payload-size-limits)
- [Operational Defaults](#operational-defaults)
- [Implementation Phasing](#implementation-phasing)

## Document Overview

- Spec ID: `CYNAI.ORCHES.Doc.NatsMessaging` <a id="spec-cynai-orches-doc-natsmessaging"></a>

This specification defines the NATS subject taxonomy, JetStream streams, consumer patterns, and message schemas for CyNodeAI.

NATS is the unified transport and event backbone for real-time communication between all CyNodeAI components.
Postgres remains the authoritative store; NATS provides real-time signaling, session lifecycle tracking, and chat streaming that eliminate polling and HTTP proxy chains across component boundaries.

Clients (cynork, Web Console) connect to NATS directly after authenticating via the HTTP API, receiving session-scoped NATS JWTs.
The worker bridges NATS to PMA containers over UDS.
The orchestrator observes NATS traffic for persistence, redaction, and auditing.

### Document Scope

- Client NATS connections and session-scoped authentication (Phase 1)
- Session activity and idle lifecycle for per-session resource provisioning (Phase 1)
- Chat request and token streaming over NATS (Phase 1)
- Node configuration change notifications (Phase 1)
- Node presence and capacity (Phase 2)
- Job dispatch and execution (Phase 3)
- Work item and requirements eventing (future)
- Policy approvals (future)
- Artifact and indexing triggers (future)
- Live progress streaming (future)

## Design Principles

- **Orchestrator is the sole source of NATS configuration.**
  No component (worker, cynork, Web Console, or gateway) ships with or requires pre-loaded NATS URLs, ports, or TLS settings.
  Every actor obtains full NATS connection details; server URL, credentials, TLS CA, and WebSocket endpoint where applicable; from the orchestrator during authentication or registration.
  This keeps NATS deployment details opaque to clients and allows the orchestrator to relocate, rescale, or re-TLS the NATS cluster without touching any downstream configuration.
- At-least-once delivery (JetStream) + idempotent consumers
- Small messages; large payloads go to object storage and are referenced by URI + hash
- RBAC enforced via NATS publish/subscribe permissions and message-level scope fields
- Deterministic schemas with explicit versioning
- Stable subject patterns; add new versions via schema versioning, not subject churn
- Clients connect to NATS directly; server-side components bridge to UDS-only services (PMA)

## Naming Conventions

- Prefix all subjects with `cynode.`
- Use lowercase tokens separated by dots
- Put tenant and project in the subject for routing, but do not include secrets or PII

### Recommended Identifiers

- `tenant_id` - stable string or UUID
- `project_id` - stable string or UUID
- `session_id` - UUID (interactive session for PMA binding)
- `node_id` - stable string or UUID
- `job_id` - UUID
- `work_item_id` - UUID (story/task/subtask/requirement/etc.)
- `event_id` - UUID (unique per emitted event)
- `message_id` - UUID (unique per chat message within a stream)

## Subject Taxonomy

Subject names are hierarchical; the following sections list canonical subjects by domain.

### Session Activity (Phase 1)

- Spec ID: `CYNAI.ORCHES.NatsSessionActivitySubjects` <a id="spec-cynai-orches-natssessionactivitysubjects"></a>

Subjects for session presence tracking, used by clients, the user-gateway, and orchestrator to manage per-session PMA idle lifecycle.

- `cynode.session.activity.<tenant_id>.<session_id>`
- `cynode.session.attached.<tenant_id>.<session_id>`
- `cynode.session.detached.<tenant_id>.<session_id>`

#### Session Activity Subject Behavior

- `activity` is published periodically at `T_heartbeat` cadence.
  NATS-connected clients (cynork, Web Console) publish directly.
  For HTTP-only API clients, the user-gateway derives liveness from API traffic and publishes on their behalf.
  Each message resets the idle clock for that session binding.
- `attached` is published once when a client establishes a session (login or reconnect after idle).
  NATS-connected clients publish `attached` immediately after connecting to NATS.
  For HTTP-only clients, the gateway publishes on login.
- `detached` is published when the client cleanly disconnects (logout or explicit close).
  If the client crashes, the absence of `activity` messages within the idle window serves as an implicit detach.
- NATS-connected clients authenticate to NATS using a session-scoped JWT and publish session lifecycle messages directly.
  See [NATS Authentication and Credentials](#nats-authentication-and-credentials).

#### Session Activity Subjects Requirements Traces

Traces To: [REQ-ORCHES-0191](../requirements/orches.md#req-orches-0191), [REQ-ORCHES-0188](../requirements/orches.md#req-orches-0188)

### Chat Streaming (Phase 1)

- Spec ID: `CYNAI.ORCHES.NatsChatStreamingSubjects` <a id="spec-cynai-orches-natschatstreamingsubjects"></a>

Subjects for interactive chat request/response streaming between clients and PMA, bridged by the worker.

- `cynode.chat.request.<session_id>` -- client publishes a chat completion request
- `cynode.chat.stream.<session_id>.<message_id>` -- worker bridge publishes token deltas from PMA
- `cynode.chat.amendment.<session_id>.<message_id>` -- orchestrator publishes post-stream redaction amendments
- `cynode.chat.done.<session_id>.<message_id>` -- worker bridge publishes stream completion signal

#### Chat Streaming Subject Behavior

- **Request flow**: The client publishes a `chat.request` message containing the chat completion payload (messages, model, options).
  The worker bridge for the target session's PMA subscribes to `cynode.chat.request.<session_id>`, forwards the request to PMA via the existing UDS HTTP proxy, and reads the PMA NDJSON response.
- **Token streaming**: As PMA produces NDJSON token deltas over UDS, the worker bridge republishes each delta to `cynode.chat.stream.<session_id>.<message_id>`.
  The client subscribes to this subject and renders tokens in real time.
- **Stream completion**: When PMA finishes producing tokens, the worker bridge publishes `chat.done` with final usage statistics and finish reason.
- **Amendments**: The orchestrator subscribes to `cynode.chat.stream.>` as an observer.
  After the stream completes, if post-stream redaction detects sensitive content, the orchestrator publishes `chat.amendment` with the redacted replacement.
  The client applies amendments to the displayed content.
- **PMA is unchanged**: PMA continues to speak HTTP/NDJSON over UDS.
  The worker is the NATS boundary that bridges between NATS subjects and the UDS proxy.
- **Orchestrator does NOT block the stream**: The orchestrator observes tokens asynchronously for persistence (Postgres), redaction (amendments), and tool-call auditing.
  It does not gate or delay token delivery to clients.

#### Chat Streaming Subjects Requirements Traces

Traces To: [REQ-ORCHES-0188](../requirements/orches.md#req-orches-0188)

### Node Configuration Notifications (Phase 1)

- Spec ID: `CYNAI.ORCHES.NatsConfigChangeNotification` <a id="spec-cynai-orches-natsconfigchangenotification"></a>

Published by the orchestrator control-plane when `managed_services`, policy, or other node configuration changes.

- `cynode.node.config_changed.<tenant_id>.<node_id>`

The node-manager subscribes and immediately fetches the updated configuration, replacing or supplementing the poll interval.
This reduces "config bump to container action" latency from poll-interval to near-zero.

### Job Subjects (Phase 3)

- `cynode.job.requested.<tenant_id>.<project_id>`
- `cynode.job.assigned.<tenant_id>.<project_id>.<node_id>`
- `cynode.job.started.<tenant_id>.<project_id>.<job_id>`
- `cynode.job.progress.<tenant_id>.<project_id>.<job_id>`
- `cynode.job.completed.<tenant_id>.<project_id>.<job_id>`
- `cynode.job.canceled.<tenant_id>.<project_id>.<job_id>`
- `cynode.job.failed.<tenant_id>.<project_id>.<job_id>`

#### Job Subject Notes

- `requested` is produced by orchestrator/dispatcher
- `assigned` is produced by dispatcher (or worker if using pull-claim)
- `started/progress/completed` are produced by the worker node

### Work Items and Requirements (Future)

- `cynode.workitem.created.<tenant_id>.<project_id>`
- `cynode.workitem.updated.<tenant_id>.<project_id>`
- `cynode.workitem.transitioned.<tenant_id>.<project_id>`
- `cynode.requirement.created.<tenant_id>.<project_id>`
- `cynode.requirement.updated.<tenant_id>.<project_id>`
- `cynode.requirement.verified.<tenant_id>.<project_id>`
- `cynode.acceptance.validated.<tenant_id>.<project_id>`
- `cynode.acceptance.failed.<tenant_id>.<project_id>`

### Policy Approvals (Future)

- `cynode.policy.requested.<tenant_id>.<project_id>`
- `cynode.policy.decided.<tenant_id>.<project_id>`

### Artifacts and Indexing (Future)

- `cynode.artifact.created.<tenant_id>.<project_id>`
- `cynode.artifact.available.<tenant_id>.<project_id>`
- `cynode.index.requested.<tenant_id>.<project_id>`
- `cynode.index.completed.<tenant_id>.<project_id>`
- `cynode.embedding.requested.<tenant_id>.<project_id>.<namespace>`
- `cynode.embedding.completed.<tenant_id>.<project_id>.<namespace>`

### Node Presence and Capacity (Phase 2)

- `cynode.node.heartbeat.<tenant_id>.<node_id>`
- `cynode.node.capacity.<tenant_id>.<node_id>`
- `cynode.node.status.<tenant_id>.<node_id>`

## NATS Authentication and Credentials

- Spec ID: `CYNAI.USRGWY.NatsClientCredentials` <a id="spec-cynai-usrgwy-natsclientcredentials"></a>

All NATS actors authenticate via HTTP first, then receive a scoped NATS JWT for direct messaging.

### NATS Account Structure

- **Operator**: One per CyNodeAI deployment, manages signing keys.
- **System Account**: Internal services (orchestrator control-plane, gateway, worker) use the system account.
  Each system service receives a JWT scoped to its role (see below).
- **Tenant Account**: One per tenant; isolates subject namespaces.
- **Session User**: One per authenticated client session; scoped to the session's subjects.

### Client Credential Issuance Flow

1. Client authenticates via `POST /v1/auth/login` (or token refresh) through the user-gateway.
2. On successful authentication, the gateway generates a **session-scoped NATS JWT** and assembles the full NATS configuration block.
3. The login response includes a `nats` object alongside the existing access and refresh tokens:
   - `url` (string) -- NATS TCP server URL (`nats://host:port`).
   - `websocket_url` (string, optional) -- NATS WebSocket URL (`ws://host:port/nats`); present when the deployment exposes a WebSocket listener.
   - `jwt` (string) -- session-scoped NATS JWT.
   - `jwt_expires_at` (string) -- RFC 3339 UTC expiry.
   - `ca_bundle_pem` (string, optional) -- TLS CA for the NATS server, when using a private CA.
4. The client connects to NATS using the URL and JWT from the response (TCP for cynork, WebSocket for Web Console).
   The client MUST NOT hardcode or cache NATS connection details across sessions; each login yields the current configuration.

### Session JWT Subject Permissions

The session JWT grants publish and subscribe permissions scoped to the session:

- **Publish allow**: `cynode.session.activity.<tenant_id>.<session_id>`, `cynode.session.attached.<tenant_id>.<session_id>`, `cynode.session.detached.<tenant_id>.<session_id>`, `cynode.chat.request.<session_id>`
- **Subscribe allow**: `cynode.chat.stream.<session_id>.>`, `cynode.chat.amendment.<session_id>.>`, `cynode.chat.done.<session_id>.>`
- **Deny all other subjects**: clients cannot subscribe to other sessions or publish to system subjects.

### Client Credential Lifecycle

- **Expiry**: JWT expiry is tied to the refresh token lifetime.
  When the refresh token expires, the NATS JWT expires and the client is disconnected.
- **Revocation**: On logout or session revocation, the gateway publishes the JWT to the NATS account revocation list.
  The NATS server disconnects the client immediately.
- **Refresh**: When the client refreshes its HTTP access token, a new NATS JWT may be issued if the previous one is near expiry.

### Worker Node Credential Issuance Flow

1. Worker authenticates with the orchestrator via HTTP(S) during [registration and bootstrap](worker_node.md#spec-cynai-worker-registrationandbootstrap) using the pre-shared key (existing flow).
2. The orchestrator generates a **node-scoped NATS JWT** under the system account, with permissions limited to the worker's role.
3. The orchestrator returns a `nats` configuration block in the bootstrap payload containing the server URL, JWT, JWT expiry, optional TLS CA, and optional subject overrides.
   See [`node_bootstrap_payload_v1`](worker_node_payloads.md#spec-cynai-worker-payload-bootstrap-v1) for the full schema.
4. The worker connects to NATS using the URL and JWT from the bootstrap payload.
   The worker MUST NOT hardcode NATS connection details; the bootstrap payload is the sole source.

### Worker JWT Subject Permissions

The worker JWT grants publish and subscribe permissions scoped to its managed sessions and system subjects:

- **Subscribe allow**: `cynode.chat.request.>` (filtered per active session on application layer), `cynode.config.node.<node_id>`
- **Publish allow**: `cynode.chat.stream.>`, `cynode.chat.done.>`, `cynode.session.activity.>`
- **Deny**: tenant-account subjects, client-facing credential subjects.

### Worker Credential Lifecycle

- **Expiry**: Worker NATS JWT has a fixed TTL (default: 24 h, configurable via orchestrator policy).
- **Rotation**: Before expiry, the worker requests a refreshed JWT from the orchestrator via HTTP.
  The orchestrator validates the node's registration status before issuing a new JWT.
- **Revocation**: If the orchestrator deregisters a node or detects compromise, it revokes the JWT via the NATS account revocation list.
  The NATS server disconnects the worker immediately.
- **Reconnect**: On NATS disconnect (network, JWT revocation, or server restart), the worker applies bounded backoff and re-authenticates with the orchestrator if the JWT is expired or revoked.

### NATS Authentication Requirements Traces

Traces To: [REQ-ORCHES-0191](../requirements/orches.md#req-orches-0191), [REQ-ORCHES-0188](../requirements/orches.md#req-orches-0188)

## NATS WebSocket Listener

- Spec ID: `CYNAI.ORCHES.NatsWebSocketListener` <a id="spec-cynai-orches-natswebsocketlistener"></a>

The NATS server exposes a WebSocket listener for browser-based clients (Web Console).

### WebSocket Configuration

- Port: `8223` (separate from the TCP client port 4222 and monitoring port 8222)
- Path: `/nats`
- TLS: Required in production deployments; optional in dev
- Auth: Same NATS JWT model as TCP connections

### WebSocket Client Behavior

- The Web Console connects via `ws://nats-host:8223/nats` (or `wss://` with TLS).
- Authentication uses the same session-scoped JWT received from the HTTP login flow.
- Subject subscriptions and publish patterns are identical to TCP clients (cynork).
- If WebSocket is unavailable, the Web Console falls back to HTTP/SSE via the user-gateway.

## JetStream Streams

Streams define durable storage and retention for each domain.

### Stream: `CYNODE_SESSION` (Phase 1)

- Spec ID: `CYNAI.ORCHES.NatsSessionStream` <a id="spec-cynai-orches-natssessionstream"></a>

This stream stores session activity and attachment lifecycle events.

#### `CYNODE_SESSION` Stream Purpose

Short-lived session presence data for PMA idle lifecycle and re-activation replay on orchestrator restart.

#### `CYNODE_SESSION` Stream Subjects

- `cynode.session.activity.*.*`
- `cynode.session.attached.*.*`
- `cynode.session.detached.*.*`

#### `CYNODE_SESSION` Stream Retention

- Time-based retention (hours); only needed for replay if the orchestrator restarts while sessions are active

Recommended max age: 1 to 6 hours.

#### `CYNODE_SESSION` Requirements Traces

Traces To: [REQ-ORCHES-0191](../requirements/orches.md#req-orches-0191), [REQ-ORCHES-0188](../requirements/orches.md#req-orches-0188)

### Stream: `CYNODE_CHAT` (Phase 1)

- Spec ID: `CYNAI.ORCHES.NatsChatStream` <a id="spec-cynai-orches-natschatstream"></a>

This stream stores chat request, token streaming, amendment, and completion events for interactive PMA sessions.

#### `CYNODE_CHAT` Stream Purpose

Short-lived chat streaming data for mid-stream replay (client reconnects) and orchestrator persistence.
Not a long-term store; messages are persisted to Postgres by the orchestrator observer after stream completion.

#### `CYNODE_CHAT` Stream Subjects

- `cynode.chat.request.*`
- `cynode.chat.stream.*.*`
- `cynode.chat.amendment.*.*`
- `cynode.chat.done.*.*`

#### `CYNODE_CHAT` Stream Retention

- Time-based retention (hours); only needed for replay if the client reconnects mid-stream or the orchestrator restarts during an active stream

Recommended max age: 1 to 4 hours.

#### `CYNODE_CHAT` Requirements Traces

Traces To: [REQ-ORCHES-0188](../requirements/orches.md#req-orches-0188)

### Stream: `CYNODE_JOBS` (Phase 3)

This stream stores job dispatch and lifecycle events.

#### `CYNODE_JOBS` Stream Subjects

- `cynode.job.requested.*.*`
- `cynode.job.assigned.*.*.*`
- `cynode.job.started.*.*.*`
- `cynode.job.completed.*.*.*`
- `cynode.job.failed.*.*.*`
- `cynode.job.canceled.*.*.*`

#### `CYNODE_JOBS` Stream Retention

- WorkQueue retention for `requested/assigned` (or Interest retention if multiple consumers must see all)
- Time-based retention for lifecycle events (days) for postmortems
- Explicit ack required for all durable consumers

Recommended max age: 3 to 14 days.

### Stream: `CYNODE_EVENTS` (Future)

This stream stores work item and requirement events.

#### `CYNODE_EVENTS` Stream Subjects

- `cynode.workitem.*.*.*`
- `cynode.requirement.*.*.*`
- `cynode.acceptance.*.*.*`
- `cynode.policy.*.*.*`
- `cynode.artifact.*.*.*`

#### `CYNODE_EVENTS` Stream Retention

Limits or time-based (weeks to months), depending on audit requirements.

Recommended max age: 30 to 180 days.

### Stream: `CYNODE_TELEMETRY` (Phase 2)

This stream stores node heartbeats and capacity data.

#### `CYNODE_TELEMETRY` Stream Subjects

- `cynode.job.progress.*.*.*`
- `cynode.node.*.*`

#### `CYNODE_TELEMETRY` Stream Retention

Short time-based retention (minutes to hours).

Recommended max age: 1 to 24 hours.

## Consumer Patterns

Recommended subscription and processing patterns per consumer type.

### Session Activity Consumer (Orchestrator, Phase 1)

- Spec ID: `CYNAI.ORCHES.NatsSessionActivityConsumer` <a id="spec-cynai-orches-natssessionactivityconsumer"></a>

The orchestrator control-plane subscribes to session activity subjects and updates binding state in Postgres.

- On `session.activity`: update `last_activity_at` on the corresponding session binding.
- On `session.attached`: ensure the session binding is `active` and the PMA managed service is in desired state (re-activation path, triggers greedy provisioning without waiting for the next scanner cycle).
- On `session.detached` (clean logout or explicit disconnect): begin accelerated idle countdown or trigger immediate teardown per policy.
- The PMA binding scanner goroutine remains as a safety net for NATS downtime, missed messages, and clock skew.

#### Session Activity Consumer Requirements Traces

Traces To: [REQ-ORCHES-0191](../requirements/orches.md#req-orches-0191), [REQ-ORCHES-0188](../requirements/orches.md#req-orches-0188)

### Gateway Session Activity Publisher (Phase 1, HTTP-Only Fallback)

- Spec ID: `CYNAI.USRGWY.NatsSessionActivityPublisher` <a id="spec-cynai-usrgwy-natssessionactivitypublisher"></a>

The user-gateway publishes session activity to NATS on behalf of HTTP-only API clients that do not connect to NATS directly (e.g., Open WebUI, webhook consumers, API scripts).

- For HTTP-only clients, the gateway tracks the last API interaction timestamp per session and publishes `session.activity` at `T_heartbeat` cadence.
- On login of an HTTP-only client, the gateway publishes `session.attached`.
- On logout or session close of an HTTP-only client, the gateway publishes `session.detached`.
- For NATS-connected clients (cynork, Web Console), the gateway does NOT publish session activity; the client publishes directly.
- The gateway determines whether a client is NATS-connected by checking whether the login response included NATS credentials and whether the session has an active NATS connection (tracked via NATS connection events or session metadata).

#### Gateway Session Activity Publisher Requirements Traces

Traces To: [REQ-ORCHES-0191](../requirements/orches.md#req-orches-0191), [REQ-ORCHES-0188](../requirements/orches.md#req-orches-0188)

### Worker Chat Bridge (Phase 1)

- Spec ID: `CYNAI.WORKER.NatsChatBridge` <a id="spec-cynai-worker-natschatbridge"></a>

The worker bridges NATS chat subjects to PMA containers over UDS.
PMA remains UDS-only; the worker is the NATS boundary.

- For each managed PMA with an active session binding, the worker subscribes to `cynode.chat.request.<session_id>`.
- On receipt of a `chat.request` message:
  1. Extract the chat completion payload from the NATS message.
  2. Forward the request to PMA via the existing UDS HTTP proxy (`POST /internal/chat/completion` with `stream: true`).
  3. Read the PMA NDJSON response incrementally.
  4. For each NDJSON line, publish a `chat.stream` message to `cynode.chat.stream.<session_id>.<message_id>`.
  5. On stream completion, publish `chat.done` to `cynode.chat.done.<session_id>.<message_id>`.
- If PMA returns an error, publish an error event on `cynode.chat.done.<session_id>.<message_id>` with error details.
- The worker MUST NOT buffer the entire PMA response; tokens are relayed incrementally.

#### Worker Chat Bridge Requirements Traces

Traces To: [REQ-ORCHES-0188](../requirements/orches.md#req-orches-0188), [REQ-WORKER-0176](../requirements/worker.md#req-worker-0176)

### Orchestrator Chat Observer (Phase 1)

- Spec ID: `CYNAI.ORCHES.NatsChatObserver` <a id="spec-cynai-orches-natschatobserver"></a>

The orchestrator subscribes to chat streaming subjects as a non-blocking observer for persistence, redaction, and auditing.

- Subscribe to `cynode.chat.stream.>`, `cynode.chat.done.>`, and `cynode.chat.request.>`.
- **Persistence**: Accumulate streamed tokens per `message_id`; on `chat.done`, persist the full message to Postgres.
- **Redaction**: After stream completion, run post-stream redaction on the accumulated content.
  If redaction produces changes, publish `chat.amendment` to `cynode.chat.amendment.<session_id>.<message_id>` with the redacted content.
- **Tool-call auditing**: Observe tool-call tokens in the stream for audit logging.
- The observer MUST NOT block, delay, or gate the token stream to clients.
  Clients receive tokens directly from the worker bridge without orchestrator mediation.

#### Orchestrator Chat Observer Requirements Traces

Traces To: [REQ-ORCHES-0188](../requirements/orches.md#req-orches-0188), [REQ-ORCHES-0191](../requirements/orches.md#req-orches-0191)

### Config Notification Consumer (Node-Manager, Phase 1)

- Spec ID: `CYNAI.WORKER.NatsConfigNotificationSubscriber` <a id="spec-cynai-worker-natsconfignotificationsubscriber"></a>

The node-manager subscribes to `cynode.node.config_changed.<tenant_id>.<node_id>` for its own node.
On receipt, the node-manager fetches updated configuration from the control-plane and reconciles managed services.
The existing poll interval remains as a fallback for missed NATS messages.

### Job Execution Consumers (Workers, Phase 3)

Pattern A - Dispatcher assigns jobs to nodes:

- Dispatcher consumes `cynode.job.requested.<tenant>.<project>`
- Dispatcher publishes `cynode.job.assigned.<tenant>.<project>.<node>`
- Worker consumes its node-specific assigned subject

Pattern B - Workers pull-claim jobs:

- Workers form a queue group consuming `cynode.job.requested.<tenant>.<project>`
- First worker to claim persists locally and acks
- Worker publishes `cynode.job.started/...` etc.

Start with Pattern A for scheduling constraints (GPU locality, model cache locality); use Pattern B for simple homogeneous clusters.

### Read Model Updaters (Future)

- Subscribe to `CYNODE_EVENTS`
- Validate schema, verify scope, then update Postgres (authoritative)
- Must be idempotent by `event_id`

### Live UX Subscribers (Future)

- Subscribe to `cynode.job.progress...` and `cynode.node.capacity...`
- Should tolerate loss or truncation; do not treat telemetry as authoritative

## Message Envelope Specification

All messages use a common envelope with strict schema validation.

### Envelope Fields

- `event_id` (UUID) - unique per published message
- `event_type` (string) - stable identifier, e.g., `session.activity`
- `event_version` (string) - semver for payload schema, e.g., `1.0.0`
- `occurred_at` (RFC3339 timestamp)
- `producer` (object)

  - `service` (string)
  - `instance_id` (string)
- `scope` (object)

  - `tenant_id` (string)
  - `project_id` (string, nullable for global)
  - `sensitivity` (public|internal|confidential|restricted)
- `correlation` (object)

  - `session_id` (string, nullable)
  - `work_item_id` (string, nullable)
  - `job_id` (string, nullable)
  - `trace_id` (string, nullable)
- `payload` (object) - schema depends on event_type/event_version

### Envelope Example

```json
{
  "event_id": "8c4ef0e9-0b3a-4e8f-a5fc-1c9c7a6c0c3a",
  "event_type": "chat.stream",
  "event_version": "1.0.0",
  "occurred_at": "2026-03-31T10:15:00Z",
  "producer": {
    "service": "cynode-worker",
    "instance_id": "worker-node-01"
  },
  "scope": {
    "tenant_id": "t-123",
    "project_id": null,
    "sensitivity": "internal"
  },
  "correlation": {
    "session_id": "s-abc",
    "work_item_id": null,
    "job_id": null,
    "trace_id": "tr-222"
  },
  "payload": {
    "message_id": "msg-456",
    "delta": {
      "role": "assistant",
      "content": "Hello"
    },
    "sequence": 1
  }
}
```

## Payload Schemas

Canonical payload shapes for core message types (versioned).

### `session.activity` `v1.0.0`

Periodic heartbeat indicating an active client session.

- `session_id` (UUID)
- `user_id` (UUID)
- `binding_key` (string, opaque session binding key)
- `client_type` (string: `cynork`|`web_console`|`other`)
- `ts` (RFC3339 timestamp)

### `session.attached` `v1.0.0`

Client has established a session activity channel.

- `session_id` (UUID)
- `user_id` (UUID)
- `binding_key` (string)
- `client_type` (string: `cynork`|`web_console`|`other`)
- `ts` (RFC3339 timestamp)

### `session.detached` `v1.0.0`

Client has cleanly disconnected from the session activity channel.

- `session_id` (UUID)
- `user_id` (UUID)
- `binding_key` (string)
- `reason` (string: `logout`|`client_close`|`timeout`)
- `ts` (RFC3339 timestamp)

### `chat.request` `v1.0.0`

Client submits a chat completion request over NATS.

- `message_id` (UUID, unique per request; used as the subject token for response streaming)
- `session_id` (UUID)
- `user_id` (UUID)
- `model` (string, e.g., `cynodeai.pm`)
- `messages` (array of objects: `{role, content}`, same schema as OpenAI chat completions)
- `stream` (boolean, always `true` for NATS-routed requests)
- `options` (object, optional: temperature, max_tokens, etc.)
- `ts` (RFC3339 timestamp)

### `chat.stream` `v1.0.0`

Worker bridge publishes a token delta from PMA.

- `message_id` (UUID, same as in the originating `chat.request`)
- `sequence` (integer, monotonically increasing per message_id for ordering)
- `delta` (object)

  - `role` (string, nullable: `assistant`)
  - `content` (string, nullable: visible text token)
  - `thinking` (string, nullable: thinking/reasoning token)
  - `tool_call` (object, nullable: tool call delta per OpenAI schema)
- `ts` (RFC3339 timestamp)

### `chat.amendment` `v1.0.0`

Orchestrator publishes a post-stream redaction amendment.

- `message_id` (UUID)
- `amended_content` (string, the redacted version of the full visible content)
- `reason` (string: `redaction`|`moderation`|`other`)
- `ts` (RFC3339 timestamp)

### `chat.done` `v1.0.0`

Worker bridge publishes stream completion.

- `message_id` (UUID)
- `finish_reason` (string: `stop`|`length`|`tool_calls`|`error`)
- `usage` (object, nullable)

  - `prompt_tokens` (integer)
  - `completion_tokens` (integer)
  - `total_tokens` (integer)
- `error` (object, nullable: `{code, message}` if `finish_reason` is `error`)
- `ts` (RFC3339 timestamp)

### `node.config_changed` `v1.0.0`

Notification that the node configuration has been updated.

- `node_id` (string)
- `config_version` (string, ULID or equivalent monotonic version)
- `changed_sections` (array of strings, optional: `managed_services`|`policy`|`inference_backend`|`other`)
- `ts` (RFC3339 timestamp)

## RBAC and Multi-Tenancy Controls

Access is enforced at both NATS and message level.

### NATS-Level Controls

- Use NATS accounts per tenant for subject isolation
- Publish/subscribe permissions are embedded in session-scoped JWTs (see [NATS Authentication and Credentials](#nats-authentication-and-credentials))
- System services (gateway, control-plane, worker) use a system account with permissions scoped to their role

### Message-Level Controls

- Every message includes `scope.tenant_id` and `scope.project_id`
- Consumers must validate scope matches their allowed set and sensitivity does not exceed role allowance
- Never rely solely on subject routing for authorization

## Idempotency and Deduplication

Consumers must handle duplicate delivery and apply updates idempotently.

- Every message includes `event_id` (unique)
- Consumers must store processed `event_id`s (or a rolling window) to avoid double-apply
- Job execution must be idempotent by `job_id`
- Chat stream messages include `sequence` for ordering and deduplication

## Ordering and Consistency

- Do not assume global ordering across subjects.
- For a single job, prefer publishing lifecycle events using the same `job_id` subject token to improve locality.
- For chat streaming, the `sequence` field within `chat.stream` messages provides per-message ordering.
- Consumers must tolerate duplicates, out-of-order progress events, and missing telemetry.

## Payload Size Limits

- Enforce a maximum message size (platform config).
- Put large content in object storage: logs, reports, job specs, artifact manifests.
- Messages carry URIs + hashes, not the bytes.
- Chat request `messages` arrays that exceed the message size limit must be uploaded to object storage and referenced by URI.

## Operational Defaults

Suggested initial defaults:

- CYNODE_SESSION max age: 6 hours
- CYNODE_CHAT max age: 4 hours
- CYNODE_TELEMETRY max age: 6 hours
- CYNODE_JOBS max age: 7 days
- CYNODE_EVENTS max age: 90 days
- session.activity publish rate: once per `T_heartbeat` (2-3 minutes) per active session
- chat.stream publish rate: as fast as PMA produces tokens (no throttling)
- heartbeat publish rate: every 5-15 seconds per node (tunable)
- job.progress publish rate: 1-2 Hz per active job (tunable)
- NATS WebSocket port: 8223

## Implementation Phasing

NATS adoption is phased to allow incremental validation and reduce risk.

### Phase 1 (Current): Full NATS Transport

Deploy NATS (single-node, JetStream enabled, WebSocket listener) as part of the dev stack.
Phase 1 delivers NATS as the primary transport for session lifecycle, chat streaming, and config notifications.

- **Infrastructure**: NATS in compose with TCP (4222), monitoring (8222), and WebSocket (8223) ports
- **Authentication**: Session-scoped NATS JWTs issued by gateway on login; revocation on logout
- **Session lifecycle subjects**: `session.activity`, `session.attached`, `session.detached` -- published by NATS-connected clients directly; gateway publishes on behalf of HTTP-only clients
- **Chat streaming subjects**: `chat.request`, `chat.stream`, `chat.amendment`, `chat.done`
- **Streams**: `CYNODE_SESSION`, `CYNODE_CHAT`
- **Worker bridge**: subscribes to `chat.request`, forwards to PMA via UDS, publishes token stream to NATS
- **Orchestrator observer**: subscribes to chat streams for persistence, redaction/amendments, auditing
- **Gateway**: issues NATS credentials, maintains HTTP/SSE backward compatibility for non-NATS API clients
- **cynork**: connects to NATS after HTTP auth, publishes session lifecycle, publishes chat requests, subscribes to token stream
- **Web Console**: connects to NATS via WebSocket, same patterns as cynork
- **Config notifications**: `node.config_changed` published by control-plane, consumed by node-manager

### Phase 2 (Next): Node Presence

Migrate node heartbeats and capacity reporting from HTTP polling to NATS pub/sub.

- Subjects: `node.heartbeat`, `node.capacity`
- Streams: CYNODE_TELEMETRY

### Phase 3 (Later): Job Pipeline

Full job dispatch and lifecycle eventing.

- Subjects: `job.requested`, `job.started`, `job.progress`, `job.completed`, `artifact.available`
- Streams: CYNODE_JOBS
- Idempotency: job_id-based worker dedupe, event_id-based consumer dedupe

Everything else (work items, requirements, policy, indexing) can be added incrementally once the job pipeline is stable.
