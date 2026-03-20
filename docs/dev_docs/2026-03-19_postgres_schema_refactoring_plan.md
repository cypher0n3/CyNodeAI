# Postgres Schema Refactoring Plan

## Overview

The `postgres_schema.md` file (1631 lines) currently serves as the single canonical source for all PostgreSQL table definitions.
This plan proposes breaking it up so that each table definition lives in the appropriate domain-specific document.

## Current State

- `postgres_schema.md` contains all table definitions
- Domain documents (e.g., `local_user_accounts.md`, `projects_and_scopes.md`) reference `postgres_schema.md` for schema details
- Some domain documents have "recommended" schemas that are less detailed than the authoritative definitions

## Proposed State

- Each domain document becomes the authoritative source for its tables
- `postgres_schema.md` becomes an index/overview document that:
  - Links to distributed table definitions
  - Provides the table creation order and dependencies
  - Maintains naming conventions
  - Keeps the "Storing This Schema in Code" section

## Table-To-Document Mapping

- **Tables:** `users`, `password_credentials`, `refresh_sessions`
  - target document: `local_user_accounts.md`
  - status: Pending
- **Tables:** `projects`, `project_plans`, `project_plan_revisions`, `specifications`, `project_git_repos`
  - target document: `projects_and_scopes.md`
  - status: Pending
- **Tables:** `groups`, `group_memberships`, `roles`, `role_bindings`
  - target document: `rbac_and_groups.md`
  - status: Pending
- **Tables:** `access_control_rules`, `access_control_audit_log`
  - target document: `access_control.md`
  - status: Pending
- **Tables:** `api_credentials`
  - target document: `api_egress_server.md`
  - status: Pending
- **Tables:** `preference_entries`, `preference_audit_log`
  - target document: `user_preferences.md`
  - status: Pending
- **Tables:** `system_settings`, `system_settings_audit_log`
  - target document: `orchestrator_bootstrap.md` or new `system_settings.md`
  - status: Pending
- **Tables:** `personas`
  - target document: New section in agent docs or new `personas.md`
  - status: Pending
- **Tables:** `tasks`, `task_dependencies`, `jobs`
  - target document: `langgraph_mvp.md` or `orchestrator.md`
  - status: Pending
- **Tables:** `nodes`, `node_capabilities`
  - target document: `worker_node.md`
  - status: Pending
- **Tables:** `workflow_checkpoints`, `task_workflow_leases`
  - target document: `langgraph_mvp.md`
  - status: Pending
- **Tables:** `sandbox_images`, `sandbox_image_versions`, `node_sandbox_image_availability`
  - target document: `sandbox_image_registry.md`
  - status: Pending
- **Tables:** `runs`, `sessions`
  - target document: `runs_and_sessions_api.md`
  - status: Pending
- **Tables:** `chat_threads`, `chat_messages`, `chat_message_attachments`
  - target document: `chat_threads_and_messages.md`
  - status: Pending
- **Tables:** `task_artifacts`
  - target document: `orchestrator_artifacts_storage.md`
  - status: Pending
- **Tables:** `vector_items`
  - target document: `pgvector_proposal_draft.md` or vector storage spec
  - status: Pending
- **Tables:** `auth_audit_log`, `mcp_tool_call_audit_log`, `chat_audit_log`
  - target document: `mcp/mcp_tool_call_auditing.md` or audit docs
  - status: Pending
- **Tables:** `models`, `model_versions`, `model_artifacts`, `node_model_availability`
  - target document: `model_management.md`
  - status: Pending

## Execution Steps

1. For each table group:
   - Extract the table definition section from `postgres_schema.md`
   - Add it to the target document (replacing "recommended" schemas if present)
   - Update the target document's TOC
   - Add a "Postgres Schema" section with Spec IDs and anchors
   - Remove normative obligations (MUST/SHOULD) and make prescriptive

2. Update `postgres_schema.md`:
   - Replace detailed table definitions with links to distributed definitions
   - Keep the overview, creation order, naming conventions, and "Storing This Schema in Code" sections
   - Update all cross-references

3. Update all documents that reference `postgres_schema.md`:
   - Update links to point to the new locations
   - Ensure Spec ID anchors are preserved

4. Validate:
   - Run `just lint-md` on all affected files
   - Run `just docs-check` to verify links
   - Ensure all Spec IDs are preserved and anchors work

## Considerations

- **Spec IDs**: Must be preserved when moving sections
- **Anchors**: All `spec-cynai-*` anchors must continue to work
- **Cross-references**: Many documents reference specific tables; all links must be updated
- **Dependencies**: Table creation order section must remain in `postgres_schema.md` or be clearly documented
- **Breaking changes**: This is a documentation refactoring, not a schema change

## Estimated Effort

- ~15-20 table groups to move
- ~50-100 cross-references to update
- Multiple validation passes required

## Recommendation

Start with a proof of concept (Identity and Authentication tables) to validate the approach, then proceed systematically through all table groups.
