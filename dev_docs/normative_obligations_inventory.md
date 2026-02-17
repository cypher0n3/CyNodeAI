# Normative Obligations Inventory (Migration Input)

## Overview

This document inventories normative obligations that must be migrated into `docs/requirements/` under Model B (requirements are the canonical normative "what").

Notes:

- `norm_anchor_count` counts occurrences of `<a id="norm-..."></a>` in the tech spec file.
- `rfc2119_token_count` counts occurrences of RFC-2119-style tokens in the file text (`MUST`, `MUST NOT`, `SHOULD`, `SHOULD NOT`, `MAY`).
- These counts are mechanical and may include matches inside examples or code blocks.
  They are used to size the migration and avoid missing obligations.

## Inventory

| tech_spec_file                               | target_requirements_domain | norm_anchor_count | rfc2119_token_count |
| -------------------------------------------- | -------------------------- | ----------------: | ------------------: |
| `docs/tech_specs/access_control.md`          | `ACCESS`                   |                 0 |                   3 |
| `docs/tech_specs/rbac_and_groups.md`         | `ACCESS`                   |                15 |                  25 |
| `docs/tech_specs/projects_and_scopes.md`     | `ACCESS`                   |                 3 |                   6 |
| `docs/tech_specs/local_user_accounts.md`     | `IDENTY`                   |                21 |                  28 |
| `docs/tech_specs/runs_and_sessions_api.md`   | `USRGWY`                   |                21 |                  26 |
| `docs/tech_specs/user_api_gateway.md`        | `USRGWY`                   |                 0 |                  17 |
| `docs/tech_specs/data_rest_api.md`           | `DATAPI`                   |                13 |                  21 |
| `docs/tech_specs/orchestrator.md`            | `ORCHES`                   |                 0 |                  16 |
| `docs/tech_specs/orchestrator_bootstrap.md`  | `BOOTST`                   |                 3 |                  12 |
| `docs/tech_specs/node.md`                    | `WORKER`                   |                22 |                  58 |
| `docs/tech_specs/worker_api.md`              | `WORKER`                   |                 9 |                  20 |
| `docs/tech_specs/node_payloads.md`           | `WORKER`                   |                 0 |                   8 |
| `docs/tech_specs/sandbox_container.md`       | `SANDBX`                   |                15 |                  30 |
| `docs/tech_specs/sandbox_image_registry.md`  | `SANDBX`                   |                 0 |                  11 |
| `docs/tech_specs/api_egress_server.md`       | `APIEGR`                   |                 0 |                  15 |
| `docs/tech_specs/git_egress_mcp.md`          | `APIEGR`                   |                 6 |                  15 |
| `docs/tech_specs/mcp_gateway_enforcement.md` | `MCPGAT`                   |                 7 |                  18 |
| `docs/tech_specs/mcp_tool_call_auditing.md`  | `MCPGAT`                   |                 3 |                   7 |
| `docs/tech_specs/mcp_tooling.md`             | `MCPTOO`                   |                 6 |                  17 |
| `docs/tech_specs/mcp_tool_catalog.md`        | `MCPTOO`                   |                 5 |                  13 |
| `docs/tech_specs/mcp_sdk_installation.md`    | `MCPTOO`                   |                 0 |                   9 |
| `docs/tech_specs/postgres_schema.md`         | `SCHEMA`                   |                12 |                  28 |
| `docs/tech_specs/model_management.md`        | `MODELS`                   |                 0 |                  12 |
| `docs/tech_specs/external_model_routing.md`  | `MODELS`                   |                 0 |                  19 |
| `docs/tech_specs/cloud_agents.md`            | `AGENTS`                   |                 6 |                  21 |
| `docs/tech_specs/project_manager_agent.md`   | `AGENTS`                   |                12 |                  21 |
| `docs/tech_specs/project_analyst_agent.md`   | `AGENTS`                   |                 4 |                  13 |
| `docs/tech_specs/langgraph_mvp.md`           | `AGENTS`                   |                 3 |                  20 |
| `docs/tech_specs/connector_framework.md`     | `CONNEC`                   |                22 |                  41 |
| `docs/tech_specs/cli_management_app.md`      | `CLIENT`                   |                18 |                  34 |
| `docs/tech_specs/admin_web_console.md`       | `CLIENT`                   |                25 |                  35 |
| `docs/tech_specs/user_preferences.md`        | `CLIENT`                   |                 3 |                   3 |
| `docs/tech_specs/secure_browser_service.md`  | `BROWSR`                   |                 0 |                  24 |
