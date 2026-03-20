# Git Egress MCP Tools

- [Document Overview](#document-overview)
- [Problem Statement](#problem-statement)
- [Goals and Non-Goals](#goals-and-non-goals)
- [Architecture and Trust Boundaries](#architecture-and-trust-boundaries)
- [Definition Compliance](#definition-compliance)
- [Tool Contracts](#tool-contracts)
  - [`git.repo.validate` Operation](#gitrepovalidate-operation)
  - [`git.changeset.apply` Operation](#gitchangesetapply-operation)
  - [`git.commit.create` Operation](#gitcommitcreate-operation)
  - [`git.branch.create` Operation](#gitbranchcreate-operation)
  - [`git.push` Operation](#gitpush-operation)
  - [`git.pr.create` Operation](#gitprcreate-operation)
- [Credential Storage](#credential-storage)
- [Access Control](#access-control)
- [Auditing](#auditing)
- [Sandbox Output Formats](#sandbox-output-formats)
- [Recommended Workflows](#recommended-workflows)
- [Failure Modes and Safety](#failure-modes-and-safety)
- [Allowlist and Scope](#allowlist-and-scope)

## Document Overview

- Spec ID: `CYNAI.MCPTOO.GitEgressTools` <a id="spec-cynai-mcptoo-gitegresstools"></a>

This document defines Git egress tools that enable validated, policy-controlled Git operations (validate repo, apply changeset, create commit, create branch, push, create PR) without placing Git credentials or unrestricted network access inside sandboxes.

Git egress enables creating commits and pull requests from sandbox-produced changes without placing Git credentials or unrestricted network access inside sandboxes.

## Problem Statement

Sandboxes should be treated as untrusted and network-restricted by default.
Direct Git host access from sandboxes risks credential leakage and data exfiltration.

Clean split model

- Sandboxes MAY run local-only Git commands against the mounted workspace.
- Sandboxes MUST NOT perform remote-affecting Git operations (for example `git clone`, `git fetch`, `git pull`, `git push`, submodule fetch/update, or Git LFS downloads).
- All Git operations that require remote access MUST be performed via Git egress using task-scoped changeset artifacts.

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

This section describes how Git operations are performed outside sandboxes and within policy-controlled boundaries.

### Applicable Requirements

- Spec ID: `CYNAI.APIEGR.GitEgressArchitecture` <a id="spec-cynai-apiegr-gitegressarch"></a>

#### Traces to Requirements

- [REQ-APIEGR-0100](../../requirements/apiegr.md#req-apiegr-0100)
- [REQ-APIEGR-0101](../../requirements/apiegr.md#req-apiegr-0101)
- [REQ-APIEGR-0102](../../requirements/apiegr.md#req-apiegr-0102)
- [REQ-APIEGR-0103](../../requirements/apiegr.md#req-apiegr-0103)

High-level flow

- A sandbox produces a changeset artifact (patch, bundle, or file set).
- The orchestrator validates policy and dispatch intent.
- Git egress applies the changeset to a controlled checkout and performs git operations.
- Results (commit SHA, branch name, PR URL, errors) are returned to the orchestrator and recorded for the task.

## Definition Compliance

Tool definitions MUST conform to the project's MCP tool definition format: `Server: default`, `Name`, `Help`, `Scope`, `Tools` (single direct invocation per tool name).
All tool calls MUST include task context for auditing and access control.

## Tool Contracts

Git egress is exposed as MCP tools.
All tool calls MUST include task context for auditing and access control.

Minimum request fields for all tools: `task_id` (uuid), `provider` (text, e.g. github, gitlab, gitea), `repo` (provider-specific repo identifier), `params` (object) where applicable.

Minimum response fields

- `status` (success|error)
- `result` (object)
- `error` (object, optional)

### `git.repo.validate` Operation

- **Inputs**: Required `task_id`, `provider`, `repo`.
  Scope: typically `pm` or sandbox when policy permits.
- **Outputs**: Validation result (allowed/denied, reason); no secrets.
- **Behavior**: Gateway enforces allowlist and scope, forwards to Git egress; egress checks provider+repo against project/task allowlist and policy.
  See [git.repo.validate Algorithm](#algo-cynai-mcptoo-gitrepovalidate).

#### `git.repo.validate` Algorithm

<a id="algo-cynai-mcptoo-gitrepovalidate"></a>

1. Resolve caller; check allowlist and scope.
2. Validate `task_id`, `provider`, `repo` (format and presence).
3. Resolve task context (e.g. project_id); load project-repo allowlist if applicable.
4. Git egress: check provider+repo against allowlist and access control; return allow/deny and optional reason.
5. Emit audit record; return validation result.

### `git.changeset.apply` Operation

- **Inputs**: Required `task_id`, `provider`, `repo`, `params` (e.g. changeset ref, patch/bundle artifact).
  Scope: pm or sandbox per policy.
- **Outputs**: Apply result (success, paths changed, or error).
- **Behavior**: Gateway forwards to Git egress; egress applies patch or bundle to controlled checkout.
  See [git.changeset.apply Algorithm](#algo-cynai-mcptoo-gitchangesetapply).

#### `git.changeset.apply` Algorithm

<a id="algo-cynai-mcptoo-gitchangesetapply"></a>

1. Resolve caller; check allowlist and scope.
2. Validate task_id, provider, repo, params (changeset ref or inline patch/bundle).
3. Git egress: resolve credential for provider/repo (server-side); obtain or create controlled checkout; apply changeset; return result.
4. Emit audit record (task_id, provider, repo, changeset hash); return result or error.

### `git.commit.create` Operation

- **Inputs**: Required `task_id`, `provider`, `repo`, `params` (message, author, etc.).
  Scope: typically `pm`.
- **Outputs**: Commit SHA and optional ref; or error.
- **Behavior**: Gateway forwards to Git egress; egress creates commit from applied changeset in controlled checkout.
  See [git.commit.create Algorithm](#algo-cynai-mcptoo-gitcommitcreate).

#### `git.commit.create` Algorithm

<a id="algo-cynai-mcptoo-gitcommitcreate"></a>

1. Resolve caller; check allowlist and scope.
2. Validate task_id, provider, repo, params (message required).
3. Git egress: create commit in controlled checkout; return commit SHA.
4. Emit audit record; return result.

### `git.branch.create` Operation

- **Inputs**: Required `task_id`, `provider`, `repo`, `params` (branch name, base ref).
  Scope: typically `pm`.
- **Outputs**: Branch name or ref; or error.
- **Behavior**: Gateway forwards to Git egress; egress creates branch from base ref in controlled checkout.
  See [git.branch.create Algorithm](#algo-cynai-mcptoo-gitbranchcreate).

#### `git.branch.create` Algorithm

<a id="algo-cynai-mcptoo-gitbranchcreate"></a>

1. Resolve caller; check allowlist and scope.
2. Validate task_id, provider, repo, params (branch name, base_ref).
3. Git egress: create branch from base ref; return branch ref.
4. Emit audit record; return result.

### `git.push` Operation

- **Inputs**: Required `task_id`, `provider`, `repo`, `params` (branch, remote).
  Scope: typically `pm`.
- **Outputs**: Push result (success, refs updated) or error.
- **Behavior**: Gateway forwards to Git egress; egress pushes branch to remote using resolved credentials.
  See [git.push Algorithm](#algo-cynai-mcptoo-gitpush).

#### `git.push` Algorithm

<a id="algo-cynai-mcptoo-gitpush"></a>

1. Resolve caller; check allowlist and scope.
2. Validate task_id, provider, repo, params (branch, remote).
3. Git egress: resolve credentials; push branch to remote; return result.
4. Emit audit record (task_id, provider, repo, branch); return result or error.

### `git.pr.create` Operation

- **Inputs**: Required `task_id`, `provider`, `repo`, `params` (source branch, target, title, body).
  Scope: typically `pm`.
- **Outputs**: PR URL or id; or error.
- **Behavior**: Gateway forwards to Git egress; egress creates pull request in supported provider.
  See [git.pr.create Algorithm](#algo-cynai-mcptoo-gitprcreate).

#### `git.pr.create` Algorithm

<a id="algo-cynai-mcptoo-gitprcreate"></a>

1. Resolve caller; check allowlist and scope.
2. Validate task_id, provider, repo, params (source, target, title).
3. Git egress: call provider API to create PR; return PR URL/id.
4. Emit audit record; return result or error.

#### Traces To

- [REQ-MCPTOO-0106](../../requirements/mcptoo.md#req-mcptoo-0106)
- [REQ-APIEGR-0100](../../requirements/apiegr.md#req-apiegr-0100)

## Credential Storage

Git credentials MUST be stored in PostgreSQL and MUST be retrievable only by Git egress.
Credentials SHOULD support both user-scoped and group-scoped ownership.

Recommended model

- Use the same ownership pattern as API egress credentials.
- Store provider tokens or SSH deploy keys encrypted.
- Support multiple credentials per owner and provider using a human-friendly `credential_name`.

See [`docs/tech_specs/api_egress_server.md`](../api_egress_server.md) for credential handling patterns.

Go code that retrieves or decrypts Git credentials MUST use `runtime/secret` when available per [REQ-STANDS-0133](../../requirements/stands.md#req-stands-0133); when not available, MUST use best-effort secure erasure before returning.

## Access Control

Git egress MUST be default-deny.
Access control SHOULD be enforced by the orchestrator and by the Git egress service.

Project-scoped repo allowlist

- When a task has a non-null `project_id`, Git egress MUST validate that the requested provider and repo are associated with that project.
  See [CYNAI.APIEGR.GitEgressProjectScope](../project_git_repos.md#spec-cynai-apiegr-gitegressprojectscope) and [REQ-APIEGR-0127](../../requirements/apiegr.md#req-apiegr-0127).
  The project-repo association model is defined in [`docs/tech_specs/project_git_repos.md`](../project_git_repos.md).

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

See [`docs/tech_specs/access_control.md`](../access_control.md).

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

### Sandbox Output Formats Applicable Requirements

- Spec ID: `CYNAI.APIEGR.GitEgressSandboxOutput` <a id="spec-cynai-apiegr-gitegressout"></a>

#### Sandbox Output Formats Applicable Requirements Requirements Traces

- [REQ-APIEGR-0104](../../requirements/apiegr.md#req-apiegr-0104)
- [REQ-APIEGR-0105](../../requirements/apiegr.md#req-apiegr-0105)

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

## Allowlist and Scope

- **Allowlist**: PMA, PAA, and optionally Worker agent per [MCP Gateway Enforcement](../mcp/mcp_gateway_enforcement.md).
- **Scope**: Typically `pm` for create/push/PR; sandbox may be allowed for validate/apply when policy permits.
