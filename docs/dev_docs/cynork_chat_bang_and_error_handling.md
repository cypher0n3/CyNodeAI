# Cynork Chat: `!` Shell Escape and Subcommand Error Handling

## Summary

**Date:** 2026-02-26

- **`! command` syntax:** In `cynork chat`, a line starting with `!` runs the rest as a shell command (`sh -c "..."`).
  Output is shown inline; the session continues.
  Empty `!` prints a usage hint.
- **Error handling:** Slash commands (e.g. `/skills list`) and `!` commands no longer cause the chat process to exit.
  Errors are printed to stderr and the loop continues.
  Only `/exit`, `/quit`, EOF, or startup failures (e.g. missing token) exit the session.

## Spec and Requirements

- **Spec:** [cli_management_app_commands_chat.md](../tech_specs/cli_management_app_commands_chat.md): added **Shell escape (`!` command)** (CliChatShellEscape) and **Slash and shell command errors must not exit the session** (CliChatSubcommandErrors).
- **Requirements:** [client.md](../requirements/client.md): REQ-CLIENT-0175 (shell escape), REQ-CLIENT-0176 (subcommand errors do not exit).

## Implementation

- **cynork/cmd/chat.go:** `runChatShellCommand(cmd)` runs `sh -c`, prints combined stdout/stderr and exit status on failure; `processChatLine` handles `!` and treats slash-command errors as non-fatal (print to stderr, return `(false, nil)`).
  Session start message mentions "! &lt;cmd&gt; run in shell".
- **Tests:** `TestProcessChatLine_ShellEscape` (e.g. `! echo hi`, empty `!`, `! false`); `TestProcessChatLine_SlashErrorDoesNotExit` (404 from `/skills list` does not return error).

## CI

`just ci` passes.
