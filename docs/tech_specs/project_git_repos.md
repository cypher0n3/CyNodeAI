# Project Git Repositories

- [Document Overview](#document-overview)
- [Goals and Non-Goals](#goals-and-non-goals)
- [Project Git Repos Model](#project-git-repos-model)
  - [`ProjectGitRepos` Type](#projectgitrepos-type)
- [Repo Identifier Format (GitHub, GitLab, Gitea)](#repo-identifier-format-github-gitlab-gitea)
  - [`RepoIdentifierFormat` Rule](#repoidentifierformat-rule)
- [Project Git Repos and Git Egress](#project-git-repos-and-git-egress)
  - [Git Egress Project-Scoped Allowlist](#git-egress-project-scoped-allowlist)
- [MCP and Admin Surfaces](#mcp-and-admin-surfaces)
  - [`ProjectGitReposMcp` Operation](#projectgitreposmcp-operation)
- [MVP Notes](#mvp-notes)

## Document Overview

This document defines how Git repositories are associated with projects so that tasks and Git egress operations can use project-scoped allowlists.
It is the single source of truth for the project-repo association model, provider and repo identifier formats, and integration with Git egress.

Related documents

- Projects and scope: [`docs/tech_specs/projects_and_scopes.md`](projects_and_scopes.md)
- Postgres schema: [`docs/tech_specs/postgres_schema.md`](postgres_schema.md) (Project Git Repositories table)
- Git egress: [`docs/tech_specs/mcp_tools/git_egress.md`](mcp_tools/git_egress.md)

## Goals and Non-Goals

Goals

- Allow multiple Git repository associations per project.
- Use a provider-agnostic storage form that works for GitHub, GitLab, and Gitea (cloud and self-hosted).
- Support project-scoped allowlist validation in Git egress when a task has a project_id.
- Expose project repo associations via MCP and admin clients, scoped to authorized projects.

Non-goals

- This document does not define Git credential storage or clone/push implementation; see Git egress and API Egress specs.
- Full CI/CD or webhook configuration is out of scope.

## Project Git Repos Model

Projects may have multiple Git repository associations stored in the database.

### `ProjectGitRepos` Type

- Spec ID: `CYNAI.PROJCT.ProjectGitRepos` <a id="spec-cynai-projct-projectgitrepos"></a>

A project MAY have zero or more associated Git repositories.
Each association is stored as a row in `project_git_repos` with `project_id`, `provider`, `repo_identifier`, optional `base_url`, and optional fields for display and organization (e.g. `display_name`, `description`, `tags`, `metadata`).
The same logical repo (same provider and repo_identifier) MAY be associated with multiple projects.
Uniqueness is enforced per project: (`project_id`, `provider`, `repo_identifier`) MUST be unique.

#### `ProjectGitRepos` Storage

- Canonical table: `project_git_repos` per [`docs/tech_specs/projects_and_scopes.md`](projects_and_scopes.md#spec-cynai-schema-projectgitrepostable).
- Foreign key: `project_id` references `projects.id`; referential integrity MUST be enforced (e.g. on delete cascade or restrict per deployment policy).
- Additional information: `display_name` (user-facing label), `description` (role or purpose in this project), `tags` (jsonb array of strings for filtering/grouping), `metadata` (jsonb for future extension).

#### Use in Tasks

- When a task has a non-null `project_id`, Git egress operations for that task MUST restrict allowed repos to those associated with that project (see [Project Git Repos and Git Egress](#project-git-repos-and-git-egress)).

#### `ProjectGitRepos` Type Requirements Traces

- [REQ-PROJCT-0106](../requirements/projct.md#req-projct-0106)

## Repo Identifier Format (GitHub, GitLab, Gitea)

Provider and repo identifier formats below align with common Git hosting as of 2026.

### `RepoIdentifierFormat` Rule

- Spec ID: `CYNAI.PROJCT.RepoIdentifierFormat` <a id="spec-cynai-projct-repoidentifierformat"></a>

#### Repo Identifier Format Requirements Traces

- [REQ-PROJCT-0107](../requirements/projct.md#req-projct-0107)
- [REQ-PROJCT-0108](../requirements/projct.md#req-projct-0108)

#### `RepoIdentifierFormat` Scope

Applicable to all project git repo associations and to Git egress request validation when project-scoped allowlist is used.

#### `RepoIdentifierFormat` Preconditions

- `provider` is any provider identifier for which the system has support (e.g. github, gitlab, gitea; additional providers MAY be added later without schema change).
- `repo_identifier` is a non-empty string that uniquely identifies the repository on the given provider (and base_url when present).

#### `RepoIdentifierFormat` Outcomes

Semantics are defined per supported provider; the following are the canonical formats for commonly supported providers.
Additional providers follow the same pattern (provider key, repo_identifier format, optional base_url).

- GitHub (cloud and GitHub Enterprise): `repo_identifier` MUST be `owner/repo` (e.g. `cynodeai/cynodeai`).
  Default base URL is `https://github.com`; for GitHub Enterprise set `base_url` (e.g. `https://github.company.com`).
  Clone URLs: HTTPS `{base_url}/owner/repo.git`, SSH `git@{host}:owner/repo.git` where host is derived from base_url.
- GitLab (cloud and self-hosted): `repo_identifier` MUST be the full project path, which MAY include subgroups (e.g. `group/subgroup/project` or `namespace/project`).
  Default base URL is `https://gitlab.com`; for self-hosted set `base_url` (e.g. `https://gitlab.company.com`).
  Clone URLs: HTTPS `{base_url}/namespace/project.git`, SSH `git@{host}:namespace/project.git`.
- Gitea (self-hosted or cloud): `repo_identifier` MUST be `owner/repo` (e.g. `org/my-repo`).
  There is no single default base URL; `base_url` MUST be set (e.g. `https://gitea.example.com`).
  Clone URLs: HTTPS `{base_url}/owner/repo.git`, SSH `git@{host}:owner/repo.git`.

Normalization

- Store `repo_identifier` in a canonical form: no leading or trailing slashes; for GitHub and Gitea exactly one slash between owner and repo; for GitLab one or more path segments separated by slashes.
- Store `base_url` without trailing slash; scheme MUST be https for HTTPS clone (SSH host derived from hostname when needed).

## Project Git Repos and Git Egress

When a task has a project_id, Git egress uses the project's associated repos as the allowlist.

### Git Egress Project-Scoped Allowlist

- Spec ID: `CYNAI.APIEGR.GitEgressProjectScope` <a id="spec-cynai-apiegr-gitegressprojectscope"></a>

When a Git egress request includes a task that has a non-null `project_id`, the service MUST resolve the task's project and MUST allow only operations targeting repos that appear in that project's `project_git_repos` set (matching `provider` and `repo_identifier`; when the request or stored row has a `base_url`, both MUST match).
If the requested provider/repo (and base_url when applicable) is not in the project's associated repos, the request MUST be denied with a structured error indicating project-scoped allowlist violation.
When the task has no project_id, existing global or user/group allowlist policy applies; this spec does not change that behavior.

See [`docs/tech_specs/mcp_tools/git_egress.md`](mcp_tools/git_egress.md) for the Git egress tool interface and access control.

#### Git Egress Project-Scoped Allowlist Requirements Traces

- [REQ-APIEGR-0127](../requirements/apiegr.md#req-apiegr-0127)

## MCP and Admin Surfaces

Project git repo associations are managed via MCP tools and admin clients with project-scoped authorization.

### `ProjectGitReposMcp` Operation

- Spec ID: `CYNAI.PROJCT.ProjectGitReposMcp` <a id="spec-cynai-projct-projectgitreposmcp"></a>

Project git repo associations MUST be manageable via MCP tools and admin clients (Web Console and CLI) with the same authorization model as project list/get: the subject MAY only list, add, update, or remove repo associations for projects they are authorized to access (default project plus projects for which they or their groups have a role binding).
Tool names and argument schemas (e.g. list repos for a project, add repo, remove repo) are defined in [MCP tool specifications](mcp_tools/README.md) (per-tool docs under `mcp_tools/`); implementations MUST enforce project-scoped access and MUST NOT return or modify repos for projects outside the subject's authorized set.

#### ProjectGitRepos MCP Requirements Traces

- [REQ-PROJCT-0109](../requirements/projct.md#req-projct-0109)

## MVP Notes

- At least one MCP tool (or equivalent) to list project git repos and to add/remove associations is required for MVP.
- Admin clients (Web Console, CLI) SHOULD expose project repo management in the same place as project settings.
- Git egress MUST enforce project-scoped allowlist when task has project_id; enforcement is required for MVP when this feature is implemented.
