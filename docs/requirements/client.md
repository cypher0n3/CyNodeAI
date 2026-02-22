# CLIENT Requirements

- [1 Overview](#1-overview)
- [2 Requirements](#2-requirements)

## 1 Overview

This document consolidates requirements for the `CLIENT` domain.
It covers user-facing management surfaces (CLI and shared client behavior) and user preference behavior.
Web Console-specific requirements live in [webcon.md](webcon.md) (REQ-WEBCON-*).

## 2 Requirements

- **REQ-CLIENT-0001:** CLI: no direct DB; gateway for all ops; no secrets in output or on disk; token auth; least privilege.
  [CYNAI.CLIENT.Doc.CliManagementApp](../tech_specs/cli_management_app.md#spec-cynai-client-doc-climanagementapp)
  [CYNAI.CLIENT.CliSecurityModel](../tech_specs/cli_management_app.md#spec-cynai-client-clisecurity)
  [CYNAI.CLIENT.CliAuthConfig](../tech_specs/cli_management_app.md#spec-cynai-client-cliauth)
  <a id="req-client-0001"></a>
- **REQ-CLIENT-0003:** Effective preferences by scope precedence; unknown keys passed through; cache per task revision with invalidation on update.
  Effective preference computation MUST be deterministic and MUST skip invalid preference entries (for example mismatched `value_type`) rather than letting invalid entries override lower-precedence valid entries.
  [CYNAI.STANDS.UserPreferencesRetrieval](../tech_specs/user_preferences.md#spec-cynai-stands-prefretrieval)
  <a id="req-client-0003"></a>
- **REQ-CLIENT-0004:** The Web Console and the CLI management app MUST provide capability parity for administrative operations.
  When adding or changing a capability in one client (for example a new credential type, preference scope, node action, or skill operation), the other client MUST be updated to match.
  [CYNAI.CLIENT.WebConsoleCapabilityParity](../tech_specs/web_console.md#spec-cynai-client-webconcapabilityparity)
  [CYNAI.CLIENT.CliCapabilityParity](../tech_specs/cli_management_app.md#spec-cynai-client-clicapabilityparity)
  <a id="req-client-0004"></a>
- **REQ-CLIENT-0100:** The CLI MUST NOT connect directly to PostgreSQL.
  [CYNAI.CLIENT.CliSecurityModel](../tech_specs/cli_management_app.md#spec-cynai-client-clisecurity)
  <a id="req-client-0100"></a>
- **REQ-CLIENT-0101:** The CLI MUST call the User API Gateway for all operations.
  [CYNAI.CLIENT.CliSecurityModel](../tech_specs/cli_management_app.md#spec-cynai-client-clisecurity)
  <a id="req-client-0101"></a>
- **REQ-CLIENT-0102:** The CLI MUST avoid printing secrets.
  [CYNAI.CLIENT.CliSecurityModel](../tech_specs/cli_management_app.md#spec-cynai-client-clisecurity)
  <a id="req-client-0102"></a>
- **REQ-CLIENT-0103:** The CLI MUST not persist plaintext secrets to disk.
  [CYNAI.CLIENT.CliSecurityModel](../tech_specs/cli_management_app.md#spec-cynai-client-clisecurity)
  [CYNAI.CLIENT.CliConfigFileLocation](../tech_specs/cli_management_app.md#spec-cynai-client-cliconfigfilelocation)
  [CYNAI.CLIENT.CliCredentialHelperProtocol](../tech_specs/cli_management_app.md#spec-cynai-client-clicredentialhelperprotocol)
  <a id="req-client-0103"></a>
- **REQ-CLIENT-0104:** The CLI MUST support least privilege and MUST fail closed on authorization errors.
  [CYNAI.CLIENT.CliSecurityModel](../tech_specs/cli_management_app.md#spec-cynai-client-clisecurity)
  <a id="req-client-0104"></a>
- **REQ-CLIENT-0105:** The CLI MUST support token-based authentication.
  [CYNAI.CLIENT.CliAuthConfig](../tech_specs/cli_management_app.md#spec-cynai-client-cliauth)
  [CYNAI.CLIENT.CliTokenResolution](../tech_specs/cli_management_app.md#spec-cynai-client-clitokenresolution)
  <a id="req-client-0105"></a>
- **REQ-CLIENT-0106:** The CLI SHOULD support reading tokens from env vars for CI usage.
  [CYNAI.CLIENT.CliAuthConfig](../tech_specs/cli_management_app.md#spec-cynai-client-cliauth)
  [CYNAI.CLIENT.CliTokenResolution](../tech_specs/cli_management_app.md#spec-cynai-client-clitokenresolution)
  <a id="req-client-0106"></a>
- **REQ-CLIENT-0107:** The CLI SHOULD support optional mTLS or pinned CA bundles for enterprise deployments.
  [CYNAI.CLIENT.CliAuthConfig](../tech_specs/cli_management_app.md#spec-cynai-client-cliauth)
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
  [CYNAI.CLIENT.CliCredentialManagement](../tech_specs/cli_management_app.md#spec-cynai-client-clicredential)
  <a id="req-client-0116"></a>
- **REQ-CLIENT-0117:** Credential read MUST return metadata only.
  [CYNAI.WEBCON.Credential](../tech_specs/web_console.md#spec-cynai-webcon-credential)
  [CYNAI.CLIENT.CliCredentialManagement](../tech_specs/cli_management_app.md#spec-cynai-client-clicredential)
  <a id="req-client-0117"></a>
- **REQ-CLIENT-0118:** Credential list MUST support filtering by provider and scope.
  [CYNAI.WEBCON.Credential](../tech_specs/web_console.md#spec-cynai-webcon-credential)
  [CYNAI.CLIENT.CliCredentialManagement](../tech_specs/cli_management_app.md#spec-cynai-client-clicredential)
  <a id="req-client-0118"></a>
- **REQ-CLIENT-0119:** Credential rotate MUST create a new encrypted secret value and invalidate the old one.
  [CYNAI.WEBCON.Credential](../tech_specs/web_console.md#spec-cynai-webcon-credential)
  [CYNAI.CLIENT.CliCredentialManagement](../tech_specs/cli_management_app.md#spec-cynai-client-clicredential)
  <a id="req-client-0119"></a>
- **REQ-CLIENT-0120:** Credential disable MUST support immediate deactivation.
  [CYNAI.WEBCON.Credential](../tech_specs/web_console.md#spec-cynai-webcon-credential)
  [CYNAI.CLIENT.CliCredentialManagement](../tech_specs/cli_management_app.md#spec-cynai-client-clicredential)
  <a id="req-client-0120"></a>
- **REQ-CLIENT-0121:** Preference edits MUST be scoped and versioned.
  Updates SHOULD use optimistic concurrency via expected-version checks to prevent lost updates.
  When an expected version is provided and does not match the current stored version, the update MUST fail with a conflict error.
  [CYNAI.WEBCON.Preferences](../tech_specs/web_console.md#spec-cynai-webcon-preferences)
  [CYNAI.CLIENT.CliPreferencesManagement](../tech_specs/cli_management_app.md#spec-cynai-client-clipreferences)
  <a id="req-client-0121"></a>
- **REQ-CLIENT-0122:** The UI MUST support preference scope selection (system, user, group, project, task).
  For the web console, this MUST include an easy method for users to create, read, update, and delete their personal (user), group, and project preferences.
  [CYNAI.WEBCON.Preferences](../tech_specs/web_console.md#spec-cynai-webcon-preferences)
  [CYNAI.CLIENT.CliPreferencesManagement](../tech_specs/cli_management_app.md#spec-cynai-client-clipreferences)
  <a id="req-client-0122"></a>
- **REQ-CLIENT-0123:** The UI SHOULD provide an \"effective preferences\" preview for a given task or project.
  [CYNAI.WEBCON.Preferences](../tech_specs/web_console.md#spec-cynai-webcon-preferences)
  [CYNAI.CLIENT.CliPreferencesManagement](../tech_specs/cli_management_app.md#spec-cynai-client-clipreferences)
  <a id="req-client-0123"></a>
- **REQ-CLIENT-0124:** The UI SHOULD provide validation for known keys and types.
  The UI MUST allow user-defined keys to be stored even when it cannot validate their semantics, as long as the key and JSON value are valid.
  [CYNAI.WEBCON.Preferences](../tech_specs/web_console.md#spec-cynai-webcon-preferences)
  [CYNAI.CLIENT.CliPreferencesManagement](../tech_specs/cli_management_app.md#spec-cynai-client-clipreferences)
  <a id="req-client-0124"></a>
- **REQ-CLIENT-0160:** The Web Console and the CLI MUST allow configuring Project Manager model selection for local inference via system settings.
  At minimum, clients MUST allow editing `agents.project_manager.model.local_default_ollama_model` through the standard system settings management surface (no bespoke API required).
  Clients SHOULD also surface the automatic selection policy keys for discoverability (for example `agents.project_manager.model.selection.execution_mode`, `agents.project_manager.model.selection.mode`, and `agents.project_manager.model.selection.prefer_orchestrator_host`).
  System settings are distinct from user preferences; see [User preferences (Terminology)](../tech_specs/user_preferences.md#2-terminology).
  [CYNAI.WEBCON.SystemSettings](../tech_specs/web_console.md#spec-cynai-webcon-systemsettings)
  [CYNAI.CLIENT.CliSystemSettingsManagement](../tech_specs/cli_management_app.md#spec-cynai-client-clisystemsettings)
  <a id="req-client-0160"></a>
- **REQ-CLIENT-0161:** The CLI MUST provide a chat command that starts an interactive chat session with the Project Manager (PM) model.
  The user MUST be able to send messages and receive responses in turn until they exit the session.
  The chat session MUST use the same gateway and authentication as other CLI commands and MUST NOT expose secrets in history or output.
  [CYNAI.CLIENT.CliChat](../tech_specs/cli_management_app.md#spec-cynai-client-clichat)
  [CYNAI.USRGWY.OpenAIChatApi](../tech_specs/openai_compatible_chat_api.md#spec-cynai-usrgwy-openaichatapi)
  <a id="req-client-0161"></a>
- **REQ-CLIENT-0162:** The CLI chat command MUST render model responses with pretty-formatted output when the response contains Markdown.
  The CLI MUST interpret common Markdown (headings, lists, code blocks, emphasis, links) and display them in a human-readable way in the terminal (e.g. indentation, styling, or syntax highlighting for code blocks).
  The CLI MUST honor `--no-color` for chat output and SHOULD support a plain-text mode (e.g. `--plain`) that prints the raw response without Markdown rendering for scripting or piping.
  [CYNAI.CLIENT.CliChat](../tech_specs/cli_management_app.md#spec-cynai-client-clichat)
  <a id="req-client-0162"></a>
- **REQ-CLIENT-0163:** The CLI MUST display all JSON as pretty-printed (indented, with newlines) whenever JSON is part of the output.
  This applies to `--output json`, JSON embedded in chat or other responses, and any other CLI output that contains JSON.
  [CYNAI.CLIENT.CliPrettyPrintJson](../tech_specs/cli_management_app.md#spec-cynai-client-cliprettyprintjson)
  <a id="req-client-0163"></a>
- **REQ-CLIENT-0164:** The CLI chat command MUST display the available slash commands (e.g. `/exit`, `/quit`, `/help`) to the user.
  Display MUST occur at session start or in response to a dedicated help command (e.g. `/help`) so users can discover slash commands.
  [CYNAI.CLIENT.CliChatSlashCommands](../tech_specs/cli_management_app_commands_chat.md#spec-cynai-client-clichatslashcommands)
  <a id="req-client-0164"></a>
- **REQ-CLIENT-0165:** The CLI chat command MUST support slash-command autocomplete and inline suggestions when the user types `/`.
  When the input line starts with `/`, the CLI MUST show the list of available slash commands with short descriptions (e.g. command on the left, description on the right) and MUST support Tab to complete or cycle and arrow-up/arrow-down to move through suggestions, with the current selection visually indicated.
  [CYNAI.CLIENT.CliChatSlashAutocomplete](../tech_specs/cli_management_app_commands_chat.md#spec-cynai-client-clichatslashautocomplete)
  <a id="req-client-0165"></a>
- **REQ-CLIENT-0166:** The CLI chat command MUST support task operations via slash commands: list, get, create (with inline prompt), cancel, result, logs, and artifacts list.
  These MUST use the same User API Gateway endpoints as `cynork task`; output MUST be shown inline in chat (pretty-printed when JSON).
  [CYNAI.CLIENT.CliChatSlashTask](../tech_specs/cli_management_app_commands_chat.md#spec-cynai-client-clichatslashtask)
  <a id="req-client-0166"></a>
- **REQ-CLIENT-0167:** The CLI chat command MUST support `/status` and `/whoami` slash commands to show gateway reachability and current identity without leaving chat.
  [CYNAI.CLIENT.CliChatSlashStatus](../tech_specs/cli_management_app_commands_chat.md#spec-cynai-client-clichatslashstatus)
  <a id="req-client-0167"></a>
- **REQ-CLIENT-0168:** The CLI chat command MUST support node listing and get via slash commands: `/nodes list`, `/nodes get <node_id>`.
  [CYNAI.CLIENT.CliChatSlashNodes](../tech_specs/cli_management_app_commands_chat.md#spec-cynai-client-clichatslashnodes)
  <a id="req-client-0168"></a>
- **REQ-CLIENT-0169:** The CLI chat command MUST support preferences via slash commands: list, get, set, delete, and effective (e.g. `/prefs list`, `/prefs get [key]`, `/prefs set ...`, `/prefs delete ...`, `/prefs effective [--task-id <id>]`).
  [CYNAI.CLIENT.CliChatSlashPrefs](../tech_specs/cli_management_app_commands_chat.md#spec-cynai-client-clichatslashprefs)
  <a id="req-client-0169"></a>
- **REQ-CLIENT-0170:** The CLI chat command MUST support skills list and get via slash commands: `/skills list`, `/skills get <skill_id>`.
  [CYNAI.CLIENT.CliChatSlashSkills](../tech_specs/cli_management_app_commands_chat.md#spec-cynai-client-clichatslashskills)
  <a id="req-client-0170"></a>
- **REQ-CLIENT-0171:** The CLI chat command MUST support selecting an OpenAI model identifier for chat completions.
  The CLI MUST support selecting the model at session start (e.g. `cynork chat --model <id>`) and within the session (e.g. `/model <id>`).
  Model selection MUST only affect `POST /v1/chat/completions` requests and MUST NOT change system settings or user preferences.
  [CYNAI.CLIENT.CliChatModelSelection](../tech_specs/cli_management_app_commands_chat.md#spec-cynai-client-clichatmodelselection)
  [CYNAI.USRGWY.OpenAIChatApi.Endpoints](../tech_specs/openai_compatible_chat_api.md#spec-cynai-usrgwy-openaichatapi-endpoints)
  <a id="req-client-0171"></a>
- **REQ-CLIENT-0172:** The CLI chat command SHOULD support listing available OpenAI model identifiers from the gateway (e.g. `/models`).
  This MUST call `GET /v1/models` and display model ids.
  [CYNAI.CLIENT.CliChatModelSelection](../tech_specs/cli_management_app_commands_chat.md#spec-cynai-client-clichatmodelselection)
  [CYNAI.USRGWY.OpenAIChatApi.Endpoints](../tech_specs/openai_compatible_chat_api.md#spec-cynai-usrgwy-openaichatapi-endpoints)
  <a id="req-client-0172"></a>
- **REQ-CLIENT-0173:** The CLI chat command MUST support setting an optional project context for the chat session.
  When set, the CLI MUST send the project context using the OpenAI-standard `OpenAI-Project` request header on `POST /v1/chat/completions`.
  When omitted, the CLI does not send the header and the gateway associates the thread with the user's default project (see [REQ-PROJCT-0104](../requirements/projct.md#req-projct-0104)).
  [CYNAI.CLIENT.CliChatProjectContext](../tech_specs/cli_management_app_commands_chat.md#spec-cynai-client-clichatprojectcontext)
  [CYNAI.ACCESS.Doc.ProjectsAndScopes](../tech_specs/projects_and_scopes.md#spec-cynai-access-doc-projectsandscopes)
  <a id="req-client-0173"></a>
- **REQ-CLIENT-0174:** The CLI and the Web Console MUST support basic project management (CRUD: create, list, get, update, delete or disable) via the User API Gateway.
  Projects have a user-friendly title and optional text description (see [REQ-PROJCT-0103](../requirements/projct.md#req-projct-0103)); clients MUST allow setting and editing these in create and update flows.
  Both clients MUST offer the same project CRUD capabilities (capability parity).
  [CYNAI.CLIENT.CliProjectManagement](../tech_specs/cli_management_app_commands_admin.md#project-management)
  [CYNAI.WEBCON.ProjectManagement](../tech_specs/web_console.md#spec-cynai-webcon-projectmanagement)
  [CYNAI.ACCESS.Doc.ProjectsAndScopes](../tech_specs/projects_and_scopes.md#spec-cynai-access-doc-projectsandscopes)
  <a id="req-client-0174"></a>
- **REQ-CLIENT-0125:** Node management MUST be mediated by the User API Gateway.
  [CYNAI.WEBCON.NodeManagement](../tech_specs/web_console.md#spec-cynai-webcon-nodemanagement)
  [CYNAI.CLIENT.CliNodeManagement](../tech_specs/cli_management_app.md#spec-cynai-client-clinodemgmt)
  <a id="req-client-0125"></a>
- **REQ-CLIENT-0126:** The UI MUST NOT connect directly to node worker APIs.
  [CYNAI.WEBCON.NodeManagement](../tech_specs/web_console.md#spec-cynai-webcon-nodemanagement)
  [CYNAI.CLIENT.CliNodeManagement](../tech_specs/cli_management_app.md#spec-cynai-client-clinodemgmt)
  <a id="req-client-0126"></a>
- **REQ-CLIENT-0127:** The UI MUST clearly distinguish between node-reported state and orchestrator-derived state.
  [CYNAI.WEBCON.NodeManagement](../tech_specs/web_console.md#spec-cynai-webcon-nodemanagement)
  <a id="req-client-0127"></a>
- **REQ-CLIENT-0128:** Potentially disruptive actions MUST be gated by admin authorization and SHOULD require confirmation.
  [CYNAI.WEBCON.NodeManagement](../tech_specs/web_console.md#spec-cynai-webcon-nodemanagement)
  [CYNAI.CLIENT.CliNodeManagement](../tech_specs/cli_management_app.md#spec-cynai-client-clinodemgmt)
  <a id="req-client-0128"></a>
- **REQ-CLIENT-0136:** The interactive CLI mode MUST provide access to the same commands and flags as the non-interactive CLI.
  [CYNAI.CLIENT.CliInteractiveMode](../tech_specs/cli_management_app.md#spec-cynai-client-cliinteractivemode)
  <a id="req-client-0136"></a>
- **REQ-CLIENT-0137:** The interactive CLI mode MUST support tab completion for commands/subcommands, flags, and known enumerated flag values.
  [CYNAI.CLIENT.CliInteractiveMode](../tech_specs/cli_management_app.md#spec-cynai-client-cliinteractivemode)
  <a id="req-client-0137"></a>
- **REQ-CLIENT-0138:** The interactive CLI mode MUST support in-session help.
  [CYNAI.CLIENT.CliInteractiveMode](../tech_specs/cli_management_app.md#spec-cynai-client-cliinteractivemode)
  <a id="req-client-0138"></a>
- **REQ-CLIENT-0139:** The interactive CLI mode MUST support `exit` and `quit`.
  [CYNAI.CLIENT.CliInteractiveMode](../tech_specs/cli_management_app.md#spec-cynai-client-cliinteractivemode)
  <a id="req-client-0139"></a>
- **REQ-CLIENT-0140:** The interactive CLI mode MUST NOT store secrets in history, and secret prompts MUST bypass history recording.
  [CYNAI.CLIENT.CliInteractiveMode](../tech_specs/cli_management_app.md#spec-cynai-client-cliinteractivemode)
  <a id="req-client-0140"></a>
- **REQ-CLIENT-0141:** If a persistent history file is implemented, it MUST be stored under the CLI config directory with permissions `0600`.
  [CYNAI.CLIENT.CliInteractiveMode](../tech_specs/cli_management_app.md#spec-cynai-client-cliinteractivemode)
  <a id="req-client-0141"></a>
- **REQ-CLIENT-0142:** Tab completion MUST NOT fetch or reveal secret values.
  [CYNAI.CLIENT.CliInteractiveMode](../tech_specs/cli_management_app.md#spec-cynai-client-cliinteractivemode)
  <a id="req-client-0142"></a>
- **REQ-CLIENT-0143:** The CLI MUST support JSON output mode.
  [CYNAI.CLIENT.CliOutputScripting](../tech_specs/cli_management_app.md#spec-cynai-client-clioutputscripting)
  <a id="req-client-0143"></a>
- **REQ-CLIENT-0144:** The CLI SHOULD support table output mode for humans.
  [CYNAI.CLIENT.CliOutputScripting](../tech_specs/cli_management_app.md#spec-cynai-client-clioutputscripting)
  <a id="req-client-0144"></a>
- **REQ-CLIENT-0145:** The CLI SHOULD return non-zero exit codes on failures and policy denials.
  [CYNAI.CLIENT.CliOutputScripting](../tech_specs/cli_management_app.md#spec-cynai-client-clioutputscripting)
  <a id="req-client-0145"></a>
- **REQ-CLIENT-0146:** The CLI MUST support full CRUD for skills (create/load, list, get, update, delete) via the User API Gateway, with the same controls as defined in the skills spec (scope visibility, scope elevation permission, auditing on write).
  [CYNAI.CLIENT.CliSkillsManagement](../tech_specs/cli_management_app.md#spec-cynai-client-cliskillsmanagement)
  [CYNAI.SKILLS.SkillManagementCrud](../tech_specs/skills_storage_and_inference.md#spec-cynai-skills-skillmanagementcrud)
  <a id="req-client-0146"></a>
- **REQ-CLIENT-0149:** The CLI MUST support a local key (gateway token) stored in the user config dir (e.g. `~/.config/cynork/config.yaml`) and SHOULD support reading the token from a password store or credential helper (kubectl-style) so the token need not be stored in plaintext.
  [CYNAI.CLIENT.CliAuthConfig](../tech_specs/cli_management_app.md#spec-cynai-client-cliauth)
  [CYNAI.CLIENT.CliConfigFileLocation](../tech_specs/cli_management_app.md#spec-cynai-client-cliconfigfilelocation)
  [CYNAI.CLIENT.CliTokenResolution](../tech_specs/cli_management_app.md#spec-cynai-client-clitokenresolution)
  [CYNAI.CLIENT.CliCredentialHelperProtocol](../tech_specs/cli_management_app.md#spec-cynai-client-clicredentialhelperprotocol)
  <a id="req-client-0149"></a>
- **REQ-CLIENT-0150:** The CLI MUST store session credentials (e.g. token after login) in a reliable way so that multiple consecutive CLI invocations can reuse the token without re-authenticating.
  The config file path MUST be resolved consistently (e.g. honoring XDG_CONFIG_HOME); writes MUST be atomic (e.g. temp file then rename) so a crash does not leave a partial file; and if the default config path cannot be resolved (e.g. no home dir), login and logout MUST fail with a clear error.
  [CYNAI.CLIENT.CliConfigFileLocation](../tech_specs/cli_management_app.md#spec-cynai-client-cliconfigfilelocation)
  [CYNAI.CLIENT.CliSessionPersistence](../tech_specs/cli_management_app.md#spec-cynai-client-clisessionpersistence)
  <a id="req-client-0150"></a>
- **REQ-CLIENT-0151:** The CLI MUST allow passing a task as inline text (e.g. `--prompt` or `--task`) or from a file (e.g. `--task-file <path>`) containing plain text or Markdown.
  The default is interpretation; the system interprets the task and uses inference when needed (no user-facing "use inference" flag).
  The CLI MUST support optionally associating a created task with a `project_id` (for example `cynork task create --project-id <id>`).
  When `project_id` is omitted, the task MUST be associated with the authenticated user's default project (see [REQ-PROJCT-0104](../requirements/projct.md#req-projct-0104)).
  Attachments are specified in REQ-CLIENT-0157.
  [CYNAI.CLIENT.CliTaskCreatePrompt](../tech_specs/cli_management_app.md#spec-cynai-client-clitaskcreateprompt)
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
  [CYNAI.CLIENT.CliTaskCreatePrompt](../tech_specs/cli_management_app.md#spec-cynai-client-clitaskcreateprompt)
  <a id="req-client-0153"></a>
- **REQ-CLIENT-0154:** The web console MUST support running a script (e.g. script file upload or path) and a short series of commands (e.g. multi-line or list input) with the same semantics as the CLI and gateway.
  [CYNAI.CLIENT.WebConsoleCapabilityParity](../tech_specs/web_console.md#spec-cynai-client-webconcapabilityparity)
  <a id="req-client-0154"></a>
- **REQ-CLIENT-0155:** The CLI command surface for tasks MUST be fully specified and MUST be treated as a stable user-facing contract.
  The CLI MUST implement the task subcommand set, flag names, argument ordering, mutual exclusions, default values, confirmation behavior, output schemas, and exit codes exactly as defined in the CLI management app spec.
  [CYNAI.CLIENT.CliCommandSurface](../tech_specs/cli_management_app.md#spec-cynai-client-clicommandsurface)
  <a id="req-client-0155"></a>
- **REQ-CLIENT-0156:** The CLI MUST return deterministic exit codes for common failure categories (usage error, auth error, gateway error, not found, conflict, validation error).
  The CLI MUST write machine-parseable output only to stdout when `--output json` is selected, and MUST write all human messages and errors to stderr in JSON mode.
  [CYNAI.CLIENT.CliCommandSurface](../tech_specs/cli_management_app.md#spec-cynai-client-clicommandsurface)
  [CYNAI.CLIENT.CliOutputScripting](../tech_specs/cli_management_app.md#spec-cynai-client-clioutputscripting)
  <a id="req-client-0156"></a>
- **REQ-CLIENT-0157:** The CLI MUST support other attachment types by accepting one or more path strings (e.g. repeatable `--attach <path>`).
  For each provided path, the CLI MUST validate the path and MUST send the attachment payload to the gateway according to the task-create API contract.
  [CYNAI.CLIENT.CliTaskCreatePrompt](../tech_specs/cli_management_app.md#spec-cynai-client-clitaskcreateprompt)
  <a id="req-client-0157"></a>
- **REQ-CLIENT-0158:** The CLI MUST support shorthand aliases (`-<x>`) for the most commonly used flags.
  The required shorthand aliases and their long forms MUST be defined in the CLI management app spec and MUST be treated as a stable user-facing contract.
  [CYNAI.CLIENT.CliCommandSurface](../tech_specs/cli_management_app.md#spec-cynai-client-clicommandsurface)
  <a id="req-client-0158"></a>
- **REQ-CLIENT-0159:** The CLI interactive mode MUST support tab-completion of task names when a task identifier is expected (e.g. for `task get`, `task result`, `task cancel`, `task logs`, `task artifacts list`, `task artifacts get`).
  [CYNAI.CLIENT.CliInteractiveMode](../tech_specs/cli_management_app.md#spec-cynai-client-cliinteractivemode)
  <a id="req-client-0159"></a>
