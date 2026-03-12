# Model Hub API Tool Specification (Draft)

- [Document Overview](#document-overview)
- [Integration Points](#integration-points)
- [Provider Model](#provider-model)
- [Egress Contract](#egress-contract)
- [Operations](#operations)
  - [Search Models](#search-models)
  - [Pull Model](#pull-model)
  - [Cache Resolution and Promotion](#cache-resolution-and-promotion)
- [Content and Model-Class Filtering](#content-and-model-class-filtering)
  - [Filter Configuration](#filter-configuration)
  - [Filter Application](#filter-application)
- [System Settings and Policy](#system-settings-and-policy)
- [Concrete Providers](#concrete-providers)
  - [Hugging Face](#hugging-face)
  - [Ollama Library](#ollama-library)
  - [ModelScope Hub](#modelscope-hub)
  - [Private or Custom Sources](#private-or-custom-sources)
- [Auditing and Safety](#auditing-and-safety)

## Document Overview

- Spec ID: `CYNAI.MODELS.Doc.ModelHubApiTool` <a id="spec-cynai-models-doc-modelhubapitool"></a>
- Status: draft
- See also: [Model Management](../tech_specs/model_management.md), [API Egress Server](../tech_specs/api_egress_server.md)

This document specifies a generic Model Hub API tool that integrates with the CyNodeAI orchestrator for searching model catalogs, pulling model artifacts, and populating the orchestrator model cache from multiple configurable providers.
Providers include public hubs (e.g. Hugging Face, Ollama library, ModelScope), private or custom model repositories, and any source that can be described by configuration (endpoint, auth, and capability flags).
The tool supports configurable filtering to exclude NSFW or adult-only models and to exclude or include specific model classes (e.g. by pipeline tag, library, or provider-specific type).

Traces To:

- [REQ-MODELS-0106](../requirements/models.md#req-models-0106) (user-directed cache population)
- [REQ-MODELS-0107](../requirements/models.md#req-models-0107) (policy-controlled, audited)
- [REQ-MODELS-0119](../requirements/models.md#req-models-0119) (external calls via API Egress)
- [REQ-MODELS-0124](../requirements/models.md#req-models-0124) (egress logging with task context)

## Integration Points

- **API Egress Server**: All outbound requests to provider APIs (REST, Hub, or custom endpoints) MUST be performed by the API Egress Server.
  Credentials (tokens, API keys, or endpoint-specific auth) MUST be stored and retrieved per [Credential Storage](../tech_specs/api_egress_server.md#spec-cynai-apiegr-credentialstorage); agents and sandboxes MUST NOT receive them.
- **Model Management**: Search results may be used to drive user-directed downloads; pull operations MUST populate the orchestrator model cache and registry per [User Directed Downloads](../tech_specs/model_management.md#spec-cynai-models-userdirecteddownloads) and [Model Cache](../tech_specs/model_management.md#spec-cynai-models-modelcache).
- **Access control**: The effective provider id and operations (`search_models`, `pull_model`, `resolve_cache`) MUST be enforced by API Egress access policy per [Access Control](../tech_specs/api_egress_server.md#spec-cynai-apiegr-accesscontrol).
  Policy MUST be defined over the set of enabled providers (and, for private sources, over configured source ids).

## Provider Model

- Spec ID: `CYNAI.MODELS.ModelHub.ProviderModel` <a id="spec-cynai-models-modelhub-providermodel"></a>

### Provider Identity and Registration

- Each model source is identified by a **provider id** (e.g. `huggingface`, `ollama_library`, `modelscope`, or a configured id for private sources).
- The set of allowed providers is defined by system settings and policy: `models.download.allowed_sources` (or equivalent) lists provider ids that may be used for pull/resolve; access control may further restrict which subjects can use which provider.
- **Built-in providers**: The system MAY ship with support for well-known providers (Hugging Face, Ollama library, ModelScope); each is identified by a fixed provider id and has defined parameter and response mappings (see [Concrete Providers](#concrete-providers)).
- **Configurable (private) providers**: Operators MAY register additional sources via configuration: endpoint base URL, credential reference, and capability flags (e.g. supports_search, supports_revision, artifact_type hint).
  Such sources use a configured provider id (e.g. `private_acme` or a stable id derived from config); they are treated as first-class providers for access control and auditing.

### Provider Capabilities

- **supports_search**: When true, the provider supports the `search_models` operation (catalog listing with optional query/filters).
  When false, only pull/resolve by known model id are supported (e.g. private blob store or simple registry).
- **supports_revision**: When true, the provider accepts a revision (branch, tag, or commit) in addition to model id; when false, a single canonical artifact per model id is assumed.
- **artifact_types**: Optional hint of supported artifact types (e.g. safetensors, gguf, ollama); used for normalized pull params and for filtering.
- Provider-specific parameters (e.g. pipeline_tag, library for Hugging Face) are passed through in `params` and documented per provider; the API Egress implementation maps them to the upstream API.

### Request Routing

- The tool is exposed to agents as an MCP tool (or equivalent) that submits requests including a **provider** (provider id).
- The orchestrator routes approved calls to the API Egress Server with that provider id, so the server can select the correct backend and credential.
- Unknown or disabled provider id MUST result in a policy/permission error before any outbound call.

## Egress Contract

- Spec ID: `CYNAI.MODELS.ModelHub.EgressContract` <a id="spec-cynai-models-modelhub-egresscontract"></a>

Requests to the API Egress Server for the model hub tool MUST use a single logical provider namespace so that allowlists and credentials can be managed per provider.
Concretely, the egress layer MAY treat "model hub" as one provider (e.g. `model_hub`) and pass the actual source as a parameter, or MAY use the source provider id directly as the egress provider (e.g. `huggingface`, `ollama_library`).
The following assumes the **provider id is the egress provider** for simplicity; implementations MAY use a wrapper provider with `params.provider` if preferred.

- `provider`: provider id (e.g. `huggingface`, `ollama_library`, or a configured private id).
- `operation`: one of `search_models`, `pull_model`, `resolve_cache`.
- `params`: operation-specific JSON including provider-specific fields where applicable (see [Operations](#operations) and [Concrete Providers](#concrete-providers)).
- `task_id`: task context for auditing.

Responses MUST follow the API Egress minimum response shape: `status` (success|error), `result` (normalized JSON), and when error, structured `error` object.
Responses MUST NOT include raw API keys or tokens.

## Operations

Operations are invoked via the API Egress Server with the chosen `provider` and the operation names below.
Parameter names are normalized where possible; provider-specific parameters are documented in [Concrete Providers](#concrete-providers).

### Search Models

- Spec ID: `CYNAI.MODELS.ModelHub.SearchModels` <a id="spec-cynai-models-modelhub-searchmodels"></a>

Operation name: `search_models`.
Supported only when the provider has `supports_search` true; otherwise the server MUST return an error (e.g. operation not supported for this provider).

#### `SearchModels` Inputs

- `search` (string, optional): free-text search query; semantics are provider-specific.
- `author` (string, optional): filter by model author or organization; optional per provider.
- `model_class` (string or array of strings, optional): normalized model class filter (e.g. pipeline tag, task type, or library); provider maps to native concepts.
- `limit` (integer, optional): maximum number of models to return; upper bound defined by system settings (global or per-provider).
- `sort` (string, optional): sort key (e.g. downloads, likes, updated); default implementation-defined; provider-specific.
- `filters` (object, optional): content and model-class filter overrides for this request (see [Content and Model-Class Filtering](#content-and-model-class-filtering)); only when policy allows.
- Provider-specific params (e.g. `pipeline_tag`, `library` for Hugging Face): allowed in `params`; see [Concrete Providers](#concrete-providers).

#### `SearchModels` Outputs

- `models`: array of model summary objects.
  Each object MUST include at least: `id` (provider-scoped model id), `author` (when available), `model_class` or equivalent (normalized or provider-specific), and a flag or list indicating whether the model was excluded by content/model-class filters (so clients can optionally show filtered counts).
  Optional: `downloads`, `likes`, `revision`, `source_uri`, `provider` (echo).
- `total_count` (integer, optional): total matching count before limit, when provided by the upstream API.
- `applied_filters`: summary of which content/model-class filters were applied (for transparency).
- `provider`: provider id that was queried (echo).

#### `SearchModels` Behavior

- The API Egress Server resolves the provider and calls the corresponding upstream API (or configured endpoint) with the given params.
- Before returning, the server MUST apply configured content and model-class filters (see [Filter Application](#filter-application)) and remove or mark excluded models.
- Results MUST be normalized to the `models` shape above; no raw credentials or internal tokens in response.

#### `SearchModels` Error Conditions

- Provider does not support search: return operation-not-supported error.
- Missing or invalid credential for the provider: return error, do not retry with empty token.
- Upstream rate limit or transient failure: return structured error with retry-after when available.
- Policy denies `search_models` for the subject or provider: return permission error.

### Pull Model

- Spec ID: `CYNAI.MODELS.ModelHub.PullModel` <a id="spec-cynai-models-modelhub-pullmodel"></a>

Operation name: `pull_model`.

#### `PullModel` Inputs

- `model_id` (string, required): provider-scoped model id (e.g. `org/model-name` for Hugging Face, `llama3.2` for Ollama library).
- `revision` (string, optional): branch, tag, or commit when provider supports it; default provider-specific (e.g. `main`).
- `artifact_types` (array of strings, optional): requested artifact types (e.g. safetensors, gguf, ollama); default implementation-defined per provider.
- `skip_content_check` (boolean, optional): if true and policy allows, skip content-filter check for this pull (e.g. pre-vetted models); default false.
- Provider-specific params: allowed; see [Concrete Providers](#concrete-providers).

#### `PullModel` Outputs

- `cache_key` or `artifact_ids`: identifier(s) for the cached artifact(s) in the orchestrator model cache/registry.
- `sha256` (or equivalent): integrity metadata per [Model Cache](../tech_specs/model_management.md#spec-cynai-models-modelcache).
- `size_bytes`: total size of pulled artifact(s).
- `source_uri`: upstream source URI for audit and display.
- `provider`: provider id (echo).

#### `PullModel` Behavior

- Resolve `model_id` (and `revision` when supported) via the provider using API Egress.
- Unless `skip_content_check` is true and policy allows, apply content and model-class filters; if the model is excluded, return an error and do not download.
- Download artifact(s) through the API Egress Server; verify integrity (e.g. sha256) and store in the orchestrator model cache; record in model registry and artifact table per [Model Management](../tech_specs/model_management.md).
- Record audit event: who requested, provider, model_id, revision, cache outcome, and applied filters.

#### `PullModel` Error Conditions

- Model not found or revision invalid: return not-found error.
- Model excluded by content or model-class filter: return policy error with reason (e.g. "excluded by NSFW filter").
- Credential missing or invalid: return error.
- Download or integrity check failure: return error and do not register partial artifact in cache.
- Policy denies `pull_model` for the subject or provider: return permission error.

### Cache Resolution and Promotion

- Spec ID: `CYNAI.MODELS.ModelHub.ResolveCache` <a id="spec-cynai-models-modelhub-resolvecache"></a>

Operation name: `resolve_cache` (or `ensure_cached`).

#### `ResolveCache` Inputs

- `model_id` (string, required): provider-scoped model id.
- `revision` (string, optional): when provider supports it; default provider-specific.
- If the artifact is already in the cache (same provider, model_id, revision, and integrity), return cache metadata without re-downloading.

#### `ResolveCache` Outputs

- `cached`: boolean; true if artifact was already present or successfully pulled.
- `cache_key` / `artifact_ids`, `sha256`, `size_bytes`, `source_uri`, `provider`: same as Pull when cached or newly pulled.

#### `ResolveCache` Behavior

- Look up orchestrator cache by provider, `model_id`, and `revision` (and optionally expected hash if provided).
- If found, return cache metadata.
- If not found, perform the same steps as Pull Model (including content/model-class filter check unless overridden by policy), then return new cache metadata.
- All outbound calls MUST go through API Egress; all cache writes MUST go through the model management layer.

#### `ResolveCache` Error Conditions

- Same as Pull Model when a pull is required; plus policy denial for `resolve_cache` when applicable.

## Content and Model-Class Filtering

- Spec ID: `CYNAI.MODELS.ModelHub.ContentFiltering` <a id="spec-cynai-models-modelhub-contentfiltering"></a>

The tool MUST support configurable filters to exclude models that are tagged or classified as NSFW or adult-only, and to exclude or restrict by model class (e.g. pipeline tag, library, or provider-specific type).
Filtering is applied uniformly across providers where the provider exposes the necessary metadata; providers that do not expose such metadata MAY be configured to allow or block entirely (e.g. private allowlisted sources may skip content filter when policy allows).

Traces To:

- [REQ-MODELS-0107](../requirements/models.md#req-models-0107) (policy-controlled)
- [REQ-MODELS-0108](../requirements/models.md#req-models-0108) (configurable via system settings)

### Filter Configuration

- Spec ID: `CYNAI.MODELS.ModelHub.FilterConfig` <a id="spec-cynai-models-modelhub-filterconfig"></a>

Configuration MUST be defined in system settings and MAY be overridden per provider or per request (when policy allows).

- **Global defaults** (apply to all providers that support the signal):
  - `models.hub.filters.exclude_nsfw` (boolean): exclude models marked NSFW or adult-only when true.
  - `models.hub.filters.excluded_model_classes` (array of strings): normalized or provider-agnostic class names to exclude (e.g. pipeline tag, library, or task type).
  - `models.hub.filters.allowed_model_classes` (array of strings, optional): when non-empty, allowlist; only these classes are allowed; exclude list applied in addition.
- **Per-provider overrides**: Optional keys such as `models.hub.providers.<provider_id>.filters.exclude_nsfw`, `excluded_model_classes`, `allowed_model_classes` to override global defaults for a given provider.
- **Default for search vs pull**: Filters MAY have different defaults for search (e.g. hide NSFW in listing) vs pull (e.g. block pull of NSFW even if search was bypassed); both MUST be configurable and audited.
- **Private/custom sources**: For configured private providers, policy MAY allow skipping content filter (e.g. trusted internal repo); when skipped, the decision MUST be audited.

### Filter Application

- Spec ID: `CYNAI.MODELS.ModelHub.FilterApplication` <a id="spec-cynai-models-modelhub-filterapplication"></a>

#### `FilterApplication` Scope

- Applies to `search_models` (filter results before returning) and to `pull_model` / `resolve_cache` (block pull if model is excluded).
- Per-request `filters` in search, or `skip_content_check` in pull, MAY override only when explicitly allowed by policy (e.g. admin or pre-vetted list).

#### `FilterApplication` Preconditions

- Filter configuration MUST be loaded from system settings (and per-provider overrides and request overrides) before calling the upstream API or returning results.
- When the upstream API does not expose NSFW/adult or model-class metadata in list responses, the implementation MAY fetch model card or metadata per candidate when exclude_nsfw is true, or apply heuristics; the spec requires that exclusion be applied when the system has or can obtain the signal.

#### `FilterApplication` Outcomes

- Search: Excluded models are omitted from the `models` array (or marked as excluded with a reason when the contract includes that field).
- Pull / Resolve: If the model is excluded, the operation returns an error and does not write to cache.
- All filter decisions (applied filters, overrides, result) SHOULD be recorded in audit logs for compliance.

#### `FilterApplication` Error Conditions

- Misconfiguration (e.g. allowlist and blocklist contradict): implementation SHOULD fail safe (e.g. exclude ambiguous models or refuse to run until config is fixed).

## System Settings and Policy

- Spec ID: `CYNAI.MODELS.ModelHub.SystemSettings` <a id="spec-cynai-models-modelhub-systemsettings"></a>

Suggested system setting keys (align with [Model Management](../tech_specs/model_management.md) (system settings and constraints)):

- **Allowed sources**: `models.download.allowed_sources` (array of strings): list of provider ids allowed for pull/resolve (and search when the provider supports it).
  MUST include each provider id that the deployment wishes to use (e.g. `huggingface`, `ollama_library`, or configured private id).
- **Global filters**: `models.hub.filters.exclude_nsfw`, `models.hub.filters.excluded_model_classes`, `models.hub.filters.allowed_model_classes` (see [Filter Configuration](#filter-configuration)).
- **Search limit**: `models.hub.search.max_limit` (integer): global upper bound for search `limit`; optional per-provider override via `models.hub.providers.<id>.search.max_limit`.
- **Per-provider config**: Optional `models.hub.providers.<provider_id>.*` for filter overrides, search limits, and (for private sources) endpoint URL, credential reference, and capability flags.
- **Private provider registration**: Configuration schema for adding a custom provider: at minimum a unique provider id, endpoint base URL (or registry type), credential reference, and capabilities (supports_search, supports_revision, optional artifact_types).

Access control MUST restrict which subjects can call `search_models`, `pull_model`, and `resolve_cache` per provider; and whether `skip_content_check` or per-request filter overrides are allowed.

## Concrete Providers

This section defines the first-class and configurable provider types.
Each built-in provider has a fixed provider id and a defined mapping of normalized params to upstream API semantics.

### Hugging Face

- Spec ID: `CYNAI.MODELS.ModelHub.Provider.HuggingFace` <a id="spec-cynai-models-modelhub-provider-huggingface"></a>

- **Provider id**: `huggingface`.
- **Capabilities**: supports_search, supports_revision; artifact types e.g. safetensors, gguf, onnx.
- **Credentials**: Hugging Face token (stored per API Egress; optional for public read).
- **Search params**: `search`, `author`, `model_class` (maps to pipeline_tag and/or library); provider-specific `pipeline_tag`, `library` accepted; `limit`, `sort` (e.g. downloads, likes).
- **Pull params**: `model_id` (e.g. `org/model-name`), `revision` (default `main`), `artifact_types`.
- **Filter mapping**: exclude_nsfw uses Hub model card tags or API metadata when available; model_class maps to pipeline_tag and library on the Hub.
- **Suggested setting keys**: `models.hub.providers.huggingface.filters.*`, `models.hub.providers.huggingface.search.max_limit` (optional overrides).

### Ollama Library

- Spec ID: `CYNAI.MODELS.ModelHub.Provider.OllamaLibrary` <a id="spec-cynai-models-modelhub-provider-ollamalibrary"></a>

- **Provider id**: `ollama_library`.
- **Capabilities**: supports_search (curated list; limited vs full-text), supports_revision in the sense of tags; artifact type ollama.
- **Credentials**: Optional (public registry); or token if using a private Ollama registry endpoint.
- **Search params**: `search` (optional name/tag filter), `limit`, `sort`; no pipeline_tag/library; model_class may map to tag categories if the registry exposes them.
- **Pull params**: `model_id` (e.g. `llama3.2`, `qwen2.5-coder`), optional `revision`/tag; artifact type typically ollama.
- **Filter mapping**: exclude_nsfw and model_class if the registry exposes tags; otherwise filtering may be minimal for the official curated list.
- **Note**: Aligns with existing use of Ollama for local inference; pull populates cache so nodes load from orchestrator cache instead of public internet.

### ModelScope Hub

- Spec ID: `CYNAI.MODELS.ModelHub.Provider.ModelScope` <a id="spec-cynai-models-modelhub-provider-modelscope"></a>

- **Provider id**: `modelscope`.
- **Capabilities**: supports_search, supports_revision; artifact types similar to Hugging Face.
- **Credentials**: ModelScope token or equivalent (stored per API Egress).
- **Search params**: Similar in spirit to Hugging Face (search, author, task/library-like filters, limit, sort); exact keys are ModelScope API-specific.
- **Pull params**: `model_id`, `revision`, `artifact_types`; mapping to ModelScope API.
- **Filter mapping**: exclude_nsfw and model_class per ModelScope metadata/tags when available.

### Private or Custom Sources

- Spec ID: `CYNAI.MODELS.ModelHub.Provider.Private` <a id="spec-cynai-models-modelhub-provider-private"></a>

- **Provider id**: Configured (e.g. `private_acme`, or a stable id derived from config).
- **Capabilities**: Configurable; typically supports_pull at minimum; supports_search and supports_revision optional (e.g. HTTP index or registry API).
- **Configuration**: Endpoint base URL, credential reference (or no auth), capability flags; optional schema for listing (e.g. path pattern, index URL).
- **Params**: `model_id` (and optionally `revision`) mapped to the custom API or path; search params if supports_search (provider-specific).
- **Filtering**: Policy MAY allow skipping content filter for trusted private sources; when skipped, MUST be audited.
- **Implementation**: The API Egress Server (or a dedicated model-hub backend) resolves the configured provider id to endpoint and credential, then performs HTTP/API calls or artifact fetch per the configured contract; normalization of responses to the common `models` and cache output shape is required.

## Auditing and Safety

- Spec ID: `CYNAI.MODELS.ModelHub.AuditingSafety` <a id="spec-cynai-models-modelhub-auditingsafety"></a>

Traces To:

- [REQ-MODELS-0109](../requirements/models.md#req-models-0109)
- [REQ-MODELS-0110](../requirements/models.md#req-models-0110)
- [REQ-MODELS-0124](../requirements/models.md#req-models-0124)

- Every call to search, pull, or resolve MUST be logged with task context, subject identity, **provider id**, operation name, and key params (e.g. model_id, revision; not full response bodies).
- Model downloads and cache writes MUST be recorded; integrity (sha256) MUST be verified before exposing artifacts to nodes per [Model Management](../tech_specs/model_management.md) (auditing and safety).
- When content or model-class filters are applied, the audit log SHOULD include which filters caused exclusion (e.g. "excluded by exclude_nsfw") for pull and for search when exclusions are applied.
- For private providers, the configured provider id and (when applicable) source endpoint or path SHOULD be included in audit context for traceability.
