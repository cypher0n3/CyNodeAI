# Proposed Specs and Requirements Updates: Worker Proxy Outstanding Gaps

- [Metadata](#1-metadata)
- [Summary](#2-summary)
- [Agent token ref schema and resolution](#3-agent-token-ref-schema-and-resolution)
- [Token expiry field in config payload](#4-token-expiry-field-in-config-payload)
- [Secure store FIPS and Go 1.26 runtime secret](#5-secure-store-fips-and-go-126-runtime-secret)
- [Capability leases vs agent tokens scope](#6-capability-leases-vs-agent-tokens-scope)
- [Orchestrator-side responsibilities](#7-orchestrator-side-responsibilities)
- [Node Manager vs Worker API process boundary](#8-node-manager-vs-worker-api-process-boundary)
- [Binding type per-service loopback listener](#9-binding-type-per-service-loopback-listener)
- [Implementation order and validation](#10-implementation-order-and-validation)
- [Document history](#11-document-history)

## 1. Metadata

- Source: [2026-03-05_worker_proxy_spec_reconciliation_plan.md](2026-03-05_worker_proxy_spec_reconciliation_plan.md) "Outstanding Spec Gaps and Ambiguities" (lines 241-288).
- Standards: [spec_authoring_writing_and_validation.md](../docs_standards/spec_authoring_writing_and_validation.md).
- Purpose: Draft proposed changes to specs and requirements to resolve the listed gaps and ambiguities before or during Phase 4-6 implementation.
- Status: Draft for review; no code or spec edits applied.

## 2. Summary

This document proposes concrete spec and requirement edits for seven items: `agent_token_ref` schema and resolution, token expiry field in config payload, secure store FIPS/Go 1.26, capability leases vs agent tokens scope, orchestrator-side responsibilities (clarification only), Node Manager vs Worker API process boundary, and binding type `per_service_loopback_listener`.

## 3. Agent Token Ref Schema and Resolution

**Gap:** No normative schema or resolution algorithm for `agent_token_ref`; Phase 4 cannot implement resolution or fail closed safely.

### 3.1 Payloads Spec: Agent_token_ref_token_ref

Add a new Spec Item for the `agent_token_ref` object under the existing `node_configuration_payload_v1.managed_services.services[].orchestrator` section (or as an adjacent subsection).

- **Spec ID:** `CYNAI.WORKER.Payload.AgentTokenRef` (new).
- **Anchor:** `spec-cynai-worker-payload-agenttokenref`.
- **Content:** Define the normative schema and semantics.
  - `agent_token_ref` is an optional object; when present, the worker MUST resolve it to a short-lived token and MUST NOT pass the reference or any resolved secret to the agent container.
  - Normative fields (minimum for v1):
    - `kind` (string, required): identifies the resolution mechanism; initial value: `orchestrator_endpoint`.
    - For `kind=orchestrator_endpoint`: `url` (string, required) - HTTPS URL the worker calls (GET or POST, method and headers to be defined in worker_node.md) to obtain a short-lived token; response shape (e.g. JSON with `token` and optional `expires_at`) must be specified in the same or linked spec.
  - Resolution MUST be performed by the component that applies config (Node Manager or equivalent); failures MUST fail closed (no token written, no secret leaked; config apply MAY report a structured error).
  - Link to a new Algorithm in `worker_node.md` for "Agent token ref resolution" that defines the exact HTTP contract, error handling, and that resolution failures do not write partial secrets.

### 3.2 Worker Node Spec: `agent_token_ref_token_ref` Resolution

Add a Spec Item and Algorithm for agent token ref resolution.

- **Spec ID:** `CYNAI.WORKER.AgentTokenRefResolution` (new).
- **Scope:** Worker (config-apply component) only; orchestrator endpoint contract is out of scope for worker_node.md but may be specified in `orchestrator.md` or a shared API spec.
- **Algorithm:** Steps: (1) When applying config, for each service with `agent_token_ref` and no `agent_token`, call the ref URL with node identity; (2) On success, parse response and write token (and optional expiry) to secure store keyed by `service_id`; (3) On failure (non-2xx, invalid body, timeout), do not write any token; report error in config ack or apply result; (4) Do not log response body or token material.
- **Trace:** Link to REQ-WORKER-0164, REQ-WORKER-0132; reference `CYNAI.WORKER.Payload.AgentTokenRef` for payload shape.

### 3.3 Worker Requirements: Agent_token_ref_token_ref

Optional: add a requirement that when config supplies `agent_token_ref`, the worker MUST resolve it using the specified contract and MUST NOT expose the reference or resolved token to agents or containers.
If the payload and algorithm are sufficiently prescriptive, this may be covered by existing REQ-WORKER-0164 and REQ-WORKER-0132; otherwise add e.g. REQ-WORKER-0171.

## 4. Token Expiry Field in Config Payload

**Ambiguity:** worker_node.md step 5 references `agent_token_expires_at` but payloads spec does not define it.

### 4.1 Payloads Spec: Agent_token_expires_at_token_expires_at

Under `managed_services.services[].orchestrator` (same block as `agent_token` and `agent_token_ref`), add:

- `agent_token_expires_at` (string, optional)
  - RFC 3339 UTC timestamp at which the current `agent_token` (or token obtained via `agent_token_ref`) is considered expired.
  - When present, the worker MUST treat the token as invalid after this time and MUST NOT use it to forward requests; the worker SHOULD request a configuration refresh where applicable.
  - Omit when the orchestrator does not supply an expiry (e.g. for long-lived or refresh-only tokens).

No new Spec ID required if added as a field under the existing config payload Spec Item; if a dedicated Spec Item is preferred for expiry semantics, use e.g. `CYNAI.WORKER.Payload.AgentTokenExpiry` and link from worker_node.md algorithm step 5.

### 4.2 Worker Node Spec: Token Expiry Cross-Reference

In the `AgentTokenStorageAndLifecycle` Algorithm, step 5 already references `agent_token_expires_at`.
Add an explicit cross-reference: "When an expiry is provided (e.g. `agent_token_expires_at` in the orchestrator object per [worker_node_payloads.md](../tech_specs/worker_node_payloads.md) `node_configuration_payload_v1.managed_services.services[].orchestrator`), the worker MUST treat expired tokens as invalid..."

No requirement change needed; REQ-WORKER-0131 (short-lived) and REQ-WORKER-0164 (worker-held tokens) already imply correct handling.

## 5. Secure Store FIPS and Go 1.26 Runtime Secret

**Gap:** Phase 4 change set did not explicitly call out FIPS and Go 1.26; implementation could miss them.

### 5.1 Worker Node Spec: FIPS and Runtime/secret/secret Clarification

The spec already contains:

- FIPS mode: "When the host system is configured for FIPS mode, the worker MUST use only FIPS-approved algorithms..."
- Go 1.26: "The worker SHOULD use `runtime/secret` (Go 1.26, via `GOEXPERIMENT=secret`) to wrap code that handles the master key or decrypted plaintext secrets so temporaries are erased before returning."

Proposed clarification (add one sentence): "On platforms or Go versions where `runtime/secret` is not available, the implementation MUST use best-effort secure erasure of temporaries (e.g. zeroing buffers) before returning from code paths that handle master key or decrypted plaintext."

Ensure the NodeLocalSecureStore section and the Phase 4 change set in the reconciliation plan both reference this sentence so implementers and reviewers see it in Phase 4 scope.

### 5.2 Worker Requirements: FIPS and Runtime/secret/secret

REQ-WORKER-0170 already covers FIPS.
No new requirement for Go 1.26 / `runtime/secret`; the spec "SHOULD" and best-effort fallback are implementation guidance only.

## 6. Capability Leases vs Agent Tokens Scope

**Ambiguity:** worker_node.md mentions both agent tokens and capability leases; plan and Phase 4/6 are agent-token-only.

### 6.1 Worker Node Spec: Scope Note for Agent Tokens Only

In the "Worker Proxy Bidirectional (Managed Agents)" section and in the NodeLocalSecureStore scope paragraph, add an explicit scope note:

- "For the managed agent internal proxy (agent-to-orchestrator forwarding), the worker holds **agent tokens** only; resolution, storage, and attachment are defined in this document and in worker_node_payloads.
  **Capability leases** (e.g. for node-local sandbox tool access per Node-Local Agent Sandbox Control) are out of scope for the managed agent proxy; they are stored in the same node-local secure store but their lifecycle and use are defined elsewhere and are not part of the agent_token_ref / agent_token_expires_at / internal proxy auth flow."

Optionally add a short "Scope" subsection under Token and Credential Handling: "This section and the AgentTokenStorageAndLifecycle algorithm apply to **agent tokens** used for managed agent to orchestrator proxy.
Capability leases for node-local sandbox control are not covered here."

### 6.2 Reconciliation Plan: Capability Leases out of Scope

In "Capability Leases vs Agent Tokens (Scope)", add a line: "Resolved by spec update: worker_node.md and this plan explicitly scope Phase 4 and Phase 6 to agent-token-only for managed agent proxy; capability leases documented as out of scope for this plan."

## 7. Orchestrator-Side Responsibilities

**Clarification:** Plan is worker-only; E2E needs orchestrator behavior too.

### 7.1 Reconciliation Plan: Orchestrator Checklist

In the reconciliation plan, under "Orchestrator-Side Responsibilities (Out of Scope)", add a short checklist or pointer:

- "For full E2E compliance, the orchestrator MUST: issue agent tokens (and optionally support agent_token_ref); associate user context for user-scoped managed agents; ingest and store `managed_services_status` and `agent_to_orchestrator_proxy`; route to managed services using worker-reported endpoints only.
  See orchestrator spec and requirements (e.g. ORCHES) for normative obligations; worker-only work is tracked in this plan."

No change to worker specs or worker requirements; optional new doc or section under `docs/tech_specs/orchestrator.md` or `docs/requirements/orches.md` for an "Orchestrator managed-services and agent-token checklist" if the project wants a single place to list orchestrator tasks for this feature.

## 8. Node Manager vs Worker API Process Boundary

**Ambiguity:** Spec says Node Manager (or config-apply) writes to secure store and worker proxy reads; multi-process trusted boundary is not specified.

### 8.1 Worker Node Spec: Secure Store Process Boundary

In NodeLocalSecureStore or in a new short subsection "Secure store access and process boundary":

- Define explicitly: "The **writer** of the secure store is the component that applies node configuration (typically the Node Manager).
  The **reader** of agent tokens from the secure store is the worker proxy (internal proxy handler).
  When the Node Manager and Worker API run in the **same process**, the trusted boundary is that process.
  When they run in **separate processes**, the implementation MUST enforce a trusted boundary (e.g. same OS user, or documented capability such as shared keyring) so that only the config-apply component can write and only the worker proxy process can read; the implementation MUST document which deployment model is used and how the boundary is enforced."

Add a Spec ID if desired, e.g. `CYNAI.WORKER.SecureStoreProcessBoundary`, or fold into `CYNAI.WORKER.NodeLocalSecureStore` as a "Process boundary" contract subsection.

### 8.2 Worker Requirements: Process Boundary

Optional: REQ-WORKER-0172 (or next available): "When Node Manager and Worker API run as separate processes, the node MUST enforce a trusted boundary for the secure store (e.g. same user or documented capability) and MUST document the deployment model and boundary enforcement."
Link to the new spec subsection.

## 9. Binding Type Per-Service Loopback Listener

**Ambiguity:** Payloads allow `per_service_loopback_listener`; worker_node.md normatively defines only per-service UDS; identity resolution for loopback is unspecified.

### 9.1 Payloads Spec: Binding Type Semantics

Under `managed_services_status.services[].agent_to_orchestrator_proxy.binding` (and any capability/feature that references binding), keep existing values `per_service_loopback_listener` | `per_service_uds` | `other`.

Add a sentence: "`per_service_uds` is the only binding type normatively defined and required in worker_node.md for the managed agent internal proxy; implementations MUST support it.
Support for `per_service_loopback_listener` is optional; when supported, the worker MUST derive caller `service_id` in a specified way (e.g. distinct loopback port per service); see worker_node.md if and when loopback binding is specified."

### 9.2 Worker Node Spec: UDS Normative and Loopback Future

In the identity-binding section (per-service UDS), add an explicit scope statement:

- "The **normative** identity-binding mechanism for the managed agent internal proxy is **per-service Unix domain sockets** (`per_service_uds`).
  Loopback bindings (`per_service_loopback_listener`) are not specified in this document; if added in a future revision, the spec MUST define how the worker maps each connection to a single `service_id` (e.g. one loopback port per service)."

No new requirement; REQ-WORKER-0163 already constrains binding to loopback or UDS.

## 10. Implementation Order and Validation

- Apply payload and algorithm changes (sections 3 and 4) before Phase 4 implementation of token lifecycle and agent_token_ref resolution.
- Apply secure store clarification (section 5) and scope clarifications (sections 6 and 9) in sync with Phase 4/5.
- Process boundary (section 8) and orchestrator checklist (section 7) can be documented in parallel; no code dependency.
- After editing specs: run `just lint-md` on changed files and fix any reported issues; ensure new Spec IDs and anchors follow [spec_authoring_writing_and_validation.md](../docs_standards/spec_authoring_writing_and_validation.md).

## 11. Document History

- 2026-03-05: Initial draft (proposed updates for worker proxy spec reconciliation gaps).
