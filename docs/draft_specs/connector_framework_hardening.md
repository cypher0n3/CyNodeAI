# Connector Framework Hardening and Gap Closure Proposal

## 1. Overview

- Document Type: Technical Specification Amendment
- Applies To: Connector Framework
- Supersedes: None
- Purpose: Close architectural, security, operational, and governance gaps identified in review

## 2. Execution and Invocation Model

This section defines how connector operations are executed and invoked.

### 2.1 Operation Execution Modes

All connector operations MUST declare an execution mode:

- sync - blocking request/response
- async - job-based execution with result retrieval

Connector types MUST specify which operations support which mode.

#### 2.1.1 Operation Requirements

- All operations MUST define:

  - max_timeout_ms
  - supports_idempotency (boolean)
  - retry_policy (transient only)
- Orchestrator MUST enforce timeout caps.
- Long-running operations MUST be async.

---

### 2.2 Async Job Model

Async operations MUST:

- Create a job record
- Emit state transitions:

  - pending
  - running
  - succeeded
  - failed
  - canceled
- Support result retrieval via job_id
- Enforce max result payload size

Dead-letter queue handling MUST be defined for repeated failure.

---

### 2.3 Idempotency and Replay Protection

All mutating operations MUST:

- Accept an idempotency_key
- Persist idempotency records for a configurable TTL
- Reject duplicate keys within TTL
- Return original result for duplicate calls

Inbound connectors MUST:

- Enforce webhook signature validation
- Enforce replay window (configurable, default 5 minutes)

---

## 3. Connector Versioning and Schema Control

Connector types and their config and operation schemas are versioned and validated.

### 3.1 Connector Type Versioning

Connector types MUST declare:

- connector_type
- connector_type_version
- operation_schema_version

Backward compatibility policy MUST be defined:

- Minor versions additive only
- Major versions may break schema

---

### 3.2 Config Schema Validation

Each connector type MUST provide:

- Versioned JSON Schema for config
- Validation at:

  - install
  - update
  - enable

Invalid configs MUST prevent enablement.

---

### 3.3 Operation Input/Output Schemas

Each operation MUST define:

- Request schema
- Response schema
- Max payload size
- Redaction rules

Responses MUST be schema-validated before returning to agents or users.

---

## 4. Expanded Data Model

Connector instance schema MUST be extended with:

- status (enum):

  - provisioning
  - active
  - degraded
  - error
  - credential_expired
  - disabled
- last_success_at
- last_error
- health_checked_at
- connector_type_version
- config_schema_version
- rate_limit_policy

---

## 5. Failure and Retry Model

Failures are classified, retried according to policy, and guarded by circuit breakers.

### 5.1 Failure Classification

All failures MUST be classified:

- transient (retryable)
- permanent (non-retryable)
- policy_denied
- quota_exceeded
- validation_error

---

### 5.2 Retry Policy

Connector types MUST define:

- max_retries
- backoff strategy (exponential required)
- retryable status codes

Global retry cap MUST be enforced.

---

### 5.3 Circuit Breaker

Connector instances MUST implement:

- Failure threshold
- Automatic degradation state
- Cooldown period
- Health check reactivation

---

## 6. Rate Limiting and Abuse Protection

The framework MUST support:

- Per-user invocation limits
- Per-project invocation limits
- Per-connector instance limits
- Burst and sustained rate policies

429 responses from providers MUST:

- Update quota state
- Trigger backoff

---

## 7. Egress and Network Isolation

Connector execution boundary MUST enforce:

- Per-connector egress allowlist
- DNS restriction
- Optional mTLS
- TLS certificate validation enforcement

OpenClaw connectors MUST execute in:

- Separate process
- Resource-limited environment
- Network-restricted boundary

---

## 8. Credential Security Model

Credential handling MUST:

- Use envelope encryption
- Integrate with KMS
- Decrypt only in connector boundary
- Zero memory after operation completion

Workers and agent sandboxes MUST NEVER receive credentials.

Credential rotation MUST:

- Invalidate previous tokens
- Trigger health validation
- Record audit event

---

## 9. Policy Model Enhancements

The policy model is expanded with a finer-grained action taxonomy and evaluation context.

### 9.1 Expanded Action Taxonomy

Replace coarse model with:

- connector.catalog.read
- connector.instance.manage
- connector.operation.invoke
- connector.operation.read
- connector.credential.manage

---

### 9.2 Policy Evaluation Context

Policy engine MUST receive:

- user_id
- group_id
- project_id
- connector_instance_id
- connector_type
- operation_name
- execution_mode

---

## 10. Observability and Audit Model

Structured audit events and metrics are required for operations and connector types.

### 10.1 Audit Event Schema

Each operation MUST emit structured audit events containing:

- request_id
- correlation_id
- user_id
- project_id
- connector_instance_id
- operation
- execution_mode
- outcome
- duration_ms
- provider_status_code
- retry_count

---

### 10.2 Metrics and SLOs

System MUST collect:

- invocation_count
- error_rate
- latency_percentiles
- retry_rate
- quota_exceeded_count
- circuit_breaker_trips

SLIs and SLOs MUST be defined per connector type.

---

## 11. Inbound Connector Model

Inbound connectors MUST define:

- Subscription lifecycle
- Webhook endpoint
- Signature verification
- Replay protection
- Event deduplication
- Internal canonical event format

Polling connectors MUST define:

- Poll interval
- Backoff strategy
- Cursor storage model

---

## 12. Multi-Tenant Isolation Rules

Rules MUST define:

- Whether group-owned connectors are project-scoped
- Whether project override is allowed
- Cross-project visibility rules
- Hard boundary enforcement

Deletion semantics MUST specify:

- Credential wipe on deletion
- Soft delete retention period
- Audit retention policy

---

## 13. Governance and Catalog Control

Connector catalog MUST support:

- Signed connector bundles
- Approval workflow for new connector types
- Feature flagging
- Deprecation lifecycle
- Security review requirement

OpenClaw connectors MUST undergo:

- Supply chain scan
- Dependency review
- Sandbox enforcement verification

---

## 14. Concurrency and Backpressure

Connector instances MUST define:

- max_parallel_invocations
- queue depth cap
- Backpressure behavior (reject vs queue)

Global concurrency guard MUST exist.

---

## 15. Testing and Certification Requirements

Each connector type MUST provide:

- Schema validation tests
- Contract tests for operations
- Failure simulation tests
- Load tests
- Fuzz testing of response normalization
- Security validation checklist

Certification MUST be required before general availability.

---

## 16. Canonical Normalized Response Model

All connector responses MUST be normalized into:

- metadata
- data
- warnings
- redactions_applied
- truncated (boolean)

Max response size MUST be enforced globally.

---

## 17. Required Spec Additions

The Connector Framework specification MUST be updated to include new Spec IDs covering:

- Execution Model
- Retry Model
- Idempotency
- Versioning
- Schema Validation
- Rate Limiting
- Egress Controls
- Observability
- Inbound Model
- Governance
- Certification

Each MUST trace to formal requirements documents.

---

## 18. Conclusion

These updates elevate the Connector Framework from:

- Conceptually secure

to:

- Operationally hardened
- Multi-tenant safe
- Failure-resilient
- Scalable
- Governable
- Certifiable

If desired, the next step can be:

- A full rewritten v2 spec integrating these sections inline
- A requirements matrix mapping new spec sections to requirement IDs
- A threat model document for connectors
- A reference architecture diagram with execution boundaries

## 19. Proposed YAML Model

This section describes a declarative YAML/JSON model for connector desired state and reconciliation.

### 19.1 Two-Layer Config: Spec vs Runtime

- ConnectorSpec (YAML/JSON): user-authored, versioned, validated, declarative
- ConnectorInstance (DB): internal state, status, health, last_error, etc.

The system continuously reconciles Spec => Instance (Kubernetes-style controller pattern).

---

### 19.2 YAML/JSON Feasibility Checklist

The following lists what fits declarative config well versus what needs extra handling.

#### 19.2.1 Works Well For

- install/enable/disable/uninstall (desired state)
- non-secret config (hostnames, channel ids, mailbox filters)
- ownership/scope (user/group/project)
- operation allowlists
- rate limit settings and concurrency caps
- inbound subscription enablement

#### 19.2.2 Needs Special Handling For

- secrets and rotation
- per-operation policy decisions
- provider OAuth flows (interactive)
- webhook verification secrets (must be generated/stored securely)
- drift detection and rollback behavior

---

### 19.3 Spec Schema

Define a strict schema and versioning.

Example YAML:

```yaml
apiVersion: cynodeai.io/v1alpha1
kind: Connector
metadata:
  name: ops-mail
  owner:
    type: group
    id: 7b2c... # uuid
  projectId: 3a91... # optional
spec:
  type: imap
  typeVersion: 1
  enabled: true

  config:
    host: imap.example.com
    port: 993
    tls: true
    mailbox: INBOX
    filter:
      fromAllowlist:
        - alerts@example.com

  credentials:
    mode: secretRef
    secretRef:
      provider: cynode-vault
      name: imap-ops-mail-creds
      keys:
        username: username
        password: password

  policy:
    allowOperations:
      - readInbox
    actionScope:
      - connector.read

  limits:
    maxParallelInvocations: 4
    rateLimit:
      rps: 2
      burst: 5
```

JSON equivalent is straightforward.

---

### 19.4 Secrets: Recommended Approaches

Secrets must never appear as plain values in spec; use references or optional encrypted inline form.

#### 19.4.1 Preferred: Secret References Only

The YAML/JSON contains only pointers:

- secretRef to a vault entry, KMS-wrapped blob, or internal secret store
- the platform resolves and injects at runtime inside the connector boundary

#### 19.4.2 Optional: Encrypted Inline Secrets (Only If You Must)

If you want single-file portability, you can support:

- SOPS-style encrypted values
- age/GPG-based encryption
- platform decrypts via KMS or agent key

Constraints:

- strict RBAC around who can decrypt
- audit every decrypt
- never log decrypted material
- key rotation story must be explicit

---

### 19.5 Reconciliation Controller

Implement a controller that:

- Watches connector spec submissions (API, GitOps repo, uploaded file)
- Validates schema + policy
- Creates/updates connector instance DB rows
- Applies enable/disable
- Verifies connectivity if enabled
- Updates status and emits audit events

Recommended statuses:

- provisioning
- active
- degraded
- credential_expired
- error
- disabled

Drift model:

- If DB differs from spec, controller converges DB to spec
- Manual changes either forbidden or overwritten (pick one and state it)

---

### 19.6 Policy Binding in the Spec

Do not allow arbitrary policy in the spec without guardrails.

Two safe patterns:

- refer to named policy templates:

  - `policyTemplate: "read-only-notifications"`
- allow only a constrained allowlist (operations + actions) validated server-side

---

### 19.7 OAuth / Interactive Auth

For connectors like Discord/Mattermost (depending on auth):

- YAML can request `credentials.mode: oauth`
- system returns a pending-auth status with an auth URL
- user completes flow in UI
- connector instance becomes active

This keeps YAML declarative while handling the interactive step.

---

### 19.8 Operational Semantics to Define

If you go this route, your spec must explicitly define:

- schema versioning and migrations
- idempotency (metadata.uid, resourceVersion)
- deletion semantics (remove spec => uninstall? or disable?)
- rollout strategy (apply immediately vs staged)
- validation and rejection behavior
- audit requirements for apply operations
- multi-tenant scope rules (group vs project precedence)

---

### 19.9 Minimal Set of MCP/API Tools Needed

Even with YAML/JSON as the main interface, you still need primitives:

- connector.spec.apply (submit YAML/JSON)
- connector.spec.validate
- connector.spec.diff
- connector.instance.status.get
- connector.op.invoke

Optional:

- connector.spec.list
- connector.spec.delete

---

### 19.10 Feasibility Verdict

Feasible and generally a good design if:

- YAML/JSON is treated as desired state
- secrets are references, not values
- interactive auth is out-of-band but tracked in status
- you implement a controller/reconciler
- you define strict schemas and policy constraints

If you want, I can draft:

- a formal ConnectorSpec CRD-like schema (OpenAPI/JSON Schema)
- reconciliation algorithm and state machine
- rules for secretRef providers (vault, kmsblob, env, oauth)
- example specs for IMAP, Mattermost, Discord aligned to your existing spec IDs
