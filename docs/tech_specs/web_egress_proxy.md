# Web Egress Proxy

- [Document Overview](#document-overview)
- [Service Purpose](#service-purpose)
- [Threat Model and Assumptions](#threat-model-and-assumptions)
- [Architecture and Data Flow](#architecture-and-data-flow)
- [Policy Model and Allowlisting](#policy-model-and-allowlisting)
- [Dynamic Temporary Allowlist Exceptions](#dynamic-temporary-allowlist-exceptions)
- [Proxy Protocol and Client Compatibility](#proxy-protocol-and-client-compatibility)
- [Node and Sandbox Integration](#node-and-sandbox-integration)
- [Auditing and Observability](#auditing-and-observability)
- [Non-Goals](#non-goals)

## Document Overview

- Spec ID: `CYNAI.WEBPRX.Doc.WebEgressProxy` <a id="spec-cynai-webprx-doc-webegressproxy"></a>

This document defines a generic, allowlist-based HTTP(S) forward proxy for sandboxed execution.
It exists to support safe dependency downloads (for example Go modules and Python packages) without granting sandboxes direct internet egress.
Sandboxes are not airgapped; when web egress is permitted, it is only through this proxy (and the node-local proxy that forwards to it).

The Web Egress Proxy is distinct from:

- API Egress Server, which performs structured API calls using stored credentials.
  See [`docs/tech_specs/api_egress_server.md`](api_egress_server.md).
- Secure Browser Service, which fetches web pages and returns sanitized text.
  See [`docs/tech_specs/secure_browser_service.md`](secure_browser_service.md).

Traces To:

- [REQ-WEBPRX-0100](../requirements/webprx.md#req-webprx-0100)
- [REQ-WEBPRX-0101](../requirements/webprx.md#req-webprx-0101)
- [REQ-WEBPRX-0102](../requirements/webprx.md#req-webprx-0102)

## Service Purpose

- Spec ID: `CYNAI.WEBPRX.Purpose.WebEgressProxy` <a id="spec-cynai-webprx-purpose-webegressproxy"></a>

The Web Egress Proxy provides outbound HTTP and HTTPS access for sandboxes, subject to strict allowlisting and auditing.
It supports tooling that expects a conventional forward proxy (for example `pip`, `go`, and `curl`) while preserving the sandbox threat model.

Primary goals:

- Enable dependency downloads required for builds and verification.
- Enforce a default-deny destination allowlist.
- Attribute egress activity to `task_id` and `job_id` for auditing.
- Avoid exposing any long-lived secrets to sandboxes.

Traces To:

- [REQ-WEBPRX-0100](../requirements/webprx.md#req-webprx-0100)
- [REQ-WEBPRX-0103](../requirements/webprx.md#req-webprx-0103)
- [REQ-SANDBX-0130](../requirements/sandbx.md#req-sandbx-0130)

## Threat Model and Assumptions

- Spec ID: `CYNAI.WEBPRX.ThreatModel.WebEgressProxy` <a id="spec-cynai-webprx-threatmodel-webegressproxy"></a>

Assumptions:

- Sandbox code is untrusted.
- Sandboxes are egress-restricted by default.
  When egress is permitted, it is only via the Web Egress Proxy (and node-local proxy); sandboxes are not airgapped but have strict egress controls.
- Sandboxes must not hold long-lived secrets.
- Sandboxes can generate arbitrary HTTP requests when a proxy is available.

Threats this service is explicitly designed to reduce:

- Data exfiltration from sandboxes to arbitrary internet hosts.
- SSRF-style access from sandboxes into private networks via crafted destinations, redirects, or DNS rebinding.
- Unattributed or un-audited outbound traffic during task execution.

Threats this service does not fully eliminate:

- Exfiltration via allowlisted destinations, including steganography within allowed protocols.
- Dependency-based attacks (malicious packages) when an allowlisted registry is used.

Traces To:

- [REQ-WEBPRX-0101](../requirements/webprx.md#req-webprx-0101)
- [REQ-WEBPRX-0102](../requirements/webprx.md#req-webprx-0102)

## Architecture and Data Flow

- Spec ID: `CYNAI.WEBPRX.Architecture.WebEgressProxy` <a id="spec-cynai-webprx-architecture-webegressproxy"></a>

High-level model:

- The sandbox talks only to a node-local proxy endpoint.
- The node-local proxy forwards requests to an orchestrator-owned Web Egress Proxy service.
- The orchestrator service enforces allowlist and policy, performs DNS and private-network protections, and emits audit records.
- The orchestrator service proxies the allowed request to the destination and streams the response back.

Rationale:

- A node-local endpoint avoids requiring sandbox authentication and avoids embedding proxy credentials in sandbox config.
- The orchestrator remains the central policy and audit point.

Notes:

- This spec intentionally does not constrain whether the node-local proxy is a process, a container sidecar, or a built-in Worker API feature.
- This spec also does not constrain the exact transport between the node-local proxy and the orchestrator, as long as it is authenticated and auditable.

Traces To:

- [REQ-WEBPRX-0100](../requirements/webprx.md#req-webprx-0100)
- [REQ-WEBPRX-0103](../requirements/webprx.md#req-webprx-0103)

## Policy Model and Allowlisting

- Spec ID: `CYNAI.WEBPRX.Policy.WebEgressProxy` <a id="spec-cynai-webprx-policy-webegressproxy"></a>

The Web Egress Proxy applies a default-deny policy over outbound HTTP(S) destinations.
Policy is evaluated per request using the request destination and the calling task context.

### Destination Normalization

The proxy normalizes every request into a destination tuple:

- `scheme`: `http` or `https`
- `host`: a DNS hostname (not an IP literal)
- `port`: integer

Rules:

- IP-literal hosts are rejected.
- `https` requests are proxied using the HTTP `CONNECT` method.
- Only ports `80` and `443` are eligible unless the allowlist entry explicitly permits a non-standard port.

### Allowlist Entry Model

An allowlist entry is evaluated against the normalized destination.

Recommended fields:

- `scope`:
  - `system` (deployment-wide)
  - `project` (by `project_id`)
  - `task` (by `task_id`)
- `host`:
  - exact hostname match (recommended default)
  - optional constrained wildcard of the form `*.example.com` (discouraged)
- `ports`:
  - optional list of allowed ports.
  - default: allow only `443` for `https` and `80` for `http`.
- `ttl_seconds`:
  - optional.
  - when present, the entry expires and must be treated as absent after TTL elapses.
- `created_by` and `reason` for audit and operator review.

Recommended evaluation:

- Resolve an effective allowlist from the applicable scopes.
- Treat the effective allowlist as an allow union, not an overwrite.
- Apply a strict host match first, then optional wildcard match.

### Recommended Default Allowlist for Dependency Downloads

Operators should prefer a small, curated default allowlist that supports common build workflows.
This reduces the need for dynamic temporary exceptions.

Recommended starting set:

- Go module proxy:
  - `proxy.golang.org`
  - `sum.golang.org`
- Python packages:
  - `pypi.org`
  - `files.pythonhosted.org`

Notes:

- Direct VCS fetches (for example Go fetching from arbitrary Git hosts) should be avoided in favor of registry or proxy patterns.
- Git remote operations remain governed by Git egress.
  See [`docs/tech_specs/git_egress_mcp.md`](git_egress_mcp.md).

### DNS and Private Network Protections

The proxy prevents access to private or local networks even if the hostname appears public.

Recommended checks:

- Resolve `host` and reject any A/AAAA result in private, loopback, link-local, multicast, or otherwise reserved ranges.
- Reject destinations that resolve to multiple IPs if any resolved IP is private or reserved.
- Enforce that redirects (including `Location` and 30x chains) are separately allowlisted under the same rules.

Traces To:

- [REQ-WEBPRX-0101](../requirements/webprx.md#req-webprx-0101)
- [REQ-WEBPRX-0102](../requirements/webprx.md#req-webprx-0102)
- [REQ-WEBPRX-0106](../requirements/webprx.md#req-webprx-0106)

## Dynamic Temporary Allowlist Exceptions

- Spec ID: `CYNAI.WEBPRX.DynamicAllowlist.WebEgressProxy` <a id="spec-cynai-webprx-dynamicallowlist-webegressproxy"></a>

Some builds require contacting additional hosts not present in the static allowlist.
The system supports dynamic, temporary allowlist entries scoped to a task, with strict constraints.

### Intended Use Cases

- A task builds a Go project and needs access to `proxy.golang.org` and `sum.golang.org`.
- A task builds a Python project and needs access to `pypi.org` and `files.pythonhosted.org`.

### Dynamic Allowlist Request Model

A dynamic allowlist request is a structured record containing at least:

- `task_id`
- `requested_hosts` (array of hostnames)
- `requested_ports` (optional; default to `443` and `80` only)
- `ttl_seconds`
- `reason`

Recommended strict defaults:

- Maximum TTL is short and bounded (for example 15 minutes).
- Only exact hostnames are permitted by default.
- Wildcards are denied by default.
- Requests for non-HTTP(S) schemes are denied.
- Requests for non-standard ports are denied by default.

### Who Can Request Dynamic Entries

Dynamic allowlist requests may be initiated by the Project Manager Agent as part of task execution planning.
Dynamic allowlist requests are not automatically approved by virtue of being initiated by an agent.

Recommended approval policy:

- Default deny unless an explicit preference or policy enables dynamic allowlist approval for the task or project.
- Require a human-visible audit record including reason, duration, and the exact hosts.
- When enabled, approve only entries that satisfy strict host and network constraints.

Traces To:

- [REQ-WEBPRX-0104](../requirements/webprx.md#req-webprx-0104)
- [REQ-WEBPRX-0105](../requirements/webprx.md#req-webprx-0105)

## Proxy Protocol and Client Compatibility

- Spec ID: `CYNAI.WEBPRX.Protocol.WebEgressProxy` <a id="spec-cynai-webprx-protocol-webegressproxy"></a>

The node-local proxy endpoint should be compatible with common tooling.
Compatibility should include:

- HTTP proxy semantics for `http://` URLs (absolute-form requests).
- HTTPS via `CONNECT` tunneling.
- Standard proxy environment variables for sandbox tooling (`HTTP_PROXY`, `HTTPS_PROXY`, and `NO_PROXY`).

Notes:

- The proxy does not need to implement content rewriting.
- The proxy should avoid requiring proxy authentication from inside the sandbox.

Traces To:

- [REQ-WEBPRX-0100](../requirements/webprx.md#req-webprx-0100)

## Node and Sandbox Integration

- Spec ID: `CYNAI.SANDBX.Integration.WebEgressProxy` <a id="spec-cynai-sandbx-integration-webegressproxy"></a>

Recommended sandbox behavior:

- Sandboxes should be configured to use the node-local proxy endpoint via proxy environment variables.
- Sandboxes should not be able to bypass the proxy to reach the internet directly.

Recommended node behavior:

- The Node Manager should set a restricted sandbox network policy by default.
- When a sandbox network policy allows allowlisted egress, the node should:
  - allow connectivity only to the node-local proxy endpoint, and
  - deny direct connections to external destinations from sandbox network namespaces.

Node configuration integration:

- The node startup configuration includes `sandbox.allowed_egress_domains` for allowlist-based policy.
  See [`docs/tech_specs/worker_node.md`](worker_node.md).

Traces To:

- [REQ-SANDBX-0101](../requirements/sandbx.md#req-sandbx-0101)
- [REQ-SANDBX-0112](../requirements/sandbx.md#req-sandbx-0112)
- [REQ-SANDBX-0130](../requirements/sandbx.md#req-sandbx-0130)

## Auditing and Observability

- Spec ID: `CYNAI.WEBPRX.Auditing.WebEgressProxy` <a id="spec-cynai-webprx-auditing-webegressproxy"></a>

The Web Egress Proxy emits audit records for all allow and deny decisions.

Recommended audit fields:

- `task_id` and `job_id` (when available)
- `subject` (user identity and agent identity when applicable)
- `destination_host`, `destination_port`, `scheme`
- `decision` (allow or deny)
- `reason_code` (machine-readable)
- `bytes_sent` and `bytes_received` (best-effort)
- `created_at`

Observability notes:

- Deny decisions should be observable to the task as actionable errors that indicate which destination was blocked.
- Audit log volume can be high for package downloads.
  Implementations may aggregate by destination and time window as long as allow and deny decisions remain auditable.

Traces To:

- [REQ-WEBPRX-0103](../requirements/webprx.md#req-webprx-0103)

## Non-Goals

This service is not intended to:

- Provide general web browsing or sanitization.
  Use the Secure Browser Service instead.
- Provide credentialed API operations.
  Use the API Egress Server instead.
- Allow arbitrary, unbounded internet access from sandboxes.
  All egress is constrained by allowlists and policy.
