# Cloud LLM API Quotas Spec Proposal

## 1 Summary

- **Date:** 2026-03-16
- **Purpose:** Draft spec for provider-side and per-credential quotas when calling cloud LLM APIs (OpenAI, Anthropic, etc.), including discovery, enforcement, and model-selection integration.
- **Status:** Draft only; not yet merged into `docs/requirements/` or `docs/tech_specs/`.
- **Related:** [Token Usage, Quotas, and Rate Limits](050_token_usage_quotas_spec_proposal.md), [External Model Routing](../tech_specs/external_model_routing.md), [API Egress Server](../tech_specs/api_egress_server.md), [Cloud Agents](../tech_specs/cloud_agents.md), [Personas and Task Scoping](integrated/personas_and_task_scoping_proposal.md).

This document defines how the system:

- Represents and configures provider-tier limits (e.g. RPM, TPM per API key or tier).
- Tracks per-credential usage and remaining quota so the orchestrator can consider "available API quota" when selecting a cloud model.
- Enforces pre-call checks and handles provider 429 (rate limit) responses.

## 2 Scope

- **In scope:** Cloud LLM providers reached via API Egress (OpenAI, Anthropic, and other OpenAI-compatible endpoints); per-credential and provider-tier quota configuration; pre-call quota check; 429 handling and optional retry/backoff.
- **Out of scope:** Internal token usage recording and user/project quotas (see [Token Usage, Quotas, and Rate Limits](050_token_usage_quotas_spec_proposal.md)); local inference limits; cost tracking (referenced but not defined here).

## 3 Proposed Requirements (Placeholder)

The following are proposed requirement labels for traceability; they would live in `docs/requirements/models.md` or `docs/requirements/apiegr.md` if accepted.

- **REQ-MODELS-0133 (proposed):** The system SHOULD support configurable provider-tier or per-credential limits (e.g. requests per minute, tokens per minute) for cloud LLM API calls so that the orchestrator can avoid exceeding provider limits and can consider "available API quota" when selecting a cloud model.
- **REQ-MODELS-0134 (proposed):** Before dispatching a cloud LLM call, the system SHOULD check that the selected credential has sufficient remaining quota (per configured limits); if not, the system MAY select another credential or deny the request with a clear error.
- **REQ-APIEGR-0128 (proposed):** When a cloud LLM provider returns a rate-limit response (e.g. HTTP 429), the API Egress Server SHOULD propagate the error to the orchestrator and MAY include retry-after or backoff guidance; the system MAY support automatic retry with backoff when policy allows.

## 4 Spec Items

Spec Items below define types, an operation, and a rule for cloud LLM API quota configuration, state, pre-call check, and 429 handling.

### 4.1 `CloudLLMQuotaConfig` Type

- Spec ID: `CYNAI.MODELS.CloudLLMQuotaConfig` <a id="spec-cynai-models-cloudllmquotaconfig"></a>
- Status: proposed
- See also: [External Model Routing - Settings](../tech_specs/external_model_routing.md#spec-cynai-orches-externalmodelrouting-settingsandconstraints)

#### 4.1.1 `CloudLLMQuotaConfig` Scope

Configuration for provider-tier or per-credential limits that constrain how much a given API key (credential) may be used for cloud LLM calls.
Used to avoid exceeding provider limits and to drive "available API quota" in model selection.

#### 4.1.2 `CloudLLMQuotaConfig` Fields

- `credential_id` (uuid, optional): When set, limits apply to this credential only; when unset, the config MAY apply as a default for a provider.
- `provider` (text): Logical provider name (e.g. `openai`, `anthropic`).
- `requests_per_minute` (integer, optional): Max completion requests per sliding 60-second window for this credential or provider default.
- `tokens_per_minute` (integer, optional): Max tokens (input + output) per sliding 60-second window.
- `tokens_per_day` (integer, optional): Max tokens per calendar day or rolling 24-hour window (implementation-defined).
- `source` (enum: configured | provider_docs | learned): How the limit was established; `configured` from operator settings, `provider_docs` from static documentation, `learned` from 429 headers or provider API when supported.

#### 4.1.3 `CloudLLMQuotaConfig` Storage

Configurations SHOULD be stored in PostgreSQL (e.g. a table or extension to existing settings) and be editable via admin API or system settings without code change.
Defaults MAY be provided per provider based on published tier limits.

#### 4.1.4 `CloudLLMQuotaConfig` Traces To

- [REQ-MODELS-0133 (proposed)](#3-proposed-requirements-placeholder)

### 4.2 `PerCredentialQuotaState` Type

- Spec ID: `CYNAI.MODELS.PerCredentialQuotaState` <a id="spec-cynai-models-percredentialquotastate"></a>
- Status: proposed

#### 4.2.1 `PerCredentialQuotaState` Scope

Runtime state used to decide whether a credential has "sufficient remaining quota" for a cloud LLM call.
Consumed by the pre-call quota check and by model selection (persona cloud model choice).

#### 4.2.2 `PerCredentialQuotaState` Fields

- `credential_id` (uuid)
- `provider` (text)
- Sliding-window counters (or equivalent): e.g. requests in the last 60 seconds, tokens in the last 60 seconds.
- Optional: tokens consumed in the current day (when `tokens_per_day` is configured).
- `last_updated_at` (timestamptz): When counters were last updated (after a completion or on pre-check).

#### 4.2.3 `PerCredentialQuotaState` Behavior

- After each successful cloud LLM completion, the system MUST update the state for the credential used (increment request count, add token count, update timestamp).
- State MAY be stored in memory with periodic persistence, or in a fast store (e.g. Redis) for distributed deployments; the spec does not prescribe storage.
- When a 429 is received, the system MAY update state to reflect that the provider has enforced a limit (e.g. treat current window as full until retry-after).

#### 4.2.4 `PerCredentialQuotaState` Traces To

- [REQ-MODELS-0133 (proposed)](#3-proposed-requirements-placeholder), [REQ-MODELS-0134 (proposed)](#3-proposed-requirements-placeholder)

### 4.3 `CloudLLMQuotaCheckBeforeCall` Operation

- Spec ID: `CYNAI.APIEGR.CloudLLMQuotaCheckBeforeCall` <a id="spec-cynai-apiegr-cloudllmquotacheckbeforecall"></a>
- Status: proposed

#### 4.3.1 `CloudLLMQuotaCheckBeforeCall` Inputs

- Request context: subject identity, task_id, provider, operation (e.g. chat_completion), chosen credential_id.
- Estimated token count for the request (e.g. prompt tokens + max_tokens) when available.

#### 4.3.2 `CloudLLMQuotaCheckBeforeCall` Outputs

- `allowed` (boolean): True if the credential has sufficient remaining quota for at least one request and, when applicable, for the estimated tokens.
- `reason` (text, optional): When not allowed, a short reason (e.g. "RPM exceeded", "TPM exceeded").

#### 4.3.3 `CloudLLMQuotaCheckBeforeCall` Behavior

- Resolve the effective quota config for the credential (or provider default).
- Load current per-credential state for the credential.
- If no config exists, the check MAY pass (no enforcement).
- If config exists: compare current window usage to limits; if adding one request (and optionally estimated tokens) would exceed limits, return allowed=false.
- If allowed=true, the caller proceeds to perform the outbound call; after completion, the system MUST update per-credential state.

#### 4.3.4 `CloudLLMQuotaCheckBeforeCall` Error Conditions

- When the credential is not found or inactive, the operation MUST not report allowed=true.
- When state store is unavailable, implementation MAY deny the request or fall back to a best-effort check; behavior MUST be configurable or documented.

#### 4.3.5 `CloudLLMQuotaCheckBeforeCall` Traces To

- [REQ-MODELS-0134 (proposed)](#3-proposed-requirements-placeholder), [REQ-APIEGR-0114](../requirements/apiegr.md#req-apiegr-0114)

### 4.4 `CloudLLM429Handling` Rule

- Spec ID: `CYNAI.APIEGR.CloudLLM429Handling` <a id="spec-cynai-apiegr-cloudllm429handling"></a>
- Status: proposed

#### 4.4.1 `CloudLLM429Handling` Scope

When a cloud LLM provider returns HTTP 429 (Too Many Requests), the API Egress Server and orchestrator behavior.

#### 4.4.2 `CloudLLM429Handling` Preconditions

- The outbound call was made through API Egress.
- The provider responded with HTTP 429 or a body indicating rate limit (e.g. OpenAI `rate_limit_exceeded`).

#### 4.4.3 `CloudLLM429Handling` Outcomes

- The API Egress Server MUST return a structured error to the orchestrator that includes: status 429, provider message when available, and `Retry-After` or equivalent when the provider supplies it.
- The system MAY update per-credential state so that the credential is not selected again until the retry-after window has passed or state is reset.
- The orchestrator MAY retry the call after backoff when policy allows; retry MUST use the same or a different credential per routing logic.
- The orchestrator MAY fall back to another cloud credential or to local inference when available and when persona/routing allows.

#### 4.4.4 `CloudLLM429Handling` Observability

- Every 429 response SHOULD be logged with credential_id (or redacted), provider, task_id, and retry-after when present.

#### 4.4.5 `CloudLLM429Handling` Traces To

- [REQ-APIEGR-0128 (proposed)](#3-proposed-requirements-placeholder)

### 4.5 Integration With Model Selection

- Spec ID: `CYNAI.MODELS.CloudQuotaInModelSelection` <a id="spec-cynai-models-cloudquotainmodelselection"></a>
- Status: proposed
- See also: [Personas and Task Scoping - Model Selection](integrated/personas_and_task_scoping_proposal.md)

#### 4.5.1 `CloudQuotaInModelSelection` Scope

The personas and task scoping proposal requires the orchestrator to consider "available API quota" when selecting a cloud model.
This spec defines how that is satisfied.

#### 4.5.2 `CloudQuotaInModelSelection` Behavior

- When the orchestrator selects a model from persona recommended cloud models, it MUST consider only providers that have an available API key (existing requirement).
- In addition, the orchestrator MUST treat a credential as having insufficient quota when `CloudLLMQuotaCheckBeforeCall` for that credential returns allowed=false.
- The orchestrator MUST NOT select a cloud model backed by a credential with insufficient quota; it MAY select another credential for the same provider or another provider, or report that no cloud model is available.

#### 4.5.3 `CloudQuotaInModelSelection` Traces To

- [REQ-MODELS-0134 (proposed)](#3-proposed-requirements-placeholder), [Personas and Task Scoping](integrated/personas_and_task_scoping_proposal.md) (available API quota).

## 5 Placement and Cross-References

- **Suggested placement:** As a new tech spec under **External Integration and Routing** or **Model Lifecycle** in [`docs/tech_specs/_main.md`](../tech_specs/_main.md).
  When promoted from draft, either:
  - `docs/tech_specs/cloud_llm_api_quotas.md`, or
  - Merge the cloud-quota content into [External Model Routing](../tech_specs/external_model_routing.md) and [API Egress Server](../tech_specs/api_egress_server.md) as new sections, with this doc as a single source of truth for the Spec IDs.
- **Token Usage, Quotas, and Rate Limits:** Records usage and enforces user/project quotas; this spec focuses on **provider-side and per-credential** limits and pre-call checks.
  Recording of token usage after a cloud call is defined in the token usage spec; this spec consumes that recording for per-credential state when applicable.
- **API Egress:** Per-user and per-task constraints (REQ-APIEGR-0114) are extended by per-credential quota checks and 429 handling defined here.
- **External Model Routing:** `model_routing.max_external_tokens` and `model_routing.max_external_cost_usd` remain global/routing constraints; this spec adds per-credential RPM/TPM so routing can avoid keys that are at limit.
