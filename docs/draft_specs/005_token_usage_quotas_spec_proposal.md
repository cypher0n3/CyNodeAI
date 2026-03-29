# Token Usage, Quotas, and Rate Limits Spec Proposal

## 1 Summary

- **Date:** 2026-03-06
- **Purpose:** Draft spec for tracking token usage, enforcing quotas, and applying rate limits across chat completions, external model calls, and local inference.
- **Status:** Draft only; not yet merged into `docs/requirements/` or `docs/tech_specs/`.
- **Related:** [External Model Routing](../tech_specs/external_model_routing.md) (max_external_tokens, max_external_cost_usd), [OpenAI-Compatible Chat API](../tech_specs/openai_compatible_chat_api.md), [API Egress Server](../tech_specs/api_egress_server.md), [Cloud Agents](../tech_specs/cloud_agents.md) (rate_limits).

This document proposes requirements and spec additions for:

- Recording token counts (input, output, total) per completion and per inference path.
- Quotas (per user, per project, per model, per time window).
- Rate limiting (requests per minute, tokens per minute).
- Cost tracking for external providers when provider pricing data is available.
- Storage schema and Data REST API for usage and quota state.

## 2 Scope

- **Recording:** Every chat completion and inference call (local or external) SHOULD record token usage.
- **Quotas:** Configurable limits per user, project, or group over rolling windows (e.g. daily, monthly).
- **Rate limits:** Per-user and per-task request and token throughput limits.
- **Cost:** Optional cost attribution when external provider pricing is configured.
- **API:** Data REST API endpoints for usage summaries and quota status.
- **Integration:** Chat pipeline, API Egress, worker inference reporting.

## 3 Proposed Requirements

The following requirement IDs are **proposed** and would live in `docs/requirements/models.md` if accepted.

### 3.1 Recording (MODELS)

- **REQ-MODELS-0126 (proposed):** The system SHOULD record token usage (input tokens, output tokens, total) for each chat completion and inference call.
  Recording MUST include: subject identity (user_id or equivalent), project_id when in scope, model identifier, inference path (local vs external provider), and timestamp.
  [CYNAI.MODELS.TokenUsageRecording](#spec-cynai-models-tokenusagerecording)
  <a id="req-models-0126"></a>

- **REQ-MODELS-0127 (proposed):** When an external provider returns usage metadata (e.g. OpenAI `usage` object), the system MUST persist that metadata with the completion record.
  For local inference, the system SHOULD obtain and persist token counts when the inference backend reports them (e.g. Ollama response metadata).
  [CYNAI.MODELS.TokenUsageRecording](#spec-cynai-models-tokenusagerecording)
  <a id="req-models-0127"></a>

### 3.2 Quotas (MODELS)

- **REQ-MODELS-0128 (proposed):** The system MAY enforce configurable quotas (e.g. max tokens per user per day, max tokens per project per month).
  If supported, quota limits MUST be configurable via PostgreSQL settings or preferences.
  Quota scope MAY be user, project, or group.
  [CYNAI.MODELS.UsageQuotas](#spec-cynai-models-usagequotas)
  <a id="req-models-0128"></a>

- **REQ-MODELS-0129 (proposed):** When a quota is exceeded, the system MUST deny the request and return a clear error indicating which quota was exceeded.
  The gateway MUST NOT leak internal quota state in error messages beyond what is necessary for the user to understand the denial.
  [CYNAI.MODELS.UsageQuotas](#spec-cynai-models-usagequotas)
  <a id="req-models-0129"></a>

### 3.3 Rate Limits (MODELS / APIEGR)

- **REQ-MODELS-0130 (proposed):** The system MAY enforce rate limits (requests per minute, tokens per minute) per user or per task.
  Rate limits SHOULD be configurable and MAY differ for local vs external inference.
  [CYNAI.MODELS.UsageRateLimits](#spec-cynai-models-usageratelimits)
  [CYNAI.APIEGR.PerTaskConstraints](../tech_specs/api_egress_server.md#spec-cynai-apiegr-accesscontrol)
  <a id="req-models-0130"></a>

### 3.4 Cost Tracking (MODELS)

- **REQ-MODELS-0131 (proposed):** When external provider pricing is configured, the system MAY compute and store estimated cost per completion.
  Cost tracking MUST NOT be required for MVP; when implemented, it MUST use provider-specific pricing tables and MUST NOT store provider API keys or billing secrets.
  [CYNAI.MODELS.UsageCostTracking](#spec-cynai-models-usagecosttracking)
  <a id="req-models-0131"></a>

### 3.5 API and Visibility (USRGWY / DATAPI)

- **REQ-MODELS-0132 (proposed):** The Data REST API SHOULD expose endpoints for usage summaries (e.g. tokens consumed per user/project over a time range) and quota status.
  Access MUST be subject to the same authorization rules as other Data REST API resources (user sees own usage; admins see scoped usage).
  [CYNAI.MODELS.UsageDataApi](#spec-cynai-models-usagedataapi)
  <a id="req-models-0132"></a>

## 4 Proposed Spec Additions

These would extend or reference existing specs.
Each Spec Item follows the mandatory structure per [spec_authoring_writing_and_validation.md](../docs_standards/spec_authoring_writing_and_validation.md).

### 4.1 Token Usage Recording

- Spec ID: `CYNAI.MODELS.TokenUsageRecording` <a id="spec-cynai-models-tokenusagerecording"></a>
- Status: proposed
- Traces To: [REQ-MODELS-0126](#req-models-0126), [REQ-MODELS-0127](#req-models-0127)

#### 4.1.1 `TokenUsageRecording` Scope

Token usage is recorded for every inference completion that flows through the orchestrator.
This includes:

- Chat completions via `POST /v1/chat/completions` (PM agent path and direct inference path).
- External model calls via API Egress.
- Local inference on worker nodes (when the worker or inference proxy reports usage).

#### 4.1.2 `TokenUsageRecording` Data Model

Each usage record MUST include at minimum:

- `id` (uuid, primary key)
- `subject_id` (uuid; user or service identity)
- `project_id` (uuid, nullable; when request is project-scoped)
- `model_id` (text; effective model identifier, e.g. `cynodeai.pm`, `gpt-4`, `llama3.2`)
- `inference_path` (text; one of: `local`, `external`)
- `provider` (text, nullable; external provider name when inference_path is external)
- `input_tokens` (integer, non-negative)
- `output_tokens` (integer, non-negative)
- `total_tokens` (integer, non-negative; input + output)
- `created_at` (timestamptz)
- `task_id` (uuid, nullable; when completion is task-scoped)
- `thread_id` (uuid, nullable; when completion is chat-thread-scoped)

Optional fields:

- `cost_usd` (numeric, nullable; when cost tracking is enabled)
- `run_id` (uuid, nullable; for run/session correlation)

#### 4.1.3 `TokenUsageRecording` Integration Points

- **Chat pipeline:** After step 7 (persist assistant output) in [OpenAI Chat API Pipeline](../tech_specs/openai_compatible_chat_api.md#spec-cynai-usrgwy-openaichatapi-pipeline), the orchestrator MUST record usage when the completion response includes a `usage` object.
  If the PM agent or inference backend does not return usage, the orchestrator MAY record zero or omit the record; the spec SHOULD define whether omission is allowed for MVP.
- **API Egress:** When API Egress performs an external model call and receives a response with usage metadata, it MUST return that metadata to the orchestrator.
  The orchestrator MUST record it per this spec.
- **Worker inference:** When a worker or inference proxy returns token counts (e.g. Ollama `eval_count`), the orchestrator MUST record them.

#### 4.1.4 `TokenUsageRecording` Retention

Usage records SHOULD be subject to configurable retention (e.g. 90 days for detail, longer for aggregated rollups).
Retention policy is out of scope for this spec but SHOULD align with [CYNAI.USRGWY.RetentionPolicies](../tech_specs/runs_and_sessions_api.md#spec-cynai-usrgwy-retention).

### 4.2 Usage Quotas

- Spec ID: `CYNAI.MODELS.UsageQuotas` <a id="spec-cynai-models-usagequotas"></a>
- Status: proposed
- Traces To: [REQ-MODELS-0128](#req-models-0128), [REQ-MODELS-0129](#req-models-0129)

#### 4.2.1 `UsageQuotas` Scope

Quotas limit total token consumption (or cost) over a rolling time window.
Quotas are optional; when not configured, no quota enforcement is applied.

#### 4.2.2 `UsageQuotas` Quota Definition

A quota definition includes:

- `scope` (user | project | group)
- `scope_id` (uuid; the user, project, or group id)
- `limit_tokens` (integer, optional; max total tokens in the window)
- `limit_cost_usd` (numeric, optional; max cost in USD in the window)
- `window` (enum: daily | monthly; or a duration)
- `inference_path_filter` (optional; apply only to local or external)

#### 4.2.3 `UsageQuotas` Enforcement Algorithm

<a id="algo-cynai-models-usagequotas-enforcement"></a>

1. Before dispatching a completion request, resolve the effective subject and project from the request context.
2. Load all active quotas that apply to that subject/project.
3. For each quota, compute current consumption in the quota window (sum of `total_tokens` or `cost_usd` from usage records).
4. If adding the estimated or actual tokens for this request would exceed any quota, deny the request.
5. If quotas are not exceeded, allow the request and record usage after completion.

Pre-check (step 4) MAY use an estimate (e.g. prompt token count + max_tokens) when exact output tokens are unknown.
Post-completion, the actual usage MUST be recorded; if a quota was exceeded by the actual completion, the system SHOULD log a warning and MAY apply a grace period or soft limit for the next request.

#### 4.2.4 `UsageQuotas` Settings

Suggested setting keys (extend the existing routing settings in [`external_model_routing.md`](../tech_specs/external_model_routing.md)):

- `usage_quotas.enabled` (boolean)
- `usage_quotas.user_daily_tokens` (integer, optional)
- `usage_quotas.project_monthly_tokens` (integer, optional)
- `usage_quotas.external_max_cost_usd_per_month` (numeric, optional; aligns with `model_routing.max_external_cost_usd`)

### 4.3 Usage Rate Limits

- Spec ID: `CYNAI.MODELS.UsageRateLimits` <a id="spec-cynai-models-usageratelimits"></a>
- Status: proposed
- Traces To: [REQ-MODELS-0130](#req-models-0130)

#### 4.3.1 `UsageRateLimits` Scope

Rate limits throttle request throughput and token throughput over a sliding window (e.g. per minute).
They complement quotas (which limit total consumption over longer windows).

#### 4.3.2 `UsageRateLimits` Limit Types

- **Requests per minute (RPM):** Maximum number of completion requests per subject (or per task) in a sliding 60-second window.
- **Tokens per minute (TPM):** Maximum number of tokens (input + output) per subject (or per task) in a sliding 60-second window.

#### 4.3.3 `UsageRateLimits` Enforcement

Rate limit checks MUST occur before the completion request is dispatched.
When a limit is exceeded, the gateway MUST return HTTP 429 with a `Retry-After` header when the client may retry.

Implementation MAY use a token-bucket or sliding-window counter.
The spec does not prescribe a specific algorithm; the implementation MUST be consistent and MUST NOT allow bursts that exceed the configured limit over the window.

#### 4.3.4 `UsageRateLimits` Settings

- `usage_rate_limits.requests_per_minute` (integer, optional)
- `usage_rate_limits.tokens_per_minute` (integer, optional)
- `usage_rate_limits.scope` (user | task; default: user)

API Egress already has [REQ-APIEGR-0114](../requirements/apiegr.md#req-apiegr-0114) for per-user and per-task constraints; rate limits for external calls SHOULD be enforced at the API Egress layer or the orchestrator layer before the call is made.

### 4.4 Usage Cost Tracking

- Spec ID: `CYNAI.MODELS.UsageCostTracking` <a id="spec-cynai-models-usagecosttracking"></a>
- Status: proposed
- Traces To: [REQ-MODELS-0131](#req-models-0131)

#### 4.4.1 `UsageCostTracking` Scope

Cost tracking is optional and applies only to external provider calls when pricing data is configured.
Local inference has no direct per-token cost in the same sense; cost tracking for local MAY be omitted or use an internal allocation model.

#### 4.4.2 `UsageCostTracking` Pricing Model

When enabled, the system uses a pricing table (configurable, not from provider API) that maps:

- (provider, model_id) => (input cost per 1K tokens, output cost per 1K tokens)

The system computes: `cost_usd = (input_tokens/1000 * input_price) + (output_tokens/1000 * output_price)`.

Pricing tables MUST be stored in PostgreSQL or config; they MUST NOT be fetched from provider APIs at request time.
Operators are responsible for keeping pricing up to date.

#### 4.4.3 `UsageCostTracking` Storage

When cost is computed, it is stored in the usage record `cost_usd` field.
Cost is used for quota enforcement (when `limit_cost_usd` is set) and for usage reporting.

### 4.5 Usage Data API

- Spec ID: `CYNAI.MODELS.UsageDataApi` <a id="spec-cynai-models-usagedataapi"></a>
- Status: proposed
- Traces To: [REQ-MODELS-0132](#req-models-0132)

#### 4.5.1 `UsageDataApi` Endpoints

Proposed Data REST API endpoints:

- `GET /v1/usage/summary` - Aggregated usage for the authenticated user (or for a scope the user is authorized to query).
  Query params: `scope` (user|project|group), `scope_id`, `from` (ISO8601), `to` (ISO8601), `group_by` (day|month).
  Response: `{ total_tokens, input_tokens, output_tokens, cost_usd?, by_model: [...], by_inference_path: {...} }`.
- `GET /v1/usage/quotas` - Current quota status for the authenticated user or specified scope.
  Response: `{ quotas: [{ scope, limit_tokens, consumed, remaining, window_end }] }`.

#### 4.5.2 `UsageDataApi` Authorization

- Users MAY query their own usage summary and quota status.
- Users with project admin or group admin role MAY query usage for their project or group.
- System admins MAY query usage for any scope.
- All access MUST be audited.

## 5 Schema Additions

The following table is proposed for PostgreSQL (see [postgres_schema.md](../tech_specs/postgres_schema.md) for schema conventions).

### 5.1 Table: `inference_usage`

- `id` (uuid, pk, default gen_random_uuid())
- `subject_id` (uuid, not null)
- `project_id` (uuid, nullable)
- `model_id` (text, not null)
- `inference_path` (text, not null; `local` | `external`)
- `provider` (text, nullable)
- `input_tokens` (integer, not null, default 0)
- `output_tokens` (integer, not null, default 0)
- `total_tokens` (integer, not null, default 0)
- `cost_usd` (numeric, nullable)
- `task_id` (uuid, nullable)
- `thread_id` (uuid, nullable)
- `run_id` (uuid, nullable)
- `created_at` (timestamptz, not null, default now())

#### 5.1.1 Index Definitions

- (`subject_id`, `created_at`) for user-scoped aggregation
- (`project_id`, `created_at`) for project-scoped aggregation
- (`created_at`) for retention purge

## 6 Integration With Existing Specs

- **OpenAI Chat API:** Add a step 7.5 or extend step 8: after persisting the assistant message, record usage from the completion response `usage` object.
- **External Model Routing:** The `model_routing.max_external_tokens` and `model_routing.max_external_cost_usd` settings align with quota limits; this spec defines the recording and enforcement mechanism.
- **API Egress:** API Egress returns usage metadata to the orchestrator; the orchestrator records it.
  Rate limits for external calls may be enforced at API Egress (per REQ-APIEGR-0114) or at the orchestrator before the call.
- **Cloud Agents:** Cloud worker `rate_limits` (RPM, TPM) align with this spec; cloud workers may report usage or the orchestrator may aggregate from API Egress responses.

## 7 Open Questions

- Whether to record usage when the completion fails (e.g. partial tokens before timeout).
- Whether quota enforcement should be pre-check only or also post-check with rollback semantics.
- Whether the usage summary API should support export (CSV, etc.) for billing reconciliation.
- Retention default and whether to support aggregated rollup tables for long-term reporting.
