# CyNodeAI Project Meta

## Project Summary

CyNodeAI is a local-first multi-agent orchestrator for self-hosted teams and small enterprises.
It coordinates sandboxed worker execution across local nodes and optional cloud capacity, with centralized task and state management.

## Repository Status

- This repository is currently **early prototype / design phase**.
- The canonical normative "what" lives in `docs/requirements/`.
- Implementation guidance ("how") lives in `docs/tech_specs/` and should trace back to requirements.
- The `worker_node/` and `orchestrator/` directories contain the Go implementation (see `go.work` and the justfile).

## Key Documents

- Primary overview: `README.md`
- Technical specifications index: `docs/tech_specs/_main.md` (grouped by Core, Execution, External, Model, Agents, Bootstrap).

## Architecture Summary

- **Central orchestrator**: owns task state, preferences, audit logs, and vector storage (PostgreSQL + pgvector).
- **Worker nodes**: register with the orchestrator, receive jobs, run inference and tools in sandboxed containers, and report results.
- **Sandboxed execution**: per-job or per-agent containers with restricted network access and orchestrator-controlled ingress/egress.
- **MCP-first tools**: agents use MCP as the standard tool interface for privileged operations and data access.
- **Controlled egress**: outbound web and API access are mediated by dedicated services and policy, not direct sandbox access.
- **REST APIs**: all REST APIs in this system MUST be implemented in Go (see `docs/tech_specs/go_rest_api_standards.md`).

## Security and Access Notes

- Worker sandboxes should be treated as untrusted and network-restricted by default.
- API credentials must not be exposed to sandboxes.
  External provider calls should be routed through the API Egress Server and audited.
- Orchestrator-side agents must not connect directly to PostgreSQL.
  Use MCP database tools for all database access (see `docs/tech_specs/project_manager_agent.md`).

## Repository Layout

- `docs/requirements/`: canonical normative requirements ("what"); entrypoint is `README.md`.
- `docs/tech_specs/`: design and implementation guidance ("how"); entrypoint is `_main.md`.
- `secure_browser/`: rules and assets for the secure browser service (e.g. `secure_browser_rules.yaml`).
- `worker_node/`: worker-node Go module (node manager, worker API); see `worker_node/README.md`.
- `orchestrator/`: orchestrator Go module (control-plane, user-gateway, api-egress, etc.); see `orchestrator/README.md`.
- `tmp/`: scratch space (ignored by Python lint configs; avoid committing generated artifacts unless intentional).

## Style and Tooling Conventions

**CRITICAL:** Do NOT modify linting rules or add linter suppression comments (e.g. `//nolint` in go files)!

- See [`markdown_conventions.md`](./docs/docs_standards/markdown_conventions.md)
- Use the project **justfile** for setup, checking, and validation.
  All changes must pass **`just ci`** before considering work complete; see the justfile for available recipes.
- Markdown formatting is governed by `.editorconfig` and `.markdownlint.yml`.
  Keep Markdown ASCII-only (avoid emoji and non-ASCII punctuation) unless explicitly allowed by the linter config.
  - **NOTE:** Use `just lint-md <path>` to apply automatic markdownlint fixes before fixing other linter issues.
    This will save a lot of manual work.
- Python linting configuration exists in `.flake8` and `.pylintrc` (line length 100).
  When adding Python code, follow these configs and keep excluded directories in mind (for example `tmp/`).

## Contribution Expectations

- Requirements in `docs/requirements/` take precedence over tech specs and code.
  If requirements, specs, and code differ, treat it as a **gap** and call it out to the user for direction.
- Tech specs in `docs/tech_specs/` take precedence over code.
  If specs and code differ, treat it as a **code gap** and call it out to the user for direction.
- AIs must not update the tech specs without explicit user direction.
- Prefer links to the relevant requirement and tech spec sections over duplicating large design explanations in code comments.
- Do not commit secrets (API keys, tokens, credentials).
