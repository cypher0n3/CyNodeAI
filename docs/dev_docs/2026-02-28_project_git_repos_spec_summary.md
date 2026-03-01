# Project Git Repos Spec and Requirements Summary

## Purpose

**Date:** 2026-02-28

Associate multiple Git repositories with projects so they can be used as part of projects/tasks and enforced by Git egress.
Specs and requirements were added/updated per [`docs/docs_standards/spec_authoring_writing_and_validation.md`](../docs_standards/spec_authoring_writing_and_validation.md), with support for GitHub, GitLab, and Gitea (cloud and self-hosted) as of Feb 2026.

## Changes Made

Summary of requirements and tech spec updates below.

### Requirement Areas

- **PROJCT** ([`docs/requirements/projct.md`](../requirements/projct.md)):
  - REQ-PROJCT-0106: Multiple Git repo associations per project for task/egress allowlists.
  - REQ-PROJCT-0107: Store provider (github, gitlab, gitea) and provider-specific repo identifier.
  - REQ-PROJCT-0108: Optional base_url override for self-hosted instances.
  - REQ-PROJCT-0109: Manage project repos via MCP and admin clients; scope to authorized projects.
- **SCHEMA** ([`docs/requirements/schema.md`](../requirements/schema.md)):
  - REQ-SCHEMA-0113: Project git repositories table with project_id, provider, repo_identifier, optional base_url.
- **APIEGR** ([`docs/requirements/apiegr.md`](../requirements/apiegr.md)):
  - REQ-APIEGR-0127: When task has project_id, Git egress MUST validate requested provider/repo against project-scoped allowlist.

### Tech Specs

- **New:** [`docs/tech_specs/project_git_repos.md`](../tech_specs/project_git_repos.md):
  - CYNAI.
    PROJCT.
    ProjectGitRepos: Model (many repos per project, uniqueness per project).
  - CYNAI.
    PROJCT.
    RepoIdentifierFormat: Provider-specific formats (GitHub owner/repo, GitLab full path, Gitea owner/repo; base_url for self-hosted).
  - CYNAI.
    APIEGR.
    GitEgressProjectScope: Egress must allow only repos associated with task's project.
  - CYNAI.
    PROJCT.
    ProjectGitReposMcp: MCP and admin surfaces, project-scoped access.
- **Updated:** [`docs/tech_specs/postgres_schema.md`](../tech_specs/postgres_schema.md): New table `project_git_repos` (id, project_id, provider, repo_identifier, base_url, display_name, timestamps); unique (project_id, provider, repo_identifier); indexes.
- **Updated:** [`docs/tech_specs/git_egress_mcp.md`](../tech_specs/git_egress_mcp.md): Access control section references project-scoped repo allowlist and REQ-APIEGR-0127.
- **Updated:** [`docs/tech_specs/projects_and_scopes.md`](../tech_specs/projects_and_scopes.md): Related doc link to project_git_repos; task scope bullet for Git egress repo allowlist.

### Git Provider Conventions (Feb 2026)

- **GitHub:** `repo_identifier` = `owner/repo`; default base `https://github.com`; Enterprise via `base_url`.
- **GitLab:** `repo_identifier` = full path (e.g. `group/subgroup/project`); default `https://gitlab.com`; self-hosted via `base_url`.
- **Gitea:** `repo_identifier` = `owner/repo`; `base_url` required (no single default).

## Validation

- `just lint-md` run on all modified docs; no errors.
