# API Egress Sanity Checker: Spec Update Proposal

- [1. Purpose and Scope](#1-purpose-and-scope)
- [2. Current API Egress Flow](#2-current-api-egress-flow)
- [3. Proposed Sanity Checker Layer](#3-proposed-sanity-checker-layer)
- [4. Sanity Checker Detection Categories](#4-sanity-checker-detection-categories)
- [5. Proposed Spec and Requirement Changes](#5-proposed-spec-and-requirement-changes)
- [6. Open Questions and Implementation Notes](#6-open-questions-and-implementation-notes)
- [7. References](#7-references)

## 1. Purpose and Scope

This proposal adds an **LLM-based sanity checker** to the API Egress path.
The sanity checker evaluates whether a proposed outbound API call is "safe to execute" from a semantic standpoint, complementing existing access control (identity, provider/operation allowlists, credentials).
It aims to catch dangerous intents that policy alone cannot express: bulk or irreversible data deletion without backup/safety, secret exposure, and other high-impact actions.

Document type: spec update proposal (docs-only).
Output: proposal for changes to [`docs/tech_specs/api_egress_server.md`](../tech_specs/api_egress_server.md) and, as needed, [`docs/requirements/apiegr.md`](../requirements/apiegr.md).

## 2. Current API Egress Flow

Per [`api_egress_server.md`](../tech_specs/api_egress_server.md):

- Agents submit a structured request: `provider`, `operation`, `params`, `task_id`.
- The orchestrator routes approved calls to the API Egress Server.
- Access control ([Access Control](../tech_specs/api_egress_server.md#spec-cynai-apiegr-accesscontrol)): subject identity, provider/operation allow policy, credential authorization, per-user/per-task constraints.
- Policy and auditing ([Policy and Auditing](../tech_specs/api_egress_server.md#spec-cynai-apiegr-policyauditing)): provider/operation allowlists, logging, response filtering to avoid secret leakage.
- There is **no semantic analysis** of what the call would do (e.g. "delete repository", "drop database", "send credentials to external endpoint").

The sanity checker would sit **after** access control and **before** credential resolution and outbound execution.

## 3. Proposed Sanity Checker Layer

The sanity checker is an optional layer that evaluates the semantic intent of the request before execution.
It uses an LLM to classify the call and can allow, deny, or escalate.

### 3.1 Placement in Request Flow

Proposed order:

1. Request received (provider, operation, params, task_id, subject context).
2. Access control evaluation (existing: identity, allow policy, credential selection).
3. **Sanity check (new):** LLM-based evaluation of the intended effect of the call.
4. If sanity check passes: resolve credential, perform outbound call, audit, return result.
5. If sanity check fails: deny with structured error, audit the denial.

The sanity checker MUST NOT receive credentials or decrypted secrets; it only receives the same request shape the agent submitted (provider, operation, params, task_id and any non-secret context needed for explanation).

### 3.2 Role of the LLM

The LLM is used as a **semantic classifier/adviser**:

- Input: provider, operation, params (and optionally provider-specific operation metadata or schemas if available).
- Output: one of: `allow`, `deny`, or `escalate` (e.g. for human review or higher confirmation).
- Optional: short reason or category for deny/escalate (for audit and user feedback).

The system MUST treat the LLM as untrusted and non-authoritative for security: it is an additional signal.
Access control remains the mandatory gate; the sanity checker is a second layer to reduce risk of obviously dangerous calls that passed policy (e.g. "delete all" operations, exports of secrets, destructive actions on critical resources).

### 3.3 Configurability and Bypass

- Sanity check SHOULD be configurable per deployment (e.g. enabled/disabled, or per provider/operation).
- When disabled, behavior MUST match current spec (no sanity check step).
- Optional: allowlist of provider/operation pairs that skip sanity check (e.g. read-only or pre-approved safe operations) to reduce latency and LLM cost.

### 3.4 Model Selection (Local and External)

The sanity checker SHOULD support both **local** inference (e.g. via Ollama or similar) and **configurable external** model usage via API.
Deployments can choose local-only for privacy and latency, external for stronger models, or a fallback chain.

#### 3.4.1 Recommended Local Model (Ollama)

- **Llama Guard 3** is a purpose-built safety classifier suitable for this layer.
  Available on Ollama as `llama-guard3:1b` (lightweight) or `llama-guard3:8b` (default; higher accuracy).
  It classifies content against the MLCommons hazard taxonomy (13 categories), exposes a chat-completion API, and can be used to label request intent as safe/unsafe with a category.
  For API-call sanity checks, the implementation can map the request (provider, operation, params) into a text description and call Llama Guard 3; custom prompts may be needed to align its output (e.g. allow/deny/escalate) with the categories in Section 4.
- **Alternatives for custom prompts or lower resource:** **SmolLM2** (e.g. `smollm2:360m-instruct` or `smollm2:1.7b`) or **Llama 3.2** (`llama3.2:1b`, `llama3.2:3b`) are small, fast, and run well on Ollama for general classification when the deployment prefers a single custom prompt over a dedicated safety model.

#### 3.4.2 Configurable External Model via API

- The implementation SHOULD allow configuring an **external model endpoint** (e.g. OpenAI, Anthropic, or any OpenAI-compatible chat completion API) for the sanity checker.
  This supports deployments that prefer a cloud-hosted model, a different safety model (e.g. provider-specific moderation APIs), or a fallback when the local model is unavailable.
- Configuration SHOULD include: endpoint URL, model identifier, and optional API key or auth method; timeout and fail-open/fail-closed behavior when the external call fails or times out.
- Credentials for the external model MUST NOT be passed into the sandbox; the API Egress Server (or orchestrator) performs the sanity-check call out-of-band from the agent request.

## 4. Sanity Checker Detection Categories

The sanity checker should look for the following (to be encoded in prompts and/or structured checks where possible).

### 4.1 Bulk or Irreversible Deletion Without Backup/Safety

- Operations that delete large or unbounded amounts of data: databases, tables, buckets, repositories, mailboxes, file stores.
- Destructive operations that are not clearly scoped (e.g. "delete all", "drop schema", "purge mailbox") and where the params do not indicate a backup or safety confirmation.
- Criteria: provider/operation suggests destructive semantics and params lack mitigating signals (e.g. backup flag, confirmation token, or narrow scope).

Examples (illustrative): GitHub `delete_repository`, Slack `delete_channel` with broad scope, cloud storage "delete bucket" or "empty bucket", email "purge folder", database "drop table" / "truncate".

### 4.2 Secret or Credential Exposure

- Operations that would send secrets, tokens, or credentials to an external system or channel where they could be logged or exposed.
- Operations that export or list credentials, API keys, or sensitive config.
- Params that appear to contain secret material (e.g. high-entropy strings in fields named "token", "secret", "password") and are being sent to a destination that is not the intended credential store.

Response filtering (REQ-APIEGR-0120) already addresses response-side leakage; the sanity checker focuses on **request-side** intent (e.g. "post this token to a webhook" or "write this key to a public channel").

### 4.3 Other Dangerous or High-Impact Actions

- Billing or quota-changing operations (e.g. upgrade plan, increase spend limits) without clear confirmation.
- Operations that alter security or access control at scale (e.g. grant broad permissions, disable MFA, change org-wide settings).
- Operations that could cause irreversible data loss or system-wide impact when the semantics of the operation and params suggest such risk.

The exact list can be refined in the spec and in requirement text; the above gives a concrete starting set for the proposal.

## 5. Proposed Spec and Requirement Changes

The following edits are proposed to align the spec and requirements with the sanity checker design.

### 5.1 Requirements (`docs/requirements/apiegr.md`)

Add new requirements (numbering to be assigned per project convention, e.g. REQ-APIEGR-0121 onward):

- **REQ-APIEGR-0121 (proposed):** The API Egress Server MAY perform a semantic sanity check on the requested call (provider, operation, params) before execution.
  When enabled, the sanity check MUST evaluate whether the call appears to involve bulk/irreversible deletion without backup, secret exposure, or other dangerous or high-impact actions.
- **REQ-APIEGR-0122 (proposed):** The sanity check MUST NOT receive or use decrypted credentials; it SHALL use only the request payload and non-secret context.
- **REQ-APIEGR-0123 (proposed):** When the sanity check denies a call, the server MUST deny the request with a structured error and MUST log the denial with task context and reason/category.
- **REQ-APIEGR-0124 (proposed):** Sanity check behavior SHOULD be configurable (e.g. enable/disable, or allowlist of operations that skip the check).
- **REQ-APIEGR-0125 (proposed):** The sanity check MAY use a local model (e.g. via Ollama or similar) or a configurable external model via API (e.g. OpenAI-compatible or provider-specific endpoint).
  When external API is configured, endpoint URL, model identifier, and authentication MUST be configurable; credentials for the external model MUST NOT be exposed to sandboxes.

### 5.2 Tech Spec (`docs/tech_specs/api_egress_server.md`)

- Add a new section **Sanity Check (Semantic Safety)** with a Spec ID (e.g. `CYNAI.APIEGR.SanityCheck`).
- Describe placement: after access control, before credential resolution and outbound call.
- Define inputs: provider, operation, params, task_id; no credentials.
- Define outputs: allow | deny | escalate; optional reason/category.
- Reference the detection categories (bulk/irreversible deletion, secret exposure, other dangerous actions) and state that the implementation SHOULD use an LLM or equivalent semantic evaluation to classify the request; allowlist-based or rule-based shortcuts for known-safe operations are permitted.
- Describe audit: log sanity-check result (allow/deny/escalate) and, on deny/escalate, reason/category.
- Describe configuration: enable/disable; optional allowlist of (provider, operation) that skip the check.
- Describe **model configuration:** support for (1) local inference (e.g. Ollama base URL + model name, with recommended default such as `llama-guard3:8b` or `llama-guard3:1b` for safety classification, or SmolLM2/Llama 3.2 for custom prompts) and (2) optional external API (OpenAI-compatible or provider-specific endpoint, model id, auth); behavior when external call fails or times out (e.g. fail open vs fail closed).
- Add a traceability subsection linking to the new REQ-APIEGR-0121 through REQ-APIEGR-0125.

### 5.3 Access Control and Policy

- In [`access_control.md`](../tech_specs/access_control.md), optionally add a short note that the API Egress Server may apply an additional semantic sanity check after policy evaluation; policy remains the authoritative allow/deny for identity and resource, and the sanity check is a separate safety layer.

## 6. Open Questions and Implementation Notes

- **LLM provider and prompt ownership:** Whether the sanity checker uses the same inference path as the job (e.g. orchestrator-mediated API Egress for a dedicated model) or a separate, internal-only model; and who owns prompt and category definitions (ops vs. product).
- **Latency and availability:** LLM calls add latency and a failure mode; spec should define timeouts and behavior when the sanity check service is unavailable (e.g. fail open vs. fail closed by policy).
- **Escalate flow:** If "escalate" is supported, how it integrates with human review or additional confirmation (e.g. queue for approval, or return a token that the client can use to confirm).
- **i18n and prompt stability:** Prompts should be versioned and designed so that provider/operation/params are passed in a consistent, parseable way to reduce prompt injection and drift.
- **Local vs external default:** Whether the default when sanity check is enabled is local (Ollama + recommended model) or external; and how to document recommended models (Llama Guard 3, SmolLM2, Llama 3.2) in the spec or operator docs.

## 7. References

- [API Egress Server](../tech_specs/api_egress_server.md)
- [Access Control](../tech_specs/access_control.md)
- [APIEGR Requirements](../requirements/apiegr.md)
- [Spec Authoring and Validation](../docs_standards/spec_authoring_writing_and_validation.md)
