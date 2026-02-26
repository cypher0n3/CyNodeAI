# Zero-Trust Security Assessment: Tech Specs Gap Analysis

- [1. Executive Summary](#1-executive-summary)
- [2. Zero-Trust Lens Used](#2-zero-trust-lens-used)
- [3. Identity and Authentication Gaps](#3-identity-and-authentication-gaps)
- [4. Transport and Network Security Gaps](#4-transport-and-network-security-gaps)
- [5. Access Control and Authorization Gaps](#5-access-control-and-authorization-gaps)
- [6. Data Protection Gaps](#6-data-protection-gaps)
- [7. Monitoring, Audit, and Assume-Breach Gaps](#7-monitoring-audit-and-assume-breach-gaps)
- [8. Summary of Recommended Spec Changes](#8-summary-of-recommended-spec-changes)
- [9. Alignment with Worker API HTTPS Spec Updates](#9-alignment-with-worker-api-https-spec-updates)
- [10. References](#10-references)

## 1. Executive Summary

This report assesses CyNodeAI technical specifications against zero-trust principles: verify explicitly, least-privilege access, and assume breach.
The specs already align with zero-trust in several areas (default-deny policy, sandbox isolation, credential isolation from sandboxes, RBAC and access control rules, audit logging).
Significant gaps remain in **transport security**, **workload/node identity**, **continuous verification**, and **explicit segmentation**.
Remediations below are framed as changes to the tech specs (and, where appropriate, to requirements) so that implementations can be zero-trust aligned by design.
Document type: security assessment (docs-only).
Date: 2026-02-26.
Scope: technical specifications in [`docs/tech_specs/`](../tech_specs/) assessed against a zero-trust security model.
Audience: security engineers, tech spec authors, and implementers.

---

## 2. Zero-Trust Lens Used

Assessment is based on:

- **Verify explicitly:** Every request is authenticated and authorized; no implicit trust by network location.
- **Least privilege:** Access is limited by identity, context, and time; just-in-time and attribute-based where applicable.
- **Assume breach:** Minimize blast radius via segmentation, encryption, and monitoring; no component is trusted by default.

References: NIST SP 800-207 (Zero Trust Architecture), CISA zero-trust maturity model, and common practice for control-plane and worker-node architectures.

---

## 3. Identity and Authentication Gaps

This section lists gaps in identity and authentication across worker, node, and user flows.

### 3.1 Worker and Component Identity

Worker API and orchestrator-to-node traffic rely on a **static bearer token** delivered via node configuration (no refresh, rotation, or certificate-based workload identity) for the **initial implementation only**.
This is documented in the spec as a temporary Phase 1 / early MVP constraint and is intended to be replaced later (e.g. short-lived tokens or mTLS).

- [`docs/tech_specs/worker_api.md`](../tech_specs/worker_api.md): under "Initial implementation (Phase 1) constraints", "Bearer token is static (delivered via node config; no refresh)."
- No spec yet defines the future workload identity model (e.g. SPIFFE/SPIRE, node attestation, or short-lived credentials) for nodes or internal services.

Long-lived static tokens increase credential theft and replay risk for the duration of Phase 1.
Compromise of one token can be reused until manually rotated.

#### 3.1.1 Remediation - Worker and Component Identity

- Add a dedicated subsection in [`worker_api.md`](../tech_specs/worker_api.md) (or a new identity spec) that:
  - Requires or recommends **short-lived tokens** or certificate-based authentication for orchestrator-to-node calls, with a defined refresh/rotation mechanism.
  - Defines **node identity** (e.g. node ID + attestation or certificate CN/SAN) and how it is bound to the token or TLS client cert.
- In [`worker_node_payloads.md`](../tech_specs/worker_node_payloads.md) and [`worker_node.md`](../tech_specs/worker_node.md), extend node registration/config to support token refresh or mTLS client identity and document lifecycle (issuance, expiry, revocation).

### 3.2 Bootstrap and First-Use Identity

Bootstrap is one-time (bootstrap token, localhost endpoint, or bootstrap file).
There is no spec requirement for **continuous verification** of node identity after registration (e.g. re-auth or re-attestation on a schedule or on config fetch).

- [`docs/tech_specs/local_user_accounts.md`](../tech_specs/local_user_accounts.md): bootstrap mechanisms are one-time; no ongoing node re-verification.
- [`docs/tech_specs/worker_node.md`](../tech_specs/worker_node.md): registration and config delivery do not mandate periodic re-authentication of the node.

#### 3.2.1 Remediation - Bootstrap and First-Use Identity

- In [`worker_node.md`](../tech_specs/worker_node.md) (Registration and Bootstrap / Configuration Delivery), add:
  - A requirement or recommendation that the orchestrator **re-verifies node identity** when delivering config (e.g. via the same credential used for Worker API, or via a dedicated config-auth mechanism).
  - Optional: periodic re-registration or heartbeat that includes identity proof, with defined interval and failure behavior (e.g. mark node non-dispatchable after missed heartbeats).

### 3.3 User and Session Identity

User API Gateway and local user accounts specify authentication and session issuance but do not explicitly state **per-request verification**.
That is: every API request should be authenticated and session/token validated on each call, not only at session creation.

- [`docs/tech_specs/user_api_gateway.md`](../tech_specs/user_api_gateway.md) and [`docs/tech_specs/local_user_accounts.md`](../tech_specs/local_user_accounts.md) describe auth and sessions but do not prescribe "verify every request."

#### 3.3.1 Remediation - User and Session Identity

- In [`user_api_gateway.md`](../tech_specs/user_api_gateway.md) (Authentication and Auditing), add an explicit rule: **every user-facing request MUST be authenticated and authorized**; no endpoint may rely on network location or previous request for identity.
- In [`local_user_accounts.md`](../tech_specs/local_user_accounts.md), add a short subsection stating that access tokens are validated on each request and that revoked or expired tokens MUST be rejected (aligns with existing refresh/session revocation).

---

## 4. Transport and Network Security Gaps

This section lists gaps in transport encryption and network segmentation.

### 4.1 In-Transit Encryption (Component-to-Component)

Worker API spec explicitly allows **HTTP (no HTTPS) for MVP** for orchestrator-to-node traffic.

- [`docs/tech_specs/worker_api.md`](../tech_specs/worker_api.md): "Component-to-component traffic MUST support HTTP (not HTTPS) for MVP.
  HTTPS MAY be added later."

Unencrypted control and job payloads are vulnerable to eavesdropping and tampering on the path between orchestrator and node.
This contradicts assume-breach and defense in depth.

#### 4.1.1 Remediation - In-Transit Encryption

- In [`worker_api.md`](../tech_specs/worker_api.md):
  - Replace or qualify the MVP constraint: e.g. "For production deployments, component-to-component traffic MUST use TLS (HTTPS or equivalent).
    MVP MAY allow HTTP only when explicitly configured for local/dev and when the deployment documentation states that HTTP is not acceptable for production."
  - Add a **Transport security** subsection: require TLS 1.2+ for production; document that certificate validation (server and, if used, client) is required when TLS is enabled.
- In [`ports_and_endpoints.md`](../tech_specs/ports_and_endpoints.md), add a note that production deployments MUST use TLS for User API Gateway, Control-plane, Worker API, MCP Gateway, and API Egress (and reference the worker_api and gateway specs).

### 4.2 Mutual TLS and Server Identity

Node startup YAML allows optional TLS for orchestrator URL (`orchestrator.tls.ca_bundle_path`, `pinned_sha256`).
There is **no spec requirement for TLS for the Worker API** (orchestrator calling node) or for **mTLS** (node or orchestrator proving identity via client certificates).

- [`docs/tech_specs/worker_node.md`](../tech_specs/worker_node.md): node can validate orchestrator TLS; Worker API listen host/port and public URL are configurable but TLS is not mandated.
- No spec defines when mTLS is required (e.g. control-plane to node, or node to orchestrator config endpoint).

#### 4.2.1 Remediation - Mutual TLS and Server Identity

- In [`worker_api.md`](../tech_specs/worker_api.md) and [`worker_node.md`](../tech_specs/worker_node.md):
  - Specify that **production** Worker API MUST be served over TLS with server certificate validation by the orchestrator.
  - Add an optional (or future) subsection for **mTLS**: when enabled, the node presents a client certificate for Worker API or config requests; orchestrator validates node identity from the certificate (e.g. CN or SAN tied to `node.id` or registration record).
- In [`worker_node_payloads.md`](../tech_specs/worker_node_payloads.md), ensure capability report and config payload can carry TLS/mTLS capability and trust material status so the orchestrator can adapt (already partially present; make TLS requirement explicit where applicable).

### 4.3 Network Segmentation and Micro-Segmentation

Ports and endpoints are documented, but **no spec describes network segmentation** (e.g. control-plane, user-gateway, and worker traffic on distinct VLANs or security zones) or **micro-segmentation** (e.g. which components may initiate connections to which, and which ports).

- [`docs/tech_specs/ports_and_endpoints.md`](../tech_specs/ports_and_endpoints.md) and [`docs/tech_specs/orchestrator.md`](../tech_specs/orchestrator.md) describe what listens where but not trust boundaries or allowed flows.

#### 4.3.1 Remediation - Network Segmentation

- Add a new subsection in [`ports_and_endpoints.md`](../tech_specs/ports_and_endpoints.md) (or a short "Network segmentation" note in [`orchestrator.md`](../tech_specs/orchestrator.md)):
  - Define **logical trust zones**: e.g. User-facing (User API Gateway), Control (control-plane, MCP Gateway, API Egress), Worker (Worker API), Data (PostgreSQL).
  - Recommend or require that deployments place components in zones and restrict **initiated connections** (e.g. only control-plane initiates to Worker API; only user-gateway and control-plane to PostgreSQL; no direct user traffic to control-plane).
  - State that internal service-to-service calls MUST authenticate (e.g. bearer or mTLS) and MUST NOT rely solely on network topology for trust.

---

## 5. Access Control and Authorization Gaps

This section lists gaps in continuous authorization, least privilege, and service-to-service authorization.

### 5.1 Continuous Authorization and Context

Access control and RBAC specify policy evaluation and default-deny, but do not explicitly require **re-evaluation on every request** with full context (user, task, resource, time) or **binding of authorization to the specific operation** (e.g. this exact tool call, this exact credential use).

- [`docs/tech_specs/access_control.md`](../tech_specs/access_control.md): policy evaluation order and default-deny are specified; "every request" and "per-operation" are not spelled out.

#### 5.1.1 Remediation - Continuous Authorization

- In [`access_control.md`](../tech_specs/access_control.md) (Policy Evaluation):
  - Add an explicit rule: **Policy MUST be evaluated for every distinct request** (every API call, every MCP tool invocation, every egress request); no caching of allow decisions across requests without re-evaluating identity and context.
  - State that evaluation MUST use the **current** subject, action, resource, and context (e.g. task_id, project_id, time); any change in context requires re-evaluation.

### 5.2 Least-Privilege and Scoping

MCP gateway and edge enforcement use allowlists and capability leases.
The **principle of least privilege** is not explicitly stated in the access control or MCP gateway specs (e.g. grants should be minimal in scope and time).

- [`docs/tech_specs/mcp_gateway_enforcement.md`](../tech_specs/mcp_gateway_enforcement.md): capability leases are short-lived and least-privilege; role allowlists are coarse; no overarching "least privilege" requirement in access_control or rbac_and_groups.

#### 5.2.1 Remediation - Least Privilege

- In [`access_control.md`](../tech_specs/access_control.md) (Core Concepts or Policy Evaluation):
  - Add: **Access MUST be granted with minimum necessary scope** (subject, action, resource, and time); default-deny is the baseline; allow rules SHOULD be as specific as possible (e.g. by resource pattern, task, or time window).
- In [`mcp_gateway_enforcement.md`](../tech_specs/mcp_gateway_enforcement.md): retain and emphasize that capability leases MUST be short-lived and least-privilege; add that tool allowlists SHOULD expose only the minimal set of tools required for the agent role.

### 5.3 Service-to-Service Authorization

Control-plane to Worker API uses a bearer token.
There is **no spec for authorization of other internal calls** (e.g. user-gateway to control-plane, MCP gateway to API Egress, or workflow runner to orchestrator) beyond "authenticated."

- [`docs/tech_specs/orchestrator.md`](../tech_specs/orchestrator.md) and related specs describe who calls whom but not how each service proves identity and what authorization (if any) is applied (e.g. only control-plane may call job dispatch).

#### 5.3.1 Remediation - Service-to-Service Authorization

- In [`orchestrator.md`](../tech_specs/orchestrator.md) or a new "Internal service authentication" subsection (or in [`ports_and_endpoints.md`](../tech_specs/ports_and_endpoints.md)):
  - List **internal call paths** (e.g. user-gateway -> control-plane, control-plane -> Worker API, MCP gateway -> API Egress).
  - For each, specify: authentication mechanism (e.g. bearer, mTLS) and that the **caller identity** (service or role) MUST be verified and that the **callee** SHOULD authorize the caller for the requested operation (e.g. only control-plane may invoke job run).

---

## 6. Data Protection Gaps

This section lists gaps in encryption at rest and handling of secrets in logs and errors.

### 6.1 Encryption at Rest

Credentials and secrets are specified as encrypted (envelope encryption, credential_ciphertext).
**PostgreSQL and general data-at-rest encryption** are not specified in the schema or orchestrator docs.

- [`docs/tech_specs/postgres_schema.md`](../tech_specs/postgres_schema.md) and [`docs/tech_specs/api_egress_server.md`](../tech_specs/api_egress_server.md): credential storage is encrypted; no requirement for TDE or disk encryption for the database.

#### 6.1.1 Remediation - Encryption at Rest

- In [`postgres_schema.md`](../tech_specs/postgres_schema.md) (Goals and Scope or a new "Security and encryption" subsection):
  - Add: **Sensitive data at rest:** Credentials and secret material MUST be stored in encrypted form (as already specified); for production deployments, **database encryption at rest** (e.g. PostgreSQL TDE or volume-level encryption) SHOULD be used and is out of scope of this schema spec but RECOMMENDED in deployment guidance.
- Optionally add a deployment or operations doc reference (or link from [`orchestrator_bootstrap.md`](../tech_specs/orchestrator_bootstrap.md)) that recommends full-disk or volume encryption for orchestrator and node hosts.

### 6.2 Secrets in Logs and Errors

Multiple specs say "do not log secrets" or "do not leak secrets in errors."
There is **no single canonical place** that enumerates all secret types and mandates redaction or exclusion from logs, audit, and error responses.

- [`docs/tech_specs/worker_api.md`](../tech_specs/worker_api.md), [`docs/tech_specs/local_user_accounts.md`](../tech_specs/local_user_accounts.md), and [`docs/tech_specs/go_rest_api_standards.md`](../tech_specs/go_rest_api_standards.md) mention it in passing.

#### 6.2.1 Remediation - Secrets in Logs and Errors

- In [`go_rest_api_standards.md`](../tech_specs/go_rest_api_standards.md) (or a new "Secrets handling" subsection in [`access_control.md`](../tech_specs/access_control.md)):
  - Add a **Secrets and logging** rule: tokens, passwords, API keys, and credential_ciphertext MUST NOT appear in logs, audit payloads (except hashed or redacted identifiers), or error responses; implementations MUST redact or omit such fields when serializing requests/responses for logging.

---

## 7. Monitoring, Audit, and Assume-Breach Gaps

This section lists gaps in audit integrity and trust boundaries.

### 7.1 Audit Coverage and Integrity

Auth, MCP tool calls, and access control decisions are audited.
**No spec requires audit record integrity** (e.g. tamper-evident logging or append-only store) or **correlation IDs** across orchestrator, node, and egress for full traceability.

- [`docs/tech_specs/access_control.md`](../tech_specs/access_control.md), [`docs/tech_specs/postgres_schema.md`](../tech_specs/postgres_schema.md): audit tables and events are defined; integrity and correlation are not.

#### 7.1.1 Remediation - Audit Coverage and Integrity

- In [`postgres_schema.md`](../tech_specs/postgres_schema.md) (Audit Logging):
  - Add: Audit tables SHOULD be treated as **append-only** for compliance and assume-breach; implementations SHOULD avoid updates/deletes of audit rows.
  - Recommend a **request or trace ID** that is propagated from gateway through MCP, API Egress, and Worker API so that all audit entries for a single user request can be correlated (reference from auth_audit_log, mcp_tool_call_audit_log, access_control_audit_log).
- In [`mcp_tool_call_auditing.md`](../tech_specs/mcp_tool_call_auditing.md) (if it exists) or in [`mcp_gateway_enforcement.md`](../tech_specs/mcp_gateway_enforcement.md): require that audit records include a stable request/trace identifier when available.

### 7.2 Blast Radius and Segment Boundaries

Sandboxes are correctly treated as untrusted and network-restricted.
**Orchestrator internal components** (control-plane, user-gateway, API Egress, MCP Gateway) are not explicitly described as **mutually untrusted** or segmented from each other.

- [`docs/tech_specs/sandbox_container.md`](../tech_specs/sandbox_container.md) and [`docs/tech_specs/worker_node.md`](../tech_specs/worker_node.md): sandbox threat model is clear; orchestrator internal trust boundaries are not.

#### 7.2.1 Remediation - Blast Radius and Segment Boundaries

- In [`orchestrator.md`](../tech_specs/orchestrator.md) (Core Responsibilities or a new "Trust boundaries" subsection):
  - State that **orchestrator sub-services** (control-plane, user-gateway, MCP Gateway, API Egress, Web Egress Proxy) SHOULD be treated as **separate trust boundaries**; each MUST authenticate callers and MUST NOT rely on co-location or shared process for authorization.
  - Recommend that production deployments run them in separate processes or containers with minimal required network access between them (align with the segmentation note in Section 4.3).

---

## 8. Summary of Recommended Spec Changes

- Worker/node identity - Spec(s): worker_api, worker_node, worker_node_payloads.
  Change: short-lived or cert-based node auth; node identity binding; optional mTLS.
- Bootstrap - Spec(s): worker_node, local_user_accounts.
  Change: re-verification of node identity on config fetch or periodically.
- User auth - Spec(s): user_api_gateway, local_user_accounts.
  Change: explicit per-request authentication and authorization.
- Transport - Spec(s): worker_api, ports_and_endpoints.
  Change: TLS required for production; TLS subsection; qualify MVP HTTP.
- Mutual TLS - Spec(s): worker_api, worker_node.
  Change: production TLS for Worker API; optional mTLS and node identity from cert.
- Segmentation - Spec(s): ports_and_endpoints, orchestrator.
  Change: logical zones; allowed initiator/callee matrix; no trust by location.
- Continuous authz - Spec(s): access_control.
  Change: policy evaluated per request with current context; no cross-request caching of allow.
- Least privilege - Spec(s): access_control, mcp_gateway_enforcement.
  Change: explicit least-privilege and minimal scope.
- Service-to-service - Spec(s): orchestrator, ports_and_endpoints.
  Change: internal call paths; auth and caller authorization.
- Data at rest - Spec(s): postgres_schema, orchestrator_bootstrap.
  Change: recommend DB and volume encryption for production.
- Secrets in logs - Spec(s): go_rest_api_standards or access_control.
  Change: canonical rule: no secrets in logs/audit/errors.
- Audit - Spec(s): postgres_schema, mcp_gateway_enforcement / mcp_tool_call_auditing.
  Change: append-only; request/trace ID for correlation.
- Trust boundaries - Spec(s): orchestrator.
  Change: orchestrator sub-services as separate boundaries; authenticate and authorize.

---

## 9. Alignment With Worker API HTTPS Spec Updates

The following spec changes were made after this assessment and align with Section 4 (Transport and Network Security):

- **worker_api.md** - New section [HTTPS Transport and Reverse Proxy](../tech_specs/worker_api.md#spec-cynai-worker-httpstransportreverseproxy): Worker API MAY be deployed behind a containerized nginx reverse proxy for HTTPS; when using HTTPS the orchestrator MUST validate the server certificate (except dev insecure skip).
  When using a self-signed cert, the initial registration data (capability report) MUST include the worker's TLS server certificate or public key so the orchestrator can trust the worker for subsequent connections.
- **worker_node_payloads.md** - Capability report `tls` object now includes optional `worker_api_server_cert_pem`: the node sends the Worker API server cert PEM at registration when serving over HTTPS with a self-signed certificate, so the orchestrator can pin/trust it.

Alignment with assessment recommendations:

- **4.1.1 (In-Transit Encryption):** Partially addressed.
  The spec now defines HTTPS deployment and mandatory server certificate validation when HTTPS is used.
  The existing MVP constraint ("MUST support HTTP for MVP") is unchanged; the assessment recommended qualifying it so production MUST use TLS and HTTP is dev/local only.
  That qualification is not yet in the spec.
- **4.2.1 (Mutual TLS and Server Identity):** Partially addressed.
  Server certificate validation by the orchestrator is specified; the capability report carries Worker API trust material (`worker_api_server_cert_pem`) for self-signed deployments.
  Not yet added: an explicit "production Worker API MUST be served over TLS" sentence, or the optional mTLS (client cert) subsection.

Remaining gaps from Section 4 for worker/orchestrator transport:

- Explicit production TLS requirement and MVP HTTP qualified as dev-only (4.1.1).
- Optional mTLS subsection and node identity from client cert (4.2.1).
- ports_and_endpoints note that production MUST use TLS for Worker API (4.1.1).

---

## 10. References

- Tech specs index: [`docs/tech_specs/_main.md`](../tech_specs/_main.md).
- Access control: [`docs/tech_specs/access_control.md`](../tech_specs/access_control.md).
- RBAC: [`docs/tech_specs/rbac_and_groups.md`](../tech_specs/rbac_and_groups.md).
- Local user accounts: [`docs/tech_specs/local_user_accounts.md`](../tech_specs/local_user_accounts.md).
- Worker API: [`docs/tech_specs/worker_api.md`](../tech_specs/worker_api.md).
- Worker node: [`docs/tech_specs/worker_node.md`](../tech_specs/worker_node.md).
- Ports and endpoints: [`docs/tech_specs/ports_and_endpoints.md`](../tech_specs/ports_and_endpoints.md).
- Orchestrator: [`docs/tech_specs/orchestrator.md`](../tech_specs/orchestrator.md).
- MCP gateway enforcement: [`docs/tech_specs/mcp_gateway_enforcement.md`](../tech_specs/mcp_gateway_enforcement.md).
- Postgres schema: [`docs/tech_specs/postgres_schema.md`](../tech_specs/postgres_schema.md).
- API Egress: [`docs/tech_specs/api_egress_server.md`](../tech_specs/api_egress_server.md).
- Sandbox: [`docs/tech_specs/sandbox_container.md`](../tech_specs/sandbox_container.md).
