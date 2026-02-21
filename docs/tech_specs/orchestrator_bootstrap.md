# Orchestrator Bootstrap Configuration

- [Document Overview](#document-overview)
- [Bootstrap Goal](#bootstrap-goal)
- [Bootstrap Source and Precedence](#bootstrap-source-and-precedence)
  - [Applicable Requirements](#applicable-requirements)
- [Bootstrap Contents](#bootstrap-contents)
- [Standalone Operation Mode](#standalone-operation-mode)

## Document Overview

This document defines how the orchestrator can load a bootstrap configuration at startup from a YAML file.
Bootstrap configuration is used to seed PostgreSQL and configure external service integration.

## Bootstrap Goal

- Provide a repeatable way to initialize an orchestrator deployment.
- Seed required user preferences, access control rules, and external service configuration into PostgreSQL.
- Support running the orchestrator as the sole service with no worker nodes.

## Bootstrap Source and Precedence

Bootstrap YAML is an import mechanism, not the source of truth.
The source of truth for system configuration and policy, and for user preferences, is PostgreSQL.
Preferences and system settings are distinct: preferences are user task-execution preferences (see [User preferences (Terminology)](user_preferences.md#2-terminology)); system settings are operator-managed operational configuration.

### Applicable Requirements

- Spec ID: `CYNAI.BOOTST.BootstrapSource` <a id="spec-cynai-bootst-bootstrapsource"></a>

Traces To:

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
- Sandbox image registry configuration (registry URL, mode, policy defaults)
- Model management defaults (cache limits and download policy)
- External model routing defaults (allowed providers and fallback order)
- Project Manager model selection defaults (automatic policy parameters and optional explicit override)
- Orchestrator-side agent external provider defaults (Project Manager and Project Analyst routing preferences)

Preference entry shape (recommended)

- `preferences` is an array of objects.
- Each object SHOULD include:
  - `key` (string)
  - `value` (YAML value; written as jsonb)
  - `value_type` (string; one of: string|number|boolean|object|array)

System settings entry shape (recommended)

- `system_settings` is an array of objects.
- Each object SHOULD include:
  - `key` (string)
  - `value` (YAML value)
  - `value_type` (string; one of: string|number|boolean|object|array)

Storage

- Imported system settings SHOULD be written to the `system_settings` table in PostgreSQL.
  See [`docs/tech_specs/postgres_schema.md`](postgres_schema.md).

Project Manager model selection system setting keys (MVP)

Semantics and the selection/warmup algorithm are defined in [Project Manager Model (Startup Selection and Warmup)](orchestrator.md#project-manager-model-startup-selection-and-warmup); only key names and recommended values are listed here.

- `agents.project_manager.model.selection.execution_mode` (string): `auto` | `force_local` | `force_external`; default `auto`
- `agents.project_manager.model.selection.mode` (string): `auto_sliding_scale` | `fixed_model`; default `auto_sliding_scale`
- `agents.project_manager.model.selection.prefer_orchestrator_host` (boolean); default true
- `agents.project_manager.model.local_default_ollama_model` (string); when set, pins the local PM model name; when unset, selection is automatic (see orchestrator spec)

Secrets

- Secrets SHOULD NOT be stored directly in YAML.
- If secrets must be provisioned at bootstrap time, they SHOULD be provided via environment variables or an external secrets manager and written to PostgreSQL encrypted.

## Standalone Operation Mode

The orchestrator SHOULD support running as the sole service with zero worker nodes.

Recommended behavior

- The orchestrator MUST ensure at least one inference-capable path is available before reporting ready.
  If no local inference is available and no external provider keys are configured, the orchestrator MUST refuse to enter a ready state because inference is unavailable.
- The orchestrator MUST select an effective Project Manager model on startup.
  If a local inference worker is available, it MUST ensure the default Project Manager model is loaded and ready before entering ready state.
- If there are no registered worker nodes, the orchestrator MAY route model calls to external providers when policy allows it.
- External model calls MUST use the API Egress Server so API keys are not exposed to agents.
- Sandbox execution SHOULD be disabled or restricted when no worker nodes are available.
