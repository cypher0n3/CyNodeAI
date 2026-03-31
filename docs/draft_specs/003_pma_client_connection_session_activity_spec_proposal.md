# PMA Session Activity, Client Connections, and Idle Lifecycle

- [Scope and Metadata](#scope-and-metadata)
- [Summary](#summary)
- [Background](#background)
- [Problem Statement](#problem-statement)
- [Session Activity Model](#session-activity-model)
  - [Term Definitions](#term-definitions)
  - [State Machine](#state-machine)
  - [Relationship to `pma-main`](#relationship-to-pma-main)
- [Policy Decisions (Resolved)](#policy-decisions-resolved)
  - [1 Idle Teardown Independent of Refresh Expiry](#1-idle-teardown-independent-of-refresh-expiry)
  - [2 NATS as Primary Session Activity Transport](#2-nats-as-primary-session-activity-transport)
  - [3 One Binding per Refresh Session](#3-one-binding-per-refresh-session)
  - [4 Reuse `service_id` with Fresh Container](#4-reuse-service_id-with-fresh-container)
  - [5 Dev and CI Idle Policy](#5-dev-and-ci-idle-policy)
- [Proposed Requirements (Draft)](#proposed-requirements-draft)
- [Technical Contracts](#technical-contracts)
  - [1 NATS Session Activity Contract](#1-nats-session-activity-contract)
  - [2 Gateway Activity Publishing](#2-gateway-activity-publishing)
  - [3 Idle Policy Constants](#3-idle-policy-constants)
  - [4 Scanner Safety Net](#4-scanner-safety-net)
- [Orchestrator and Worker Behavior](#orchestrator-and-worker-behavior)
- [Client Obligations (Cynork and Web)](#client-obligations-cynork-and-web)
- [Traceability](#traceability)
- [References](#references)

## Scope and Metadata

- Date: 2026-03-31
- Status: Draft proposal (`docs/draft_specs`; prepared for promotion to canonical)
- Scope: Define how **active** interactive sessions are tracked for per-session PMA (`pma-sb-*`), including **client connection** semantics, **idle timeout**, teardown of managed containers, and **re-provision** when a session becomes active again.
- Non-scope: Replacing refresh-token authentication; full Web Console UX; changing the default shared PMA (`pma-main`) except where noted.

## Summary

Per-session PMA bindings are driven by **refresh session lifecycle** (login, refresh, logout, revoke) and a **periodic PMA binding scanner** that tears down bindings when refresh rows are missing, inactive, expired, or **idle** beyond a policy window.

This spec makes **"session active"** normatively tied to **client attachment**: a user agent (Cynork CLI, Web Console, or other first-party clients) maintains an explicit **activity signal** while the user is connected or actively using the product.

When that signal is absent beyond an **idle timeout** (`T_idle`), the orchestrator marks the binding for teardown and the worker removes the corresponding `cynodeai-managed-pma-sb-*` container.

When the user returns and the client re-attaches (or logs in again), the system **re-establishes** the desired managed service for that session using the same `service_id` and binding key, starting a fresh container per existing managed-service reconciliation rules.

The activity signal flows through **NATS** as the primary transport: the user-gateway publishes session activity messages to a `CYNODE_SESSION` JetStream stream and the orchestrator subscribes for event-driven idle detection.
NATS is deployed alongside this feature as its first production use-case; future phases will expand NATS usage across the architecture as the unified transport layer (see [002_nats_messaging.md](002_nats_messaging.md)).

## Background

- Per-session PMA is described in [orchestrator_bootstrap.md](../tech_specs/orchestrator_bootstrap.md) and related orchestrator requirements.
- The user-gateway runs `RunPMABindingScanner` on an interval; it calls `TeardownPMAForInteractiveSession` when refresh state or binding idle policy fails.
- Gateway code calls `TouchActiveSessionBindingsForUser` on some authenticated routes to advance **last activity** on bindings.
- Node-manager reconciles **desired** `managed_services` from the control-plane payload and stops undesired `cynodeai-managed-*` containers.

## Problem Statement

Three problems motivate this spec:

- Refresh validity alone is not equivalent to "user is connected."
  A long-lived refresh token can remain valid while no client is attached, which keeps per-session PMA desired state longer than operators expect.
- Operators expect (after stack stop/start or at idle) at most one non-session PMA instance (for example `pma-main`) unless n clients hold n genuinely active sessions.
- After idle teardown, a session that becomes active again must re-acquire a running PMA instance without requiring a full stack reset.

## Session Activity Model

The following definitions and state machine describe the intended runtime behavior.

### Term Definitions

- **Interactive session:** The refresh-session row (and JWT access rotation) created for a user at login or refresh, bound to a **session binding key** used for PMA `service_id` derivation.
- **Client attachment:** A first-party client maintains a **session activity channel** while the user is considered "active" for PMA purposes.
- **Session activity channel:** The user-gateway publishes `cynode.session.activity` messages to NATS at `T_heartbeat` interval on behalf of the connected client.
  The gateway determines activity from authenticated API requests (chat, MCP tool calls, explicit pings) and from active SSE or streaming chat connections.
  An active streaming connection MAY suppress the need for separate activity pings (the gateway treats stream-open as equivalent to activity).
- **T_heartbeat:** Maximum interval between client heartbeats (2 to 3 minutes, configurable).
- **T_idle:** Maximum time without any activity before teardown (aligns with or replaces `PMA_BINDING_IDLE_TIMEOUT_MIN`; default 30 minutes).

### State Machine

1. **Attached and active:** Gateway publishes `session.activity` to NATS at least once per `T_heartbeat` while the client is active.
   Binding remains `active`; desired managed service includes `pma-sb-<binding>`.
2. **Idle:** No activity message for `T_idle` => orchestrator transitions binding to `teardown_pending` and bumps config; worker stops the container.
3. **Re-activation:** User returns => client re-authenticates if needed, gateway publishes `session.attached` => orchestrator ensures binding is `active` again with the **same `service_id`** and desired state includes the managed service; node-manager starts a **fresh container** for that `service_id` per reconciliation.

### Relationship to `pma-main`

- `pma-main` remains the **default** shared PMA instance for the node when policy includes it.
- `pma-sb-*` instances are **only** for interactive session bindings that are **active** under this model (plus any transition states explicitly allowed by policy).

## Policy Decisions (Resolved)

The following decisions resolve the open points from the initial proposal.

### 1 Idle Teardown Independent of Refresh Expiry

Independent (dual) policy: idle teardown fires when `T_idle` passes without an explicit activity signal, regardless of refresh token validity.

- Refresh tokens represent *authorization* to act, not *intent* to act.
  Coupling teardown to refresh validity would keep idle PMA containers alive for the full token lifetime.
- Refresh expiry is a separate safety-net trigger: if the client crashes without a clean logout, the refresh token eventually expires and the scanner cleans up the binding.
- The current scanner already implements both branches independently (`idle_timeout` and `refresh_session_expired_or_inactive`).

### 2 NATS as Primary Session Activity Transport

NATS is deployed alongside this feature as its first production use-case; the user-gateway publishes session activity messages directly to NATS.
No separate HTTP heartbeat endpoint is required.

- The user-gateway publishes `session.activity`, `session.attached`, and `session.detached` messages to the `CYNODE_SESSION` JetStream stream on behalf of authenticated clients.
- The gateway determines activity from any authenticated API request (chat, MCP tool calls) or active streaming connection; it consolidates these into `session.activity` publishes at `T_heartbeat` cadence.
- NATS credentials remain internal to server-side components; clients interact with the gateway via existing authenticated HTTP/SSE endpoints.
- No dedicated client-facing heartbeat endpoint or WebSocket protocol is needed; the gateway derives session liveness from normal API traffic patterns and publishes to NATS server-side.
- This approach avoids a throwaway HTTP heartbeat that would be replaced by NATS later, and validates the NATS integration pattern before broader adoption across the architecture.

### 3 One Binding per Refresh Session

Each refresh session gets its own `pma-sb-*` binding and activity channel; activity from any tab sharing the same refresh session keeps that binding alive.

- Each login creates a distinct refresh session; each refresh session gets one `pma-sb-*`.
  This is already implemented via the binding key derivation (`user + session + thread`).
- Multiple browser tabs sharing the same refresh session share the same binding.
  Activity from any tab resets the idle clock for that binding.
- A second device with a separate login gets a second refresh session and a second PMA binding.
  This is the correct behavior: distinct sessions, distinct agent state.
- Clients SHOULD send a `X-Session-ID` header (or include `session_id` in the heartbeat body) so the gateway can attribute activity to the correct binding rather than touching all bindings for the user.

### 4 Reuse `service_id` With Fresh Container

On re-activation, the orchestrator reuses the same `service_id` (deterministically derived from the binding key) and the worker starts a fresh container.

- The `service_id` (`pma-sb-<hash12>`) is derived deterministically from the binding key.
  On re-activation, the orchestrator flips state back to `active` and re-adds the same `service_id` to desired state.
- The worker starts a **fresh** container for that `service_id` (reconciler: "desired but no running container => start").
  This gives a clean PMA state without stale in-memory context from the previous idle period.
- No new allocation logic is needed; the existing reconciler already handles "desired but missing" => start.

### 5 Dev and CI Idle Policy

Idle policy is controlled via environment variables; dev environments use long or disabled timeouts.

- `PMA_BINDING_IDLE_TIMEOUT_MIN` (default 30 minutes) is sufficient for dev.
  Setting it to `0` disables idle teardown entirely; setting it to a large value prevents scanner interference during manual testing.
- E2E tests that validate idle teardown set a short idle timeout via env override in the test harness.
- `just setup-dev stop` continues to use the dev-reset endpoint (`/v1/dev/reset-pma-state`).
- Document interaction in [development_setup.md](../development_setup.md) when stable.

## Proposed Requirements (Draft)

These are candidates for `docs/requirements/` after review; IDs are placeholders.

- **REQ-IDENTY-0XXX (proposed):** The system SHALL distinguish **authenticated** (valid refresh) from **session-attached** (client activity within `T_idle`) for purposes of per-session resource provisioning.
- **REQ-ORCHES-0XXX (proposed):** The orchestrator SHALL tear down per-session PMA bindings when the binding is **idle** per policy, **independent** of refresh token wall-clock validity.
  Refresh expiry is a separate safety-net trigger.
- **REQ-USRGWY-0XXX (proposed):** The user-gateway SHALL publish `session.activity`, `session.attached`, and `session.detached` messages to NATS on behalf of authenticated clients, deriving session liveness from API traffic patterns.
- **REQ-CLIENT-0XXX (proposed):** Cynork SHALL maintain authenticated API interaction (chat, MCP tool calls, or explicit pings) at least once per `T_heartbeat` interval while an interactive session is in use, so the gateway can derive session liveness.
- **REQ-WEBCON-0XXX (proposed):** The Web Console SHALL maintain authenticated API interaction at least once per `T_heartbeat` interval while a browser session is foregrounded and logged in.
  Background tabs MAY reduce interaction frequency or pause.
- **REQ-WORKER-0XXX (proposed):** The node SHALL converge managed PMA containers to **exactly** the desired set from the orchestrator, including starting a **fresh container** for a `service_id` that re-appears in desired state after idle teardown.

## Technical Contracts

This section defines gateway and policy surfaces; wire formats belong in the canonical user API spec after acceptance.

### 1 NATS Session Activity Contract

NATS is the primary transport for session activity (see [002_nats_messaging.md](002_nats_messaging.md) section 4.6).

- The gateway publishes `session.activity`, `session.attached`, and `session.detached` messages to the `CYNODE_SESSION` JetStream stream.
- The orchestrator subscribes and updates `last_activity_at` with sub-second latency, replacing the polling-based idle check for connected sessions.
- On `session.attached`, the orchestrator ensures the binding is `active` and the PMA managed service is in desired state (re-activation without waiting for the next scanner cycle).
- On `session.detached` (clean logout or explicit disconnect), the orchestrator may begin an accelerated idle countdown per policy.
- If the client crashes without a clean detach, the absence of `session.activity` messages within `T_idle` serves as an implicit detach and triggers teardown via the scanner safety net.

### 2 Gateway Activity Publishing

The user-gateway is responsible for deriving session liveness and publishing to NATS.
No dedicated client-facing heartbeat endpoint is required.

- The gateway tracks the last API interaction timestamp per session (from any authenticated request: chat, MCP tool calls, token refresh, etc.).
- At `T_heartbeat` cadence, the gateway publishes `session.activity` to `cynode.session.activity.<tenant_id>.<session_id>` for each session that has had API interaction since the last publish.
- On login or reconnect after idle, the gateway publishes `session.attached`.
- On logout or explicit session close, the gateway publishes `session.detached`.
- NATS credentials remain internal to server-side components; clients use existing authenticated HTTP/SSE endpoints and are unaware of NATS.

### 3 Idle Policy Constants

- `T_idle`: Maximum time without activity before teardown.
  Environment variable: `PMA_BINDING_IDLE_TIMEOUT_MIN` (default: 30 minutes).
  Set to `0` to disable idle teardown (dev use).
- `T_heartbeat`: Maximum interval between client heartbeats.
  Recommended: 2 to 3 minutes.
  Configurable per deployment.

### 4 Scanner Safety Net

- The PMA binding scanner **continues** as a safety net for orphaned rows, clock skew, NATS downtime, and clients that crash without a clean detach.
- **Control-plane startup** (optional enhancement): run one scan immediately so stale bindings are reconciled quickly after deploy.

## Orchestrator and Worker Behavior

- **Teardown:** `TeardownPMAForInteractiveSession` (or successor) marks `teardown_pending`, invalidates MCP credential intent per existing rules, and bumps node config version so the worker removes undesired containers.
- **Re-provision:** On return to `active`, the orchestrator flips binding state, re-adds the same `service_id` to desired state, and bumps config.
  The node-manager starts a **fresh container** for that `service_id`.
  The greedy provision path (or explicit upsert) runs so that `buildPMAManagedServiceList` includes the binding again.
- **Config push via NATS:** After bumping config, the orchestrator publishes `cynode.node.config_changed.<tenant_id>.<node_id>` so the worker fetches the updated config immediately rather than waiting for the next poll.

## Client Obligations (Cynork and Web)

Clients do not publish to NATS directly; the user-gateway derives session liveness from authenticated API traffic.
Clients are responsible for maintaining interaction at a frequency that keeps the session alive.

- **Cynork:** When the user has logged in and the process is the primary interactive session, Cynork must make at least one authenticated API request (chat, MCP tool call, or explicit session ping) within each `T_heartbeat` window.
  An active streaming connection (for example SSE chat) inherently satisfies this requirement.
- **Web Console:** While the tab or app is foregrounded and logged in, the Web Console must maintain authenticated API interaction within each `T_heartbeat` window.
  Background tabs MAY reduce interaction frequency or pause; the binding remains alive as long as at least one tab drives API traffic within `T_idle`.
- **Session scope:** Clients SHOULD include `session_id` (or `X-Session-ID` header) in API requests so the gateway attributes activity to the correct binding.
  If omitted, the gateway touches all active bindings for the user.

## Traceability

Informative mapping for future requirement IDs (not normative until promoted):

- **Session and auth:** REQ-IDENTY-* (session lifecycle); local user accounts.
- **PMA and bindings:** REQ-ORCHES-* (orchestrator bootstrap, PMA host).
- **Gateway API:** REQ-USRGWY-*.
- **Cynork:** REQ-CLIENT-*.
- **Web Console:** REQ-WEBCON-*.
- **Worker managed services:** REQ-WORKER-*; [worker_node_payloads.md](../tech_specs/worker_node_payloads.md).
- **NATS messaging:** [002_nats_messaging.md](002_nats_messaging.md) (session activity subjects, `CYNODE_SESSION` stream).

## References

- [002_nats_messaging.md](002_nats_messaging.md) (NATS subjects, streams, and payloads for session activity)
- [orchestrator_bootstrap.md](../tech_specs/orchestrator_bootstrap.md) (PMA startup and per-session binding)
- [worker_node.md](../tech_specs/worker_node.md) (managed services, reconciliation)
- [user_api_gateway.md](../tech_specs/user_api_gateway.md) (gateway contracts, if present)
- [110_node_manager_restart_and_pma_redeploy_spec_proposal.md](110_node_manager_restart_and_pma_redeploy_spec_proposal.md) (related lifecycle)
- [_plan_005_pma_provisioning.md](../dev_docs/_plan_005_pma_provisioning.md) (implementation plan context)
