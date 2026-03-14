# CLIENT Requirements

- [1 Overview](#1-overview)
- [2 Requirements](#2-requirements)

## 1 Overview

This document consolidates requirements for the `CLIENT` domain.
It covers user-facing management surfaces (CLI and shared client behavior) and user preference behavior.
Web Console-specific requirements live in [webcon.md](webcon.md) (REQ-WEBCON-*).

## 2 Requirements

- **REQ-CLIENT-0001:** CLI: no direct DB; gateway for all ops; no secrets in output or on disk; token auth; least privilege.
  [CYNAI.CLIENT.Doc.CliManagementApp](../tech_specs/cynork_cli.md#spec-cynai-client-doc-climanagementapp)
  [CYNAI.CLIENT.CliSecurityModel](../tech_specs/cynork_cli.md#spec-cynai-client-clisecurity)
  [CYNAI.CLIENT.CliAuthConfig](../tech_specs/cynork_cli.md#spec-cynai-client-cliauth)
  <a id="req-client-0001"></a>
- **REQ-CLIENT-0003:** Effective preferences by scope precedence; unknown keys passed through; cache per task revision with invalidation on update.
  Effective preference computation MUST be deterministic and MUST skip invalid preference entries (for example mismatched `value_type`) rather than letting invalid entries override lower-precedence valid entries.
  [CYNAI.STANDS.UserPreferencesRetrieval](../tech_specs/user_preferences.md#spec-cynai-stands-prefretrieval)
  <a id="req-client-0003"></a>
- **REQ-CLIENT-0004:** The Web Console and the CLI management app MUST provide capability parity for administrative operations.
  When adding or changing a capability in one client (for example a new credential type, preference scope, node action, or skill operation), the other client MUST be updated to match.
  [CYNAI.CLIENT.WebConsoleCapabilityParity](../tech_specs/web_console.md#spec-cynai-client-webconcapabilityparity)
  [CYNAI.CLIENT.CliCapabilityParity](../tech_specs/cynork_cli.md#spec-cynai-client-clicapabilityparity)
  <a id="req-client-0004"></a>
- **REQ-CLIENT-0100:** The CLI MUST NOT connect directly to PostgreSQL.
  [CYNAI.CLIENT.CliSecurityModel](../tech_specs/cynork_cli.md#spec-cynai-client-clisecurity)
  <a id="req-client-0100"></a>
- **REQ-CLIENT-0101:** The CLI MUST call the User API Gateway for all operations.
  [CYNAI.CLIENT.CliSecurityModel](../tech_specs/cynork_cli.md#spec-cynai-client-clisecurity)
  <a id="req-client-0101"></a>
- **REQ-CLIENT-0102:** The CLI MUST avoid printing secrets.
  [CYNAI.CLIENT.CliSecurityModel](../tech_specs/cynork_cli.md#spec-cynai-client-clisecurity)
  <a id="req-client-0102"></a>
- **REQ-CLIENT-0103:** The CLI MUST not persist plaintext secrets to disk.
  [CYNAI.CLIENT.CliSecurityModel](../tech_specs/cynork_cli.md#spec-cynai-client-clisecurity)
  [CYNAI.CLIENT.CliConfigFileLocation](../tech_specs/cynork_cli.md#spec-cynai-client-cliconfigfilelocation)
  [CYNAI.CLIENT.CliCredentialHelperProtocol](../tech_specs/cynork_cli.md#spec-cynai-client-clicredentialhelperprotocol)
  [REQ-STANDS-0133](../requirements/stands.md#req-stands-0133) (Go code handling secrets)
  <a id="req-client-0103"></a>
- **REQ-CLIENT-0104:** The CLI MUST support least privilege and MUST fail closed on authorization errors.
  [CYNAI.CLIENT.CliSecurityModel](../tech_specs/cynork_cli.md#spec-cynai-client-clisecurity)
  <a id="req-client-0104"></a>
- **REQ-CLIENT-0105:** The CLI MUST support token-based authentication.
  [CYNAI.CLIENT.CliAuthConfig](../tech_specs/cynork_cli.md#spec-cynai-client-cliauth)
  [CYNAI.CLIENT.CliTokenResolution](../tech_specs/cynork_cli.md#spec-cynai-client-clitokenresolution)
  [REQ-STANDS-0133](../requirements/stands.md#req-stands-0133) (Go code handling tokens)
  <a id="req-client-0105"></a>
- **REQ-CLIENT-0106:** The CLI SHOULD support reading tokens from env vars for CI usage.
  [CYNAI.CLIENT.CliAuthConfig](../tech_specs/cynork_cli.md#spec-cynai-client-cliauth)
  [CYNAI.CLIENT.CliTokenResolution](../tech_specs/cynork_cli.md#spec-cynai-client-clitokenresolution)
  <a id="req-client-0106"></a>
- **REQ-CLIENT-0107:** The CLI SHOULD support optional mTLS or pinned CA bundles for enterprise deployments.
  [CYNAI.CLIENT.CliAuthConfig](../tech_specs/cynork_cli.md#spec-cynai-client-cliauth)
  <a id="req-client-0107"></a>
- **REQ-CLIENT-0113:** MUST compute effective preferences for the task by merging scopes in precedence order.
  The precedence order MUST be task => project => user => group => system.
  If multiple entries exist for the same key at the same scope, clients MUST apply deterministic tie-breakers (for example by timestamp and stable identifier).
  [CYNAI.STANDS.UserPreferencesRetrieval](../tech_specs/user_preferences.md#spec-cynai-stands-prefretrieval)
  <a id="req-client-0113"></a>
- **REQ-CLIENT-0114:** MUST treat unknown keys as opaque and pass them through to verification/tooling.
  Unknown keys include user-defined keys not documented by CyNodeAI specs.
  [CYNAI.STANDS.UserPreferencesRetrieval](../tech_specs/user_preferences.md#spec-cynai-stands-prefretrieval)
  <a id="req-client-0114"></a>
- **REQ-CLIENT-0115:** SHOULD cache effective preferences per task revision, but MUST invalidate on preference update.
  Cache invalidation MUST occur when any relevant preference entry changes for the task context (system, user, group, project, or task scope).
  [CYNAI.STANDS.UserPreferencesRetrieval](../tech_specs/user_preferences.md#spec-cynai-stands-prefretrieval)
  <a id="req-client-0115"></a>
- **REQ-CLIENT-0116:** Credential create MUST accept secrets only on create or rotate operations.
  [CYNAI.WEBCON.Credential](../tech_specs/web_console.md#spec-cynai-webcon-credential)
  [CYNAI.CLIENT.CliCredentialManagement](../tech_specs/cli_management_app_commands_admin.md#spec-cynai-client-clicredential)
  <a id="req-client-0116"></a>
- **REQ-CLIENT-0117:** Credential read MUST return metadata only.
  [CYNAI.WEBCON.Credential](../tech_specs/web_console.md#spec-cynai-webcon-credential)
  [CYNAI.CLIENT.CliCredentialManagement](../tech_specs/cli_management_app_commands_admin.md#spec-cynai-client-clicredential)
  <a id="req-client-0117"></a>
- **REQ-CLIENT-0118:** Credential list MUST support filtering by provider and scope.
  [CYNAI.WEBCON.Credential](../tech_specs/web_console.md#spec-cynai-webcon-credential)
  [CYNAI.CLIENT.CliCredentialManagement](../tech_specs/cli_management_app_commands_admin.md#spec-cynai-client-clicredential)
  <a id="req-client-0118"></a>
- **REQ-CLIENT-0119:** Credential rotate MUST create a new encrypted secret value and invalidate the old one.
  [CYNAI.WEBCON.Credential](../tech_specs/web_console.md#spec-cynai-webcon-credential)
  [CYNAI.CLIENT.CliCredentialManagement](../tech_specs/cli_management_app_commands_admin.md#spec-cynai-client-clicredential)
  <a id="req-client-0119"></a>
- **REQ-CLIENT-0120:** Credential disable MUST support immediate deactivation.
  [CYNAI.WEBCON.Credential](../tech_specs/web_console.md#spec-cynai-webcon-credential)
  [CYNAI.CLIENT.CliCredentialManagement](../tech_specs/cli_management_app_commands_admin.md#spec-cynai-client-clicredential)
  <a id="req-client-0120"></a>
- **REQ-CLIENT-0121:** Preference edits MUST be scoped and versioned.
  Updates SHOULD use optimistic concurrency via expected-version checks to prevent lost updates.
  When an expected version is provided and does not match the current stored version, the update MUST fail with a conflict error.
  [CYNAI.WEBCON.Preferences](../tech_specs/web_console.md#spec-cynai-webcon-preferences)
  [CYNAI.CLIENT.CliPreferencesManagement](../tech_specs/cli_management_app_commands_admin.md#spec-cynai-client-clipreferences)
  <a id="req-client-0121"></a>
- **REQ-CLIENT-0122:** The UI MUST support preference scope selection (system, user, group, project, task).
  For the web console, this MUST include an easy method for users to create, read, update, and delete their personal (user), group, and project preferences.
  [CYNAI.WEBCON.Preferences](../tech_specs/web_console.md#spec-cynai-webcon-preferences)
  [CYNAI.CLIENT.CliPreferencesManagement](../tech_specs/cli_management_app_commands_admin.md#spec-cynai-client-clipreferences)
  <a id="req-client-0122"></a>
- **REQ-CLIENT-0123:** The UI SHOULD provide an \"effective preferences\" preview for a given task or project.
  [CYNAI.WEBCON.Preferences](../tech_specs/web_console.md#spec-cynai-webcon-preferences)
  [CYNAI.CLIENT.CliPreferencesManagement](../tech_specs/cli_management_app_commands_admin.md#spec-cynai-client-clipreferences)
  <a id="req-client-0123"></a>
- **REQ-CLIENT-0124:** The UI SHOULD provide validation for known keys and types.
  The UI MUST allow user-defined keys to be stored even when it cannot validate their semantics, as long as the key and JSON value are valid.
  [CYNAI.WEBCON.Preferences](../tech_specs/web_console.md#spec-cynai-webcon-preferences)
  [CYNAI.CLIENT.CliPreferencesManagement](../tech_specs/cli_management_app_commands_admin.md#spec-cynai-client-clipreferences)
  <a id="req-client-0124"></a>
- **REQ-CLIENT-0125:** Node management MUST be mediated by the User API Gateway.
  [CYNAI.WEBCON.NodeManagement](../tech_specs/web_console.md#spec-cynai-webcon-nodemanagement)
  [CYNAI.CLIENT.CliNodeManagement](../tech_specs/cli_management_app_commands_admin.md#spec-cynai-client-clinodemgmt)
  <a id="req-client-0125"></a>
- **REQ-CLIENT-0126:** The UI MUST NOT connect directly to node worker APIs.
  [CYNAI.WEBCON.NodeManagement](../tech_specs/web_console.md#spec-cynai-webcon-nodemanagement)
  [CYNAI.CLIENT.CliNodeManagement](../tech_specs/cli_management_app_commands_admin.md#spec-cynai-client-clinodemgmt)
  <a id="req-client-0126"></a>
- **REQ-CLIENT-0127:** The UI MUST clearly distinguish between node-reported state and orchestrator-derived state.
  [CYNAI.WEBCON.NodeManagement](../tech_specs/web_console.md#spec-cynai-webcon-nodemanagement)
  <a id="req-client-0127"></a>
- **REQ-CLIENT-0128:** Potentially disruptive actions MUST be gated by admin authorization and SHOULD require confirmation.
  [CYNAI.WEBCON.NodeManagement](../tech_specs/web_console.md#spec-cynai-webcon-nodemanagement)
  [CYNAI.CLIENT.CliNodeManagement](../tech_specs/cli_management_app_commands_admin.md#spec-cynai-client-clinodemgmt)
  <a id="req-client-0128"></a>
- **REQ-CLIENT-0136:** The interactive CLI mode MUST provide access to the same commands and flags as the non-interactive CLI.
  [CYNAI.CLIENT.CliInteractiveMode](../tech_specs/cli_management_app_shell_output.md#spec-cynai-client-cliinteractivemode)
  <a id="req-client-0136"></a>
- **REQ-CLIENT-0137:** The interactive CLI mode MUST support tab completion for commands/subcommands, flags, and known enumerated flag values.
  [CYNAI.CLIENT.CliInteractiveMode](../tech_specs/cli_management_app_shell_output.md#spec-cynai-client-cliinteractivemode)
  <a id="req-client-0137"></a>
- **REQ-CLIENT-0138:** The interactive CLI mode MUST support in-session help.
  [CYNAI.CLIENT.CliInteractiveMode](../tech_specs/cli_management_app_shell_output.md#spec-cynai-client-cliinteractivemode)
  <a id="req-client-0138"></a>
- **REQ-CLIENT-0139:** The interactive CLI mode MUST support `exit` and `quit`.
  [CYNAI.CLIENT.CliInteractiveMode](../tech_specs/cli_management_app_shell_output.md#spec-cynai-client-cliinteractivemode)
  <a id="req-client-0139"></a>
- **REQ-CLIENT-0140:** The interactive CLI mode MUST NOT store secrets in history, and secret prompts MUST bypass history recording.
  [CYNAI.CLIENT.CliInteractiveMode](../tech_specs/cli_management_app_shell_output.md#spec-cynai-client-cliinteractivemode)
  <a id="req-client-0140"></a>
- **REQ-CLIENT-0141:** If a persistent history file is implemented, it MUST be stored under the CLI config directory with permissions `0600`.
  [CYNAI.CLIENT.CliInteractiveMode](../tech_specs/cli_management_app_shell_output.md#spec-cynai-client-cliinteractivemode)
  <a id="req-client-0141"></a>
- **REQ-CLIENT-0142:** Tab completion MUST NOT fetch or reveal secret values.
  [CYNAI.CLIENT.CliInteractiveMode](../tech_specs/cli_management_app_shell_output.md#spec-cynai-client-cliinteractivemode)
  <a id="req-client-0142"></a>
- **REQ-CLIENT-0143:** The CLI MUST support JSON output mode.
  [CYNAI.CLIENT.CliOutputScripting](../tech_specs/cli_management_app_shell_output.md#spec-cynai-client-clioutputscripting)
  <a id="req-client-0143"></a>
- **REQ-CLIENT-0144:** The CLI SHOULD support table output mode for humans.
  [CYNAI.CLIENT.CliOutputScripting](../tech_specs/cli_management_app_shell_output.md#spec-cynai-client-clioutputscripting)
  <a id="req-client-0144"></a>
- **REQ-CLIENT-0145:** The CLI SHOULD return non-zero exit codes on failures and policy denials.
  [CYNAI.CLIENT.CliOutputScripting](../tech_specs/cli_management_app_shell_output.md#spec-cynai-client-clioutputscripting)
  <a id="req-client-0145"></a>
- **REQ-CLIENT-0146:** The CLI MUST support full CRUD for skills (create/load, list, get, update, delete) via the User API Gateway, with the same controls as defined in the skills spec (scope visibility, scope elevation permission, auditing on write).
  When a CLI skills command references an existing skill, it MUST accept either the backend skill identifier or a user-typeable skill selector.
  [CYNAI.CLIENT.CliSkillsManagement](../tech_specs/cli_management_app_commands_admin.md#spec-cynai-client-cliskillsmanagement)
  [CYNAI.SKILLS.SkillManagementCrud](../tech_specs/skills_storage_and_inference.md#spec-cynai-skills-skillmanagementcrud)
  <a id="req-client-0146"></a>
- **REQ-CLIENT-0149:** The CLI MUST support a local key (gateway token) stored in the user config dir (e.g. `~/.config/cynork/config.yaml`) and SHOULD support reading the token from a password store or credential helper (kubectl-style) so the token need not be stored in plaintext.
  [CYNAI.CLIENT.CliAuthConfig](../tech_specs/cynork_cli.md#spec-cynai-client-cliauth)
  [CYNAI.CLIENT.CliConfigFileLocation](../tech_specs/cynork_cli.md#spec-cynai-client-cliconfigfilelocation)
  [CYNAI.CLIENT.CliTokenResolution](../tech_specs/cynork_cli.md#spec-cynai-client-clitokenresolution)
  [CYNAI.CLIENT.CliCredentialHelperProtocol](../tech_specs/cynork_cli.md#spec-cynai-client-clicredentialhelperprotocol)
  <a id="req-client-0149"></a>
- **REQ-CLIENT-0150:** The CLI MUST store session credentials (e.g. token after login) in a reliable way so that multiple consecutive CLI invocations can reuse the token without re-authenticating.
  The config file path MUST be resolved consistently (e.g. honoring XDG_CONFIG_HOME); writes MUST be atomic (e.g. temp file then rename) so a crash does not leave a partial file; and if the default config path cannot be resolved (e.g. no home dir), login and logout MUST fail with a clear error.
  [CYNAI.CLIENT.CliConfigFileLocation](../tech_specs/cynork_cli.md#spec-cynai-client-cliconfigfilelocation)
  [CYNAI.CLIENT.CliSessionPersistence](../tech_specs/cynork_cli.md#spec-cynai-client-clisessionpersistence)
  <a id="req-client-0150"></a>
- **REQ-CLIENT-0151:** The CLI MUST allow passing a task as inline text (e.g. `--prompt` or `--task`) or from a file (e.g. `--task-file <path>`) containing plain text or Markdown.
  The default is interpretation; the system interprets the task and uses inference when needed (no user-facing "use inference" flag).
  The CLI MUST support optionally associating a created task with a `project_id` (for example `cynork task create --project-id <id>`).
  When `project_id` is omitted, the task MUST be associated with the authenticated user's default project (see [REQ-PROJCT-0104](../requirements/projct.md#req-projct-0104)).
  Attachments are specified in REQ-CLIENT-0157.
  [CYNAI.CLIENT.CliTaskCreatePrompt](../tech_specs/cli_management_app_commands_tasks.md#spec-cynai-client-clitaskcreateprompt)
  [CYNAI.ACCESS.Doc.ProjectsAndScopes](../tech_specs/projects_and_scopes.md#spec-cynai-access-doc-projectsandscopes)
  <a id="req-client-0151"></a>
- **REQ-CLIENT-0152:** The web console MUST allow submitting a task as plain text or Markdown (inline or from paste) and MUST support attaching files or other artifacts (e.g. file upload) with the same semantics as the CLI and gateway.
  The console MUST support optionally associating the submitted task with a `project_id`.
  When `project_id` is omitted, the task MUST be associated with the authenticated user's default project (see [REQ-PROJCT-0104](../requirements/projct.md#req-projct-0104)).
  [CYNAI.CLIENT.WebConsoleCapabilityParity](../tech_specs/web_console.md#spec-cynai-client-webconcapabilityparity)
  [CYNAI.ACCESS.Doc.ProjectsAndScopes](../tech_specs/projects_and_scopes.md#spec-cynai-access-doc-projectsandscopes)
  <a id="req-client-0152"></a>
- **REQ-CLIENT-0153:** The CLI MUST support running a script via a flag (e.g. `--script <path>`) and a short series of commands (e.g. `--commands` with one or more commands or repeatable `--command`).
  These are explicit "run script" / "run commands" modes; the system runs the script or commands in the sandbox rather than interpreting as natural language.
  [CYNAI.CLIENT.CliTaskCreatePrompt](../tech_specs/cli_management_app_commands_tasks.md#spec-cynai-client-clitaskcreateprompt)
  <a id="req-client-0153"></a>
- **REQ-CLIENT-0154:** The web console MUST support running a script (e.g. script file upload or path) and a short series of commands (e.g. multi-line or list input) with the same semantics as the CLI and gateway.
  [CYNAI.CLIENT.WebConsoleCapabilityParity](../tech_specs/web_console.md#spec-cynai-client-webconcapabilityparity)
  <a id="req-client-0154"></a>
- **REQ-CLIENT-0155:** The CLI command surface for tasks MUST be fully specified and MUST be treated as a stable user-facing contract.
  The CLI MUST implement the task subcommand set, flag names, argument ordering, mutual exclusions, default values, confirmation behavior, output schemas, task-selector semantics, and exit codes exactly as defined in the CLI management app spec.
  When a CLI task command references an existing task, it MUST accept either the backend task UUID or a user-typeable human-readable task name.
  [CYNAI.CLIENT.CliCommandSurface](../tech_specs/cynork_cli.md#spec-cynai-client-clicommandsurface)
  <a id="req-client-0155"></a>
- **REQ-CLIENT-0156:** The CLI MUST return deterministic exit codes for common failure categories (usage error, auth error, gateway error, not found, conflict, validation error).
  The CLI MUST write machine-parseable output only to stdout when `--output json` is selected, and MUST write all human messages and errors to stderr in JSON mode.
  [CYNAI.CLIENT.CliCommandSurface](../tech_specs/cynork_cli.md#spec-cynai-client-clicommandsurface)
  [CYNAI.CLIENT.CliOutputScripting](../tech_specs/cli_management_app_shell_output.md#spec-cynai-client-clioutputscripting)
  <a id="req-client-0156"></a>
- **REQ-CLIENT-0157:** The CLI MUST support other attachment types by accepting one or more path strings (e.g. repeatable `--attach <path>`).
  For each provided path, the CLI MUST validate the path and MUST send the attachment payload to the gateway according to the task-create API contract.
  [CYNAI.CLIENT.CliTaskCreatePrompt](../tech_specs/cli_management_app_commands_tasks.md#spec-cynai-client-clitaskcreateprompt)
  <a id="req-client-0157"></a>
- **REQ-CLIENT-0158:** The CLI MUST support shorthand aliases (`-<x>`) for the most commonly used flags.
  The required shorthand aliases and their long forms MUST be defined in the CLI management app spec and MUST be treated as a stable user-facing contract.
  [CYNAI.CLIENT.CliCommandSurface](../tech_specs/cynork_cli.md#spec-cynai-client-clicommandsurface)
  <a id="req-client-0158"></a>
- **REQ-CLIENT-0159:** The CLI interactive mode MUST support tab-completion of task names when a task identifier is expected (e.g. for `task get`, `task result`, `task cancel`, `task logs`, `task artifacts list`, `task artifacts get`).
  [CYNAI.CLIENT.CliInteractiveMode](../tech_specs/cli_management_app_shell_output.md#spec-cynai-client-cliinteractivemode)
  <a id="req-client-0159"></a>
- **REQ-CLIENT-0160:** The Web Console and the CLI MUST allow configuring Project Manager model selection for local inference via system settings.
  At minimum, clients MUST allow editing `agents.project_manager.model.local_default_ollama_model` through the standard system settings management surface (no bespoke API required).
  Clients SHOULD also surface the automatic selection policy keys for discoverability (for example `agents.project_manager.model.selection.execution_mode`, `agents.project_manager.model.selection.mode`, and `agents.project_manager.model.selection.prefer_orchestrator_host`).
  System settings are distinct from user preferences; see [User preferences (Terminology)](../tech_specs/user_preferences.md#spec-cynai-stands-preferenceterminology).
  [CYNAI.WEBCON.SystemSettings](../tech_specs/web_console.md#spec-cynai-webcon-systemsettings)
  [CYNAI.CLIENT.CliSystemSettingsManagement](../tech_specs/cli_management_app_commands_admin.md#spec-cynai-client-clisystemsettings)
  <a id="req-client-0160"></a>
- **REQ-CLIENT-0161:** The CLI MUST provide a single interactive cynork chat surface for chatting with the Project Manager (PM) model.
  The user MUST be able to send messages and receive responses in turn until they exit the session.
  The interactive surface MUST use the same gateway and authentication as other CLI commands and MUST NOT expose secrets in history or output.
  `cynork tui` MUST be the canonical explicit entrypoint for that interactive surface, and interactive `cynork chat` MUST remain available as a user-facing path to the same surface rather than a separate long-term implementation.
  The interactive surface MUST support the gateway's OpenAI-compatible interactive chat surfaces as defined by the corresponding tech spec.
  [CYNAI.CLIENT.CliChat](../tech_specs/cli_management_app_commands_chat.md#spec-cynai-client-clichat)
  [CYNAI.CLIENT.CynorkTui.EntryPoint](../tech_specs/cynork_tui.md#spec-cynai-client-cynorktui-entrypoint)
  [CYNAI.USRGWY.OpenAIChatApi](../tech_specs/openai_compatible_chat_api.md#spec-cynai-usrgwy-openaichatapi)
  <a id="req-client-0161"></a>
- **REQ-CLIENT-0162:** The interactive cynork chat surface MUST render model responses with pretty-formatted output when the response contains Markdown.
  The CLI MUST interpret common Markdown (headings, lists, code blocks, emphasis, links) and display them in a human-readable way in the terminal (e.g. indentation, styling, or syntax highlighting for code blocks).
  The CLI MUST honor `--no-color` for chat output and SHOULD support a plain-text mode (e.g. `--plain`) that prints the raw response without Markdown rendering for scripting or piping.
  [CYNAI.CLIENT.CliChat](../tech_specs/cli_management_app_commands_chat.md#spec-cynai-client-clichat)
  [CYNAI.CLIENT.CynorkChat.TUILayout](../tech_specs/cynork_tui.md#spec-cynai-client-cynorkchat-tuilayout)
  <a id="req-client-0162"></a>
- **REQ-CLIENT-0163:** The CLI MUST display all JSON as pretty-printed (indented, with newlines) whenever JSON is part of the output.
  This applies to `--output json`, JSON embedded in chat or other responses, and any other CLI output that contains JSON.
  [CYNAI.CLIENT.CliPrettyPrintJson](../tech_specs/cli_management_app_shell_output.md#spec-cynai-client-cliprettyprintjson)
  <a id="req-client-0163"></a>
- **REQ-CLIENT-0164:** The interactive cynork chat surface MUST display the available slash commands (e.g. `/exit`, `/quit`, `/help`) to the user.
  Display MUST occur at session start or in response to a dedicated help command (e.g. `/help`) so users can discover slash commands.
  [CYNAI.CLIENT.CliChatSlashCommands](../tech_specs/cli_management_app_commands_chat.md#spec-cynai-client-clichatslashcommands)
  [CYNAI.CLIENT.CynorkTui.LocalSlashCommands](../tech_specs/cynork_tui_slash_commands.md#spec-cynai-client-cynorktui-localslashcommands)
  <a id="req-client-0164"></a>
- **REQ-CLIENT-0165:** The interactive cynork chat surface MUST support slash-command autocomplete and inline suggestions when the user types `/`.
  When the input line starts with `/`, the CLI MUST show the list of available slash commands with short descriptions (e.g. command on the left, description on the right) and MUST support Tab to complete or cycle and arrow-up/arrow-down to move through suggestions, with the current selection visually indicated.
  [CYNAI.CLIENT.CliChatSlashAutocomplete](../tech_specs/cli_management_app_commands_chat.md#spec-cynai-client-clichatslashautocomplete)
  <a id="req-client-0165"></a>
- **REQ-CLIENT-0166:** The interactive cynork chat surface MUST support task operations via slash commands: list, get, create (with inline prompt), cancel, result, logs, and artifacts list.
  These MUST use the same User API Gateway endpoints as `cynork task`; output MUST be shown inline in chat (pretty-printed when JSON).
  When a chat slash command references an existing task, it MUST accept either the backend task UUID or a user-typeable human-readable task name.
  [CYNAI.CLIENT.CliChatSlashTask](../tech_specs/cli_management_app_commands_chat.md#spec-cynai-client-clichatslashtask)
  [CYNAI.CLIENT.CynorkTui.TaskSlashCommands](../tech_specs/cynork_tui_slash_commands.md#spec-cynai-client-cynorktui-taskslashcommands)
  <a id="req-client-0166"></a>
- **REQ-CLIENT-0167:** The interactive cynork chat surface MUST support `/status` and `/whoami` slash commands to show gateway reachability and current identity without leaving chat.
  [CYNAI.CLIENT.CliChatSlashStatus](../tech_specs/cli_management_app_commands_chat.md#spec-cynai-client-clichatslashstatus)
  [CYNAI.CLIENT.CynorkTui.StatusSlashCommands](../tech_specs/cynork_tui_slash_commands.md#spec-cynai-client-cynorktui-statusslashcommands)
  <a id="req-client-0167"></a>
- **REQ-CLIENT-0168:** The interactive cynork chat surface MUST support node listing and get via slash commands: `/nodes list`, `/nodes get <node_id>`.
  [CYNAI.CLIENT.CliChatSlashNodes](../tech_specs/cli_management_app_commands_chat.md#spec-cynai-client-clichatslashnodes)
  [CYNAI.CLIENT.CynorkTui.NodeSlashCommands](../tech_specs/cynork_tui_slash_commands.md#spec-cynai-client-cynorktui-nodeslashcommands)
  <a id="req-client-0168"></a>
- **REQ-CLIENT-0169:** The interactive cynork chat surface MUST support preferences via slash commands: list, get, set, delete, and effective (e.g. `/prefs list`, `/prefs get [key]`, `/prefs set ...`, `/prefs delete ...`, `/prefs effective [--task-id <id>]`).
  [CYNAI.CLIENT.CliChatSlashPrefs](../tech_specs/cli_management_app_commands_chat.md#spec-cynai-client-clichatslashprefs)
  [CYNAI.CLIENT.CynorkTui.PreferenceSlashCommands](../tech_specs/cynork_tui_slash_commands.md#spec-cynai-client-cynorktui-preferenceslashcommands)
  <a id="req-client-0169"></a>
- **REQ-CLIENT-0170:** The interactive cynork chat surface MUST support full skills CRUD via slash commands: `/skills list`, `/skills get <skill_selector>`, `/skills load <file.md>`, `/skills update <skill_selector> <file.md>`, and `/skills delete <skill_selector>`.
  When a slash command references an existing skill, it MUST accept either the backend skill identifier or a user-typeable skill selector.
  [CYNAI.CLIENT.CliChatSlashSkills](../tech_specs/cli_management_app_commands_chat.md#spec-cynai-client-clichatslashskills)
  [CYNAI.CLIENT.CynorkTui.SkillSlashCommands](../tech_specs/cynork_tui_slash_commands.md#spec-cynai-client-cynorktui-skillslashcommands)
  <a id="req-client-0170"></a>
- **REQ-CLIENT-0171:** The interactive cynork chat surface MUST support selecting an OpenAI model identifier for chat completions.
  The CLI MUST support selecting the model at session start (e.g. `cynork chat --model <id>`) and within the session (e.g. `/model <id>`).
  Model selection MUST only affect interactive chat requests (for example `POST /v1/chat/completions` or `POST /v1/responses`) and MUST NOT change system settings or user preferences.
  [CYNAI.CLIENT.CliChatModelSelection](../tech_specs/cli_management_app_commands_chat.md#spec-cynai-client-clichatmodelselection)
  [CYNAI.CLIENT.CynorkTui.ModelSlashCommands](../tech_specs/cynork_tui_slash_commands.md#spec-cynai-client-cynorktui-modelslashcommands)
  [CYNAI.USRGWY.OpenAIChatApi.Endpoints](../tech_specs/openai_compatible_chat_api.md#spec-cynai-usrgwy-openaichatapi-endpoints)
  <a id="req-client-0171"></a>
- **REQ-CLIENT-0172:** The interactive cynork chat surface SHOULD support listing available OpenAI model identifiers from the gateway (e.g. `/models`).
  This MUST call `GET /v1/models` and display model ids.
  [CYNAI.CLIENT.CliChatModelSelection](../tech_specs/cli_management_app_commands_chat.md#spec-cynai-client-clichatmodelselection)
  [CYNAI.CLIENT.CynorkTui.ModelSlashCommands](../tech_specs/cynork_tui_slash_commands.md#spec-cynai-client-cynorktui-modelslashcommands)
  [CYNAI.USRGWY.OpenAIChatApi.Endpoints](../tech_specs/openai_compatible_chat_api.md#spec-cynai-usrgwy-openaichatapi-endpoints)
  <a id="req-client-0172"></a>
- **REQ-CLIENT-0173:** The interactive cynork chat surface MUST support setting an optional project context for the chat session.
  When set, the CLI MUST send the project context using the OpenAI-standard `OpenAI-Project` request header on interactive chat requests that support that header (for example `POST /v1/chat/completions`).
  When omitted, the CLI does not send the header and the gateway associates the thread with the user's default project (see [REQ-PROJCT-0104](../requirements/projct.md#req-projct-0104)).
  [CYNAI.CLIENT.CliChatProjectContext](../tech_specs/cli_management_app_commands_chat.md#spec-cynai-client-clichatprojectcontext)
  [CYNAI.CLIENT.CynorkTui.ProjectSlashCommands](../tech_specs/cynork_tui_slash_commands.md#spec-cynai-client-cynorktui-projectslashcommands)
  [CYNAI.ACCESS.Doc.ProjectsAndScopes](../tech_specs/projects_and_scopes.md#spec-cynai-access-doc-projectsandscopes)
  <a id="req-client-0173"></a>
- **REQ-CLIENT-0174:** The CLI and the Web Console MUST support basic project management (CRUD: create, list, get, update, delete or disable) via the User API Gateway.
  Projects have a user-friendly title and optional text description (see [REQ-PROJCT-0103](../requirements/projct.md#req-projct-0103)); clients MUST allow setting and editing these in create and update flows.
  Both clients MUST offer the same project CRUD capabilities (capability parity).
  [CYNAI.CLIENT.CliProjectManagement](../tech_specs/cli_management_app_commands_admin.md#spec-cynai-client-cliprojectmanagement)
  [CYNAI.WEBCON.ProjectManagement](../tech_specs/web_console.md#spec-cynai-webcon-projectmanagement)
  [CYNAI.ACCESS.Doc.ProjectsAndScopes](../tech_specs/projects_and_scopes.md#spec-cynai-access-doc-projectsandscopes)
  <a id="req-client-0174"></a>
- **REQ-CLIENT-0175:** The interactive cynork chat surface MUST support a shell-escape syntax: input starting with `!` runs the remainder of the line as a shell command; the command's output is displayed inline and the chat session continues.
  [CYNAI.CLIENT.CliChatShellEscape](../tech_specs/cli_management_app_commands_chat.md#spec-cynai-client-clichatshellescape)
  <a id="req-client-0175"></a>
- **REQ-CLIENT-0176:** When a slash command or shell-escape command fails in the interactive cynork chat surface (e.g. gateway 404, command not found), the CLI MUST display the error and MUST NOT exit the chat session or show the top-level command Usage.
  [CYNAI.CLIENT.CliChatSubcommandErrors](../tech_specs/cli_management_app_commands_chat.md#spec-cynai-client-clichatsubcommanderrors)
  <a id="req-client-0176"></a>
- **REQ-CLIENT-0177:** Clients that provide an interactive chat session (e.g. CLI chat command, Web Console chat UI) SHOULD call the gateway chat warm-up endpoint after auth and before the first user prompt when the gateway exposes it.
  Warm-up MUST be non-blocking (e.g. fire-and-forget or short timeout) so the user can type immediately.
  [CYNAI.USRGWY.OpenAIChatApi.WarmUp](../tech_specs/openai_compatible_chat_api.md#spec-cynai-usrgwy-openaichatapi-warmup)
  [CYNAI.CLIENT.CliChatWarmUp](../tech_specs/cli_management_app_commands_chat.md#spec-cynai-client-clichatwarmup)
  <a id="req-client-0177"></a>
- **REQ-CLIENT-0178:** The CLI chat command MUST support a one-shot mode: when invoked with `-m, --message <text>`, the CLI MUST send that single message to the gateway via the configured OpenAI-compatible interactive chat surface, print the completion content (subject to `--plain` and `--no-color`), and exit.
  The CLI MUST NOT enter the interactive loop when `--message` is provided.
  One-shot mode MUST use the same auth, gateway URL, model, and project context as interactive chat (e.g. `--model`, `--project-id` apply when present).
  [CYNAI.CLIENT.CliChatOneShot](../tech_specs/cli_management_app_commands_chat.md#spec-cynai-client-clichatoneshot)
  <a id="req-client-0178"></a>
- **REQ-CLIENT-0179:** The Web Console and the CLI MUST support full CRUD for Agent personas (create, list, get, update, delete) via the User API Gateway, with capability parity between both clients.
  Agent personas are reusable SBA role/identity descriptions (not customer or end-user personas); title, description, scope; create/update/delete subject to RBAC per scope; see [cynode_sba.md - Persona on the Job](../tech_specs/cynode_sba.md#spec-cynai-sbagnt-jobpersona).
  [CYNAI.CLIENT.CliPersonasManagement](../tech_specs/cli_management_app_commands_admin.md#spec-cynai-client-clipersonasmanagement)
  [CYNAI.WEBCON.PersonasManagement](../tech_specs/web_console.md#spec-cynai-webcon-personasmanagement)
  <a id="req-client-0179"></a>
- **REQ-CLIENT-0180:** The Web Console and the CLI MUST support project plan review (view plan, view revision history) and plan approve (and re-approve) via the User API Gateway, with capability parity between both clients.
  [CYNAI.ACCESS.ProjectPlanReviewApprove](../tech_specs/projects_and_scopes.md#spec-cynai-access-projectplanreviewapprove)
  [CYNAI.USRGWY.ProjectPlanApi](../tech_specs/user_api_gateway.md#spec-cynai-usrgwy-projectplanapi)
  <a id="req-client-0180"></a>
- **REQ-CLIENT-0181:** The interactive cynork chat surface MUST start each session with a **new thread by default**.
  Resuming a previous thread MUST require an explicit startup option (e.g. `cynork tui --resume-thread <thread_selector>` or `cynork chat --resume-thread <thread_selector>`).
  The surface MUST support explicit fresh-thread creation during an active session (e.g. `/thread new`).
  Thread creation MUST use the gateway `POST /v1/chat/threads` and MUST respect the current project context (e.g. `OpenAI-Project` header or active project) for the new thread.
  Subsequent chat completion requests MUST remain OpenAI-compatible and MUST NOT require any CyNodeAI-specific thread identifier in the request body or headers.
  [CYNAI.CLIENT.CliChatThreadControls](../tech_specs/cli_management_app_commands_chat.md#spec-cynai-client-clichatthreadcontrols)
  [CYNAI.CLIENT.CynorkTui.ThreadSlashCommands](../tech_specs/cynork_tui_slash_commands.md#spec-cynai-client-cynorktui-threadslashcommands)
  [REQ-USRGWY-0135](../requirements/usrgwy.md#req-usrgwy-0135)
  <a id="req-client-0181"></a>
- **REQ-CLIENT-0182:** Clients with a rich chat UI MUST prefer the structured chat-turn representation when the gateway provides it and MUST fall back to canonical plain-text transcript content when it does not.
  [CYNAI.CLIENT.CynorkTui.TranscriptRendering](../tech_specs/cynork_tui.md#spec-cynai-client-cynorktui-transcriptrendering)
  [CYNAI.CLIENT.CliChatResponseOutput](../tech_specs/cli_management_app_commands_chat.md#spec-cynai-client-clichatresponseoutput)
  <a id="req-client-0182"></a>
- **REQ-CLIENT-0183:** Clients with a rich chat UI MUST NOT render model thinking or reasoning content as normal assistant transcript text by default.
  When thinking data is available, the client MUST keep it collapsed by default behind a visible, visually distinct placeholder or block affordance rather than rendering raw reasoning inline as normal assistant prose.
  [CYNAI.CLIENT.CynorkTui.ThinkingVisibilityBehavior](../tech_specs/cynork_tui_slash_commands.md#spec-cynai-client-cynorktui-thinkingvisibilitybehavior)
  [CYNAI.CLIENT.CynorkTui.TranscriptRendering](../tech_specs/cynork_tui.md#spec-cynai-client-cynorktui-transcriptrendering)
  [CYNAI.CLIENT.CliChatResponseOutput](../tech_specs/cli_management_app_commands_chat.md#spec-cynai-client-clichatresponseoutput)
  <a id="req-client-0183"></a>
- **REQ-CLIENT-0184:** When one user prompt yields multiple assistant-side output items, clients with a rich chat UI MUST render those items in order as one logical assistant turn rather than as unrelated transcript entries.
  At minimum, the client MUST distinguish visible assistant text from tool activity, and SHOULD render download or attachment references as explicit non-prose items when present.
  [CYNAI.CLIENT.CynorkTui.TranscriptRendering](../tech_specs/cynork_tui.md#spec-cynai-client-cynorktui-transcriptrendering)
  [CYNAI.CLIENT.CliChatResponseOutput](../tech_specs/cli_management_app_commands_chat.md#spec-cynai-client-clichatresponseoutput)
  <a id="req-client-0184"></a>
- **REQ-CLIENT-0185:** Interactive chat UIs SHOULD update the in-flight assistant turn progressively when the gateway or provider exposes streaming or incremental output.
  While a turn is still being processed, the interactive UI SHOULD show a visible in-flight status indicator for the active assistant turn.
  When the final assistant turn is committed, the client SHOULD reconcile in-flight placeholders, thinking indicators, and tool-activity rows into the final ordered transcript without duplicating visible assistant text.
  [CYNAI.CLIENT.CynorkTui.GenerationState](../tech_specs/cynork_tui.md#spec-cynai-client-cynorktui-generationstate)
  <a id="req-client-0185"></a>
- **REQ-CLIENT-0186:** Plain or one-shot chat output intended for piping or scripting MUST emit only the canonical visible assistant text.
  Hidden thinking data, tool metadata, and other non-text structured items MUST NOT be emitted in `--plain` output unless the user explicitly opts into a structured output mode defined elsewhere.
  [CYNAI.CLIENT.CliChatOneShot](../tech_specs/cli_management_app_commands_chat.md#spec-cynai-client-clichatoneshot)
  [CYNAI.CLIENT.CliChatResponseOutput](../tech_specs/cli_management_app_commands_chat.md#spec-cynai-client-clichatresponseoutput)
  <a id="req-client-0186"></a>
- **REQ-CLIENT-0187:** The interactive cynork chat surface MAY persist local configuration for UI preferences such as default model, composer mode, context-pane visibility, and keybinding overrides.
  Any persisted interactive-chat configuration MUST use the same config directory as the rest of cynork and MUST NOT store secrets, tokens, passwords, or message content.
  [CYNAI.CLIENT.CynorkChat.LocalConfig](../tech_specs/cynork_tui.md#spec-cynai-client-cynorkchat-localconfig)
  <a id="req-client-0187"></a>
- **REQ-CLIENT-0188:** The interactive cynork chat surface MAY use a local cache for completion and list data such as task identifiers, project identifiers, model identifiers, and thread-list metadata.
  Any such cache MUST live under the CLI cache directory, MUST NOT store secrets or message content, and SHOULD define bounded TTL or invalidation behavior.
  [CYNAI.CLIENT.CynorkChat.LocalCache](../tech_specs/cynork_tui.md#spec-cynai-client-cynorkchat-localcache)
  <a id="req-client-0188"></a>
- **REQ-CLIENT-0189:** When the user invokes a shell command through chat shell escape syntax, the CLI MUST be interactive-subprocess safe.
  Full-screen or TTY-owning subprocesses MUST receive the real terminal, and the TUI MUST restore itself cleanly after the subprocess exits.
  [CYNAI.CLIENT.CliChatShellEscape](../tech_specs/cli_management_app_commands_chat.md#spec-cynai-client-clichatshellescape)
  [CYNAI.CLIENT.CynorkChat.TUILayout](../tech_specs/cynork_tui.md#spec-cynai-client-cynorkchat-tuilayout)
  <a id="req-client-0189"></a>
- **REQ-CLIENT-0190:** When the interactive cynork chat surface starts without a usable login token, or loses authorization during a session, the interactive client MUST offer an in-session login and recovery path instead of forcing the user to restart outside the UI.
  Login prompts MUST protect secret input and MUST resume startup or offer to retry the interrupted session flow when authentication succeeds.
  [CYNAI.CLIENT.CynorkChat.AuthRecovery](../tech_specs/cynork_tui.md#spec-cynai-client-cynorkchat-authrecovery)
  <a id="req-client-0190"></a>
- **REQ-CLIENT-0191:** The CLI SHOULD support a web-based login flow suitable for SSO-capable deployments in addition to username/password login.
  This flow MUST avoid printing or persisting secrets in shell history or logs, MUST support a bounded authorization lifetime or expiry, and MUST integrate with the existing cynork token-storage model.
  [CYNAI.CLIENT.CliWebLogin](../tech_specs/cynork_tui.md#spec-cynai-client-cliweblogin)
  <a id="req-client-0191"></a>
- **REQ-CLIENT-0192:** Clients with a chat UI MUST NOT render model reasoning or thinking blocks as normal assistant transcript content.
  While a response is in progress, the UI MAY show ephemeral progress text such as `Thinking`, but that status MUST NOT be persisted as normal transcript text.
  [CYNAI.CLIENT.CynorkTui.TranscriptRendering](../tech_specs/cynork_tui.md#spec-cynai-client-cynorktui-transcriptrendering)
  [CYNAI.CLIENT.CynorkChat.TUILayout](../tech_specs/cynork_tui.md#spec-cynai-client-cynorkchat-tuilayout)
  <a id="req-client-0192"></a>
- **REQ-CLIENT-0193:** Clients with a rich chat UI SHOULD render tool calls and tool results as structured transcript items distinct from assistant prose.
  Tool argument and result previews SHOULD be redacted, truncated, and collapsible when verbose.
  [CYNAI.CLIENT.CynorkTui.TranscriptRendering](../tech_specs/cynork_tui.md#spec-cynai-client-cynorktui-transcriptrendering)
  [CYNAI.CLIENT.CynorkChat.TUILayout](../tech_specs/cynork_tui.md#spec-cynai-client-cynorkchat-tuilayout)
  <a id="req-client-0193"></a>
- **REQ-CLIENT-0194:** Chat UIs SHOULD support explicit authenticated download actions for assistant-provided files when the gateway exposes them.
  The UI MUST present file metadata clearly and MUST NOT auto-download assistant files without explicit user action.
  [CYNAI.USRGWY.ChatThreadsMessages.DownloadRefs](../tech_specs/chat_threads_and_messages.md#spec-cynai-usrgwy-chatthreadsmessages-downloadrefs)
  [CYNAI.CLIENT.CynorkChat.TUILayout](../tech_specs/cynork_tui.md#spec-cynai-client-cynorkchat-tuilayout)
  <a id="req-client-0194"></a>
- **REQ-CLIENT-0195:** Rich chat UIs SHOULD provide a user-level toggle to show or hide available thinking blocks.
  Thinking MUST be hidden by default; when hidden, the UI SHOULD render a compact secondary-styled collapsed block that indicates the assistant is thinking and SHOULD hint how to expand it (for example `/show-thinking` in cynork).
  [CYNAI.CLIENT.CynorkTui.ThinkingVisibilityBehavior](../tech_specs/cynork_tui_slash_commands.md#spec-cynai-client-cynorktui-thinkingvisibilitybehavior)
  [CYNAI.CLIENT.CynorkTui.TranscriptRendering](../tech_specs/cynork_tui.md#spec-cynai-client-cynorktui-transcriptrendering)
  [CYNAI.CLIENT.CynorkChat.TUILayout](../tech_specs/cynork_tui.md#spec-cynai-client-cynorkchat-tuilayout)
  <a id="req-client-0195"></a>
- **REQ-CLIENT-0196:** The cynork chat TUI SHOULD support queueing one or more drafted messages for later send.
  Queued drafts MUST remain clearly distinct from sent messages, MUST remain editable or removable before they are sent, and SHOULD support reorder plus explicit send-one or send-all actions.
  [CYNAI.CLIENT.CynorkChat.TUILayout](../tech_specs/cynork_tui.md#spec-cynai-client-cynorkchat-tuilayout)
  [CYNAI.CLIENT.CynorkChat.LocalConfig](../tech_specs/cynork_tui.md#spec-cynai-client-cynorkchat-localconfig)
  <a id="req-client-0196"></a>
- **REQ-CLIENT-0197:** The CLI SHOULD expose the full-screen TUI explicitly as `cynork tui`.
  After that surface is feature-complete for the intended rollout, interactive `cynork chat` SHOULD invoke that same TUI as an alias rather than maintaining a separate interactive implementation.
  After the same rollout milestone, invoking bare `cynork` with no subcommand SHOULD launch the same TUI by default while keeping explicit command paths available during migration.
  [CYNAI.CLIENT.CynorkTui.EntryPoint](../tech_specs/cynork_tui.md#spec-cynai-client-cynorktui-entrypoint)
  <a id="req-client-0197"></a>
- **REQ-CLIENT-0198:** Chat UIs MAY support an `@` shorthand in the composer for referencing local files.
  When such references are used, the client MUST resolve each reference at send time, upload or include the file per the gateway contract, and surface a clear validation error if a referenced file cannot be read or accepted.
  [CYNAI.CLIENT.CynorkChat.AtFileReferences](../tech_specs/cynork_tui.md#spec-cynai-client-cynorkchat-atfilereferences)
  [CYNAI.USRGWY.OpenAIChatApi.TextInput](../tech_specs/openai_compatible_chat_api.md#spec-cynai-usrgwy-openaichatapi-textinput)
  <a id="req-client-0198"></a>
- **REQ-CLIENT-0199:** Clients that provide a chat UI MUST expose a way for the user to view chat history for the current user and effective project scope.
  The history list MUST show a display title or fallback label and SHOULD show recent activity time.
  [CYNAI.USRGWY.ChatThreadsMessages.HistoryList](../tech_specs/chat_threads_and_messages.md#spec-cynai-usrgwy-chatthreadsmessages-historylist)
  <a id="req-client-0199"></a>
- **REQ-CLIENT-0200:** Clients that provide a chat UI MUST allow the user to rename the current thread and SHOULD allow rename from the thread list when such a list is present.
  [CYNAI.USRGWY.ChatThreadsMessages.ThreadTitle](../tech_specs/chat_threads_and_messages.md#spec-cynai-usrgwy-chatthreadsmessages-threadtitle)
  <a id="req-client-0200"></a>
- **REQ-CLIENT-0201:** When the gateway provides a thread summary, clients SHOULD display that summary in the thread list or sidebar so users can identify conversations without opening them.
  [CYNAI.USRGWY.ChatThreadsMessages.ThreadSummary](../tech_specs/chat_threads_and_messages.md#spec-cynai-usrgwy-chatthreadsmessages-threadsummary)
  <a id="req-client-0201"></a>
- **REQ-CLIENT-0202:** The cynork CLI MUST provide a single primary interactive chat UI surface centered on the full-screen TUI.
  `cynork shell` is deprecated as the primary interactive experience in favor of the TUI exposed through `cynork tui`.
  Interactive `cynork chat` MUST converge on that same TUI surface and, once the TUI rollout is complete, MUST behave as an alias to it rather than a distinct interactive UI path.
  User-visible interactive behavior, slash commands, transcript rendering, and session-state semantics MUST NOT diverge between `cynork tui` and interactive `cynork chat`.
  [CYNAI.CLIENT.CynorkTui.EntryPoint](../tech_specs/cynork_tui.md#spec-cynai-client-cynorktui-entrypoint)
  [CYNAI.CLIENT.CynorkChat.TUILayout](../tech_specs/cynork_tui.md#spec-cynai-client-cynorkchat-tuilayout)
  <a id="req-client-0202"></a>
- **REQ-CLIENT-0203:** The interactive cynork chat surface SHOULD provide a cursor-agent-like experience with a multi-line composer, scrollback, search and copy behavior, a persistent status bar, an optional context pane, message-history recall, structured-turn rendering, and completion for relevant chat actions.
  [CYNAI.CLIENT.CynorkChat.TUILayout](../tech_specs/cynork_tui.md#spec-cynai-client-cynorkchat-tuilayout)
  [CYNAI.CLIENT.CynorkChat.Completion](../tech_specs/cynork_tui.md#spec-cynai-client-cynorkchat-completion)
  <a id="req-client-0203"></a>
- **REQ-CLIENT-0204:** The interactive cynork chat surface MUST support newline insertion, cancellation, clean exit, and loading older history while scrolling back.
  The focused composer MUST show a visible text cursor or caret at the current insertion point.
  [CYNAI.CLIENT.CynorkChat.TUILayout](../tech_specs/cynork_tui.md#spec-cynai-client-cynorkchat-tuilayout)
  <a id="req-client-0204"></a>
- **REQ-CLIENT-0205:** Mouse-wheel scrolling in the interactive cynork chat surface MUST navigate transcript or output history in the scrollback and MUST NOT cycle composer history or mutate previously submitted messages.
  [CYNAI.CLIENT.CynorkChat.TUILayout](../tech_specs/cynork_tui.md#spec-cynai-client-cynorkchat-tuilayout)
  <a id="req-client-0205"></a>
- **REQ-CLIENT-0206:** The interactive cynork chat surface MUST hint the availability of slash commands, `@` file lookup or attachment, and `!` shell shorthand in or adjacent to the composer.
  [CYNAI.CLIENT.CynorkChat.TUILayout](../tech_specs/cynork_tui.md#spec-cynai-client-cynorkchat-tuilayout)
  <a id="req-client-0206"></a>
- **REQ-CLIENT-0207:** Slash commands in interactive chat MUST provide parity with the previously available shell command surface for tasks, status, identity, nodes, preferences, skills, model, project, and thread controls.
  The `! command` shell-escape shorthand MUST be supported and documented as part of the chat interaction model.
  [CYNAI.CLIENT.CliChatSlashCommandReference](../tech_specs/cli_management_app_commands_chat.md#spec-cynai-client-clichatslashcommandreference)
  [CYNAI.CLIENT.CynorkTuiSlashCommands](../tech_specs/cynork_tui_slash_commands.md#spec-cynai-client-cynorktuislashcommands)
  [CYNAI.CLIENT.CliChatShellEscape](../tech_specs/cli_management_app_commands_chat.md#spec-cynai-client-clichatshellescape)
  [CYNAI.CLIENT.CynorkChat.TUILayout](../tech_specs/cynork_tui.md#spec-cynai-client-cynorkchat-tuilayout)
  <a id="req-client-0207"></a>
- **REQ-CLIENT-0208:** The cynork interactive chat surface MUST support `/show-thinking` and `/hide-thinking` slash commands.
  `/show-thinking` MUST reveal retained thinking blocks for the current session, including already loaded transcript rows and older assistant turns loaded later through scrollback history.
  `/hide-thinking` MUST return retained thinking blocks to the hidden-by-default collapsed presentation without changing canonical visible assistant text.
  [CYNAI.CLIENT.CynorkTui.ThinkingVisibilityBehavior](../tech_specs/cynork_tui_slash_commands.md#spec-cynai-client-cynorktui-thinkingvisibilitybehavior)
  [CYNAI.CLIENT.CynorkTui.LocalSlashCommands](../tech_specs/cynork_tui_slash_commands.md#spec-cynai-client-cynorktui-localslashcommands)
  [CYNAI.CLIENT.CynorkTui.TranscriptRendering](../tech_specs/cynork_tui.md#spec-cynai-client-cynorktui-transcriptrendering)
  <a id="req-client-0208"></a>
- **REQ-CLIENT-0209:** The cynork interactive chat surface MUST request streaming output by default for normal interactive turns and MUST progressively update the single in-flight assistant turn as visible text or structured progress arrives.
  If streaming is temporarily unavailable for a selected backend path, the client MUST fall back to a degraded in-flight indicator and final-turn replacement without duplicating transcript content.
  [CYNAI.CLIENT.CynorkTui.GenerationState](../tech_specs/cynork_tui.md#spec-cynai-client-cynorktui-generationstate)
  [CYNAI.USRGWY.OpenAIChatApi.Streaming](../tech_specs/openai_compatible_chat_api.md#spec-cynai-usrgwy-openaichatapi-streaming)
  <a id="req-client-0209"></a>
- **REQ-CLIENT-0210:** Clients that provide chat-thread switching controls MUST expose a user-typeable thread selector for each visible thread and MUST allow switching by that selector rather than requiring the user to type a raw backend UUID.
  The selector MAY be a stable short handle, a list ordinal within the current thread list view, an unambiguous displayed title form, or another compact human-typable token, but it MUST be shown to the user wherever thread switching is offered.
  [CYNAI.CLIENT.CynorkChat.TUILayout](../tech_specs/cynork_tui.md#spec-cynai-client-cynorkchat-tuilayout)
  [CYNAI.CLIENT.CynorkTui.ThreadSlashCommands](../tech_specs/cynork_tui_slash_commands.md#spec-cynai-client-cynorktui-threadslashcommands)
  <a id="req-client-0210"></a>
- **REQ-CLIENT-0211:** In the cynork interactive chat surface, `/show-thinking` and `/hide-thinking` MUST update a persisted local TUI preference in the cynork YAML config file and that preference MUST be loaded on future executions of cynork.
  The persisted preference MUST control the default thinking visibility for newly started TUI or interactive chat sessions, while still allowing the user to change it again with the same slash commands.
  [CYNAI.CLIENT.CynorkTui.ThinkingVisibilityBehavior](../tech_specs/cynork_tui_slash_commands.md#spec-cynai-client-cynorktui-thinkingvisibilitybehavior)
  [CYNAI.CLIENT.CynorkChat.LocalConfig](../tech_specs/cynork_tui.md#spec-cynai-client-cynorkchat-localconfig)
  [CYNAI.CLIENT.CynorkTui.LocalSlashCommands](../tech_specs/cynork_tui_slash_commands.md#spec-cynai-client-cynorktui-localslashcommands)
  <a id="req-client-0211"></a>
- **REQ-CLIENT-0212:** The TUI MUST display the current thread title (or a fallback label) and MUST update the displayed thread title whenever it changes (e.g. after auto-title or `/thread rename`) and when the user switches to a different thread.
  [CYNAI.CLIENT.CynorkTui](../tech_specs/cynork_tui.md#spec-cynai-client-cynorktui)
  <a id="req-client-0212"></a>
