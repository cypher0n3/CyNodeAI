# User Preferences

- [Goals](#goals)
- [Terminology](#terminology)
  - [Settings vs Preferences](#settings-vs-preferences)
- [Data Model](#data-model)
  - [Postgres Schema](#postgres-schema)
- [Key Semantics](#key-semantics)
  - [Key Examples](#key-examples)
- [Value Semantics](#value-semantics)
- [Known Keys and User-Defined Keys](#known-keys-and-user-defined-keys)
  - [Agent Additional Context](#agent-additional-context)
- [Code Language Preferences](#code-language-preferences)
- [Effective Preference Resolution](#effective-preference-resolution)
  - [Applicable Requirements](#applicable-requirements)
  - [Resolution Inputs](#resolution-inputs)
  - [Resolution Algorithm](#resolution-algorithm)
  - [Output Shape](#output-shape)
  - [MCP Preference Tools](#mcp-preference-tools)
- [Caching and Invalidation](#caching-and-invalidation)
- [Write Semantics and Concurrency](#write-semantics-and-concurrency)
- [Auditing](#auditing)
- [Standard Use Cases](#standard-use-cases)
- [Edge Cases](#edge-cases)

## Goals

Store user task-execution preferences and constraints in PostgreSQL so orchestrator-side agents and user clients can retrieve them deterministically for planning and verification.
Preferences cover standards and constraints such as acceptance criteria, writing style, language preferences, code language preferences, security constraints, definition-of-done, and reporting style.

## Terminology

- Spec ID: `CYNAI.STANDS.PreferencesTerminology` <a id="spec-cynai-stands-preferenceterminology"></a>

In this repository, the term "preferences" refers only to user-facing task-execution preferences and constraints.
Preferences are intended to be retrieved by agents and, when appropriate, passed to AI models or queried during task execution.
They are stored in `preference_entries`, scoped (system, user, group, project, task), and managed via preference surfaces (e.g. Web Console preferences UI, `cynork prefs`).

### Settings vs Preferences

- **System settings** are operator- and deployment-level configuration (e.g. orchestrator operational knobs, model selection keys, cache limits, deployment config such as ports, hostnames, database DSNs, service endpoints).
  They are stored in `system_settings`, managed via system settings surfaces (e.g. Web Console system settings UI, `cynork settings`), and are distinct from preferences.
- Preferences and system settings are distinct; do not conflate them in specs or UI.

See [`docs/tech_specs/orchestrator_bootstrap.md`](orchestrator_bootstrap.md) for bootstrap-time seeding of preferences versus system settings.

## Data Model

- Spec ID: `CYNAI.STANDS.PreferencesDataModel` <a id="spec-cynai-stands-preferencesdatamodel"></a>

Preferences are stored as key-value entries with scope and precedence.

### Scope Types

- `system`: deployment-wide preference defaults (operator-managed).
- `user`: defaults for a user.
- `group`: defaults for a group (for example a team).
- `project`: overrides for a named project or workspace.
- `task`: overrides for a specific task.

### Precedence Order

Highest wins: task => project => user => group => system

The Postgres schema is defined in [`docs/tech_specs/postgres_schema.md`](postgres_schema.md).
See [Preferences](postgres_schema.md#spec-cynai-schema-preferences).

### Postgres Schema

- Spec ID: `CYNAI.SCHEMA.Preferences` <a id="spec-cynai-schema-preferences"></a>

Preference entries store user task-execution preferences and constraints.
Preference entries are scoped (system, user, group, project, task) with precedence.
Deployment and service configuration (ports, hostnames, database DSNs, and secrets) are not stored as preferences.
The distinction between preferences and system settings is defined in [User preferences (Terminology)](#terminology).
The `users` table is shared with identity and RBAC.

#### Preference Entries Table

- Spec ID: `CYNAI.SCHEMA.PreferenceEntriesTable` <a id="spec-cynai-schema-preferenceentriestable"></a>

- `id` (uuid, pk)
- `scope_type` (text)
  - one of: system, user, group, project, task
- `scope_id` (uuid, nullable)
  - null allowed only for system scope
- `key` (text)
- `value` (jsonb)
- `value_type` (text)
- `version` (int)
- `updated_at` (timestamptz)
- `updated_by` (text)

##### Preference Entries Table Constraints

- Unique: (`scope_type`, `scope_id`, `key`)
- Index: (`scope_type`, `scope_id`)
- Index: (`key`)

#### Preference Audit Log Table

- Spec ID: `CYNAI.SCHEMA.PreferenceAuditLogTable` <a id="spec-cynai-schema-preferenceauditlogtable"></a>

- `id` (uuid, pk)
- `entry_id` (uuid, fk to `preference_entries.id`)
- `old_value` (jsonb)
- `new_value` (jsonb)
- `changed_at` (timestamptz)
- `changed_by` (text)
- `reason` (text, nullable)

##### Preference Audit Log Table Constraints

- Index: (`entry_id`)
- Index: (`changed_at`)

## Key Semantics

- Spec ID: `CYNAI.STANDS.PreferencesKeySemantics` <a id="spec-cynai-stands-preferencekeysemantics"></a>

Preference keys are opaque strings.
Preference keys are treated as case-sensitive.

### Recommended Key Format

- Use lowercase dot-separated namespaces.
- Use digits where meaningful.
- Avoid whitespace.

### Key Examples

- `output.summary_style`
- `language.preferred`
- `standards.markdown.line_length`
- `security.web_browse.allowed_domains`

### Reserved Namespaces

- Namespaces used by built-in behavior (for example `standards.`, `security.`, `model_routing.`, and `agents.`) are reserved for future built-in keys.
- User-defined keys should avoid reserved namespaces to reduce collision risk.

## Value Semantics

- Spec ID: `CYNAI.STANDS.PreferencesValueSemantics` <a id="spec-cynai-stands-preferencevaluesemantics"></a>

The canonical stored value is `value` as jsonb.
The `value_type` field exists to support stable user-facing display and to support validation and safe consumption.

### Value Type Rules

- When `value_type=string`, `value` is a JSON string.
- When `value_type=number`, `value` is a JSON number.
- When `value_type=boolean`, `value` is a JSON boolean.
- When `value_type=object`, `value` is a JSON object.
- When `value_type=array`, `value` is a JSON array.

### Additional Rules

- `null` is a valid JSON value.
- If a higher-precedence scope sets a key to `null`, lower-precedence values are masked for that key.
- Consumers may treat `null` as equivalent to "unset" for specific known keys, but unknown keys are passed through unchanged.

## Known Keys and User-Defined Keys

- Spec ID: `CYNAI.STANDS.PreferencesKnownAndUserDefinedKeys` <a id="spec-cynai-stands-preferenceknownanduserdefinedkeys"></a>

Preferences support both known keys (with documented semantics) and user-defined keys (opaque).

### Known Keys

- Known keys are keys whose semantics are defined by one or more tech specs.
- Known keys may be validated for type and shape by clients and gateways.

### User-Defined Keys

- User-defined keys are keys whose semantics are defined by operators, projects, or users, not by CyNodeAI specs.
- User-defined keys are stored and retrieved without the system attempting to interpret them.
- User-defined keys should use a collision-resistant namespace, such as:
  - `custom.<org_or_team>.<name>`
  - `user.<handle>.<name>`
  - `project.<slug>.custom.<name>`

### User-Defined Key Examples

- `custom.acme.writing.tone`
- `user.alice.acceptance.extra_checks`
- `project.docs.custom.release_notes_template`

### Agent Additional Context

- Spec ID: `CYNAI.STANDS.AgentAdditionalContext` <a id="spec-cynai-stands-agentadditionalcontext"></a>

Agents that leverage LLMs support user-configurable additional context included with LLM prompts.
See [REQ-AGENTS-0133](../requirements/agents.md#req-agents-0133) and [LLM Context (Baseline and User-Configurable)](project_manager_agent.md#spec-cynai-agents-llmcontext).

### Agent Additional Context Known Keys

Reserved namespace `agents.<agent_id>.additional_context` or role-based:

- `agents.project_manager.additional_context` (string or array of strings)
  - User-supplied text merged into the context passed to the Project Manager Agent's LLM (after baseline context and role instructions).
- `agents.project_analyst.additional_context` (string or array of strings)
  - User-supplied text merged into the context passed to the Project Analyst Agent's LLM.
- `agents.sandbox_agent.additional_context` (string or array of strings)
  - User-supplied text merged into the context passed to the Sandbox Agent's LLM when it performs inference (resolved at job-creation time and supplied in job context).

#### Agent Additional Context Value Semantics

- Value may be a string (single block of text) or an array of strings (concatenated in order).
- Resolution uses the same scope precedence as other preferences (task > project > user > group > system).
- Invalid or unknown keys are skipped during resolution; valid entries are included in the effective context supplied to the agent runtime.

## Code Language Preferences

- Spec ID: `CYNAI.STANDS.CodeLanguagePreferences` <a id="spec-cynai-stands-codelanguagepreferences"></a>

Code language preferences support ranked choices and context.
Code language preferences also support explicit deny lists (global and context-specific) so operators can prohibit specific languages entirely or prohibit them for specific classes of work.

### Recommended Known Keys

- `code.language.rank_ordered` (array)
  - Ordered list of preferred code languages (first is most preferred).
  - Each array item is an object with:
    - `language` (string, required)
    - `preferred_for` (object, optional)
      - `project_kinds` (array of string, optional)
      - `task_kinds` (array of string, optional)
    - `notes` (string, optional)
- `code.language.disallowed` (array of string)
  - Languages that are not used for any generated or modified code.
- `code.language.disallowed_by_project_kind` (object)
  - Map of `project_kind` => array of disallowed languages for that project kind.
- `code.language.disallowed_by_task_kind` (object)
  - Map of `task_kind` => array of disallowed languages for that task kind.

### Recommended Enumerations

- `project_kinds` examples: backend, frontend, cli, infra, data, security, docs, mobile
- `task_kinds` examples: feature, bugfix, refactor, tests, docs, tooling

### Resolution Rules

- If a language is present in `code.language.disallowed`, it is treated as disallowed regardless of rank.
- If a language is present in the relevant context-specific disallow map, it is treated as disallowed for that context regardless of rank.
- If all ranked languages are disallowed for the context, the system fails closed with an actionable error (do not silently pick a disallowed language).

### Backward-Compatible Simple Key

- `code.language.preferred` (string) may be used as a simple default, but it does not support ranking or context.
  Implementations prefer `code.language.rank_ordered` when present.

## Effective Preference Resolution

This section defines how effective preferences are computed.

### Applicable Requirements

- Spec ID: `CYNAI.STANDS.UserPreferencesRetrieval` <a id="spec-cynai-stands-prefretrieval"></a>

#### Traces To

- [REQ-CLIENT-0003](../requirements/client.md#req-client-0003)
- [REQ-CLIENT-0113](../requirements/client.md#req-client-0113)
- [REQ-CLIENT-0114](../requirements/client.md#req-client-0114)
- [REQ-CLIENT-0115](../requirements/client.md#req-client-0115)
- [REQ-AGENTS-0111](../requirements/agents.md#req-agents-0111)
- [REQ-AGENTS-0112](../requirements/agents.md#req-agents-0112)

### Resolution Inputs

Effective preference resolution requires:

- A `task_id`.
- The task's `project_id` (nullable).
- The requesting `user_id` (nullable for system-driven tasks).
- The requesting user's `group_ids` (nullable or empty).
- The scope precedence order.

### Resolution Algorithm

- Spec ID: `CYNAI.STANDS.PreferencesResolutionAlgorithm` <a id="spec-cynai-stands-preferenceresolutionalgorithm"></a>

The effective preferences map is computed as follows.

1. Collect all preference entries in scope types applicable to the task context:
   - system scope (always).
   - user scope when `user_id` is available.
   - group scope for each `group_id` when group membership is available.
   - project scope when `project_id` is available.
   - task scope for the specific `task_id`.
2. Normalize entries into a list of `(scope_type, scope_id, key, value, value_type, version, updated_at)` tuples.
3. Sort tuples by precedence with deterministic tie-breakers:
   - primary: scope precedence (task > project > user > group > system).
   - secondary: `updated_at` ascending (older first).
   - tertiary: `id` ascending (stable tie-breaker).
4. Fold the sorted tuples into a map `effective[key] = value` by applying later tuples over earlier tuples.
5. The system does not drop unknown keys during resolution.

#### Group Scope Determinism

- If the user is a member of multiple groups, group-scoped preference entries are applied in a deterministic order.
- Recommended deterministic order is ascending by `group_id`.
- If multiple groups specify different values for the same key, the later-applied group wins.

#### Error Handling During Resolution

- If an entry has an invalid `value_type` for the stored JSON value, the entry is treated as invalid.
- Invalid entries do not override lower-precedence valid entries.
- Invalid entries may be surfaced in diagnostics (for example, in an "effective preferences preview" response), but do not block task execution by default.

### Output Shape

- Spec ID: `CYNAI.STANDS.PreferencesOutputShape` <a id="spec-cynai-stands-preferenceoutputshape"></a>

The effective preference result is representable as:

- A JSON object mapping `key` to `value`, where `value` is JSON.

The effective preference result may also include metadata fields for UI and auditing:

- The source scope for each key.
- The applied entry version for each key.
- A list of invalid entries skipped during resolution.

### MCP Preference Tools

- Spec ID: `CYNAI.STANDS.McpPreferenceTools` <a id="spec-cynai-stands-mcppreferencetools"></a>

Agents retrieve preferences and effective preferences via MCP database tools so preference access remains policy-controlled, audited, and consistent across workflows.
See [REQ-MCPTOO-0117](../requirements/mcptoo.md#req-mcptoo-0117).

Tool names, argument schemas, behavior, and allowlists are defined in [Preference tools](mcp_tools/preference_tools.md); see [MCP tool specifications](mcp_tools/README.md) for the full index and [MCP Tooling](mcp_tooling.md) for common arguments and response model.

## Caching and Invalidation

- Spec ID: `CYNAI.STANDS.PreferencesCaching` <a id="spec-cynai-stands-preferencecaching"></a>

Effective preference resolution is deterministic.
Clients and agents cache effective preferences to avoid repeated database reads.
See [REQ-CLIENT-0115](../requirements/client.md#req-client-0115).

### Minimum Cache Key

- `task_id` and the task revision identifier (or equivalent monotonically increasing task version).

### Cache Invalidation Triggers

Cache invalidation occurs when:

- Any preference entry relevant to the task context changes.
- The task's `project_id` changes.
- The task revision changes.

### Cache Invalidation Implementation

Cache invalidation is implementable by using:

- Version checks on relevant preference entries, and
- Event-driven invalidation (for example, gateway events), when available.

## Write Semantics and Concurrency

- Spec ID: `CYNAI.STANDS.PreferencesWriteSemantics` <a id="spec-cynai-stands-preferencewritesemantics"></a>

Preference writes are scoped.
Preference writes are versioned.

Create semantics:

- Creating a preference entry for a `(scope_type, scope_id, key)` that does not exist creates a new entry with `version=1`.

Update semantics:

- Updating an existing preference entry increments `version` by 1.
- Updates require an expected version to prevent lost updates.
- If an expected version is provided and does not match the current entry version, the update fails with a conflict error.

Delete semantics:

- Deletion may be modeled as removal of the entry or as a tombstone.
- If deletion is modeled as removal, lower-precedence values become effective again.
- If deletion is modeled as a tombstone, the tombstone is represented as a `null` value at the higher-precedence scope.

Validation semantics:

- Known keys are validated by type and, when feasible, by schema.
- User-defined keys are accepted as long as they satisfy storage invariants (key constraints and JSON validity).

## Auditing

- Spec ID: `CYNAI.STANDS.PreferencesAuditing` <a id="spec-cynai-stands-preferenceauditing"></a>

Preference writes are auditable.

### Audit Requirements

- Each successful create, update, or delete creates an audit log entry.
- Audit entries include `changed_by` and may include a human-entered `reason`.
- Audit entries capture the old value and new value as jsonb.

## Standard Use Cases

User defaults:

- A user sets `output.summary_style=concise` at user scope so all tasks use concise summaries by default.

Project standards:

- A project sets `standards.markdown.line_length=100` at project scope so all tasks in the project follow the same formatting defaults.

Task-specific override:

- A task sets `language.preferred=es` at task scope for a task that must produce output in Spanish.

Security constraints:

- A project sets `security.web_browse.allowed_domains` at project scope to constrain browsing to an allowlist.

User-defined coordination key:

- A team stores `custom.acme.writing.tone=formal` and uses it in prompts passed to models.

## Edge Cases

This section describes edge cases and special handling for preference resolution and storage.

### Missing Scope Identifiers

- If `project_id` is null, project-scoped entries are not considered.
- If `user_id` is null, user-scoped entries are not considered.

### Unknown Keys

- Unknown keys do not cause failures.
- Unknown keys are passed through unchanged in effective preference results.

### Conflicting Types Across Scopes

- If the same key is set to different JSON types at different scopes, the highest-precedence valid entry wins.
- Consumers of known keys reject invalid types for those known keys and treat them as absent.

### Large Values

- Preferences are used for small configuration values, not for large documents.
- Implementations enforce reasonable size limits per entry.

### High-Churn Updates

- Caches are invalidated when relevant entries change.
- Systems avoid tight loops that repeatedly rewrite preferences during task execution.
