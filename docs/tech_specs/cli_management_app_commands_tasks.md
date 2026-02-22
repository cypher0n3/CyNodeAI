# CLI Management App - Task Commands

- [Task Commands](#task-commands)
- [Task Creation (Task Input and Attachments)](#task-creation-task-input-and-attachments)

## Document Overview

This document specifies the `cynork task` subcommands and task creation (task input and attachments).
It is part of the [CLI management app](cli_management_app.md) specification.

## Task Commands

- Spec ID: `CYNAI.CLIENT.CliCommandSurface` (task subset) <a id="spec-cynai-client-clicommandsurface"></a>

The CLI MUST implement the following `task` subcommands.
All `task` subcommands MUST require auth.

Task identifier

- Where a task is referenced (e.g. `task get`, `task result`, `task cancel`, `task logs`, `task artifacts list`, `task artifacts get`), the CLI MUST accept either the task UUID or the human-readable task name (see [Project Manager Agent - Task Naming](project_manager_agent.md#task-naming)).
- Task list and task get output MUST include the task name when the system provides one (e.g. in table mode as `task_name=<name>` and in JSON as `task_name`).
  For task name format and semantics, see [Project Manager Agent - Task Naming](project_manager_agent.md#task-naming).

Task status enum

- `queued`
- `running`
- `completed`
- `failed`
- `canceled`

### `cynork task create`

Invocation

- `cynork task create` followed by exactly one task input mode.

Task input modes (exactly one MUST be provided)

- `-t, --task <string>` or `-p, --prompt <string>`.
- `-f, --task-file <path>`.
- `-s, --script <path>`.
- `--command <string>` repeated one or more times.
- `--commands-file <path>`.

Attachment flags (optional)

- `-a, --attach <path>` repeated zero or more times.

Optional flags

- `--name <string>`.
  Suggested human-readable name for the task.
  When provided, the CLI MUST include it in the task create request.
  The orchestrator accepts the value, normalizes it per [Task Naming](project_manager_agent.md#task-naming), and ensures uniqueness (e.g. appends a number) when the normalized name already exists in scope.
- `--project-id <project_id>`.
  Optional project association for the task.
  When provided, the CLI MUST include it in the task create request.
  When omitted, the gateway MUST associate the task with the authenticated user's default project (see [Default project](projects_and_scopes.md#default-project)).
  See [Projects and Scope Model](projects_and_scopes.md).
- `--result`.
  Default is false.
  When set, after creating the task the CLI MUST poll the gateway for the task result until the task reaches a terminal status (`completed`, `failed`, or `canceled`), then MUST print the result in the same format as `cynork task result`.
  If the user interrupts (e.g. Ctrl+C) before the task reaches a terminal status, the CLI MUST exit without printing the result.

Behavior

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

Path and file validation

- For `--task-file`, `--script`, `--commands-file`, and each `--attach`, the CLI MUST validate that the path exists, is a regular file, and is readable by the current user.
  If any path fails validation, the CLI MUST exit with code 2 before making a gateway request.
- The CLI MUST reject directories and symlinks for these file inputs.
  This is a usage error and MUST return exit code 2.

Size limits

- `--task-file` contents MUST be <= 1 MiB.
- `--script` contents MUST be <= 256 KiB.
- `--commands-file` contents MUST be <= 64 KiB.
- Each `--attach` file MUST be <= 10 MiB.
- The number of `--attach` occurrences MUST be <= 16.
- If a limit is exceeded, the CLI MUST exit with code 2 before making a gateway request.

Output

- When `--result` is not set: table mode MUST print a single line containing `task_id=<id>` and when the system provides a task name, `task_name=<name>`; JSON mode MUST print at least `task_id`, and when provided, `task_name`.
- When `--result` is set: after the task reaches a terminal status, the CLI MUST print the result in the same format as `cynork task result` (task_id, task_name when provided, status, jobs and their results).

### `cynork task list`

Invocation

- `cynork task list`.

Optional flags

- `--status <status>`.
  Allowed values are `queued`, `running`, `completed`, `failed`, and `canceled`.
- `-l, --limit <n>`.
  Default is `50`.
  Allowed range is `1` to `200`.
- `--cursor <opaque>`.
  Default is empty.

Output

- Table mode MUST print one task per line.
  Table mode MUST include at least `task_id=<id>`, `status=<status>`, and when the system provides a task name, `task_name=<name>`.
- JSON mode MUST print `{"tasks":[...],"next_cursor":"<opaque>"}`.
  Each task object MUST include at least `task_id`, `status`, and when provided, `task_name`.

### `cynork task get <task_id>`

Invocation

- `cynork task get <task_id>`, where `<task_id>` is the task UUID or the human-readable task name.

Output

- Table mode MUST print exactly one line and MUST include at least `task_id=<id>`, `status=<status>`, and when provided, `task_name=<name>`.
- JSON mode MUST print a single JSON object representing the task.
  The JSON object MUST include at least `task_id`, `status`, and when provided, `task_name`.

### `cynork task cancel <task_id>`

Invocation

- `cynork task cancel <task_id>`.

Optional flags

- `-y, --yes`.

Behavior

- If `--yes` is not provided, the CLI MUST prompt for confirmation.
- The confirmation prompt MUST be `Cancel task <task_id>? [y/N]`.
- If the user does not enter `y` or `Y`, the CLI MUST exit with code 0 and MUST NOT make a gateway request.
- On success, table mode MUST print exactly one line including `task_id=<id>`, `canceled=true`, and when the system provides a task name, `task_name=<name>`.
- On success, JSON mode MUST print at least `task_id`, `canceled`, and when provided, `task_name`.

### `cynork task result <task_id>`

Invocation

- `cynork task result <task_id>`.

Optional flags

- `-w, --wait`.
  Default is false.

Output

- If `--wait` is set, the CLI MUST poll the gateway until the task reaches a terminal status.
  Terminal statuses are `completed`, `failed`, and `canceled`.
- Table mode MUST print exactly one line and MUST include at least `task_id=<id>`, `status=<status>`, and when the system provides a task name, `task_name=<name>`.
- If the task is in a terminal status, table mode MUST also include `stdout=<...>` and `stderr=<...>`.
- JSON mode MUST print a single JSON object with at least `task_id`, `status`, and when provided, `task_name`; and when terminal, `stdout` and `stderr`.

### `cynork task watch <task_id>`

Invocation

- `cynork task watch <task_id>`.

Behavior

- The CLI MUST poll the gateway for the task result at a fixed interval and redraw the output, similar to the Linux `watch(1)` command.
- The CLI MUST use the same output format as `cynork task result` (task_id, task_name when provided, status, jobs and results).
- When stdout is a terminal and `--no-clear` is not set, the CLI MUST clear the screen before each redraw so the display updates in place.
- The CLI MUST exit with code 0 when the task reaches a terminal status (`completed`, `failed`, or `canceled`), or when the user interrupts (e.g. Ctrl+C).

Optional flags

- `-n, --interval <duration>`.
  Poll interval (e.g. `2s`, `500ms`).
  Default is `2s`.
  Minimum is `1s`.
- `--no-clear`.
  Do not clear the screen between polls; output scrolls instead.
  Useful when stdout is not a terminal or when capturing output.

### `cynork task logs <task_id>`

Invocation

- `cynork task logs <task_id>`.

Optional flags

- `--stream <stream>`.
  Allowed values are `stdout`, `stderr`, and `all`.
  Default is `all`.
- `-F, --follow`.
  Default is false.

Output

- Table mode MUST print raw log lines to stdout.
- JSON mode MUST print an object with at least `task_id`, `stream`, `lines`; and when the system provides a task name, `task_name`.

### `cynork task artifacts list <task_id>`

Invocation

- `cynork task artifacts list <task_id>`.

Output

- Table mode MUST print a header line with these tab-separated columns in this exact order.
  `artifact_id`, `name`, `content_type`, `size_bytes`.
- Table mode MUST then print one row per artifact.
- JSON mode MUST print an object with at least `task_id`, `artifacts`; and when the system provides a task name, `task_name`.
  Each artifact object MUST include at least `artifact_id`, `name`, and `size_bytes`.

### `cynork task artifacts get <task_id> <artifact_id> --out <path>`

Invocation

- `cynork task artifacts get <task_id> <artifact_id> --out <path>`.

Required flags

- `--out <path>`.

Behavior

- The CLI MUST write the artifact bytes to the `--out` path.
- The CLI MUST create parent directories if needed.
- If the output file already exists, the CLI MUST refuse to overwrite it unless `--force` is provided.

Optional flags

- `--force`.

Output

- Table mode MUST print exactly one line containing `saved=true` and `path=<path>`.
- JSON mode MUST print `{"saved":true,"path":"<path>"}`.

## Task Creation (Task Input and Attachments)

- Spec ID: `CYNAI.CLIENT.CliTaskCreatePrompt` <a id="spec-cynai-client-clitaskcreateprompt"></a>

Traces To:

- [REQ-ORCHES-0121](../requirements/orches.md#req-orches-0121)
- [REQ-ORCHES-0125](../requirements/orches.md#req-orches-0125)
- [REQ-ORCHES-0126](../requirements/orches.md#req-orches-0126)
- [REQ-ORCHES-0127](../requirements/orches.md#req-orches-0127)
- [REQ-CLIENT-0151](../requirements/client.md#req-client-0151)
- [REQ-CLIENT-0153](../requirements/client.md#req-client-0153)
- [REQ-CLIENT-0157](../requirements/client.md#req-client-0157)

Task create MUST accept the task as **inline text** (e.g. `--prompt "..."` or `--task "..."`) or from a **file** (e.g. `--task-file <path>`) containing plain text or Markdown.
Exactly one task input mode MUST be supplied per `cynork task create` invocation.
The CLI MUST support attachments via repeatable `--attach <path>`.
The CLI MUST support running a script via `--script <path>`.
The CLI MUST support running a short series of commands via repeatable `--command <string>` or via `--commands-file <path>`.
The user task text MUST NOT be executed as a literal shell command unless the user explicitly selects `--script`, `--command`, or `--commands-file`.
