# User Preferences

- [1 Goals](#1-goals)
- [2 Terminology](#2-terminology)
  - [2.1 Settings vs Preferences](#21-settings-vs-preferences)
- [3 Data Model](#3-data-model)
  - [3.1 Preferences Tables](#31-preferences-tables)
- [4 Key Semantics](#4-key-semantics)
- [5 Value Semantics](#5-value-semantics)
- [6 Known Keys and User-Defined Keys](#6-known-keys-and-user-defined-keys)
  - [6.1 Agent additional context](#61-agent-additional-context)
- [7 Code Language Preferences](#7-code-language-preferences)
- [8 Effective Preference Resolution](#8-effective-preference-resolution)
  - [8.1 Applicable Requirements](#81-applicable-requirements)
  - [8.2 Resolution Inputs](#82-resolution-inputs)
  - [8.3 Resolution Algorithm](#83-resolution-algorithm)
  - [8.4 Output Shape](#84-output-shape)
  - [8.5 MCP Preference Tools](#85-mcp-preference-tools)
- [9 Caching and Invalidation](#9-caching-and-invalidation)
- [10 Write Semantics and Concurrency](#10-write-semantics-and-concurrency)
- [11 Auditing](#11-auditing)
- [12 Standard Use Cases](#12-standard-use-cases)
- [13 Edge Cases](#13-edge-cases)

## 1 Goals

Store user task-execution preferences and constraints in PostgreSQL so orchestrator-side agents and user clients can retrieve them deterministically for planning and verification.
Preferences cover standards and constraints such as acceptance criteria, writing style, language preferences, code language preferences, security constraints, definition-of-done, and reporting style.

## 2 Terminology

- Spec ID: `CYNAI.STANDS.PreferencesTerminology` <a id="spec-cynai-stands-preferenceterminology"></a>

In this repository, the term "preferences" refers only to user-facing task-execution preferences and constraints.
Preferences are intended to be retrieved by agents and, when appropriate, passed to AI models or queried during task execution.
They are stored in `preference_entries`, scoped (system, user, group, project, task), and managed via preference surfaces (e.g. Web Console preferences UI, `cynork prefs`).

### 2.1 Settings vs Preferences

- **System settings** are operator- and deployment-level configuration (e.g. orchestrator operational knobs, model selection keys, cache limits, deployment config such as ports, hostnames, database DSNs, service endpoints).
  They are stored in `system_settings`, managed via system settings surfaces (e.g. Web Console system settings UI, `cynork settings`), and MUST NOT be described as preferences.
- Preferences and system settings are distinct; do not conflate them in specs or UI.

See [`docs/tech_specs/orchestrator_bootstrap.md`](orchestrator_bootstrap.md) for bootstrap-time seeding of preferences versus system settings.

## 3 Data Model

Preferences are stored as key-value entries with scope and precedence.

Scope types:

- `system`: deployment-wide preference defaults (operator-managed).
- `user`: defaults for a user.
- `group`: defaults for a group (for example a team).
- `project`: overrides for a named project or workspace.
- `task`: overrides for a specific task.

Precedence order (highest wins):

- task => project => user => group => system

The Postgres schema is defined in [`docs/tech_specs/postgres_schema.md`](postgres_schema.md).
See [Preferences](postgres_schema.md#spec-cynai-schema-preferences).

### 3.1 Preferences Tables

The users table is shared by local authentication and RBAC.
See [`docs/tech_specs/local_user_accounts.md`](local_user_accounts.md) and [`docs/tech_specs/rbac_and_groups.md`](rbac_and_groups.md).

Preference entries are stored in `preference_entries`.

- `id` (uuid, pk)
- `scope_type` (text, one of: system|user|group|project|task)
- `scope_id` (uuid, nullable)
  - null is allowed only for `system` scope
- `key` (text)
- `value` (jsonb)
- `value_type` (text)
  - examples: string|number|boolean|object|array
- `version` (int)
- `updated_at` (timestamptz)
- `updated_by` (text)

Constraints:

- Unique: (`scope_type`, `scope_id`, `key`)
- Index: (`scope_type`, `scope_id`)
- Index: (`key`)

Preference change history is stored in `preference_audit_log`.

- `id` (uuid, pk)
- `entry_id` (uuid, fk to `preference_entries.id`)
- `old_value` (jsonb)
- `new_value` (jsonb)
- `changed_at` (timestamptz)
- `changed_by` (text)
- `reason` (text)

Constraints:

- Index: (`entry_id`)
- Index: (`changed_at`)

## 4 Key Semantics

Preference keys are opaque strings.
Preference keys SHOULD be treated as case-sensitive.

Recommended key format:

- Use lowercase dot-separated namespaces.
- Use digits where meaningful.
- Avoid whitespace.

Examples:

- `output.summary_style`
- `language.preferred`
- `standards.markdown.line_length`
- `security.web_browse.allowed_domains`

Reserved namespaces:

- Namespaces used by built-in behavior (for example `standards.`, `security.`, `model_routing.`, and `agents.`) are reserved for future built-in keys.
- User-defined keys SHOULD avoid reserved namespaces to reduce collision risk.

## 5 Value Semantics

The canonical stored value is `value` as jsonb.
The `value_type` field exists to support stable user-facing display and to support validation and safe consumption.

Value type rules:

- When `value_type=string`, `value` MUST be a JSON string.
- When `value_type=number`, `value` MUST be a JSON number.
- When `value_type=boolean`, `value` MUST be a JSON boolean.
- When `value_type=object`, `value` MUST be a JSON object.
- When `value_type=array`, `value` MUST be a JSON array.

Additional rules:

- `null` is a valid JSON value.
- If a higher-precedence scope sets a key to `null`, lower-precedence values are masked for that key.
- Consumers MAY treat `null` as equivalent to "unset" for specific known keys, but unknown keys MUST be passed through unchanged.

## 6 Known Keys and User-Defined Keys

Preferences support both known keys (with documented semantics) and user-defined keys (opaque).

Known keys:

- Known keys are keys whose semantics are defined by one or more tech specs.
- Known keys MAY be validated for type and shape by clients and gateways.

User-defined keys:

- User-defined keys are keys whose semantics are defined by operators, projects, or users, not by CyNodeAI specs.
- User-defined keys MUST be stored and retrieved without the system attempting to interpret them.
- User-defined keys SHOULD use a collision-resistant namespace, such as:
  - `custom.<org_or_team>.<name>`
  - `user.<handle>.<name>`
  - `project.<slug>.custom.<name>`

User-defined key examples:

- `custom.acme.writing.tone`
- `user.alice.acceptance.extra_checks`
- `project.docs.custom.release_notes_template`

### 6.1 Agent Additional Context

- Spec ID: `CYNAI.STANDS.AgentAdditionalContext` <a id="spec-cynai-stands-agentadditionalcontext"></a>

Agents that leverage LLMs MUST support user-configurable additional context included with LLM prompts.
See [REQ-AGENTS-0133](../requirements/agents.md#req-agents-0133) and [LLM Context (Baseline and User-Configurable)](project_manager_agent.md#spec-cynai-agents-llmcontext).

Recommended known keys (reserved namespace `agents.<agent_id>.additional_context` or role-based):

- `agents.project_manager.additional_context` (string or array of strings)
  - User-supplied text merged into the context passed to the Project Manager Agent's LLM (after baseline context and role instructions).
- `agents.project_analyst.additional_context` (string or array of strings)
  - User-supplied text merged into the context passed to the Project Analyst Agent's LLM.
- `agents.sandbox_agent.additional_context` (string or array of strings)
  - User-supplied text merged into the context passed to the Sandbox Agent's LLM when it performs inference (resolved at job-creation time and supplied in job context).

Semantics:

- Value MAY be a string (single block of text) or an array of strings (concatenated in order).
- Resolution uses the same scope precedence as other preferences (task > project > user > group > system).
- Invalid or unknown keys MUST be skipped during resolution; valid entries MUST be included in the effective context supplied to the agent runtime.

## 7 Code Language Preferences

Code language preferences MUST support ranked choices and context.
Code language preferences MUST also support explicit deny lists (global and context-specific) so operators can prohibit specific languages entirely or prohibit them for specific classes of work.

Recommended known keys:

- `code.language.rank_ordered` (array)
  - Ordered list of preferred code languages (first is most preferred).
  - Each array item MUST be an object with:
    - `language` (string, required)
    - `preferred_for` (object, optional)
      - `project_kinds` (array of string, optional)
      - `task_kinds` (array of string, optional)
    - `notes` (string, optional)
- `code.language.disallowed` (array of string)
  - Languages that MUST NOT be used for any generated or modified code.
- `code.language.disallowed_by_project_kind` (object)
  - Map of `project_kind` => array of disallowed languages for that project kind.
- `code.language.disallowed_by_task_kind` (object)
  - Map of `task_kind` => array of disallowed languages for that task kind.

Recommended enumerations:

- `project_kinds` examples: backend, frontend, cli, infra, data, security, docs, mobile
- `task_kinds` examples: feature, bugfix, refactor, tests, docs, tooling

Resolution rules:

- If a language is present in `code.language.disallowed`, it MUST be treated as disallowed regardless of rank.
- If a language is present in the relevant context-specific disallow map, it MUST be treated as disallowed for that context regardless of rank.
- If all ranked languages are disallowed for the context, the system MUST fail closed with an actionable error (do not silently pick a disallowed language).

Backward-compatible simple key:

- `code.language.preferred` (string) MAY be used as a simple default, but it does not support ranking or context.
  Implementations SHOULD prefer `code.language.rank_ordered` when present.

## 8 Effective Preference Resolution

This section defines how effective preferences are computed.

### 8.1 Applicable Requirements

- Spec ID: `CYNAI.STANDS.UserPreferencesRetrieval` <a id="spec-cynai-stands-prefretrieval"></a>

Traces To:

- [REQ-CLIENT-0003](../requirements/client.md#req-client-0003)
- [REQ-CLIENT-0113](../requirements/client.md#req-client-0113)
- [REQ-CLIENT-0114](../requirements/client.md#req-client-0114)
- [REQ-CLIENT-0115](../requirements/client.md#req-client-0115)
- [REQ-AGENTS-0111](../requirements/agents.md#req-agents-0111)
- [REQ-AGENTS-0112](../requirements/agents.md#req-agents-0112)

### 8.2 Resolution Inputs

Effective preference resolution requires:

- A `task_id`.
- The task's `project_id` (nullable).
- The requesting `user_id` (nullable for system-driven tasks).
- The requesting user's `group_ids` (nullable or empty).
- The scope precedence order.

### 8.3 Resolution Algorithm

The effective preferences map MUST be computed as follows.

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
5. The system MUST NOT drop unknown keys during resolution.

Group scope determinism:

- If the user is a member of multiple groups, group-scoped preference entries MUST be applied in a deterministic order.
- Recommended deterministic order is ascending by `group_id`.
- If multiple groups specify different values for the same key, the later-applied group wins.

Error handling during resolution:

- If an entry has an invalid `value_type` for the stored JSON value, the entry MUST be treated as invalid.
- Invalid entries MUST NOT override lower-precedence valid entries.
- Invalid entries SHOULD be surfaced in diagnostics (for example, in an "effective preferences preview" response), but must not block task execution by default.

### 8.4 Output Shape

The effective preference result SHOULD be representable as:

- A JSON object mapping `key` to `value`, where `value` is JSON.

The effective preference result MAY also include metadata fields for UI and auditing:

- The source scope for each key.
- The applied entry version for each key.
- A list of invalid entries skipped during resolution.

### 8.5 MCP Preference Tools

Agents and the models they run MUST be able to retrieve preferences and effective preferences via MCP database tools.
This is required so preference access remains policy-controlled, audited, and consistent across workflows.

The MCP tool catalog defines the typed database tool names.
See [`docs/tech_specs/mcp_tool_catalog.md`](mcp_tool_catalog.md) and [`docs/tech_specs/mcp_tooling.md`](mcp_tooling.md).

Minimum preference tools (recommended):

- `db.preference.get`
  - Read one preference entry in a specific scope by key.
- `db.preference.list`
  - List preference entries in a specific scope, with optional filtering and pagination.
- `db.preference.effective`
  - Compute and return effective preferences for a task (task => project => user => group => system).

Tool behavior notes:

- These tools MUST be typed operations and MUST NOT accept raw SQL.
- These tools MUST support user-defined keys (unknown keys) without interpretation.
- Listing tools MUST support pagination and MUST be size-limited.

## 9 Caching and Invalidation

Effective preference resolution is deterministic.
Clients and agents SHOULD cache effective preferences to avoid repeated database reads.

Minimum cache key:

- `task_id` and the task revision identifier (or equivalent monotonically increasing task version).

Cache invalidation MUST occur when:

- Any preference entry relevant to the task context changes.
- The task's `project_id` changes.
- The task revision changes.

Cache invalidation SHOULD be implementable by using:

- Version checks on relevant preference entries, and
- Event-driven invalidation (for example, gateway events), when available.

## 10 Write Semantics and Concurrency

Preference writes MUST be scoped.
Preference writes MUST be versioned.

Create semantics:

- Creating a preference entry for a `(scope_type, scope_id, key)` that does not exist creates a new entry with `version=1`.

Update semantics:

- Updating an existing preference entry MUST increment `version` by 1.
- Updates SHOULD require an expected version to prevent lost updates.
- If an expected version is provided and does not match the current entry version, the update MUST fail with a conflict error.

Delete semantics:

- Deletion MAY be modeled as removal of the entry or as a tombstone.
- If deletion is modeled as removal, lower-precedence values become effective again.
- If deletion is modeled as a tombstone, the tombstone SHOULD be represented as a `null` value at the higher-precedence scope.

Validation semantics:

- Known keys SHOULD be validated by type and, when feasible, by schema.
- User-defined keys MUST be accepted as long as they satisfy storage invariants (key constraints and JSON validity).

## 11 Auditing

Preference writes SHOULD be auditable.

Audit requirements:

- Each successful create, update, or delete MUST create an audit log entry.
- Audit entries MUST include `changed_by` and SHOULD include a human-entered `reason`.
- Audit entries MUST capture the old value and new value as jsonb.

## 12 Standard Use Cases

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

## 13 Edge Cases

Missing scope identifiers:

- If `project_id` is null, project-scoped entries are not considered.
- If `user_id` is null, user-scoped entries are not considered.

Unknown keys:

- Unknown keys MUST not cause failures.
- Unknown keys MUST be passed through unchanged in effective preference results.

Conflicting types across scopes:

- If the same key is set to different JSON types at different scopes, the highest-precedence valid entry wins.
- Consumers of known keys SHOULD reject invalid types for those known keys and treat them as absent.

Large values:

- Preferences SHOULD be used for small configuration values, not for large documents.
- Implementations SHOULD enforce reasonable size limits per entry.

High-churn updates:

- Caches MUST be invalidated when relevant entries change.
- Systems SHOULD avoid tight loops that repeatedly rewrite preferences during task execution.
