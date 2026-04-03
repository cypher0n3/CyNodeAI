# Orchestrator Bootstrap Configuration

- [Document Overview](#document-overview)
- [Bootstrap Goal](#bootstrap-goal)
- [Bootstrap Source and Precedence](#bootstrap-source-and-precedence)
  - [Applicable Requirements](#applicable-requirements)
- [Example](#example)
- [Bootstrap Contents](#bootstrap-contents)
- [Postgres Schema](#postgres-schema)
  - [System Settings Table](#system-settings-table)
  - [System Settings Audit Log Table](#system-settings-audit-log-table)
- [Orchestrator Independent Startup](#orchestrator-independent-startup)
- [Worker Node Requirement](#worker-node-requirement)
- [Deployment and Auto-Start](#deployment-and-auto-start)
- [Orchestrator Readiness and PMA Startup](#orchestrator-readiness-and-pma-startup)
  - [Inference Path](#inference-path)
  - [Worker Reports Ready](#worker-reports-ready)
  - [PMA Instance Per Session Binding](#pma-instance-per-session-binding)
  - [PMA Warm Pool](#pma-warm-pool)
  - [PMA Startup](#pma-startup)
  - [PMA Inference Capability Check (Readiness)](#pma-inference-capability-check-readiness)
  - [PMA Informs Orchestrator](#pma-informs-orchestrator)
- [Recommended Behavior (Summary)](#recommended-behavior-summary)

## Document Overview

This document defines how the orchestrator can load a bootstrap configuration at startup from a YAML file.
Bootstrap configuration is used to seed PostgreSQL and configure external service integration.

## Bootstrap Goal

- Provide a repeatable way to initialize an orchestrator deployment.
- Seed required user preferences, access control rules, and external service configuration into PostgreSQL.
- Support deployments that always include at least one worker node (which may be on the same host as the orchestrator for single-system setups).

## Bootstrap Source and Precedence

Bootstrap YAML is an import mechanism, not the source of truth.
The source of truth for system configuration and policy, and for user preferences, is PostgreSQL.
Preferences and system settings are distinct: preferences are user task-execution preferences (see [User preferences (Terminology)](user_preferences.md#spec-cynai-stands-preferenceterminology)); system settings are operator-managed operational configuration.

### Applicable Requirements

- Spec ID: `CYNAI.BOOTST.BootstrapSource` <a id="spec-cynai-bootst-bootstrapsource"></a>

#### Traces to Requirements

- [REQ-BOOTST-0100](../requirements/bootst.md#req-bootst-0100)
- [REQ-BOOTST-0101](../requirements/bootst.md#req-bootst-0101)
- [REQ-BOOTST-0102](../requirements/bootst.md#req-bootst-0102)

## Example

See [`docs/examples/orchestrator_bootstrap_example.yaml`](../examples/orchestrator_bootstrap_example.yaml) for a minimal example.
Secrets MUST be provided via environment variables or a secrets manager.

## Bootstrap Contents

Bootstrap YAML SHOULD support seeding:

- System-scoped preference defaults (entries in `preference_entries` with `scope_type` system)
- System settings (operational configuration and policy parameters)
- Access control rules and default policy
- Sandbox image registries: rank-ordered list (optional; when omitted, `docker.io` only), per-registry URLs and credentials, policy defaults
- Model management defaults (cache limits and download policy)
- External model routing defaults (allowed providers and fallback order)
- Project Manager model selection defaults (automatic policy parameters and optional explicit override)
- Orchestrator-side agent external provider defaults (Project Manager and Project Analyst routing preferences)
- **Default API endpoints:** A default set of MCP (or other API) endpoints that are loaded into the endpoint registry at bootstrap.
  Each entry has a stable slug (e.g. `builtin-git`, `builtin-filesystem`), `owner_type=system`, `scope=shared`; credentials come from environment or secrets manager, not from YAML.
  See [MCP Endpoint Registry](mcp/mcp_endpoint_registry.md#spec-cynai-mcptoo-defaultendpointsbootstrap) for the complete specification.

### Preference Entry Shape

- `preferences` is an array of objects.
- Each object SHOULD include:
  - `key` (string)
  - `value` (YAML value; written as jsonb)
  - `value_type` (string; one of: string|number|boolean|object|array)

### System Settings Entry Shape

- `system_settings` is an array of objects.
- Each object SHOULD include:
  - `key` (string)
  - `value` (YAML value)
  - `value_type` (string; one of: string|number|boolean|object|array)

### System Settings Storage

- Imported system settings are written to the `system_settings` table in PostgreSQL.
  See [Postgres Schema](#postgres-schema) below.

## Postgres Schema

- Spec ID: `CYNAI.SCHEMA.SystemSettings` <a id="spec-cynai-schema-systemsettings"></a>

System settings store operator-managed operational configuration and policy parameters.
System settings are not user task-execution preferences; for the distinction, see [User preferences (Terminology)](user_preferences.md#spec-cynai-stands-preferenceterminology).
System settings do not store secrets in plaintext.

### System Settings Table

- Spec ID: `CYNAI.SCHEMA.SystemSettingsTable` <a id="spec-cynai-schema-systemsettingstable"></a>

Table name: `system_settings`.

- `key` (text, pk)
- `value` (jsonb)
- `value_type` (text)
  - examples: string|number|boolean|object|array
- `version` (int)
- `updated_at` (timestamptz)
- `updated_by` (text)

#### System Settings Table Constraints

- Unique: (`key`)
- Index: (`updated_at`)

### System Settings Audit Log Table

- Spec ID: `CYNAI.SCHEMA.SystemSettingsAuditLogTable` <a id="spec-cynai-schema-systemsettingsauditlogtable"></a>

Table name: `system_settings_audit_log`.

- `id` (uuid, pk)
- `key` (text)
- `old_value` (jsonb)
- `new_value` (jsonb)
- `changed_at` (timestamptz)
- `changed_by` (text)
- `reason` (text, nullable)

#### System Settings Audit Log Table Constraints

- Index: (`key`)
- Index: (`changed_at`)

### Project Manager Model Selection System Setting Keys

Semantics and the selection/warmup algorithm are defined in [Project Manager Model (Startup Selection and Warmup)](orchestrator.md#spec-cynai-orches-projectmanagermodelstartup); only key names and recommended values are listed here.

- `agents.project_manager.model.selection.execution_mode` (string): `auto` | `force_local` | `force_external`; default `auto`
- `agents.project_manager.model.selection.mode` (string): `auto_sliding_scale` | `fixed_model`; default `auto_sliding_scale`
- `agents.project_manager.model.selection.prefer_orchestrator_host` (boolean); default true
- `agents.project_manager.model.local_default_ollama_model` (string); when set, pins the local PM model name; when unset, selection is automatic (see orchestrator spec)

### Project Manager Sandbox Allowed Images

- `agents.project_manager.sandbox.allow_add_to_allowed_images` (boolean); default `false`
  - When `true`, the Project Manager agent MAY add container images to the allowed-images list via MCP tools (`sandbox.allowed_images.add`).
  - When `false` (default), the PM agent MUST NOT add images; the MCP gateway MUST reject `sandbox.allowed_images.add` calls from the PM agent.
  - See [sandbox_image_registry.md](sandbox_image_registry.md) and [mcp_tools/](mcp_tools/README.md).

### Secrets Management

- Secrets SHOULD NOT be stored directly in YAML.
- If secrets must be provisioned at bootstrap time, they SHOULD be provided via environment variables or an external secrets manager and written to PostgreSQL encrypted.

## Orchestrator Independent Startup

- Spec ID: `CYNAI.BOOTST.OrchestratorIndependentStartup` <a id="spec-cynai-bootst-orchestratorindependentstartup"></a>

### Orchestrator Independent Startup Requirements Traces

- [REQ-BOOTST-0105](../requirements/bootst.md#req-bootst-0105)

The orchestrator control-plane and core services (user-gateway, api-egress) MUST start and run independently of any OLLAMA or node-local inference container.
OLLAMA (or equivalent local inference backend) is a **node-side** concern: worker nodes start and manage the inference container after registering with the orchestrator and receiving configuration that instructs them to do so (including backend variant, e.g. ROCm or CUDA).

- The orchestrator MUST NOT require an OLLAMA container as part of its own process or compose stack for correct operation.
- Dev or single-host convenience setups MAY include OLLAMA in the same compose as the orchestrator for local testing; such setups are optional and MUST NOT be the only supported deployment pattern.
- Production and multi-node deployments MUST use the prescribed startup sequence: orchestrator services start first (without OLLAMA); nodes start Worker API, register with capabilities, receive config; then nodes start OLLAMA when the orchestrator instructs them to (via node configuration payload).

## Worker Node Requirement

- Spec ID: `CYNAI.BOOTST.WorkerNodeRequirement` <a id="spec-cynai-bootst-workernoderequirement"></a>

### Worker Node Requirement Requirements Traces

- [REQ-ORCHES-0116](../requirements/orches.md#req-orches-0116)

The system always requires at least one worker node for normal operation.
For single-system setups, that node MAY be on the same host as the orchestrator (e.g. Node Manager and Worker API run on the same machine as control-plane and user-gateway).
The orchestrator MUST NOT assume it can run as the sole service with zero worker nodes.

## Deployment and Auto-Start

- Spec ID: `CYNAI.BOOTST.DeploymentAutoStart` <a id="spec-cynai-bootst-deploymentautostart"></a>

### Deployment and Auto-Start Requirements Traces

- [REQ-BOOTST-0104](../requirements/bootst.md#req-bootst-0104)

Orchestrator deployments MUST support auto-start on the host so that the orchestrator stack (e.g. PostgreSQL, control-plane, user-gateway) starts on boot or on demand without manual invocation.

- **Linux:** The implementation MUST provide or document systemd unit files for the orchestrator (e.g. container or process units for postgres, control-plane, user-gateway).
  Both user (rootless) and system (root) installs MUST be supported; see [`orchestrator/systemd/README.md`](../../orchestrator/systemd/README.md) for the reference layout and generation steps.
- **macOS:** The implementation MUST provide or document equivalent auto-start (e.g. launchd plist files) so that the orchestrator services can start on boot or on user login, with equivalent capability to the Linux systemd approach.

## Orchestrator Readiness and PMA Startup

- Spec ID: `CYNAI.BOOTST.OrchestratorReadinessAndPmaStartup` <a id="spec-cynai-bootst-orchestratorreadinessandpmastartup"></a>

### Orchestrator Readiness and PMA Startup Requirements Traces

- [REQ-BOOTST-0002](../requirements/bootst.md#req-bootst-0002)
- [REQ-ORCHES-0117](../requirements/orches.md#req-orches-0117)
- [REQ-ORCHES-0150](../requirements/orches.md#req-orches-0150)
- [REQ-ORCHES-0151](../requirements/orches.md#req-orches-0151)
- [REQ-ORCHES-0188](../requirements/orches.md#req-orches-0188)
- [REQ-ORCHES-0189](../requirements/orches.md#req-orches-0189)
- [REQ-ORCHES-0192](../requirements/orches.md#req-orches-0192)
- [REQ-WORKER-0176](../requirements/worker.md#req-worker-0176)

The orchestrator cannot report fully ready until at least one inference path exists, until at least one PMA instance has informed the orchestrator that it is online, and until the **PMA inference capability check** succeeds for the bootstrap path (see [PMA Inference Capability Check (Readiness)](#pma-inference-capability-check-readiness)).

### Inference Path

- An **inference path** is either:
  - A worker node that has registered, been instructed to start inference (via node config `inference_backend`), has started its inference container, and has **reported ready** to the orchestrator (e.g. via config ack with status applied after services are up), or
  - An LLM API key (or equivalent) configured for the Project Manager Agent via the API Egress Server so PMA can use an external provider for inference.
- The orchestrator MUST NOT report ready until at least one inference path exists.

### Worker Reports Ready

- The worker MUST report to the orchestrator when it has become ready (e.g. after applying config and starting Worker API and, when instructed, the local inference container) so the orchestrator can treat the node as an inference path.
  This report MAY be the config ack with status applied, or a dedicated readiness notification as defined in the worker node spec.

### PMA Instance per Session Binding

- Spec ID: `CYNAI.ORCHES.PmaInstancePerSessionBinding` <a id="spec-cynai-orches-pmainstancepersessionbinding"></a>

Normative model

- The orchestrator MUST provision **one `cynode-pma` managed service row per session binding** at a time: each binding is assigned a distinct pool `service_id` (pattern `pma-pool-<n>`) in `managed_services.services[]` while the binding is active.
- Each pool slot used by a binding receives its own identity-bound worker-proxy UDS mount and **at most one** active PMA MCP credential binding in the secure store for that `service_id`, matching [CYNAI.MCPGAT.PmaSessionTokens](mcp/mcp_gateway_enforcement.md#spec-cynai-mcpgat-pmasessiontokens).
- **Project Analyst Agent (PAA)** remains a separate concern: **PAA** uses `cynode-pma` in `project_analyst` mode for **task-scoped** verification and analysis; it is **not** a replacement for a per-session Project Manager PMA instance.

#### PMA Instance per Session Binding Requirements Traces

- [REQ-ORCHES-0188](../requirements/orches.md#req-orches-0188)
- [REQ-WORKER-0176](../requirements/worker.md#req-worker-0176)

### PMA Warm Pool

- Spec ID: `CYNAI.ORCHES.PmaWarmPool` <a id="spec-cynai-orches-pmawarmpool"></a>

Normative model

- The orchestrator MUST include **N** PMA entries in managed-services desired state with `service_id` values `pma-pool-0` through `pma-pool-(N-1)` where **N** is derived from: (count of active session bindings whose refresh session is valid) **plus** a minimum spare count **F** (operator-configurable, default **1**), bounded by a maximum slot count **M** (operator-configurable, default **16**), and **N** MUST be at least **1** when PMA managed services are enabled on the host worker.
- Entries for slots **not** currently assigned to an active binding MUST be emitted **without** per-session container environment (`CYNODE_SESSION_ID`, `CYNODE_USER_ID`, `CYNODE_BINDING_KEY`, and related keys) so the worker runs **idle** PMA containers that are already warm for fast assignment.
- When greedy provisioning assigns a binding to slot **k**, the orchestrator MUST update that binding's `service_id` to `pma-pool-k` and MUST include session env on the corresponding managed-service row; the worker MUST reconcile (restart) that container when env changes.
- When a binding is torn down, the orchestrator MUST remove session env from that slot (return it to the idle pool) and MAY shrink **N** when the formula allows, so workers stop excess `pma-pool-*` containers.

Environment variables (orchestrator):

- `PMA_WARM_POOL_MIN_FREE` - non-negative integer spare slots (default `1`).
- `PMA_WARM_POOL_MAX_SLOTS` - maximum **N** (default `16`).

#### PMA Warm Pool Requirements Traces

- [REQ-ORCHES-0192](../requirements/orches.md#req-orches-0192)
- [REQ-ORCHES-0188](../requirements/orches.md#req-orches-0188)
- [REQ-ORCHES-0150](../requirements/orches.md#req-orches-0150)

### PMA Startup

- The orchestrator MUST start the Project Manager Agent by instructing a worker node to run **at least one** PMA **managed service instance** when the **first** inference path becomes available: either the first worker node has reported ready and is inference-capable, or the orchestrator has an LLM API key configured for PMA via API Egress.
  The first warm-pool slot (typically `pma-pool-0`) satisfies bootstrap readiness and the **PMA inference capability check**; additional slots are added per [PMA Warm Pool](#pma-warm-pool) and [PMA Instance Per Session Binding](#pma-instance-per-session-binding).
- The orchestrator MUST deliver PMA start bundles via node configuration (managed services desired state); see [`docs/tech_specs/worker_node_payloads.md`](worker_node_payloads.md) `node_configuration_payload_v1` `managed_services`.
- The orchestrator MUST NOT instruct a node to start PMA before at least one of these conditions is satisfied.

### PMA Inference Capability Check (Readiness)

- Spec ID: `CYNAI.BOOTST.PmaInferenceCapabilityCheck` <a id="spec-cynai-bootst-pmainferencecapabilitycheck"></a>

After the orchestrator has directed the worker to start the **bootstrap** PMA instance and the worker has reported that instance `state=ready`, the orchestrator MUST run a **PMA inference capability check** before reporting the control plane **fully ready**:

- The check MUST exercise **inference through PMA** (not only the raw inference backend or container health) so that worker proxy + PMA + inference stack are validated together.
- The check MUST include this **function test**: the orchestrator directs PMA to execute the **`help.list`** MCP tool call (per [Help MCP Tools](mcp_tools/help_tools.md)) and return the tool results; the check MUST fail closed if PMA does not successfully complete that tool invocation (including MCP gateway authorization and worker proxy token attachment for the bootstrap credential).
- Additional minimal orchestrator-initiated steps (for example a tiny completion) MAY be combined with the same readiness pass; the **`help.list`** function test is **required**.

Traces To: [REQ-ORCHES-0189](../requirements/orches.md#req-orches-0189), [REQ-BOOTST-0002](../requirements/bootst.md#req-bootst-0002).

### PMA Informs Orchestrator

- The orchestrator MUST learn that PMA is online via **worker-reported managed service status** (and endpoints) for **each** PMA instance (`service_id`).
  The worker determines readiness by health checking each PMA container per the configured health contract and reports `state=ready`.
  See [`docs/tech_specs/worker_node_payloads.md`](worker_node_payloads.md) `node_capability_report_v1` `managed_services_status`.

## Recommended Behavior (Summary)

- The orchestrator MUST ensure at least one inference path is available before reporting ready.
  If no worker has reported ready with inference and no PMA-facing LLM API key is configured, the orchestrator MUST refuse to enter a ready state.
- The orchestrator MUST start **at least one** PMA instance when the first inference path exists (worker ready and inference-capable, or API Egress key for PMA); the orchestrator MUST maintain a **warm pool** per [PMA Warm Pool](#pma-warm-pool) and assign slots per session binding.
- The orchestrator MUST NOT report ready until the bootstrap PMA instance is online and reachable **and** the **PMA inference capability check** has succeeded (per [PMA Inference Capability Check (Readiness)](#pma-inference-capability-check-readiness)).
  PMA is a core system feature and is always required; disabling PMA is not supported.
- **PMA inference preference:** PMA configuration MUST prefer local inference via worker nodes unless overridden by user-specified config (e.g. `agents.project_manager.model.selection.execution_mode=force_external`).
  When a dispatchable local inference worker is available, the orchestrator MUST prefer a local Project Manager model; external inference via API Egress is used only when no local worker is available or when the user has explicitly overridden to force external.
- The orchestrator MUST select an effective Project Manager model once a local inference worker is available and ensure the model is loaded and ready before entering ready state when using local inference.
- External model calls MUST use the API Egress Server so API keys are not exposed to agents.
- Sandbox execution SHOULD be disabled or restricted when no worker nodes are available.
