# Worker Identity, Continuous Verification, and Node Versioning: Spec Update Proposal

- [1. Purpose and Scope](#1-purpose-and-scope)
- [2. Source of Requirements](#2-source-of-requirements)
- [3. Proposed Spec Changes: Worker and Component Identity](#3-proposed-spec-changes-worker-and-component-identity)
- [4. Proposed Spec Changes: Bootstrap and Continuous Verification](#4-proposed-spec-changes-bootstrap-and-continuous-verification)
- [5. Proposed Spec Changes: Node Versioning and API Compatibility](#5-proposed-spec-changes-node-versioning-and-api-compatibility)
- [6. Summary of Spec and Requirement Edits](#6-summary-of-spec-and-requirement-edits)
- [7. References](#7-references)

## 1. Purpose and Scope

This proposal updates technical specifications to address identity and continuous-verification gaps from the zero-trust assessment (Worker/Component Identity, Bootstrap and First-Use Identity) and adds explicit **node versioning and API compatibility** rules between worker nodes and the orchestrator.

Document type: spec update proposal (docs-only).
Date: 2026-02-26.
Output: proposed edits to [`docs/tech_specs/worker_api.md)](../tech_specs/worker_api.md), [`docs/tech_specs/worker_node.md)](../tech_specs/worker_node.md), [`docs/tech_specs/worker_node_payloads.md)](../tech_specs/worker_node_payloads.md), and optionally a dedicated identity spec; plus requirements in [`docs/requirements/worker.md)](../requirements/worker.md) if new normative obligations are introduced.

## 2. Source of Requirements

- **Identity gaps:** [`dev_docs/zero_trust_tech_specs_assessment.md)](zero_trust_tech_specs_assessment.md) sections 3.1 (Worker and Component Identity) and 3.2 (Bootstrap and First-Use Identity).
- **Existing contracts:** Worker API and node payloads already define versioned payloads and `/v1/` API prefix; this proposal adds **orchestrator-to-node version compatibility checks** and **node software version** reporting so the orchestrator can enforce compatibility policy.

## 3. Proposed Spec Changes: Worker and Component Identity

This section proposes additions for workload identity and node binding.

### 3.1 Identity Problem

Worker API and orchestrator-to-node traffic use a **static bearer token** (Phase 1 only).
No spec defines the future workload identity model (short-lived tokens, mTLS, or attestation), node identity binding, or token/certificate lifecycle.

### 3.2 Proposed Identity Additions

The following edits are proposed for worker_api and node payloads.

#### 3.2.1 In [`worker_api.md](../tech_specs/worker_api.md)

- Add a subsection **Workload identity (post-Phase 1)** (or reference a new identity spec) that:
  - **Requires or recommends** short-lived tokens or certificate-based authentication for orchestrator-to-node calls, with a defined refresh/rotation mechanism (e.g. token expiry, refresh endpoint, or cert rotation window).
  - Defines **node identity**: how node identity (e.g. node ID plus attestation or certificate CN/SAN) is bound to the credential (bearer token or TLS client cert), and that the Worker API MUST validate this binding on each authenticated request.
- Retain the existing Phase 1 constraint (static bearer token) as explicitly temporary and reference the new subsection for the target state.

#### 3.2.2 In [`worker_node_payloads.md](../tech_specs/worker_node_payloads.md) and [``worker_node.md``](../tech_specs/worker_node.md)

- Extend node registration and node configuration payloads to support (in a post-Phase 1 schema):
  - **Token refresh:** e.g. `worker_api.token_refresh_url`, `worker_api.token_expires_at`, or equivalent; and/or
  - **mTLS client identity:** reference to client certificate and private key (or slot) for TLS client auth.
- Document **credential lifecycle**: issuance, expiry, revocation, and node behavior when credentials expire or are revoked (e.g. stop accepting new jobs, re-register, or fetch new token).

## 4. Proposed Spec Changes: Bootstrap and Continuous Verification

This section proposes continuous verification of node identity after bootstrap.

### 4.1 Bootstrap Problem

Bootstrap is one-time; there is no spec requirement for **continuous verification** of node identity after registration (e.g. re-auth or re-attestation on config fetch or on a schedule).

### 4.2 Proposed Bootstrap Additions

The following edits are proposed for worker_node.md.

#### 4.2.1 In [`worker_node.md](../tech_specs/worker_node.md) (Registration and Bootstrap / Configuration Delivery)

- Add a requirement or recommendation that the orchestrator **re-verifies node identity** when delivering configuration (e.g. using the same credential used for Worker API calls, or a dedicated config-auth mechanism).
  - Config delivery endpoints MUST authenticate the node (e.g. same bearer token or mTLS client cert as Worker API) before returning node-specific configuration.
- Add an **optional** mechanism:
  - **Periodic re-registration or heartbeat** that includes identity proof (e.g. same credential or attestation as used for Worker API).
  - Define a **heartbeat interval** (or re-registration interval) and **failure behavior**: e.g. after N missed heartbeats or failed re-auth, the orchestrator marks the node non-dispatchable and optionally revokes or shortens credential validity.

## 5. Proposed Spec Changes: Node Versioning and API Compatibility

This section proposes version reporting and compatibility checks between orchestrator and nodes.

### 5.1 Versioning Goals

- Ensure the orchestrator can **refuse to dispatch** to nodes whose Worker API or payload version is incompatible.
- Ensure nodes can **reject** config or API requests from an orchestrator version they do not support.
- Provide a single place (spec + optional compatibility matrix) for **supported version combinations**.

### 5.2 Node Software and API Version Reporting

Nodes should report Worker API version and optional software/config versions.

#### 5.2.1 In [`worker_node_payloads.md](../tech_specs/worker_node_payloads.md) (Capability Report)

- Add to the node capability report (e.g. `node_capability_report_v1` or a new version):
  - **`worker_api_version`** (string): version of the Worker API contract the node implements (e.g. `"1"` matching `/v1/`).
  - **`node_software_version`** (string, optional): build or semantic version of the node software (e.g. `"1.2.0"` or git commit).
  - **`supported_config_payload_versions`** (array of integers, optional): list of `node_configuration_payload` versions the node accepts (e.g. `[1]`).

#### 5.2.2 In [`worker_node.md](../tech_specs/worker_node.md) (Capability Reporting)

- Require that the node reports Worker API version and, when available, node software version and supported config payload versions in the capability report so the orchestrator can apply compatibility policy.

### 5.3 Orchestrator-Side Compatibility Checks

The orchestrator must enforce version compatibility before dispatch.

#### 5.3.1 In [`worker_node.md](../tech_specs/worker_node.md) or a New Subsection in [``worker_api.md``](../tech_specs/worker_api.md)

- Define **orchestrator compatibility behavior**:
  - The orchestrator MUST record each node's reported `worker_api_version` (and optionally `node_software_version`, `supported_config_payload_versions`) from the capability report.
  - Before dispatching a job to a node, the orchestrator MUST ensure the node's Worker API version is **supported** by the orchestrator (e.g. orchestrator maintains a supported Worker API versions list, or a compatibility matrix).
  - When the orchestrator delivers node configuration, it MUST use a `node_configuration_payload` version that the node supports (if the node advertised `supported_config_payload_versions`); otherwise it MUST use a version documented as supported by the node's reported Worker API version.
  - If a node's Worker API or config version is not supported, the orchestrator MUST NOT dispatch jobs to that node and SHOULD mark the node as incompatible (e.g. status or label) and optionally alert.
- Define **node-side compatibility behavior**:
  - The node MUST reject Worker API requests that use a path version the node does not implement (e.g. return 404 or 501 for unknown `/vN/`).
  - The node SHOULD reject node configuration payloads with an unsupported `version` (already specified in [`worker_node_payloads.md)](../tech_specs/worker_node_payloads.md) Compatibility and Versioning).

### 5.4 Version Compatibility Matrix (Recommendation)

- Add a **Version compatibility** subsection (in [`worker_api.md`](../tech_specs/worker_api.md) or [`worker_node.md`](../tech_specs/worker_node.md) that:
  - States the **current** supported Worker API version(s) (e.g. `v1`).
  - Documents that the orchestrator and node implementations MUST adhere to the compatibility rules above; for future API or payload versions, a **compatibility matrix** (e.g. in the spec or a linked doc) SHOULD define which orchestrator versions work with which node/Worker API versions.
  - Optionally: define **deprecation** policy (e.g. support at least N previous Worker API versions, with advance notice before removal).

## 6. Summary of Spec and Requirement Edits

- **Target spec:** `worker_api.md)
  - change: Add "Workload identity (post-Phase 1)" subsection (short-lived tokens or mTLS, node identity binding).
    Add or reference orchestrator/node compatibility checks and version compatibility subsection.
- **Target spec:** `worker_node.md)
  - change: Re-verify node identity on config delivery.
    Optional: heartbeat/re-registration with identity proof and failure behavior.
    Require capability report to include Worker API version and optional node software/supported config versions.
    Add orchestrator compatibility behavior (no dispatch to incompatible nodes).
- **Target spec:** `worker_node_payloads.md)
  - change: Extend capability report with `worker_api_version`, optional `node_software_version`, `supported_config_payload_versions`.
    Extend config/registration payloads (post-Phase 1) for token refresh or mTLS client identity and lifecycle.
- **Target spec:** `docs/requirements/worker.md)
  - change: If new normative obligations are desired (e.g. "orchestrator MUST NOT dispatch to nodes with unsupported Worker API version"), add REQ-WORKER-* and trace from specs.

## 7. References

- [`dev_docs/zero_trust_tech_specs_assessment.md)](zero_trust_tech_specs_assessment.md) (sections 3.1, 3.2)
- [`docs/tech_specs/worker_api.md)](../tech_specs/worker_api.md)
- [`docs/tech_specs/worker_node.md)](../tech_specs/worker_node.md)
- [`docs/tech_specs/worker_node_payloads.md)](../tech_specs/worker_node_payloads.md)
- [`docs/tech_specs/local_user_accounts.md)](../tech_specs/local_user_accounts.md)
- [`docs/docs_standards/spec_authoring_writing_and_validation.md)](../docs_standards/spec_authoring_writing_and_validation.md)
