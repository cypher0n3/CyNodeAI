# Git Egress MCP

- [Document Overview](#document-overview)
- [Problem Statement](#problem-statement)
- [Goals and Non-Goals](#goals-and-non-goals)
- [Architecture and Trust Boundaries](#architecture-and-trust-boundaries)
- [MCP Tool Interface](#mcp-tool-interface)
- [Credential Storage](#credential-storage)
- [Access Control](#access-control)
- [Auditing](#auditing)
- [Sandbox Output Formats](#sandbox-output-formats)
- [Recommended Workflows](#recommended-workflows)
- [Failure Modes and Safety](#failure-modes-and-safety)

## Document Overview

This document defines a Git egress service exposed as MCP tools.
Git egress enables creating commits and pull requests from sandbox-produced changes without placing Git credentials or unrestricted network access inside sandboxes.

## Problem Statement

Sandboxes should be treated as untrusted and network-restricted by default.
Direct Git host access from sandboxes risks credential leakage and data exfiltration.

## Goals and Non-Goals

Goals

- Keep Git credentials out of sandboxes and out of agent processes.
- Provide a policy-controlled and auditable path to push changes to Git remotes.
- Support user-scoped and group-scoped Git credentials for enterprise workflows.
- Support patch-based and bundle-based promotion of changes from sandboxes.

Non-goals

- This document does not define a full CI system.
- This document does not define a full code review workflow beyond PR creation.

## Architecture and Trust Boundaries

Normative requirements

- Sandboxes MUST NOT store Git credentials.
- Sandboxes MUST NOT make arbitrary outbound network calls to Git hosts.
- Git operations that require remote access MUST be performed by the Git egress service.
- The orchestrator SHOULD act as the policy and routing point for Git egress operations.

High-level flow

- A sandbox produces a changeset artifact (patch, bundle, or file set).
- The orchestrator validates policy and dispatch intent.
- Git egress applies the changeset to a controlled checkout and performs git operations.
- Results (commit SHA, branch name, PR URL, errors) are returned to the orchestrator and recorded for the task.

## MCP Tool Interface

Git egress is exposed as MCP tools.
All tool calls MUST include task context for auditing and access control.

Recommended tools

- `git.repo.validate`
  - Validate a target repo reference against allowlists and policy.
- `git.changeset.apply`
  - Apply a patch or bundle artifact to a controlled checkout.
- `git.commit.create`
  - Create a commit from an applied changeset.
- `git.branch.create`
  - Create a new branch from a base ref.
- `git.push`
  - Push a branch to a remote.
- `git.pr.create`
  - Create a pull request in a supported provider.

Minimum request fields

- `task_id` (uuid)
- `provider` (text)
  - Examples: github, gitlab, gitea.
- `repo` (text)
  - Repo identifier (provider-specific).
- `operation` (text)
  - Named operation (commit, push, pr_create).
- `params` (object)
  - Operation-specific inputs.

Minimum response fields

- `status` (success|error)
- `result` (object)
- `error` (object, optional)

## Credential Storage

Git credentials MUST be stored in PostgreSQL and MUST be retrievable only by Git egress.
Credentials SHOULD support both user-scoped and group-scoped ownership.

Recommended model

- Use the same ownership pattern as API egress credentials.
- Store provider tokens or SSH deploy keys encrypted.
- Support multiple credentials per owner and provider using a human-friendly `credential_name`.

See [`docs/tech_specs/api_egress_server.md`](api_egress_server.md) for credential handling patterns.

## Access Control

Git egress MUST be default-deny.
Access control SHOULD be enforced by the orchestrator and by the Git egress service.

Recommended access control dimensions

- Subject identity (user and agent identity).
- Action (git operation, such as `git.push` or `git.pr.create`).
- Resource (repo and branch targets).
- Credential owner (user-scoped vs group-scoped).
- Context (task_id, project_id, environment labels).

Example policies

- Allow PR creation only for repos in an allowlist.
- Deny pushes to protected branches.
- Restrict group-scoped credentials to tasks with an explicit group context.
- Require that sandbox changesets are created by jobs for the same task_id.

See [`docs/tech_specs/access_control.md`](access_control.md).

## Auditing

All Git egress calls MUST be audited with task context.

Minimum audit fields

- `task_id`
- `subject_type` and `subject_id`
- `provider` and `repo`
- `operation`
- `branch` and `base_ref` when applicable
- `changeset_hash` (hash of patch or bundle bytes)
- `decision` (allow|deny)
- `result` (commit SHA or PR id, when successful)

## Sandbox Output Formats

Sandboxes SHOULD export changes in formats that do not require network access.

Recommended formats

- Patch-based
  - Unified diff artifact (example: `diff.patch`).
- Bundle-based
  - Git bundle artifact (example: `changes.bundle`).
- File-set based
  - Explicit list of files and content, stored as artifacts.

Normative requirements

- A changeset artifact MUST be associated with a single `task_id`.
- A changeset artifact MUST NOT include credentials.

## Recommended Workflows

This section lists safe, common end-to-end flows for promoting sandbox-produced changes into Git.

### Patch-Based Promotion

- Sandbox runs tool steps and produces `diff.patch` as an artifact.
- Git egress checks out the target repo at `base_ref`.
- Git egress applies the patch, runs optional safety checks, commits, pushes, and opens a PR.

### Bundle-Based Promotion

- Sandbox produces a git bundle artifact.
- Git egress imports the bundle into a controlled checkout and pushes the resulting refs.

### Direct PR Creation

- Git egress creates a branch, applies changes, pushes, and creates a PR.
- The orchestrator records PR metadata on the task.

## Failure Modes and Safety

Recommended safety checks

- Reject patches that modify files outside an allowed path set, when configured.
- Enforce maximum diff size limits.
- Enforce branch naming rules (task-scoped branch prefixes).
- Require successful sandbox test artifacts before allowing a push, when policy requires it.

Error handling

- Git egress SHOULD return structured error types for auth failures, policy denials, merge conflicts, and patch apply failures.
- The orchestrator SHOULD surface these errors to the Project Manager Agent for remediation planning.
