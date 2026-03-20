# CyNodeAI Project Meta

## Project Summary

CyNodeAI is a local-first multi-agent orchestrator for self-hosted teams and small enterprises.
It coordinates sandboxed worker execution across local nodes and optional cloud capacity, with centralized task and state management.

## Repository Status

- This repository is currently **early prototype / design phase**.
- The canonical normative "what" lives in `docs/requirements/`.
- Implementation guidance ("how") lives in `docs/tech_specs/` and should trace back to requirements.
- The Go implementation lives in four modules (see root `go.work` and the justfile): `go_shared_libs/`, `orchestrator/`, `worker_node/`, `cynork/`.

## Key Documents

- Primary overview: `README.md`
- Technical specifications index: `docs/tech_specs/_main.md` (grouped by Core, Execution, External, Model, Agents, Bootstrap).
- **AI coding agents:** All AI-assisted coding MUST follow the BDD/TDD process and workflow in [ai_files/ai_coding_instructions.md](ai_files/ai_coding_instructions.md).
  That document is required reading for any agent making code or test changes; it defines Red/Green/Refactor, including adding Python E2E tests in the Red phase for user- or API-facing behavior.
- `justfile` contains useful tools for setup and checks.

## Architecture Summary

- **Central orchestrator**: owns task state, user task-execution preferences, audit logs, and vector storage (PostgreSQL + pgvector).
- **Worker nodes**: register with the orchestrator, receive jobs, run inference and tools in sandboxed containers, and report results.
- **Sandboxed execution**: per-job or per-agent containers with restricted network access and orchestrator-controlled ingress/egress.
- **MCP-first tools**: agents use MCP as the standard tool interface for privileged operations and data access.
  Canonical tool names, per-tool specs, and allowlists: [`docs/tech_specs/mcp_tools/README.md`](docs/tech_specs/mcp_tools/README.md); cross-cutting conventions: [`docs/tech_specs/mcp_tooling.md`](docs/tech_specs/mcp_tooling.md); enforcement: [`docs/tech_specs/mcp_gateway_enforcement.md`](docs/tech_specs/mcp_gateway_enforcement.md).
- **Controlled egress**: outbound web and API access are mediated by dedicated services and policy, not direct sandbox access.
- **REST APIs**: all REST APIs in this system MUST be implemented in Go (see `docs/tech_specs/go_rest_api_standards.md`).
- **Admin clients parity**: the Web Console and the CLI management app (`cynork`) MUST offer the same administrative capabilities.
  When adding or changing a capability in one client, the other MUST be updated to match.
  See `docs/requirements/client.md` (REQ-CLIENT-0004) and the capability-parity sections in `docs/tech_specs/web_console.md` and `docs/tech_specs/cynork_cli.md`.

## Security and Access Notes

- Worker sandboxes should be treated as untrusted and network-restricted by default.
- API credentials must not be exposed to sandboxes.
  External provider calls should be routed through the API Egress Server and audited.
- Orchestrator-side agents must not connect directly to PostgreSQL.
  Use MCP database tools for all database access (see `docs/tech_specs/project_manager_agent.md`).

## Repository Layout

- `docs/`: project documentation; entrypoint is [docs/README.md](docs/README.md).
- `docs/dev_docs/`: temporary files that must be cleaned up/deleted/moved before merging to default branch.
  Do not create links to these files.
- `docs/requirements/`: canonical normative requirements ("what"); entrypoint is `README.md`.
- `docs/tech_specs/`: design and implementation guidance ("how"); entrypoint is `_main.md` (see also [docs/tech_specs/README.md](docs/tech_specs/README.md)).
- `secure_browser/`: rules and assets for the secure browser service (e.g. `secure_browser_rules.yaml`).
- `go_shared_libs/`: shared Go contracts and types used by orchestrator and worker node; see `go_shared_libs/README.md`.
- `orchestrator/`: orchestrator Go module (control-plane, user-gateway, api-egress, etc.); see `orchestrator/README.md`.
- `worker_node/`: worker-node Go module (node manager, worker API); see `worker_node/README.md`.
- `cynork/`: CLI management client (Go, Cobra); see `cynork/README.md` and `docs/tech_specs/cynork_cli.md`.
- `scripts/`: E2E tests and aux scripts; see `scripts/README.md`.
  Dev setup is implemented in `scripts/justfile` (just/shell); use `just setup-dev` or `just setup-dev help`.
  Python: `test_scripts/` (E2E suite).
    Use `just e2e` or `just setup-dev full-demo` from the justfile.
- `tmp/`: scratch space (ignored by Python lint configs; avoid committing generated artifacts unless intentional).

## Style and Tooling Conventions

**CRITICAL:** Do NOT modify linting rules or add linter suppression comments (e.g. `//nolint` in go files)!

- Correct capitalizations:
  - CyNodeAI
  - CyNode
- See [`markdown_conventions.md`](./docs/docs_standards/markdown_conventions.md)
- Use the project **justfile** for setup, checking, and validation.
  All changes must pass **`just ci`** before considering work complete; see the justfile for available recipes.
  - **ALWAYS** lint all changed files with appropriate `just lint-*` command, or lint all with `just lint`.
  - If only docs were updated, **`just docs-check`** is sufficient instead of `just ci`.
- Markdown formatting is governed by `.editorconfig` and `.markdownlint.yml`.
  - Comply with standards outlined in `docs/docs_standards/markdown_conventions.md`.
  - Keep Markdown ASCII-only (avoid emoji and non-ASCII punctuation) unless explicitly allowed by the linter config.
  - Do NOT use markdown tables unless explicitly directed.
  - **NOTE:** Use `just lint-md <path>` to apply automatic markdownlint fixes before fixing other linter issues.
    This will save a lot of manual work.
- Python linting configuration exists in `.flake8` and `.pylintrc` (line length 100).
  When adding Python code, follow these configs and keep excluded directories in mind (for example `tmp/`).

## Contribution Expectations

- When authoring or editing **tech specs**, **requirements**, **features**, or related design docs, you MUST follow the project's spec standards: [docs/docs_standards/spec_authoring_writing_and_validation.md](docs/docs_standards/spec_authoring_writing_and_validation.md).
- Agents and AIs must not guess or simulate output from tasks, database calls, tool calls, or external services.
  Use real tool/system results and report errors or missing data; do not invent or assume values.
- Requirements in `docs/requirements/` take precedence over tech specs and code.
  If requirements, specs, and code differ, treat it as a **gap** and call it out to the user for direction.
- Tech specs in `docs/tech_specs/` take precedence over code.
  If specs and code differ, treat it as a **code gap** and call it out to the user for direction.
- AIs must not update the tech specs without explicit user direction.
- Prefer links to the relevant requirement and tech spec sections over duplicating large design explanations in code comments.
- Do not commit secrets (API keys, tokens, credentials).
