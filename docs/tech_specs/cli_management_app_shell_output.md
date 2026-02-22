# CLI Management App - Interactive Mode and Output

- [Document overview](#document-overview)
- [Interactive Mode (REPL)](#interactive-mode-repl)
- [Output and Scripting](#output-and-scripting)

## Document Overview

This document specifies the interactive shell (REPL) and output/scripting rules for the CLI.
It is part of the [cynork CLI](cynork_cli.md) specification.

## Interactive Mode (REPL)

The CLI SHOULD provide an interactive mode that exposes the same command surface as the non-interactive CLI,
and provides tab completion to accelerate discovery and reduce typing errors.

Entrypoint and invocation

- Command MUST be `cynork shell`; optional `-c "command"` for single-command mode (then exit with that command's exit code).
- Config and token resolution MUST run the same way as for non-interactive; the same resolved gateway URL and token MUST be used for all commands entered in the shell.

### Interactive Mode Applicable Requirements

- Spec ID: `CYNAI.CLIENT.CliInteractiveMode` <a id="spec-cynai-client-cliinteractivemode"></a>

Traces To:

- [REQ-CLIENT-0136](../requirements/client.md#req-client-0136)
- [REQ-CLIENT-0137](../requirements/client.md#req-client-0137)
- [REQ-CLIENT-0138](../requirements/client.md#req-client-0138)
- [REQ-CLIENT-0139](../requirements/client.md#req-client-0139)
- [REQ-CLIENT-0140](../requirements/client.md#req-client-0140)
- [REQ-CLIENT-0141](../requirements/client.md#req-client-0141)
- [REQ-CLIENT-0142](../requirements/client.md#req-client-0142)
- [REQ-CLIENT-0159](../requirements/client.md#req-client-0159)

Required behaviors

- The prompt MUST show the active gateway URL (or a short label) and SHOULD show auth identity when available (e.g. handle from whoami).
- Commands entered in the shell MUST behave identically to non-interactive invocation: same flags, same `--output table|json`, same exit codes.
- Tab completion MUST be provided for commands, subcommands, and known flag values; MUST NOT suggest or expose secret values (REQ-CLIENT-0142).
- Tab completion MUST be provided for task names when a task identifier is expected (e.g. after `task get`, `task result`, `task watch`, `task cancel`, `task logs`, `task artifacts list`, `task artifacts get`); completion MAY be driven by gateway-backed list of task names available to the user (REQ-CLIENT-0159).
- History (if implemented) MUST NOT record lines that contain secrets or that were entered during secret prompts; secret prompts MUST bypass history.
- When invoked as `cynork shell -c "..."`, the CLI MUST run the given command once and exit with that command's exit code (zero or non-zero).

## Output and Scripting

The CLI MUST be scriptable: JSON output and non-zero exit on failure are required for automation.

### Pretty-Printed JSON Output

- Spec ID: `CYNAI.CLIENT.CliPrettyPrintJson` <a id="spec-cynai-client-cliprettyprintjson"></a>

Traces To:

- [REQ-CLIENT-0163](../requirements/client.md#req-client-0163)

Whenever the CLI emits or displays JSON as part of its output, that JSON MUST be pretty-printed (indented with newlines) for human readability.
This applies to: (1) stdout when `--output json` is selected; (2) JSON embedded in table or chat output; (3) any other CLI output that contains JSON.
Pretty-printing MUST use consistent indentation (e.g. two or four spaces per level) and MUST not emit a single compact line unless the value is trivial (e.g. a short string or number).
Output MUST remain valid JSON and parseable by tools such as `jq`.

### Output and Scripting Applicable Requirements

- Spec ID: `CYNAI.CLIENT.CliOutputScripting` <a id="spec-cynai-client-clioutputscripting"></a>

Traces To:

- [REQ-CLIENT-0143](../requirements/client.md#req-client-0143)
- [REQ-CLIENT-0144](../requirements/client.md#req-client-0144)
- [REQ-CLIENT-0145](../requirements/client.md#req-client-0145)
- [REQ-CLIENT-0163](../requirements/client.md#req-client-0163)

Required and optional flags

- `--output` (string): MUST be supported globally or on list/get commands; values `table` (default, human-readable) and `json` (one JSON value to stdout, no extra text).
  When `json`, the CLI MUST output only the JSON document so that `cynork ... --output json` is parseable by `jq` or equivalent.
  JSON MUST be pretty-printed per [Pretty-Printed JSON Output](#pretty-printed-json-output).
- `--quiet` (bool, optional): suppress non-essential output; errors MUST still be printed to stderr.
- `--no-color` (bool, optional): disable colored output; MUST be honored when set.
