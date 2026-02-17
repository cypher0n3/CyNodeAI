# Model Management

- [Document Overview](#document-overview)
- [Model Cache](#model-cache)
- [Model Registry](#model-registry)
- [Node Load Workflow](#node-load-workflow)
- [User Directed Downloads](#user-directed-downloads)
- [Preferences and Constraints](#preferences-and-constraints)
- [Auditing and Safety](#auditing-and-safety)

## Document Overview

- Spec ID: `CYNAI.MODELS.Doc.ModelManagement` <a id="spec-cynai-models-doc-modelmanagement"></a>

This document defines how the orchestrator manages local model artifacts and model metadata.
It includes a local model cache, a model registry in PostgreSQL, and workflows for worker nodes to load models without pulling from the public internet.

## Model Cache

- Spec ID: `CYNAI.MODELS.ModelCache` <a id="spec-cynai-models-modelcache"></a>

The orchestrator maintains a local cache of model artifacts.
Worker nodes SHOULD load models from the orchestrator cache instead of downloading from external sources.

Traces To:

- [REQ-MODELS-0100](../requirements/models.md#req-models-0100)
- [REQ-MODELS-0101](../requirements/models.md#req-models-0101)
- [REQ-MODELS-0102](../requirements/models.md#req-models-0102)
- [REQ-MODELS-0103](../requirements/models.md#req-models-0103)

Cache properties

- The cache SHOULD be content-addressed by a strong hash (e.g. sha256).
- The cache MUST record artifact size and integrity metadata.
- The cache MAY store multiple formats for the same logical model (e.g. Ollama blobs, gguf files).

## Model Registry

The model registry stores model metadata used for routing and verification.
It allows the Project Manager Agent to select appropriate models for tasks based on capabilities and availability.
The Postgres schema is defined in [`docs/tech_specs/postgres_schema.md`](postgres_schema.md).
See [Model Registry](postgres_schema.md#model-registry).

### Models Table

- `id` (uuid, pk)
- `name` (text)
  - logical name (e.g. llama3, qwen2.5-coder)
- `vendor` (text, nullable)
- `description` (text, nullable)
- `created_at` (timestamptz)
- `updated_at` (timestamptz)
- `updated_by` (text)

Constraints

- Unique: (`name`, `vendor`)
- Index: (`name`)

### Model Versions Table

- `id` (uuid, pk)
- `model_id` (uuid)
  - foreign key to `models.id`
- `version` (text)
  - semantic version, tag, or digest label
- `capabilities` (jsonb)
  - examples: context_length, tool_use, vision, json_mode, languages, gpu_required
- `default_parameters` (jsonb, nullable)
  - examples: temperature, top_p
- `created_at` (timestamptz)
- `updated_at` (timestamptz)

Constraints

- Unique: (`model_id`, `version`)
- Index: (`model_id`)

### Model Artifacts Table

- `id` (uuid, pk)
- `model_version_id` (uuid)
  - foreign key to `model_versions.id`
- `artifact_type` (text)
  - examples: ollama, gguf, safetensors
- `sha256` (text)
- `size_bytes` (bigint)
- `cache_path` (text)
  - local path on orchestrator storage
- `source_uri` (text, nullable)
  - upstream model source used to populate cache
- `created_at` (timestamptz)

Constraints

- Unique: (`sha256`)
- Index: (`model_version_id`)

### Node Model Availability Table

- `id` (uuid, pk)
- `node_id` (uuid)
- `model_version_id` (uuid)
- `status` (text)
  - examples: available, loading, failed, evicted
- `last_checked_at` (timestamptz)
- `details` (jsonb, nullable)

Constraints

- Unique: (`node_id`, `model_version_id`)
- Index: (`node_id`)

## Node Load Workflow

- Spec ID: `CYNAI.MODELS.NodeLoadWorkflow` <a id="spec-cynai-models-nodeloadworkflow"></a>

The orchestrator directs nodes to load models based on task needs and node capabilities.
Nodes SHOULD retrieve model artifacts from the orchestrator cache using local network paths.

Traces To:

- [REQ-MODELS-0104](../requirements/models.md#req-models-0104)
- [REQ-MODELS-0105](../requirements/models.md#req-models-0105)

Recommended behavior

- The orchestrator selects a model version that satisfies task capability requirements.
- The orchestrator checks `node_model_availability` to find a suitable node with the model already available.
- When selecting a node for a task, the Project Manager Agent SHOULD prefer nodes where the required model is already loaded.
- The orchestrator configures nodes with the model cache endpoint and any required pull credentials during node registration.
- If no node has the model, the orchestrator requests the target node to load the model.
- The node retrieves the artifact from the orchestrator cache and installs it into Ollama or a compatible runtime.
- The node reports status updates, and the orchestrator updates `node_model_availability`.

## User Directed Downloads

- Spec ID: `CYNAI.MODELS.UserDirectedDownloads` <a id="spec-cynai-models-userdirecteddownloads"></a>

The orchestrator MUST support user-directed actions to populate the cache.

Traces To:

- [REQ-MODELS-0106](../requirements/models.md#req-models-0106)
- [REQ-MODELS-0107](../requirements/models.md#req-models-0107)

Examples

- Download model artifacts from configured upstream sources.
- Import a local model artifact uploaded by a user.
- Promote a cached artifact to be available for worker nodes.

These actions MUST be policy-controlled and audited.

## Preferences and Constraints

- Spec ID: `CYNAI.MODELS.PreferencesConstraints` <a id="spec-cynai-models-preferencesconstraints"></a>

Model management SHOULD be configurable via PostgreSQL preferences.

Traces To:

- [REQ-MODELS-0108](../requirements/models.md#req-models-0108)

Suggested preference keys

- `models.cache.max_bytes` (number)
- `models.cache.evict_policy` (string)
- `models.download.allowed_sources` (array)
- `models.download.require_user_approval` (boolean)
- `models.nodes.prefer_local_cache` (boolean)

## Auditing and Safety

- Spec ID: `CYNAI.MODELS.AuditingSafety` <a id="spec-cynai-models-auditingsafety"></a>

Traces To:

- [REQ-MODELS-0109](../requirements/models.md#req-models-0109)
- [REQ-MODELS-0110](../requirements/models.md#req-models-0110)
- [REQ-MODELS-0111](../requirements/models.md#req-models-0111)

- The orchestrator SHOULD record all model downloads, imports, and evictions.
- The orchestrator SHOULD verify model artifact integrity using `sha256` before exposing artifacts to nodes.
- Nodes SHOULD be configured to avoid downloading models directly from the public internet.
