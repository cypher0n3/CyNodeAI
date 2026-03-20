# Git Egress MCP Tools

- [Document Overview](#document-overview)
- [Definition Compliance](#definition-compliance)
- [Tool Contracts](#tool-contracts)
  - [`git.repo.validate` Operation](#gitrepovalidate-operation)
  - [`git.changeset.apply` Operation](#gitchangesetapply-operation)
  - [`git.commit.create` Operation](#gitcommitcreate-operation)
  - [`git.branch.create` Operation](#gitbranchcreate-operation)
  - [`git.push` Operation](#gitpush-operation)
  - [`git.pr.create` Operation](#gitprcreate-operation)
- [Allowlist and Scope](#allowlist-and-scope)

## Document Overview

Git egress tools enable validated, policy-controlled Git operations (validate repo, apply changeset, create commit, create branch, push, create PR) without placing Git credentials or unrestricted network access inside sandboxes.
Canonical tool names and behavior are defined in the Git Egress MCP spec; this document provides the per-tool spec and alignment with the definition format.

Related documents

- [Git Egress MCP](../git_egress_mcp.md)

## Definition Compliance

Tool definitions MUST conform to the project's MCP tool definition format: `Server: default`, `Name`, `Help`, `Scope`, `Tools` (single direct invocation per tool name).
All tool calls MUST include task context for auditing and access control.

## Tool Contracts

- Spec ID: `CYNAI.MCPTOO.GitEgressTools` <a id="spec-cynai-mcptoo-gitegresstools"></a>

Minimum request fields for all tools: `task_id` (uuid), `provider` (text, e.g. github, gitlab, gitea), `repo` (provider-specific repo identifier), `params` (object) where applicable.

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
- [REQ-APIEGR-0100](../../requirements/apiegr.md#req-apiegr-0100) (via [Git Egress MCP](../git_egress_mcp.md))

## Allowlist and Scope

- **Allowlist**: PMA, PAA, and optionally Worker agent per [MCP Gateway Enforcement](../mcp_gateway_enforcement.md) and [Git Egress MCP](../git_egress_mcp.md).
- **Scope**: Typically `pm` for create/push/PR; sandbox may be allowed for validate/apply when policy permits.
