# CLI Management App - Admin and Resource Commands

- [Document overview](#document-overview)
- [Credential Management](#credential-management)
- [Preferences Management](#preferences-management)
- [System Settings Management](#system-settings-management)
- [Node Management](#node-management)
- [Project Management](#project-management)
- [Personas Management](#personas-management)
- [Skills Management](#skills-management)
- [Audit Commands](#audit-commands)

## Document Overview

This document specifies credential management, preferences, system settings, node management, skills management, and audit commands.
It is part of the [cynork CLI](cynork_cli.md) specification.

## Credential Management

- Spec ID: `CYNAI.CLIENT.CliCredentialManagement` <a id="spec-cynai-client-clicredential"></a>

Traces To:

- [REQ-CLIENT-0116](../requirements/client.md#req-client-0116)
- [REQ-CLIENT-0117](../requirements/client.md#req-client-0117)
- [REQ-CLIENT-0118](../requirements/client.md#req-client-0118)
- [REQ-CLIENT-0119](../requirements/client.md#req-client-0119)
- [REQ-CLIENT-0120](../requirements/client.md#req-client-0120)

The CLI MUST support credential workflows for API Egress and Git Egress using the gateway endpoints defined in [API Egress Server - Admin API (Gateway Endpoints)](api_egress_server.md#spec-cynai-apiegr-adminapigatewayendpoints).
Responses MUST return metadata only; the CLI MUST NOT print or log secret values.

### `cynork creds list`

Invocation

- `cynork creds list`.

Optional flags

- `--provider <provider>`.
- `--owner-type <owner_type>`.
  Allowed values are `user` and `group`.
- `--owner-id <uuid>`.
- `--active <bool>`.
  Allowed values are `true` and `false`.
- `-l, --limit <n>`.
  Default is `50`.
  Allowed range is `1` to `200`.
- `--cursor <opaque>`.
  Default is empty.

Output

- Table mode MUST print a header line with these tab-separated columns in this exact order.
  `credential_id`, `provider`, `name`, `owner_type`, `owner_id`, `active`, `created_at`, `updated_at`.
- Table mode MUST then print one row per credential with the same tab-separated column order.
- JSON mode MUST print `{"credentials":[...],"next_cursor":"<opaque>"}`.
  Each credential object MUST include `credential_id`, `provider`, `name`, `owner_type`, `owner_id`, `active`, `created_at`, and `updated_at`.

### `cynork creds get <credential_id>`

Invocation

- `cynork creds get <credential_id>`.

Output

- Table mode MUST print exactly one line containing at least `credential_id=<id>` and `provider=<provider>` and `name=<name>` and `active=<bool>`.
- JSON mode MUST print a single JSON object containing at least `credential_id`, `provider`, `name`, and `active`.

### `cynork creds create`

Invocation

- `cynork creds create` with required flags and exactly one secret input method.

Required flags

- `--provider <provider>`.
- `--name <name>`.

Optional flags

- `--owner-type <owner_type>`.
  Allowed values are `user` and `group`.
  Default is `user`.
- `--owner-id <uuid>`.
  If `--owner-type user` and `--owner-id` is omitted, the CLI MUST default the owner to the authenticated user.

Secret input methods (exactly one MUST be used)

- `--secret-from-stdin`.
- `--secret-file <path>`.
- Interactive secret prompt.

Secret handling

- If `--secret-from-stdin` is set, the CLI MUST read the secret from stdin as UTF-8 text.
  The CLI MUST trim exactly one trailing newline if present.
- If `--secret-file` is set, the CLI MUST read the secret from the specified file.
  The CLI MUST trim exactly one trailing newline if present.
- If neither `--secret-from-stdin` nor `--secret-file` is set, the CLI MUST prompt `Secret:` on stderr and read the secret without echo.
- The CLI MUST reject invocations that specify more than one secret input method.
  This is a usage error and MUST return exit code 2.
- The CLI MUST NOT print or log the secret.

Output

- Table mode MUST print exactly one line containing `credential_id=<id>`.
- JSON mode MUST print `{"credential_id":"<id>"}`.

### `cynork creds rotate <credential_id>`

Invocation

- `cynork creds rotate <credential_id>` with exactly one secret input method.

Secret input methods and secret handling

- Secret input methods and secret handling MUST match `cynork creds create`.

Output

- Table mode MUST print exactly one line containing `credential_id=<id> rotated=true`.
- JSON mode MUST print `{"credential_id":"<id>","rotated":true}`.

### `cynork creds disable <credential_id>`

Invocation

- `cynork creds disable <credential_id>`.

Optional flags

- `-y, --yes`.

Behavior

- If `--yes` is not provided, the CLI MUST prompt for confirmation.
- The confirmation prompt MUST be `Disable credential <credential_id>? [y/N]`.
- If the user does not enter `y` or `Y`, the CLI MUST exit with code 0 and MUST NOT make a gateway request.

Output

- Table mode MUST print exactly one line containing `credential_id=<id> disabled=true`.
- JSON mode MUST print `{"credential_id":"<id>","disabled":true}`.

## Preferences Management

- Spec ID: `CYNAI.CLIENT.CliPreferencesManagement` <a id="spec-cynai-client-clipreferences"></a>

Traces To:

- [REQ-CLIENT-0121](../requirements/client.md#req-client-0121)
- [REQ-CLIENT-0122](../requirements/client.md#req-client-0122)
- [REQ-CLIENT-0123](../requirements/client.md#req-client-0123)
- [REQ-CLIENT-0124](../requirements/client.md#req-client-0124)

The CLI MUST support reading and writing preferences via the Data REST API; scope and key semantics are defined in [User preferences](user_preferences.md).

All preference commands MUST require auth.

Recommended keys to support (MVP)

- `output.summary_style` (string)
  - examples: concise, detailed
- `definition_of_done.required_checks` (array)
  - examples: lint, unit_tests, docs_updated
- `language.preferred` (string)
  - examples: en, en-US
- `code.language.rank_ordered` (array)
  - Rank-ordered code language choices with optional context (project kind, task kind).
- `code.language.disallowed` (array)
  - Globally disallowed languages.
- `code.language.disallowed_by_project_kind` (object)
  - Per-project-kind disallowed languages.
- `code.language.disallowed_by_task_kind` (object)
  - Per-task-kind disallowed languages.
- `standards.markdown.line_length` (number)

Scope type enum

- `system`
- `user`
- `project`
- `task`

## System Settings Management

- Spec ID: `CYNAI.CLIENT.CliSystemSettingsManagement` <a id="spec-cynai-client-clisystemsettings"></a>

Traces To:

- [REQ-CLIENT-0160](../requirements/client.md#req-client-0160)

The CLI MUST support reading and writing system settings via the User API Gateway.
System settings are not user preferences and are not managed via `cynork prefs`.
User preferences are managed via `cynork prefs`; see [User preferences](user_preferences.md).

Recommended keys to support (MVP)

Semantics: [Project Manager Model (Startup Selection and Warmup)](orchestrator.md#spec-cynai-orches-projectmanagermodelstartup).

- `agents.project_manager.model.local_default_ollama_model` (string)
- `agents.project_manager.model.selection.execution_mode` (string)
- `agents.project_manager.model.selection.mode` (string)
- `agents.project_manager.model.selection.prefer_orchestrator_host` (boolean)

Command group

- `cynork settings ...`

### `cynork prefs list`

Invocation

- `cynork prefs list --scope-type <scope_type> [--scope-id <id>]`.

Required flags

- `--scope-type <scope_type>`.
  Allowed values are `system`, `user`, `project`, and `task`.

Optional flags

- `--scope-id <id>`.
  If `--scope-type user` and `--scope-id` is omitted, the CLI MUST default the scope id to the authenticated user id.
  If `--scope-type project` or `--scope-type task`, `--scope-id` is required.

Output

- Table mode MUST print a header line with these tab-separated columns in this exact order.
  `key`, `value_json`, `updated_at`.
- Table mode MUST then print one row per preference.
- JSON mode MUST print `{"preferences":[... ]}`.
  Each preference object MUST include `key` and `value`.

### `cynork prefs get`

Invocation

- `cynork prefs get --scope-type <scope_type> [--scope-id <id>] --key <key>`.

Required flags

- `--scope-type <scope_type>`.
- `--key <key>`.

Optional flags

- `--scope-id <id>`.
  Scope id rules MUST match `cynork prefs list`.

Output

- Table mode MUST print exactly one line containing `key=<key>` and `value=<json>`.
- JSON mode MUST print `{"key":"<key>","value":<json>}`.

### `cynork prefs set`

Invocation

- `cynork prefs set --scope-type <scope_type> [--scope-id <id>] --key <key>` with exactly one value input method.

Required flags

- `--scope-type <scope_type>`.
- `--key <key>`.

Value input methods (exactly one MUST be used)

- `--value <json>`.
- `--value-file <path>`.

Optional flags

- `--scope-id <id>`.
  Scope id rules MUST match `cynork prefs list`.
- `--reason <string>`.

Behavior

- If `--value` is used, the CLI MUST parse the argument as JSON.
- If `--value-file` is used, the CLI MUST read the file as UTF-8 text and parse it as JSON.
- If JSON parsing fails, the CLI MUST exit with code 2 before making a gateway request.

Output

- Table mode MUST print exactly one line containing `ok=true`.
- JSON mode MUST print `{"ok":true}`.

### `cynork prefs delete`

Invocation

- `cynork prefs delete --scope-type <scope_type> [--scope-id <id>] --key <key>`.

Required flags

- `--scope-type <scope_type>`.
- `--key <key>`.

Optional flags

- `--scope-id <id>`.
  Scope id rules MUST match `cynork prefs list`.

Output

- Table mode MUST print exactly one line containing `deleted=true`.
- JSON mode MUST print `{"deleted":true}`.

### `cynork prefs effective`

Invocation

- `cynork prefs effective` with exactly one selector.

Selectors (exactly one MUST be provided)

- `--task-id <task_id>`.
- `--project-id <project_id>`.

Output

- Table mode MUST print the merged JSON object on stdout.
- JSON mode MUST print `{"effective":<json>,"sources":[... ]}`.
  The `sources` array MUST contain one object per resolved key.
  Each source object MUST include `key`, `scope_type`, and `scope_id`.

## Node Management

- Spec ID: `CYNAI.CLIENT.CliNodeManagement` <a id="spec-cynai-client-clinodemgmt"></a>

Traces To:

- [REQ-CLIENT-0125](../requirements/client.md#req-client-0125)
- [REQ-CLIENT-0126](../requirements/client.md#req-client-0126)
- [REQ-CLIENT-0128](../requirements/client.md#req-client-0128)

The CLI MUST support node inventory and admin actions via the User API Gateway (no direct worker API calls); semantics align with [Node](worker_node.md) and the [Web Console](web_console.md).

All node commands MUST require auth.

### `cynork nodes list`

Invocation

- `cynork nodes list`.

Optional flags

- `-l, --limit <n>`.
  Default is `50`.
  Allowed range is `1` to `200`.
- `--cursor <opaque>`.
  Default is empty.

Output

- Table mode MUST print a header line with these tab-separated columns in this exact order.
  `node_id`, `status`, `enabled`, `last_heartbeat`, `capability_summary`.
- Table mode MUST then print one row per node.
- JSON mode MUST print `{"nodes":[...],"next_cursor":"<opaque>"}`.
  Each node object MUST include at least `node_id`, `status`, and `enabled`.

### `cynork nodes get <node_id>`

Invocation

- `cynork nodes get <node_id>`.

Output

- Table mode MUST print exactly one line containing at least `node_id=<id>` and `status=<status>` and `enabled=<bool>`.
- JSON mode MUST print a single JSON object containing at least `node_id`, `status`, and `enabled`.

### `cynork nodes enable <node_id>`

Invocation

- `cynork nodes enable <node_id>`.

Optional flags

- `-y, --yes`.

Behavior

- If `--yes` is not provided, the CLI MUST prompt for confirmation.
- The confirmation prompt MUST be `Enable node <node_id>? [y/N]`.
- If the user does not enter `y` or `Y`, the CLI MUST exit with code 0 and MUST NOT make a gateway request.

Output

- Table mode MUST print exactly one line containing `node_id=<id> enabled=true`.
- JSON mode MUST print `{"node_id":"<id>","enabled":true}`.

### `cynork nodes disable <node_id>`

Invocation

- `cynork nodes disable <node_id>`.

Optional flags

- `-y, --yes`.

Behavior

- Confirmation behavior MUST match `cynork nodes enable`.
  The confirmation prompt MUST be `Disable node <node_id>? [y/N]`.

Output

- Table mode MUST print exactly one line containing `node_id=<id> enabled=false`.
- JSON mode MUST print `{"node_id":"<id>","enabled":false}`.

### `cynork nodes drain <node_id>`

Invocation

- `cynork nodes drain <node_id>`.

Optional flags

- `-y, --yes`.

Behavior

- Confirmation behavior MUST match `cynork nodes enable`.
  The confirmation prompt MUST be `Drain node <node_id>? [y/N]`.

Output

- Table mode MUST print exactly one line containing `node_id=<id> drained=true`.
- JSON mode MUST print `{"node_id":"<id>","drained":true}`.

### `cynork nodes refresh-config <node_id>`

Invocation

- `cynork nodes refresh-config <node_id>`.

Output

- Table mode MUST print exactly one line containing `node_id=<id> refresh_config_requested=true`.
- JSON mode MUST print `{"node_id":"<id>","refresh_config_requested":true}`.

### `cynork nodes prefetch-image <node_id> <image_ref>`

Invocation

- `cynork nodes prefetch-image <node_id> <image_ref>`.

Output

- Table mode MUST print exactly one line containing `node_id=<id> prefetch_requested=true`.
- JSON mode MUST print `{"node_id":"<id>","prefetch_requested":true}`.

## Project Management

- Spec ID: `CYNAI.CLIENT.CliProjectManagement` <a id="spec-cynai-client-cliprojectmanagement"></a>

Traces To:

- [REQ-CLIENT-0174](../requirements/client.md#req-client-0174)
- [REQ-CLIENT-0179](../requirements/client.md#req-client-0179)

The CLI MUST support basic project CRUD (create, list, get, update, delete or disable) via the User API Gateway.
The CLI MUST support project plan review (view plan, view revision history) and plan approve (re-approve) per [Project Plan API](user_api_gateway.md#spec-cynai-usrgwy-projectplanapi).
Projects have a user-friendly title (`display_name`) and an optional text description; see [Projects and Scope Model](projects_and_scopes.md) and [REQ-PROJCT-0103](../requirements/projct.md#req-projct-0103).
All project commands MUST require auth.

### `cynork project create`

Invocation

- `cynork project create`.

Required flags

- `--slug <slug>`.
  Unique URL-safe identifier; must match schema constraints.
- `--title <title>`.
  User-friendly display name (stored as `display_name`).

Optional flags

- `--description <text>`.
  Optional text description for the project.

Output

- Table mode MUST print exactly one line containing `project_id=<id>`.
- JSON mode MUST print `{"project_id":"<id>"}`.

### `cynork project set <project_id>`

Invocation

- `cynork project set <project_id>`.
  `<project_id>` MAY be the project UUID or the project slug.
  The CLI MUST support clearing the active project (e.g. `cynork project set --none`, or `cynork project unset`, or passing a reserved value per implementation).

Behavior

- Sets the **active project** for the CLI.
  The active project is the default project context for the current process and for child sessions (e.g. when starting `cynork chat`, the chat session inherits this as its initial project context).
  When running outside chat, the CLI SHOULD persist the active project (e.g. in config or session state) so that subsequent invocations use it as the default for commands that accept an optional `--project-id` (e.g. `cynork task create`).
  When the user clears the active project, subsequent commands use the user's default project (no explicit project id; gateway resolves default project when needed).

Output

- Table mode MUST print exactly one line containing `active_project=<id>` or `active_project=none` when cleared.
- JSON mode MUST print `{"active_project":"<id>"}` or `{"active_project":null}`.

### `cynork project list`

Invocation

- `cynork project list`.

Optional flags

- `-l, --limit <n>`.
  Default is `50`.
  Allowed range is `1` to `200`.
- `--cursor <opaque>`.
  Default is empty.
- `--active-only`.
  When set, list only projects with `is_active` true.

Output

- Table mode MUST print a header line with these tab-separated columns in this exact order.
  `project_id`, `slug`, `title`, `is_active`, `updated_at`.
  `title` is the display name.
- Table mode MUST then print one row per project.
- JSON mode MUST print `{"projects":[...],"next_cursor":"<opaque>"}`.
  Each project object MUST include at least `project_id`, `slug`, `display_name` (title), `description`, and `is_active`.

### `cynork project get <project_id>`

Invocation

- `cynork project get <project_id>`.
  `<project_id>` MAY be the project UUID or the project slug.

Output

- Table mode MUST print one line per field (e.g. `project_id=`, `slug=`, `title=`, `description=`, `is_active=`, `created_at=`, `updated_at=`).
- JSON mode MUST print a single JSON object with at least `project_id`, `slug`, `display_name`, `description`, `is_active`, `created_at`, `updated_at`.

### `cynork project update <project_id>`

Invocation

- `cynork project update <project_id>`.

Optional flags (at least one MUST be provided for a meaningful update)

- `--title <title>`.
  User-friendly display name.
- `--description <text>`.
  Optional text description; pass empty string to clear.
- `--active <bool>`.
  Set `is_active` (true/false).

Output

- Table mode MUST print exactly one line containing `project_id=<id> updated=true`.
- JSON mode MUST print `{"project_id":"<id>","updated":true}`.

### `cynork project delete <project_id>`

Invocation

- `cynork project delete <project_id>`.

Optional flags

- `-y, --yes`.

Behavior

- If `--yes` is not provided, the CLI MUST prompt for confirmation.
- The confirmation prompt MUST be `Delete project <project_id>? This may break references. [y/N]`.
- Delete MUST be policy-gated; the gateway MAY implement soft delete (set `is_active` false) or hard delete per policy.

Output

- Table mode MUST print exactly one line containing `project_id=<id> deleted=true`.
- JSON mode MUST print `{"project_id":"<id>","deleted":true}`.

### Project Plan Review and Approve

The CLI MUST support listing a project's plans, viewing a plan, listing plan revisions, and approving a plan via the User API Gateway; see [Project Plan API](user_api_gateway.md#spec-cynai-usrgwy-projectplanapi).
A project may have multiple plans; only one plan per project may be active at a time.

#### `cynork project plans list <project_id>`

- Invocation: `cynork project plans list <project_id>`.
- Optional flags: `--state draft|active|completed`, `--limit`, `--cursor`.
- Output: List of plans (plan_id, plan_name, state, plan_approved_at, plan_approved_by, is_plan_locked, updated_at).

#### `cynork project plan get <plan_id>`

- Invocation: `cynork project plan get <plan_id>`.
- Output: Plan document (plan_name, plan_body), state, task list with task dependencies, plan_approved_at, plan_approved_by, is_plan_locked, project_id.
- Table mode: one line per field or a formatted block for plan_body; JSON mode: single object with plan fields.

#### `cynork project plan revisions list <plan_id>`

- Invocation: `cynork project plan revisions list <plan_id>`.
- Optional flags: `--limit`, `--cursor` (pagination).
- Output: List of revisions (version, created_at, created_by); newest first.

#### `cynork project plan approve <plan_id>`

- Invocation: `cynork project plan approve <plan_id>`.
- Behavior: Calls gateway approve endpoint; sets this plan's state to **ready** (not active); backend tasks the PMA to add or update tasks.
  Plan must be activated separately to run workflow.
- Output: Table mode: `plan_id=<id> approved=true state=ready`; JSON mode: `{"plan_id":"<id>","approved":true,"state":"ready"}`.

#### `cynork project plan activate <plan_id>`

- Invocation: `cynork project plan activate <plan_id>`.
- Behavior: Calls gateway activate endpoint; sets this plan's state from ready to **active** so workflow may run.
  Rejected if plan is archived or not in state ready.
  Any other active plan in the same project is set to draft, suspended, or completed.
- Output: Table mode: `plan_id=<id> state=active`; JSON mode: `{"plan_id":"<id>","state":"active"}`.

### Project RBAC (Role Bindings)

The CLI MUST allow setting and listing RBAC role bindings scoped to a project.
Role bindings assign a role to a user or group within project scope; see [RBAC Model](rbac_and_groups.md#spec-cynai-access-rbacmodel) and [Projects and Scope Model](projects_and_scopes.md#spec-cynai-access-rbacscope).

#### `cynork project rbac list <project_id>`

Invocation

- `cynork project rbac list <project_id>`.

Output

- Table mode MUST print a header line with columns such as `binding_id`, `subject_type`, `subject_id`, `role`, `is_active`, `updated_at`, then one row per binding.
- JSON mode MUST print `{"bindings":[...]}`.
  Each binding MUST include at least `subject_type`, `subject_id`, `role` (name or id), and `is_active`.

#### `cynork project rbac grant <project_id>`

Invocation

- `cynork project rbac grant <project_id>`.

Required flags (subject: exactly one)

- `--user <user_id>`.
  Grant the role to this user.
- `--group <group_id>`.
  Grant the role to this group.

Required flags

- `--role <role_name>`.
  Role name (e.g. `admin`, `member`, `viewer`).

Output

- Table mode MUST print exactly one line containing `binding_id=<id> scope=project scope_id=<project_id>`.
- JSON mode MUST print `{"binding_id":"<id>","scope_type":"project","scope_id":"<project_id>"}`.

#### `cynork project rbac revoke <project_id>`

Invocation

- `cynork project rbac revoke <project_id>`.

Required flags (subject: exactly one)

- `--user <user_id>`.
- `--group <group_id>`.

Required flags

- `--role <role_name>`.

Behavior

- Revokes (deactivates or removes) the role binding for the given subject and role in project scope.
  Confirmation MAY be required when not passing `--yes`.

Output

- Table mode MUST print exactly one line containing `revoked=true`.
- JSON mode MUST print `{"revoked":true}`.

## Personas Management

- Spec ID: `CYNAI.CLIENT.CliPersonasManagement` <a id="spec-cynai-client-clipersonasmanagement"></a>

Traces To:

- [REQ-CLIENT-0178](../requirements/client.md#req-client-0178)

The CLI MUST support full CRUD for **Agent personas** (list, get, create, update, delete) via the User API Gateway, with the same capability set as the Web Console per [REQ-CLIENT-0004](../requirements/client.md#req-client-0004).
Agent personas are reusable SBA role/identity descriptions (not customer or end-user personas); see [cynode_sba.md - Persona on the Job](cynode_sba.md#spec-cynai-sbagnt-jobpersona) and [postgres_schema.md - Personas Table](postgres_schema.md#spec-cynai-schema-personastable).
Create, update, and delete are subject to RBAC: only users with appropriate roles may edit Agent personas in a given scope (e.g. admin for system-scoped; user for own user-scoped); see [data_rest_api.md - Core Resources](data_rest_api.md#spec-cynai-datapi-coreresources).
All persona commands MUST require auth.

### `cynork persona list`

Invocation

- `cynork persona list`.

Optional flags

- `--scope-type <scope_type>` (e.g. `system`, `project`, `user`).
- `--scope-id <uuid>`.
- `-l, --limit <n>`.
  Default is `50`.
  Allowed range is `1` to `200`.
- `--cursor <opaque>`.

Output

- Table mode MUST print a header line with columns: `persona_id`, `title`, `scope_type`, `scope_id`, `updated_at`, then one row per persona.
- JSON mode MUST print `{"personas":[...],"next_cursor":"<opaque>"}`.

### `cynork persona get <persona_id>`

Invocation

- `cynork persona get <persona_id>`.

Output

- Table mode MUST print at least `persona_id`, `title`, `description`, `scope_type`, `scope_id`, `created_at`, `updated_at`.
- JSON mode MUST print a single JSON object with the same fields.

### `cynork persona create`

Invocation

- `cynork persona create`.

Required flags

- `--title <title>`.
- `--description <description>` (short prose: "You are a ...").

Optional flags

- `--scope-type <scope_type>` (e.g. `system`, `project`, `user`).
- `--scope-id <uuid>`.

Output

- Table mode MUST print exactly one line containing `persona_id=<id>`.
- JSON mode MUST print `{"persona_id":"<id>"}`.

### `cynork persona update <persona_id>`

Invocation

- `cynork persona update <persona_id>`.

Optional flags (at least one for a meaningful update)

- `--title <title>`.
- `--description <description>`.
- `--scope-type <scope_type>`.
- `--scope-id <uuid>`.

Output

- Table mode MUST print exactly one line containing `persona_id=<id> updated=true`.
- JSON mode MUST print `{"persona_id":"<id>","updated":true}`.

### `cynork persona delete <persona_id>`

Invocation

- `cynork persona delete <persona_id>`.

Optional flags

- `-y, --yes`.

Behavior

- If `--yes` is not provided, the CLI MUST prompt for confirmation when the persona is referenced by jobs (if the gateway reports references).
- The confirmation prompt MUST warn that deleting may affect job provenance.

Output

- Table mode MUST print exactly one line containing `persona_id=<id> deleted=true`.
- JSON mode MUST print `{"persona_id":"<id>","deleted":true}`.

## Skills Management

- Spec ID: `CYNAI.CLIENT.CliSkillsManagement` <a id="spec-cynai-client-cliskillsmanagement"></a>

Traces To:

- [REQ-CLIENT-0146](../requirements/client.md#req-client-0146)

The CLI MUST support full CRUD for skills (create/load, list, get, update, delete) via the User API Gateway, with the same controls as defined in [Skill Management CRUD (Web and CLI)](skills_storage_and_inference.md#spec-cynai-skills-skillmanagementcrud).

All skills commands MUST require auth.

Scope enum

- `user`
- `group`
- `project`
- `global`

### `cynork skills load <file.md>`

Invocation

- `cynork skills load <file.md>`.

Optional flags

- `--name <name>`.
- `--scope <scope>`.
  Allowed values are `user`, `group`, `project`, and `global`.
  Default is `user`.
- `--scope-id <id>`.
  Required when `--scope` is `group` or `project`.
  Forbidden when `--scope` is `global`.

Behavior

- The CLI MUST read `<file.md>` as UTF-8 text.
- The CLI MUST reject unreadable files and MUST exit with code 2 before making a gateway request.

Output

- Table mode MUST print exactly one line containing `skill_id=<id>`.
- JSON mode MUST print `{"skill_id":"<id>"}`.

### `cynork skills list`

Invocation

- `cynork skills list`.

Optional flags

- `--scope <scope>`.
- `--owner <owner_id>`.
- `--limit <n>`.
  Default is `50`.
  Allowed range is `1` to `200`.
- `--cursor <opaque>`.
  Default is empty.

Output

- Table mode MUST print a header line with these tab-separated columns in this exact order.
  `skill_id`, `name`, `scope`, `scope_id`, `owner_id`, `updated_at`.
- Table mode MUST then print one row per skill.
- JSON mode MUST print `{"skills":[...],"next_cursor":"<opaque>"}`.
  Each skill object MUST include at least `skill_id`, `name`, and `scope`.

### `cynork skills get <skill_id>`

Invocation

- `cynork skills get <skill_id>`.

Output

- Table mode MUST print the skill metadata followed by the skill content.
- Table mode MUST include a metadata line containing at least `skill_id=<id>` and `scope=<scope>`.
- JSON mode MUST print `{"skill_id":"<id>","name":"<name>","scope":"<scope>","scope_id":"<id_or_empty>","content_md":"<markdown>"}`.

### `cynork skills update <skill_id> <file.md>`

Invocation

- `cynork skills update <skill_id> <file.md>`.

Optional flags

- `--name <name>`.
- `--scope <scope>`.
- `--scope-id <id>`.
  Scope and scope-id rules MUST match `cynork skills load`.

Behavior

- The CLI MUST read `<file.md>` as UTF-8 text.

Output

- Table mode MUST print exactly one line containing `skill_id=<id> updated=true`.
- JSON mode MUST print `{"skill_id":"<id>","updated":true}`.

### `cynork skills delete <skill_id>`

Invocation

- `cynork skills delete <skill_id>`.

Optional flags

- `-y, --yes`.

Behavior

- If `--yes` is not provided, the CLI MUST prompt for confirmation.
- The confirmation prompt MUST be `Delete skill <skill_id>? [y/N]`.
- If the user does not enter `y` or `Y`, the CLI MUST exit with code 0 and MUST NOT make a gateway request.

Output

- Table mode MUST print exactly one line containing `skill_id=<id> deleted=true`.
- JSON mode MUST print `{"skill_id":"<id>","deleted":true}`.

## Audit Commands

- Spec ID: `CYNAI.CLIENT.CliAuditCommands` <a id="spec-cynai-client-cliauditcommands"></a>

The CLI MUST support querying audit events via the User API Gateway.

All audit commands MUST require auth.

### `cynork audit list`

Invocation

- `cynork audit list`.

Optional flags

- `--resource-type <type>`.
- `--actor-id <id>`.
- `--since <rfc3339>`.
- `--until <rfc3339>`.
- `--limit <n>`.
  Default is `50`.
  Allowed range is `1` to `200`.
- `--cursor <opaque>`.
  Default is empty.

Output

- Table mode MUST print a header line with these tab-separated columns in this exact order.
  `event_id`, `ts`, `actor_id`, `action`, `resource_type`, `resource_id`, `decision`.
- Table mode MUST then print one row per event.
- JSON mode MUST print `{"events":[...],"next_cursor":"<opaque>"}`.
  Each event object MUST include at least `event_id`, `ts`, `actor_id`, `action`, `resource_type`, and `resource_id`.

### `cynork audit get <event_id>`

Invocation

- `cynork audit get <event_id>`.

Output

- Table mode MUST print the event as key-value pairs.
- JSON mode MUST print a single JSON object representing the event.
