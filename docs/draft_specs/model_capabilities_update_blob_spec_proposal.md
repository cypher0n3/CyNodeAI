# Model Capabilities Update Blob: Spec Proposal

- [1. Purpose and Status](#1-purpose-and-status)
- [2. Scope and Consumers](#2-scope-and-consumers)
- [3. Update File Format](#3-update-file-format)
- [4. YAML Payload Schema](#4-yaml-payload-schema)
- [5. Ingest Operation](#5-ingest-operation)
- [6. Result and Error Codes](#6-result-and-error-codes)
- [7. Integrity and Validation](#7-integrity-and-validation)
- [8. Auditing and Policy](#8-auditing-and-policy)
- [9. References](#9-references)

## 1. Purpose and Status

**Status:** Proposal (draft_specs).
No code or normative spec changes unless explicitly directed.

**Purpose:** Define how the orchestrator ingests a single binary blob that represents a compressed YAML update file for AI model capabilities.
This supports bulk updates to the model registry (e.g. adding or updating model version capability metadata) without per-record API calls or manual DB edits.

## 2. Scope and Consumers

- **Consumer:** Orchestrator model registry only.
  Capability data from the blob is applied to the model registry as defined in [Model Management](../tech_specs/model_management.md).
- **Out of scope:** Pushing model artifacts (binaries) or cache population; node delivery; worker API.
  Artifacts and cache remain under [CYNAI.MODELS.ModelCache](../tech_specs/model_management.md#spec-cynai-models-modelcache) and user-directed flows.

Traces To:

- [REQ-MODELS-0106](../requirements/models.md#req-models-0106)
- [REQ-MODELS-0107](../requirements/models.md#req-models-0107)
- [REQ-MODELS-0109](../requirements/models.md#req-models-0109)
- [REQ-MODELS-0110](../requirements/models.md#req-models-0110)

## 3. Update File Format

The update file is a single binary blob: the raw gzip-compressed byte stream of one UTF-8 YAML document.
No envelope, length prefix, or magic bytes; the first byte of the blob is the first byte of the gzip stream.

### 3.1 `ModelCapabilitiesUpdateBlob` Type

- Spec ID: `CYNAI.MODELS.ModelCapabilitiesUpdateBlob` <a id="spec-cynai-models-modelcapabilitiesupdateblob"></a>
- Status: draft

#### 3.1.1 `ModelCapabilitiesUpdateBlob` Layout and Encoding

- **Logical content:** Exactly one YAML document (single root; no multi-document stream).
- **Encoding of logical content:** UTF-8.
- **Wire format:** The UTF-8 octets of that YAML document are compressed with gzip (RFC 1952).
  The blob is the resulting gzip byte stream only.
- **Size limit:** Implementations MUST enforce a maximum blob size.
  The maximum is defined by constant `ModelCapabilitiesUpdateBlobMaxBytes` (see [Section 6](#6-result-and-error-codes)).

#### 3.1.2 `ModelCapabilitiesUpdateBlob` Decode Contract

- **Input:** Opaque binary blob (bytes), and optionally a SHA-256 digest (32 bytes or hex string) for integrity check.
- **Output:** On success: decoded root object conforming to `ModelCapabilitiesUpdateDocument`.
  On failure: structured error with one of the codes defined in [Section 6](#6-result-and-error-codes).
- **Decode order:** If an optional digest is supplied, verify blob bytes against digest first; on mismatch return `decode_failed`.
  Then decompress with gzip; on failure return `decode_failed`.
  Then parse as YAML; on parse failure return `parse_failed`.
  Then validate against `ModelCapabilitiesUpdateDocument`; on failure return `validation_failed`.

## 4. YAML Payload Schema

The decompressed YAML has a versioned root object and a list of model entries.
Unknown root or entry keys cause validation failure.

### 4.1 `ModelCapabilitiesUpdateDocument` Type

- Spec ID: `CYNAI.MODELS.ModelCapabilitiesUpdateDocument` <a id="spec-cynai-models-modelcapabilitiesupdatedocument"></a>
- Status: draft

#### 4.1.1 `ModelCapabilitiesUpdateDocument` Required Root Fields

- `version` (string or integer): Document schema version.
  The only supported value is `1` (integer) or `"1"` (string).
  Any other value causes validation failure.
- `models` (sequence): List of model capability entries; may be empty.
  Each element MUST conform to the Model Capability Entry type (see [Section 4.2](#42-model-capability-entry-type)).

#### 4.1.2 `ModelCapabilitiesUpdateDocument` Optional Root Fields

- `source` (string): Origin identifier (e.g. vendor name, bundle name); informational only.
- `generated_at` (string): ISO 8601 timestamp; informational only.

#### 4.1.3 `ModelCapabilitiesUpdateDocument` Validation Rules

- The root MUST be a YAML mapping (object).
- No keys other than `version`, `models`, `source`, `generated_at` are allowed at the root.
  Presence of any other key causes validation failure.
- `models` MUST be a sequence; each item MUST be a mapping.

### 4.2 Model Capability Entry Type

- Spec ID: `CYNAI.MODELS.ModelCapabilityEntry` <a id="spec-cynai-models-modelcapabilityentry"></a>
- Status: draft

Each element of `models` is a mapping that describes one model (or model version) and its capabilities.

#### 4.2.1 `ModelCapabilityEntry` Required Entry Fields

- `name` (string): Logical model name (e.g. `qwen3.5:0.8b`, `llama3`).
  MUST be non-empty after trimming; empty or whitespace-only causes validation failure.
- `capabilities` (mapping): Key-value capability metadata.
  Keys and value types align with the model registry `model_versions.capabilities` jsonb; see [Model Management](../tech_specs/model_management.md).
  Allowed capability keys (canonical set): `context_length` (integer), `tool_use` (boolean), `vision` (boolean), `json_mode` (boolean), `languages` (sequence of strings), `gpu_required` (boolean).
- **Unknown entry keys:** Any key in an entry other than `name`, `capabilities`, `version`, `vendor`, `default_parameters` causes validation failure.
  Any key inside `capabilities` other than the canonical set above causes validation failure.

#### 4.2.2 `ModelCapabilityEntry` Optional Entry Fields

- `version` (string): Version or tag (e.g. semantic version, digest label).
  If omitted, the default version string used for upsert is the single value `""` (empty string) so that at most one "unversioned" row exists per (`name`, `vendor`).
- `vendor` (string): Vendor or provider label; nullable/omitted stored as null in the registry.
- `default_parameters` (mapping): Default inference parameters (e.g. `temperature`, `top_p`) stored in `model_versions.default_parameters`.
  Keys and values MUST be JSON-serializable; stored as jsonb.

#### 4.2.3 `ModelCapabilityEntry` Validation Rules

- No keys other than `name`, `capabilities`, `version`, `vendor`, `default_parameters` are allowed.
  Presence of any other key causes validation failure.
- `capabilities` MUST be a mapping; values MUST be JSON-serializable (boolean, number, string, sequence, mapping).

## 5. Ingest Operation

The orchestrator exposes an operation that accepts the blob and applies it to the model registry using replace semantics for capabilities and default_parameters.

### 5.1 `IngestModelCapabilitiesUpdate` Operation

- Spec ID: `CYNAI.MODELS.IngestModelCapabilitiesUpdate` <a id="spec-cynai-models-ingestmodelcapabilitiesupdate"></a>
- Status: draft

#### 5.1.1 `IngestModelCapabilitiesUpdate` Inputs

- `blob` (bytes): The compressed YAML update file (gzip stream only).
- `digest_sha256` (optional): If present, 32 raw bytes or 64-character hex string; digest of `blob` before any processing.
  When present, the implementation MUST verify `blob` against this digest before decompression and MUST return `decode_failed` on mismatch.
- `correlation_id` (optional): Opaque string for idempotency or audit correlation.
- `actor` (optional): Identity of the caller for audit; when not provided, implementation uses request context or system identity.

#### 5.1.2 `IngestModelCapabilitiesUpdate` Outputs

- **Success:** A result object containing at least: `ok: true`, `models_created` (non-negative integer), `models_updated` (non-negative integer), `versions_created` (non-negative integer), `versions_updated` (non-negative integer).
  Optional: `correlation_id` echo, `document_version`, `source` from document.
- **Failure:** A structured error object containing at least: `ok: false`, `code` (one of the error codes in [Section 6](#6-result-and-error-codes)), `message` (human-readable), and optionally `details` (e.g. field path for validation errors).

#### 5.1.3 `IngestModelCapabilitiesUpdate` Behavior

The operation decodes the blob, validates the document, applies policy, then applies each model entry to the registry in document order.
Registry rows are created or updated by (`name`, `vendor`) for `models` and (`model_id`, `version`) for `model_versions`.
For an existing `model_versions` row, `capabilities` and `default_parameters` are **replaced** in full (no deep merge).
Full procedure: [IngestModelCapabilitiesUpdate Algorithm](#51-ingestmodelcapabilitiesupdate-operation) (algorithm anchor: `algo-cynai-models-ingestmodelcapabilitiesupdate`).

#### 5.1.4 `IngestModelCapabilitiesUpdate` Error Conditions

- `decode_failed`: Blob exceeds `ModelCapabilitiesUpdateBlobMaxBytes`, gzip decompression fails, or optional digest verification fails.
- `parse_failed`: YAML parse error (syntax, encoding).
- `validation_failed`: Document version unsupported, root not a mapping, missing required root or entry fields, invalid types, unknown root or entry keys, or empty `name`.
- `policy_denied`: Caller not allowed to perform model registry updates (e.g. RBAC or system setting).
- `conflict`: Database constraint violation (e.g. unique constraint) during apply; may be retriable.

#### 5.1.5 `IngestModelCapabilitiesUpdate` Algorithm

<a id="algo-cynai-models-ingestmodelcapabilitiesupdate"></a>

1. If `digest_sha256` is present, compute SHA-256 of `blob` and compare to `digest_sha256`; on mismatch return error with code `decode_failed`. <a id="algo-cynai-models-ingestmodelcapabilitiesupdate-step-1"></a>
2. If length of `blob` exceeds `ModelCapabilitiesUpdateBlobMaxBytes`, return error with code `decode_failed`. <a id="algo-cynai-models-ingestmodelcapabilitiesupdate-step-2"></a>
3. Decompress `blob` with gzip; on failure return error with code `decode_failed`. <a id="algo-cynai-models-ingestmodelcapabilitiesupdate-step-3"></a>
4. Parse decompressed bytes as UTF-8 YAML (single document); on parse failure return error with code `parse_failed`. <a id="algo-cynai-models-ingestmodelcapabilitiesupdate-step-4"></a>
5. Validate root against `ModelCapabilitiesUpdateDocument` (version, required and allowed keys, `models` sequence); validate each entry against `ModelCapabilityEntry`; on failure return error with code `validation_failed` and details. <a id="algo-cynai-models-ingestmodelcapabilitiesupdate-step-5"></a>
6. Perform policy check (caller allowed to update model registry); on denial return error with code `policy_denied`. <a id="algo-cynai-models-ingestmodelcapabilitiesupdate-step-6"></a>
7. For each entry in `models` in order: resolve or create `models` row by (`name`, `vendor`); resolve or create `model_versions` row by (`model_id`, `version` where `version` is entry's `version` or `""` if omitted); set `capabilities` and `default_parameters` to the entry's values (full replace); set `updated_at` and `updated_by` per registry rules. <a id="algo-cynai-models-ingestmodelcapabilitiesupdate-step-7"></a>
8. Record audit event (see [Section 8](#8-auditing-and-policy)); return success result with counts. <a id="algo-cynai-models-ingestmodelcapabilitiesupdate-step-8"></a>

## 6. Result and Error Codes

Explicit success result shape and error code constants for the ingest operation.

### 6.1 `IngestModelCapabilitiesUpdate` Result and Error Codes

- Spec ID: `CYNAI.MODELS.IngestModelCapabilitiesUpdate.Codes` <a id="spec-cynai-models-ingestmodelcapabilitiesupdate-codes"></a>
- Status: draft

**Success result fields:** `ok` = true, `models_created`, `models_updated`, `versions_created`, `versions_updated` (all integers); optional `correlation_id`, `document_version`, `source`.

#### 6.1.1 Error Codes (String Constants)

- `decode_failed`: Blob size, digest mismatch, or gzip failure.
- `parse_failed`: YAML syntax or encoding failure.
- `validation_failed`: Schema validation failure (version, keys, types).
- `policy_denied`: Caller not authorized.
- `conflict`: Database constraint violation.

### 6.2 `ModelCapabilitiesUpdateBlobMaxBytes` Constant

- Spec ID: `CYNAI.MODELS.ModelCapabilitiesUpdateBlobMaxBytes` <a id="spec-cynai-models-modelcapabilitiesupdateblobmaxbytes"></a>
- Status: draft

- **Meaning:** Maximum allowed size in bytes of the blob input to decode or ingest.
- **Default value:** 10 *1024* 1024 (10 MiB).
- **Configurability:** MAY be overridden by system setting or deployment config; if so, the effective value MUST be documented and MUST not exceed a safe upper bound (e.g. 100 MiB) to prevent resource exhaustion.

## 7. Integrity and Validation

- **Integrity:** When `digest_sha256` is provided, the implementation MUST verify the blob against it before decompression and MUST fail with `decode_failed` on mismatch.
  See [REQ-MODELS-0110](../requirements/models.md#req-models-0110).
- **Validation:** After decompression and YAML parse, the root and each `models` entry MUST be validated per `ModelCapabilitiesUpdateDocument` and `ModelCapabilityEntry`.
  Invalid documents MUST NOT be applied; the operation MUST return `validation_failed` with sufficient details (e.g. path and reason).

## 8. Auditing and Policy

- **Auditing:** The orchestrator MUST record each ingest attempt (success and failure) with: timestamp, actor (or system identity), blob size or digest, document version, counts (on success), outcome (success or error code).
  See [REQ-MODELS-0109](../requirements/models.md#req-models-0109).
- **Policy:** Ingest MUST be gated so that only authorized callers can perform it; denial MUST return `policy_denied`.
  See [REQ-MODELS-0107](../requirements/models.md#req-models-0107).

## 9. References

- Model management: [model_management.md](../tech_specs/model_management.md)
- Model registry: [model_management.md](../tech_specs/model_management.md), [postgres_schema.md](../tech_specs/postgres_schema.md)
- Requirements: [models.md](../requirements/models.md)
