# Workspace Provisioning and Project Repos (Proposal)

- [Scope and Metadata](#scope-and-metadata)
- [Summary](#summary)
- [Goals and Non-Goals](#goals-and-non-goals)
- [Problem Statement](#problem-statement)
- [Proposed Specification Changes](#proposed-specification-changes)
  - [1. Project Git Repos: Use and Workspace Provisioning](#1-project-git-repos-use-and-workspace-provisioning)
  - [2. Credential Boundary and Clone Responsibility](#2-credential-boundary-and-clone-responsibility)
  - [3. Job Spec: Workspace Source](#3-job-spec-workspace-source)
  - [4. Worker Node: Workspace Population](#4-worker-node-workspace-population)
  - [5. Worker API: Run Job Workspace Content](#5-worker-api-run-job-workspace-content)
  - [6. Sandbox Container: Workspace Pre-Population](#6-sandbox-container-workspace-pre-population)
- [Requirements to Add or Extend](#requirements-to-add-or-extend)
- [References](#references)

## Scope and Metadata

- Date: 2026-03-18
- Status: Proposal (draft_specs; not merged to tech_specs)
- Scope: Implementation "how" for getting Git repo content into the SBA sandbox workspace without exposing credentials; single allowlist (project_git_repos) for both Git egress and workspace provisioning.

## Summary

Canonical specs define that SBAs run in sandboxes with `/workspace` and that Git credentials and remote Git operations stay outside the sandbox (Git egress; project-scoped repo allowlist).
They do not specify how repo content reaches `/workspace` before or when the job runs.
This proposal adds prescriptive "how": who performs clone, where credentials live, how the allowlist is applied at provisioning time, and how the job payload and node contract support workspace content so implementers have no ambiguity.

## Goals and Non-Goals

This section bounds the proposal scope.

### Proposal Goals

- Remove implementation ambiguity: one place (project_git_repos + this proposal) defines use of project repos for both egress and workspace provisioning.
- Preserve strict no-secrets-in-sandbox: the component that holds Git credentials performs clone and produces a content-only bundle; the sandbox never receives credentials or direct remote Git access.
- Define the data flow: task/job specifies workspace source (repo); credential-holding component validates allowlist, clones, produces bundle; orchestrator or node delivers bundle to node; node unpacks into workspace before container start.

### Proposal Non-Goals

- This proposal does not introduce "related" vs "primary" repo roles; the same project_git_repos allowlist is used for both egress and provisioning.
- Defining automated "tracking" (e.g. webhooks, notify on upstream change) is out of scope.
- Changing Git egress push/PR tool contracts or credential storage; only clarifying who may perform clone and how workspace provisioning ties to the same allowlist.

## Problem Statement

- Sandbox has `/workspace` (bind-mounted) and may run local-only Git; remote Git (clone, fetch, push) is forbidden from inside the sandbox.
- Git egress specifies push/PR from sandbox-produced changesets; project-scoped allowlist (project_git_repos) is used when task has project_id.
- It is unspecified how repo content gets into `/workspace` so the SBA can work against a clone.
- Without a defined flow, implementers cannot decide: who clones, where, when, and how the node receives content without ever handling credentials.

## Proposed Specification Changes

This section lists concrete spec edits by document.

### 1. Project Git Repos: Use and Workspace Provisioning

- **Location:** [project_git_repos.md](../../tech_specs/project_git_repos.md)
- **Add: Subsection "Use of Project Repos (Allowlist)".**
  The set of repos associated with a project (`project_git_repos` for that `project_id`) is used for:
  1. **Git egress:** Push/PR allowlist when a task has `project_id` (existing; see [Git Egress Project-Scoped Allowlist](../../tech_specs/project_git_repos.md#spec-cynai-apiegr-gitegressprojectscope)).
  2. **Workspace provisioning:** The only repos that MAY be cloned to populate a job's `/workspace` when the task has `project_id`.
- Enforcement for (1) is in [git_egress.md](../../tech_specs/mcp_tools/git_egress.md); for (2) in the new "Workspace Provisioning" subsection below.
- **Add: Subsection "Workspace Provisioning (Repo to Sandbox)".**
  Spec ID (proposed): `CYNAI.PROJCT.WorkspaceProvisioning`
- **Responsibility:** Clone or fetch for workspace provisioning MUST be performed by a component that holds Git credentials (e.g. the same service that implements Git egress, or a dedicated Git workspace provisioner).
  The sandbox, Node Manager, and worker node MUST NOT perform Git clone or fetch using credentials; they MAY unpack a content-only bundle produced by the credential-holding component.
- **Allowlist:** When provisioning is requested for a task, the provisioning component MUST resolve `task_id` to `project_id` (from `tasks.project_id`).
  The requested repo (provider, repo_identifier, and base_url when present) MUST appear in that project's `project_git_repos` set.
  If the task has no `project_id`, provisioning from a Git repo MUST be denied unless a separate allowlist or policy is defined (out of scope for this proposal; implementations MUST document behavior).
- **Output:** The provisioning component MUST produce a content-only bundle (e.g. tarball of the checkout).
  The bundle MUST NOT contain credentials, `.git/config` or remotes that could be used to authenticate to the Git host, or any secret material.
  The bundle MAY contain a `.git` directory with local-only history so the sandbox can run local Git commands (e.g. `git status`, `git diff`, `git commit`); the implementation MUST ensure no embedded credentials or remote URLs with embedded secrets.
- **Sequence (normative):**
  1. Orchestrator or job builder decides this job needs workspace from repo R (from task/project/plan).
  2. Orchestrator (or a service it calls) requests a workspace bundle for (provider, repo_identifier, ref, task_id).
  3. Provisioning component resolves task_id to project_id, loads project_git_repos for that project, validates (provider, repo_identifier, base_url) against the allowlist; if not allowed, returns an error.
  4. Provisioning component clones (or fetches) using stored credentials, produces a tarball (or equivalent), returns it to the orchestrator or a reference the node can use to fetch.
  5. Orchestrator includes the bundle (or reference) in the Run Job request (or a dedicated prepare-workspace step).
  6. Node receives the bundle, unpacks it into the job's workspace directory, then starts the container.
  7. Sandbox sees a populated `/workspace`; it never receives credentials or direct Git remote access.

### 2. Credential Boundary and Clone Responsibility

- **Location:** [git_egress.md](../../tech_specs/mcp_tools/git_egress.md) or [project_git_repos.md](../../tech_specs/project_git_repos.md).
- **Add: Short subsection "Git Credential Boundary" (or extend Credential Storage).**
  Git credentials (tokens, deploy keys) MUST be stored and used only by the component(s) that implement Git egress and/or workspace provisioning (e.g. orchestrator-side Git egress service, API Egress Server, or a dedicated Git provisioner).
  They MUST NOT be passed to worker nodes, Node Manager, or sandbox containers.
- Clone or fetch for workspace provisioning MUST be performed by the same credential-holding component (or a dedicated provisioner that has its own credentials and enforces the same project_git_repos allowlist).
  The result MUST be a content-only bundle with no secrets and no remotes that could be used to authenticate to the Git host.

### 3. Job Spec: Workspace Source

- **Location:** [cynode_sba.md](../../tech_specs/cynode_sba.md) (Job Specification).
- **Add: Optional `workspace` (or `workspace_source`) in job payload.**
  Schema (proposed): optional top-level object `workspace` with:
  - `source`: `"git"` (required when workspace block is present).
  - `provider` (string, required): e.g. `github`, `gitlab`, `gitea`.
  - `repo_identifier` (string, required): provider-specific identifier (e.g. `owner/repo`).
  - `base_url` (string, optional): override for self-hosted instances.
  - `ref` (string, optional): branch or ref to clone (e.g. `main`, `develop`); default implementation-defined, e.g. `main`.
- Semantics: when `workspace` is present, the orchestrator (or node, per spec) MUST trigger the workspace provisioning path per the Workspace Provisioning subsection proposed in [project_git_repos.md](../../tech_specs/project_git_repos.md) (this draft).
  The requested repo MUST be in the task's project allowlist (project_git_repos for `tasks.project_id`).
  Otherwise the job MUST be rejected or provisioning MUST fail with a structured error.
- Binding: for a job with `task_id`, the workspace source (provider, repo_identifier, base_url) MUST be in the set of repos associated with that task's `project_id`; resolution and enforcement are defined in the project_git_repos Workspace Provisioning subsection.

### 4. Worker Node: Workspace Population

- **Location:** [worker_node.md](../../tech_specs/worker_node.md) (Sandbox Workspace and Job Mounts).
- **Add or extend.**
  The node MUST create the workspace directory before starting the container.
  When the Run Job request (or a separate workspace-prep step) includes **workspace content** (e.g. tarball or reference to fetch from the orchestrator), the node MUST populate the workspace directory with that content (e.g. unpack tarball into the workspace path) before starting the sandbox container.
  The node MUST NOT perform Git clone or fetch itself unless it has been given a content bundle produced by the orchestrator or provisioning service.
  The node MUST NOT receive or use Git credentials.

### 5. Worker API: Run Job Workspace Content

- **Location:** [worker_api.md](../../tech_specs/worker_api.md) (Run Job request/response).
- **Add.**
  The Run Job request MAY include workspace content so the node can populate `/workspace` before starting the container.
  Option A: request body includes a field `workspace.bundle` (e.g. base64-encoded tarball or inline content).
  Option B: request includes `workspace.source` (e.g. `git` with provider, repo_identifier, ref) and the orchestrator has pre-staged the bundle elsewhere.
  The request includes a reference (e.g. URL or artifact ID) the node uses to fetch the bundle from the orchestrator (or gateway).
  The implementation MUST specify one of these (or both with clear precedence).
  When workspace content or a workspace reference is present, the node MUST unpack or fetch and unpack the content into the job's workspace directory before starting the container; see [Sandbox Workspace and Job Mounts](../../tech_specs/worker_node.md#spec-cynai-worker-sandboxworkspacejobmounts).

### 6. Sandbox Container: Workspace Pre-Population

- **Location:** [sandbox_container.md](../../tech_specs/sandbox_container.md) (Git Behavior or Filesystem).
- **Add one sentence.**
  Workspace content MAY be pre-populated by the system from a Git repo before the container starts.
  That operation is performed outside the sandbox and MUST NOT expose credentials or remote Git access to the container.

## Requirements to Add or Extend

- **REQ-APIEGR** or **REQ-PROJCT** (proposed): The system MUST support populating the sandbox workspace from a Git repo associated with the task's project (project_git_repos) using a credential-holding component that produces a content-only bundle.
  The sandbox MUST NOT receive credentials or perform remote Git operations for workspace population.
- **REQ-WORKER** (extend): When the Run Job request includes workspace content (or a reference to fetch it), the node MUST populate the workspace directory with that content before starting the container and MUST NOT perform Git clone or use Git credentials.
- **REQ-SANDBX** (extend): Workspace MAY be pre-populated from a Git repo by the system outside the sandbox.
  The sandbox MUST NOT be given credentials or remote Git access for that population.

Exact REQ IDs and domain (APIEGR vs PROJCT vs new domain) to be assigned when promoting to canonical requirements.

## References

- [project_git_repos.md](../../tech_specs/project_git_repos.md)
- [git_egress.md](../../tech_specs/mcp_tools/git_egress.md)
- [cynode_sba.md](../../tech_specs/cynode_sba.md)
- [worker_node.md](../../tech_specs/worker_node.md)
- [worker_api.md](../../tech_specs/worker_api.md)
- [sandbox_container.md](../../tech_specs/sandbox_container.md)
- [projects_and_scopes.md](../../tech_specs/projects_and_scopes.md)
- [draft_specs README](../README.md) (historical incorporation report removed)
