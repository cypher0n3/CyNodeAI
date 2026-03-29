# CLI Management App - Task Commands

- [Task Commands](#task-commands)

## Document Overview

This document specifies the `cynork task` subcommands and task creation (task input and attachments).
It is part of the [cynork CLI](cynork_cli.md) specification.

## Task Commands

- Spec ID: `CYNAI.CLIENT.CliCommandSurface` (task subset) <a id="spec-cynai-client-clicommandsurface"></a>

The CLI MUST implement the following `task` subcommands and support full task CRUD (create, list, get, update, delete).
Task delete is implemented as **archive** (soft delete): the task is marked archived and excluded from default `task list`; use an optional flag (e.g. `--archived`) to include or list only archived tasks.
All `task` subcommands MUST require auth.

### Task Identifier

- Where a task is referenced (e.g. `task get`, `task update`, `task delete`, `task cancel`, `task result`, `task logs`, `task artifacts list`, `task artifacts get`), the CLI MUST accept either the task UUID or the human-readable task name (see [Project Manager Agent - Task Naming](../project_manager_agent.md#spec-cynai-agents-pmtasknaming)).
- Task list and task get output MUST include the task name when the system provides one (e.g. in table mode as `task_name=<name>` and in JSON as `task_name`).
  For task name format and semantics, see [Project Manager Agent - Task Naming](../project_manager_agent.md#spec-cynai-agents-pmtasknaming).

### Task Status Enum

- `queued`
- `running`
- `completed`
- `failed`
- `canceled`
- `superseded`

The task result `status` field (from gateway or CLI output) MUST be exactly one of these values when returning task state to the client.

### Task `planning_state`

- Task create returns `planning_state=draft`; workflow execution is gated on `planning_state=ready` (see [postgres_schema.md - Tasks Table](../postgres_schema.md#spec-cynai-schema-taskstable)).
- Task list, get, and result output MUST include `planning_state` when the gateway provides it.
- A task MAY be transitioned to `ready` by the Project Manager Agent after review, or by an explicit ready operation (e.g. `POST /v1/tasks/{id}/ready` or `cynork task ready <task_id>` when exposed).

### `cynork task create`

- Spec ID: `CYNAI.CLIENT.CliTaskCreatePrompt` <a id="spec-cynai-client-clitaskcreateprompt"></a>

Task create MUST accept the task as **inline text** (e.g. `--prompt "..."` or `--task "..."`) or from a **file** (e.g. `--task-file <path>`) containing plain text or Markdown.
Exactly one task input mode MUST be supplied per invocation.
The CLI MUST support attachments via repeatable `--attach <path>`, running a script via `--script <path>`, and running commands via `--command <string>` or `--commands-file <path>`.
The user task text MUST NOT be executed as a literal shell command unless the user explicitly selects `--script`, `--command`, or `--commands-file`.

`POST /v1/tasks` (and thus `cynork task create`) returns the task identifier in the response **without waiting for task completion**; clients poll task get or task result for status and outcome.

#### `cynork task create` Invocation

- `cynork task create` followed by exactly one task input mode.

Task input modes (exactly one MUST be provided)

- `-t, --task <string>` or `-p, --prompt <string>`.
- `-f, --task-file <path>`.
- `-s, --script <path>`.
- `--command <string>` repeated one or more times.
- `--commands-file <path>`.

#### `cynork task create` Attachment Flags (Optional)

- `-a, --attach <path>` repeated zero or more times.

#### `cynork task create` Optional Flags

- `--name <string>`.
  Suggested human-readable name for the task.
  When provided, the CLI MUST include it in the task create request.
  The orchestrator accepts the value, normalizes it per [Task Naming](../project_manager_agent.md#spec-cynai-agents-pmtasknaming), and ensures uniqueness (e.g. appends a number) when the normalized name already exists in scope.
- `--project-id <project_id>`.
  Optional project association for the task.
  When provided, the CLI MUST include it in the task create request.
  When omitted, the gateway MUST associate the task with the authenticated user's default project (see [Default project](../projects_and_scopes.md#spec-cynai-access-defaultproject)).
  See [Projects and Scope Model](../projects_and_scopes.md).
- `--result`.
  Default is false.
  When set, after creating the task the CLI MUST poll the gateway for the task result until the task reaches a closed (terminal) status (`completed`, `failed`, `canceled`, or `superseded`), then MUST print the result in the same format as `cynork task result`.
  If the user interrupts (e.g. Ctrl+C) before the task reaches a terminal status, the CLI MUST exit without printing the result.

#### `cynork task create` Behavior

- The CLI MUST reject invocations that provide zero or more than one task input mode.
  This is a usage error and MUST return exit code 2.
- If `--task` or `--prompt` is provided, the system MUST interpret the task input as plain text or Markdown.
- If `--task-file` is provided, the CLI MUST read the file contents and send it as task input.
  The file contents MUST be treated as plain text or Markdown.
- If `--script` is provided, the CLI MUST read the script file contents and request script execution mode.
  The system MUST run the script in the sandbox.
- If `--command` is provided, the CLI MUST preserve the order of occurrences and MUST send the ordered list of commands.
  The system MUST run the commands in the sandbox in that order.
- If `--commands-file` is provided, the CLI MUST read the file and split it by `\n`.
  Empty lines and lines that are only whitespace MUST be ignored.
  Remaining lines are commands and MUST be run in file order.
- If any `--attach` paths are provided, the CLI MUST include them as attachments in the task create request.

#### `cynork task create` Path and File Validation

- For `--task-file`, `--script`, `--commands-file`, and each `--attach`, the CLI MUST validate that the path exists, is a regular file, and is readable by the current user.
  If any path fails validation, the CLI MUST exit with code 2 before making a gateway request.
- The CLI MUST reject directories and symlinks for these file inputs.
  This is a usage error and MUST return exit code 2.

#### `cynork task create` Size Limits

- `--task-file` contents MUST be <= 1 MiB.
- `--script` contents MUST be <= 256 KiB.
- `--commands-file` contents MUST be <= 64 KiB.
- Each `--attach` file MUST be <= 10 MiB.
- The number of `--attach` occurrences MUST be <= 16.
- If a limit is exceeded, the CLI MUST exit with code 2 before making a gateway request.

#### `cynork task create` Output

- When `--result` is not set: table mode MUST print a single line containing `task_id=<id>`, `planning_state=<draft|ready>` when the gateway provides it, and when the system provides a task name, `task_name=<name>`; JSON mode MUST print at least `task_id`, and when provided, `planning_state` and `task_name`.
- When `--result` is set: after the task reaches a terminal status, the CLI MUST print the result in the same format as `cynork task result` (task_id, task_name when provided, status, jobs and their results).

#### `cynork task create` Traces To

- [REQ-ORCHES-0122](../../requirements/orches.md#req-orches-0122)
- [REQ-ORCHES-0126](../../requirements/orches.md#req-orches-0126)
- [REQ-ORCHES-0127](../../requirements/orches.md#req-orches-0127)
- [REQ-ORCHES-0128](../../requirements/orches.md#req-orches-0128)
- [REQ-CLIENT-0151](../../requirements/client.md#req-client-0151)
- [REQ-CLIENT-0153](../../requirements/client.md#req-client-0153)
- [REQ-CLIENT-0157](../../requirements/client.md#req-client-0157)

### `cynork task list`

- Spec ID: `CYNAI.CLIENT.CliTaskList` <a id="spec-cynai-client-clitasklist"></a>

List tasks with optional status filter and pagination.

#### `cynork task list` Invocation

- `cynork task list`.

#### `cynork task list` Optional Flags

- `--status <status>`.
  Allowed values include `queued`, `running`, `completed`, `failed`, `canceled`, and `superseded`.
- `--archived`.
  When set, include archived tasks in the list (or show only archived, per gateway API); when absent, default list excludes archived tasks.
- `-l, --limit <n>`.
  Default is `50`.
  Allowed range is `1` to `200`.
- `--cursor <opaque>`.
  Default is empty.

#### `cynork task list` Output

- Table mode MUST print one task per line.
  Table mode MUST include at least `task_id=<id>`, `status=<status>`, and when the system provides a task name, `task_name=<name>`; when the gateway provides `planning_state`, table mode MUST include `planning_state=<draft|ready>`.
- JSON mode MUST print `{"tasks":[...],"next_cursor":"<opaque>"}`.
  Each task object MUST include at least `task_id`, `status`, and when provided, `task_name` and `planning_state`.

### `cynork task get <task_selector>`

- Spec ID: `CYNAI.CLIENT.CliTaskGet` <a id="spec-cynai-client-clitaskget"></a>

Invocation

- `cynork task get <task_selector>`, where `<task_selector>` is the task UUID or the human-readable task name.

Output

- Table mode MUST print exactly one line and MUST include at least `task_id=<id>`, `status=<status>`, and when provided, `task_name=<name>` and `planning_state=<draft|ready>`.
- JSON mode MUST print a single JSON object representing the task; when the gateway provides `planning_state`, it MUST be included.
  The JSON object MUST include at least `task_id`, `status`, and when provided, `task_name`.

### `cynork task update <task_selector>`

- Spec ID: `CYNAI.CLIENT.CliTaskUpdate` <a id="spec-cynai-client-clitaskupdate"></a>

Update mutable task fields (e.g. name, description, acceptance_criteria, persona_id, recommended_skill_ids) via the gateway PATCH (or PUT) API.
Allowed updates MAY be restricted when the task is closed or when the plan is locked (e.g. comments only).

#### `cynork task update` Invocation

- `cynork task update <task_selector> [options]`, where `<task_selector>` is the task UUID or the human-readable task name.

#### `cynork task update` Optional Flags

- Flags or arguments for fields to update (e.g. `--name`, `--description`, `--description-file`), per gateway contract.

#### `cynork task update` Behavior

- The CLI MUST send the update request to the gateway and print the updated task (e.g. same format as `task get`) on success.

### `cynork task delete <task_selector>`

- Spec ID: `CYNAI.CLIENT.CliTaskDelete` <a id="spec-cynai-client-clitaskdelete"></a>

Delete is implemented as **archive** (soft delete): the task is marked archived and excluded from default list views; the task row is retained for audit and history.

#### `cynork task delete` Invocation

- `cynork task delete <task_selector>`, where `<task_selector>` is the task UUID or the human-readable task name.

#### `cynork task delete` Optional Flags

- `-y, --yes`.
  Skip confirmation.

#### `cynork task delete` Behavior

- If `--yes` is not provided, the CLI MUST prompt for confirmation (e.g. `Archive task <task_selector>? [y/N]`).
- On success, the CLI MUST print confirmation (e.g. `task_id=<id>`, `archived=true`).

### `cynork task cancel <task_selector>`

- Spec ID: `CYNAI.CLIENT.CliTaskCancel` <a id="spec-cynai-client-clitaskcancel"></a>

Cancel a task by ID or name; optionally skip confirmation with `--yes`.

#### `cynork task cancel` Invocation

- `cynork task cancel <task_selector>`, where `<task_selector>` is the task UUID or the human-readable task name.

#### `cynork task cancel` Optional Flags

- `-y, --yes`.

#### `cynork task cancel` Behavior

- If `--yes` is not provided, the CLI MUST prompt for confirmation.
- The confirmation prompt MUST be `Cancel task <task_selector>? [y/N]`.
- If the user does not enter `y` or `Y`, the CLI MUST exit with code 0 and MUST NOT make a gateway request.
- On success, table mode MUST print exactly one line including `task_id=<id>`, `canceled=true`, and when the system provides a task name, `task_name=<name>`.
- On success, JSON mode MUST print at least `task_id`, `canceled`, and when provided, `task_name`.

### `cynork task result <task_selector>`

- Spec ID: `CYNAI.CLIENT.CliTaskResult` <a id="spec-cynai-client-clitaskresult"></a>

Fetch task result; optionally wait until the task reaches a terminal status.

#### `cynork task result` Invocation

- `cynork task result <task_selector>`, where `<task_selector>` is the task UUID or the human-readable task name.

#### `cynork task result` Optional Flags

- `-w, --wait`.
  Default is false.

#### `cynork task result` Output

- If `--wait` is set, the CLI MUST poll the gateway until the task reaches a terminal status.
  Closed (terminal) statuses are `completed`, `failed`, `canceled`, and `superseded`; see [Task status and closed state](../postgres_schema.md#spec-cynai-schema-taskstatusandclosed).
- Table mode MUST print exactly one line and MUST include at least `task_id=<id>`, `status=<status>`, and when the system provides a task name, `task_name=<name>`; when the gateway provides `planning_state`, table mode MUST include it.
- If the task is in a terminal status, table mode MUST also include `stdout=<...>` and `stderr=<...>`.
- JSON mode MUST print a single JSON object with at least `task_id`, `status`, and when provided, `task_name`; and when terminal, `stdout` and `stderr`.

### `cynork task watch <task_selector>`

- Spec ID: `CYNAI.CLIENT.CliTaskWatch` <a id="spec-cynai-client-clitaskwatch"></a>

Poll task result at an interval and redraw output until the task reaches a terminal status.

#### `cynork task watch` Invocation

- `cynork task watch <task_selector>`, where `<task_selector>` is the task UUID or the human-readable task name.

#### `cynork task watch` Behavior

- The CLI MUST poll the gateway for the task result at a fixed interval and redraw the output, similar to the Linux `watch(1)` command.
- The CLI MUST use the same output format as `cynork task result` (task_id, task_name when provided, status, jobs and results).
- When stdout is a terminal and `--no-clear` is not set, the CLI MUST clear the screen before each redraw so the display updates in place.
- The CLI MUST exit with code 0 when the task reaches a closed (terminal) status (`completed`, `failed`, `canceled`, or `superseded`), or when the user interrupts (e.g. Ctrl+C).

#### `cynork task watch` Optional Flags

- `-n, --interval <duration>`.
  Poll interval (e.g. `2s`, `500ms`).
  Default is `2s`.
  Minimum is `1s`.
- `--no-clear`.
  Do not clear the screen between polls; output scrolls instead.
  Useful when stdout is not a terminal or when capturing output.

### `cynork task logs <task_selector>`

- Spec ID: `CYNAI.CLIENT.CliTaskLogs` <a id="spec-cynai-client-clitasklogs"></a>

Stream or fetch task log lines (stdout, stderr, or both).

#### `cynork task logs` Invocation

- `cynork task logs <task_selector>`, where `<task_selector>` is the task UUID or the human-readable task name.

#### `cynork task logs` Optional Flags

- `--stream <stream>`.
  Allowed values are `stdout`, `stderr`, and `all`.
  Default is `all`.
- `-F, --follow`.
  Default is false.

#### `cynork task logs` Output

- Table mode MUST print raw log lines to stdout.
- JSON mode MUST print an object with at least `task_id`, `stream`, `lines`; and when the system provides a task name, `task_name`.

### `cynork task artifacts list <task_selector>`

- Spec ID: `CYNAI.CLIENT.CliTaskArtifactsList` <a id="spec-cynai-client-clitaskartifactslist"></a>

List artifacts produced by a task.

#### `cynork task artifacts list` Invocation

- `cynork task artifacts list <task_selector>`, where `<task_selector>` is the task UUID or the human-readable task name.

#### `cynork task artifacts list` Output

- Table mode MUST print a header line with these tab-separated columns in this exact order.
  `artifact_id`, `name`, `content_type`, `size_bytes`.
- Table mode MUST then print one row per artifact.
- JSON mode MUST print an object with at least `task_id`, `artifacts`; and when the system provides a task name, `task_name`.
  Each artifact object MUST include at least `artifact_id`, `name`, and `size_bytes`.

### `cynork task artifacts get <task_selector> <artifact_id> --out <path>`

- Spec ID: `CYNAI.CLIENT.CliTaskArtifactsGet` <a id="spec-cynai-client-clitaskartifactsget"></a>

Download a single task artifact to a local path.

#### `cynork task artifacts get` Invocation

- `cynork task artifacts get <task_selector> <artifact_id> --out <path>`, where `<task_selector>` is the task UUID or the human-readable task name.

#### `cynork task artifacts get` Required Flags

- `--out <path>`.

#### `cynork task artifacts get` Behavior

- The CLI MUST write the artifact bytes to the `--out` path.
- The CLI MUST create parent directories if needed.
- If the output file already exists, the CLI MUST refuse to overwrite it unless `--force` is provided.

#### `cynork task artifacts get` Optional Flags

- `--force`.

#### `cynork task artifacts get` Output

- Table mode MUST print exactly one line containing `saved=true` and `path=<path>`.
- JSON mode MUST print `{"saved":true,"path":"<path>"}`.
