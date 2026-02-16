# User Preferences

## Preference Goal

Store user preferences in PostgreSQL so orchestrator-side agents can retrieve them deterministically for planning and verification.
Preferences cover standards and policies such as formatting, security constraints, definition-of-done, and reporting style.

## Preference Data Model

Preferences are stored as key-value entries with scope and precedence.

- Scope types
  - `system`: defaults shipped with the orchestrator
  - `user`: defaults for a user
  - `project`: overrides for a named project or workspace
  - `task`: overrides for a specific task
- Precedence order (highest wins)
  - task > project > user > system

## Tables

These tables provide preference storage with clear scoping and precedence.
The Postgres schema is defined in [`docs/tech_specs/postgres_schema.md`](postgres_schema.md).
See [Preferences](postgres_schema.md#preferences).

### Users Table

- `id` (uuid, pk)
- `handle` (text, unique)
- `external_source` (text, nullable)
  - examples: entra_id, google_workspace, scim
- `external_id` (text, nullable)
  - stable identifier from the external system
- `created_at` (timestamptz)

The users table is shared by local authentication and RBAC.
See [`docs/tech_specs/local_user_accounts.md`](local_user_accounts.md) and [`docs/tech_specs/rbac_and_groups.md`](rbac_and_groups.md).

### Preference Entries Table

- `id` (uuid, pk)
- `scope_type` (text, one of: system|user|project|task)
- `scope_id` (uuid, nullable)
  - null is allowed only for `system` scope
- `key` (text)
- `value` (jsonb)
- `value_type` (text)
  - examples: string|number|boolean|object|array
- `version` (int)
- `updated_at` (timestamptz)
- `updated_by` (text)

Constraints

- Unique: (`scope_type`, `scope_id`, `key`)
- Index: (`scope_type`, `scope_id`)
- Index: (`key`)

### Preference Audit Log Table

- `id` (uuid, pk)
- `entry_id` (uuid)
  - foreign key to `preference_entries.id`
- `old_value` (jsonb)
- `new_value` (jsonb)
- `changed_at` (timestamptz)
- `changed_by` (text)
- `reason` (text)

Constraints

- Index: (`entry_id`)
- Index: (`changed_at`)

## Example Preference Keys

- `standards.markdown.line_length`
- `standards.python.formatter`
- `standards.python.lint_profile`
- `security.sandbox.network_mode` (e.g. deny_all, allowlist, orchestrator_only)
- `security.sandbox.allowed_egress_hosts` (array)
- `security.api_egress.allowed_providers` (array)
- `security.api_egress.allowed_operations` (array)
- `security.web_browse.allowed_domains` (array)
- `security.web_browse.max_chars` (number)
- `security.web_browse.mode_default` (string)
- `security.web_browse.respect_robots` (boolean)
- `security.web_browse.max_redirect_hops` (number)
- `security.web_browse.rules` (object)
- `model_routing.prefer_local` (boolean)
- `model_routing.allowed_external_providers` (array)
- `model_routing.fallback_provider_order` (array)
- `model_routing.allow_standalone_external_fallback` (boolean)
- `agents.project_manager.model_routing.prefer_local` (boolean)
- `agents.project_manager.model_routing.allowed_external_providers` (array)
- `agents.project_manager.model_routing.fallback_provider_order` (array)
- `agents.project_analyst.model_routing.prefer_local` (boolean)
- `agents.project_analyst.model_routing.allowed_external_providers` (array)
- `agents.project_analyst.model_routing.fallback_provider_order` (array)
- `models.cache.max_bytes` (number)
- `models.cache.evict_policy` (string)
- `models.download.allowed_sources` (array)
- `models.download.require_user_approval` (boolean)
- `models.nodes.prefer_local_cache` (boolean)
- `sandbox_images.registry.mode` (string)
- `sandbox_images.registry.url` (string)
- `sandbox_images.nodes.prefer_cached_images` (boolean)
- `sandbox_images.allow_public_internet` (boolean)
- `sandbox_images.require_digest_pinning` (boolean)
- `messaging.enabled` (boolean)
- `messaging.allowed_destination_types` (array)
- `messaging.rate_limit.per_user_per_minute` (number)
- `messaging.redact.secrets` (boolean)
- `orchestrator.bootstrap.yaml_path` (string)
- `data_api.rest.enabled` (boolean)
- `data_api.graphql.enabled` (boolean)
- `definition_of_done.required_checks` (array)
- `output.summary_style` (e.g. concise, detailed)

## Retrieval Rules

Normative requirements for planning and verification.

When verifying a task, the Project Manager Agent:

- MUST compute the effective preferences for the task by merging scopes in precedence order.
- MUST treat unknown keys as opaque and pass them through to verification/tooling.
- SHOULD cache effective preferences per task revision, but MUST invalidate on preference update.
