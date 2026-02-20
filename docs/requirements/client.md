# CLIENT Requirements

- [1 Overview](#1-overview)
- [2 Requirements](#2-requirements)

## 1 Overview

This document consolidates requirements for the `CLIENT` domain.
It covers user-facing management surfaces and user preference behavior.

## 2 Requirements

- **REQ-CLIENT-0001:** CLI: no direct DB; gateway for all ops; no secrets in output or on disk; token auth; least privilege.
  [CYNAI.CLIENT.Doc.CliManagementApp](../tech_specs/cli_management_app.md#spec-cynai-client-doc-climanagementapp)
  [CYNAI.CLIENT.CliSecurityModel](../tech_specs/cli_management_app.md#spec-cynai-client-clisecurity)
  [CYNAI.CLIENT.CliAuthConfig](../tech_specs/cli_management_app.md#spec-cynai-client-cliauth)
  <a id="req-client-0001"></a>
- **REQ-CLIENT-0002:** Admin console: no direct DB; gateway only; secrets write-only in UI; least privilege; no embedded credentials.
  [CYNAI.CLIENT.AdminWebConsoleSecurity](../tech_specs/admin_web_console.md#spec-cynai-client-awcsecurity)
  <a id="req-client-0002"></a>
- **REQ-CLIENT-0003:** Effective preferences by scope precedence; unknown keys passed through; cache per task revision with invalidation on update.
  [CYNAI.STANDS.UserPreferencesRetrieval](../tech_specs/user_preferences.md#spec-cynai-stands-prefretrieval)
  <a id="req-client-0003"></a>
- **REQ-CLIENT-0004:** The Admin Web Console and the CLI management app MUST provide capability parity for administrative operations.
  When adding or changing a capability in one client (for example a new credential type, preference scope, node action, or skill operation), the other client MUST be updated to match.
  [CYNAI.CLIENT.AdminWebConsoleCapabilityParity](../tech_specs/admin_web_console.md#spec-cynai-client-awccapabilityparity)
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
  <a id="req-client-0103"></a>
- **REQ-CLIENT-0104:** The CLI MUST support least privilege and MUST fail closed on authorization errors.
  [CYNAI.CLIENT.CliSecurityModel](../tech_specs/cli_management_app.md#spec-cynai-client-clisecurity)
  <a id="req-client-0104"></a>
- **REQ-CLIENT-0105:** The CLI MUST support token-based authentication.
  [CYNAI.CLIENT.CliAuthConfig](../tech_specs/cli_management_app.md#spec-cynai-client-cliauth)
  <a id="req-client-0105"></a>
- **REQ-CLIENT-0106:** The CLI SHOULD support reading tokens from env vars for CI usage.
  [CYNAI.CLIENT.CliAuthConfig](../tech_specs/cli_management_app.md#spec-cynai-client-cliauth)
  <a id="req-client-0106"></a>
- **REQ-CLIENT-0107:** The CLI SHOULD support optional mTLS or pinned CA bundles for enterprise deployments.
  [CYNAI.CLIENT.CliAuthConfig](../tech_specs/cli_management_app.md#spec-cynai-client-cliauth)
  <a id="req-client-0107"></a>
- **REQ-CLIENT-0108:** The web console MUST NOT connect directly to PostgreSQL.
  [CYNAI.CLIENT.AdminWebConsoleSecurity](../tech_specs/admin_web_console.md#spec-cynai-client-awcsecurity)
  <a id="req-client-0108"></a>
- **REQ-CLIENT-0109:** The web console MUST call the User API Gateway for all operations.
  [CYNAI.CLIENT.AdminWebConsoleSecurity](../tech_specs/admin_web_console.md#spec-cynai-client-awcsecurity)
  <a id="req-client-0109"></a>
- **REQ-CLIENT-0110:** Secrets MUST be write-only in the UI.
  [CYNAI.CLIENT.AdminWebConsoleSecurity](../tech_specs/admin_web_console.md#spec-cynai-client-awcsecurity)
  <a id="req-client-0110"></a>
- **REQ-CLIENT-0111:** The UI MUST never display plaintext secret values after creation.
  [CYNAI.CLIENT.AdminWebConsoleSecurity](../tech_specs/admin_web_console.md#spec-cynai-client-awcsecurity)
  <a id="req-client-0111"></a>
- **REQ-CLIENT-0112:** The UI MUST support least privilege and MUST not expose admin features to non-admin users.
  [CYNAI.CLIENT.AdminWebConsoleSecurity](../tech_specs/admin_web_console.md#spec-cynai-client-awcsecurity)
  <a id="req-client-0112"></a>
- **REQ-CLIENT-0113:** MUST compute effective preferences for the task by merging scopes in precedence order.
  [CYNAI.STANDS.UserPreferencesRetrieval](../tech_specs/user_preferences.md#spec-cynai-stands-prefretrieval)
  <a id="req-client-0113"></a>
- **REQ-CLIENT-0114:** MUST treat unknown keys as opaque and pass them through to verification/tooling.
  [CYNAI.STANDS.UserPreferencesRetrieval](../tech_specs/user_preferences.md#spec-cynai-stands-prefretrieval)
  <a id="req-client-0114"></a>
- **REQ-CLIENT-0115:** SHOULD cache effective preferences per task revision, but MUST invalidate on preference update.
  [CYNAI.STANDS.UserPreferencesRetrieval](../tech_specs/user_preferences.md#spec-cynai-stands-prefretrieval)
  <a id="req-client-0115"></a>
- **REQ-CLIENT-0116:** Credential create MUST accept secrets only on create or rotate operations.
  [CYNAI.CLIENT.AdminWebConsoleCredential](../tech_specs/admin_web_console.md#spec-cynai-client-awccredential)
  [CYNAI.CLIENT.CliCredentialManagement](../tech_specs/cli_management_app.md#spec-cynai-client-clicredential)
  <a id="req-client-0116"></a>
- **REQ-CLIENT-0117:** Credential read MUST return metadata only.
  [CYNAI.CLIENT.AdminWebConsoleCredential](../tech_specs/admin_web_console.md#spec-cynai-client-awccredential)
  [CYNAI.CLIENT.CliCredentialManagement](../tech_specs/cli_management_app.md#spec-cynai-client-clicredential)
  <a id="req-client-0117"></a>
- **REQ-CLIENT-0118:** Credential list MUST support filtering by provider and scope.
  [CYNAI.CLIENT.AdminWebConsoleCredential](../tech_specs/admin_web_console.md#spec-cynai-client-awccredential)
  [CYNAI.CLIENT.CliCredentialManagement](../tech_specs/cli_management_app.md#spec-cynai-client-clicredential)
  <a id="req-client-0118"></a>
- **REQ-CLIENT-0119:** Credential rotate MUST create a new encrypted secret value and invalidate the old one.
  [CYNAI.CLIENT.AdminWebConsoleCredential](../tech_specs/admin_web_console.md#spec-cynai-client-awccredential)
  [CYNAI.CLIENT.CliCredentialManagement](../tech_specs/cli_management_app.md#spec-cynai-client-clicredential)
  <a id="req-client-0119"></a>
- **REQ-CLIENT-0120:** Credential disable MUST support immediate deactivation.
  [CYNAI.CLIENT.AdminWebConsoleCredential](../tech_specs/admin_web_console.md#spec-cynai-client-awccredential)
  [CYNAI.CLIENT.CliCredentialManagement](../tech_specs/cli_management_app.md#spec-cynai-client-clicredential)
  <a id="req-client-0120"></a>
- **REQ-CLIENT-0121:** Preference edits MUST be scoped and versioned.
  [CYNAI.CLIENT.AdminWebConsolePreferences](../tech_specs/admin_web_console.md#spec-cynai-client-awcpreferences)
  [CYNAI.CLIENT.CliPreferencesManagement](../tech_specs/cli_management_app.md#spec-cynai-client-clipreferences)
  <a id="req-client-0121"></a>
- **REQ-CLIENT-0122:** The UI MUST support preference scope selection (system, user, project, task).
  [CYNAI.CLIENT.AdminWebConsolePreferences](../tech_specs/admin_web_console.md#spec-cynai-client-awcpreferences)
  [CYNAI.CLIENT.CliPreferencesManagement](../tech_specs/cli_management_app.md#spec-cynai-client-clipreferences)
  <a id="req-client-0122"></a>
- **REQ-CLIENT-0123:** The UI SHOULD provide an \"effective preferences\" preview for a given task or project.
  [CYNAI.CLIENT.AdminWebConsolePreferences](../tech_specs/admin_web_console.md#spec-cynai-client-awcpreferences)
  [CYNAI.CLIENT.CliPreferencesManagement](../tech_specs/cli_management_app.md#spec-cynai-client-clipreferences)
  <a id="req-client-0123"></a>
- **REQ-CLIENT-0124:** The UI SHOULD provide validation for known keys and types.
  [CYNAI.CLIENT.AdminWebConsolePreferences](../tech_specs/admin_web_console.md#spec-cynai-client-awcpreferences)
  [CYNAI.CLIENT.CliPreferencesManagement](../tech_specs/cli_management_app.md#spec-cynai-client-clipreferences)
  <a id="req-client-0124"></a>
- **REQ-CLIENT-0125:** Node management MUST be mediated by the User API Gateway.
  [CYNAI.CLIENT.AdminWebConsoleNodeManagement](../tech_specs/admin_web_console.md#spec-cynai-client-awcnodemgmt)
  [CYNAI.CLIENT.CliNodeManagement](../tech_specs/cli_management_app.md#spec-cynai-client-clinodemgmt)
  <a id="req-client-0125"></a>
- **REQ-CLIENT-0126:** The UI MUST NOT connect directly to node worker APIs.
  [CYNAI.CLIENT.AdminWebConsoleNodeManagement](../tech_specs/admin_web_console.md#spec-cynai-client-awcnodemgmt)
  [CYNAI.CLIENT.CliNodeManagement](../tech_specs/cli_management_app.md#spec-cynai-client-clinodemgmt)
  <a id="req-client-0126"></a>
- **REQ-CLIENT-0127:** The UI MUST clearly distinguish between node-reported state and orchestrator-derived state.
  [CYNAI.CLIENT.AdminWebConsoleNodeManagement](../tech_specs/admin_web_console.md#spec-cynai-client-awcnodemgmt)
  <a id="req-client-0127"></a>
- **REQ-CLIENT-0128:** Potentially disruptive actions MUST be gated by admin authorization and SHOULD require confirmation.
  [CYNAI.CLIENT.AdminWebConsoleNodeManagement](../tech_specs/admin_web_console.md#spec-cynai-client-awcnodemgmt)
  [CYNAI.CLIENT.CliNodeManagement](../tech_specs/cli_management_app.md#spec-cynai-client-clinodemgmt)
  <a id="req-client-0128"></a>
- **REQ-CLIENT-0129:** The console MUST not embed privileged service credentials.
  [CYNAI.CLIENT.AdminWebConsoleImplementation](../tech_specs/admin_web_console.md#spec-cynai-client-awcimpl)
  <a id="req-client-0129"></a>
- **REQ-CLIENT-0130:** The console MUST not bypass gateway authorization and auditing.
  [CYNAI.CLIENT.AdminWebConsoleImplementation](../tech_specs/admin_web_console.md#spec-cynai-client-awcimpl)
  <a id="req-client-0130"></a>
- **REQ-CLIENT-0131:** The console MUST treat gateway responses as the source of truth.
  [CYNAI.CLIENT.AdminWebConsoleImplementation](../tech_specs/admin_web_console.md#spec-cynai-client-awcimpl)
  <a id="req-client-0131"></a>
- **REQ-CLIENT-0132:** The console MUST avoid storing bearer tokens in localStorage.
  [CYNAI.CLIENT.AdminWebConsoleAuthModel](../tech_specs/admin_web_console.md#spec-cynai-client-awcauth)
  <a id="req-client-0132"></a>
- **REQ-CLIENT-0133:** The console MUST support logout and token invalidation.
  [CYNAI.CLIENT.AdminWebConsoleAuthModel](../tech_specs/admin_web_console.md#spec-cynai-client-awcauth)
  <a id="req-client-0133"></a>
- **REQ-CLIENT-0134:** The console MUST enforce HTTPS in production deployments.
  [CYNAI.CLIENT.AdminWebConsoleDeployment](../tech_specs/admin_web_console.md#spec-cynai-client-awcdeploy)
  <a id="req-client-0134"></a>
- **REQ-CLIENT-0135:** CORS SHOULD be avoided by preferring same-origin hosting behind the gateway.
  [CYNAI.CLIENT.AdminWebConsoleDeployment](../tech_specs/admin_web_console.md#spec-cynai-client-awcdeploy)
  <a id="req-client-0135"></a>
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
- **REQ-CLIENT-0147:** The admin web console MUST support full CRUD for skills (create, list, view, update, delete) via the User API Gateway, with the same controls as defined in the skills spec (scope visibility, scope elevation permission, auditing on write).
  [CYNAI.CLIENT.AdminWebConsoleSkillsManagement](../tech_specs/admin_web_console.md#spec-cynai-client-awcskillsmanagement)
  [CYNAI.SKILLS.SkillManagementCrud](../tech_specs/skills_storage_and_inference.md#spec-cynai-skills-skillmanagementcrud)
  <a id="req-client-0147"></a>
- **REQ-CLIENT-0148:** The admin web console MUST provide Swagger UI (or equivalent API documentation UI) for the User API Gateway so that authenticated admins can discover and try API endpoints.
  [CYNAI.CLIENT.AdminWebConsoleSwaggerUi](../tech_specs/admin_web_console.md#spec-cynai-client-awcswaggerui)
  <a id="req-client-0148"></a>
