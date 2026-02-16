# AI Coding Instructions for CyNodeAI Development

## 1. Project Overview

**Project:** CyNodeAI - Local-First Multi-Agent Orchestrator
**Context:** Self-hosted teams and small enterprises; coordinates sandboxed worker execution across local nodes and optional cloud capacity.
**Status:** Early prototype / design phase.
Authoritative details live in [`docs/tech_specs/`](../docs/tech_specs/).
[`node/`](../node/) and [`orchestrator/`](../orchestrator/) are placeholders for implementation.
**References:** [`meta.md`](../meta.md), [`docs/tech_specs/_main.md`](../docs/tech_specs/_main.md).

## 2. Documentation and Style Standards

- **Tech specs:** [`docs/tech_specs/`](../docs/tech_specs/) is the primary source of truth; entrypoint is `_main.md`.
- **Markdown:** Formatting governed by [`.editorconfig`](../.editorconfig) and [`.markdownlint.yml`](../.markdownlint.yml).
  Keep Markdown ASCII-only unless explicitly allowed by the linter config.
- **Python:** Linting in [`.flake8`](../.flake8) and [`.pylintrc`](../.pylintrc) (line length 100).
  Excluded directories (e.g. `tmp/`) remain excluded when adding code.

## 3. Core Principles

### 3.1 Specification Adherence

- **Primary rule:** Follow specifications in [`docs/tech_specs/`](../docs/tech_specs/) exactly as written.
- **No modifications:** Do not update tech specs without explicit user direction.
- **Gaps:** If specs and code differ, treat it as a **code gap** and call it out to the user for direction.
- **Comments:** Prefer links to the relevant tech spec section over duplicating large design explanations in code comments.

### 3.2 Code Preservation

- **Golden rule:** Preserve existing working code.
- **Minimal changes:** Make only necessary modifications.
- **No refactoring:** Avoid refactoring unless directly required.

### 3.3 Repository Conventions

- Do not disable or bypass linters; fix issues instead.
- Do not commit secrets (API keys, tokens, credentials).
- [`tmp/`](../tmp/) is scratch space; avoid committing generated artifacts unless intentional.

## 4. Architecture and Security Constraints

- **REST APIs:** All REST APIs in this system MUST be implemented in Go ([`docs/tech_specs/go_rest_api_standards.md`](../docs/tech_specs/go_rest_api_standards.md)).
- **Worker sandboxes:** Treat as untrusted and network-restricted by default.
  API credentials must not be exposed to sandboxes; external provider calls go through the API Egress Server and are audited.
- **Orchestrator agents:** Must not connect directly to PostgreSQL.
  Use MCP database tools for all database access ([`docs/tech_specs/project_manager_agent.md`](../docs/tech_specs/project_manager_agent.md)).
- **MCP:** Agents use MCP as the standard tool interface for privileged operations and data access.

## 5. Development Workflow

### 5.1 BDD/TDD (Red -> Green -> Refactor)

- **Write the user story first (BDD):** Create or update a Gherkin `.feature` file under `features/` that describes the behavior from a user's perspective.
  - **Directory rule:** Feature files live in `features/` (create the directory if it does not exist).
- **Red:** Add/adjust tests so the new scenario(s) fail for the right reason.
- **Green:** Implement the smallest change that makes tests pass.
- **Refactor:** Improve structure/readability while keeping tests green.
- **Keep specs authoritative:** If the feature implies behavior that contradicts `docs/tech_specs/`, treat it as a spec issue and halt for user direction.

### 5.2 Analysis and Planning

- Map existing code (when present) to specs in `docs/tech_specs/`.
- Identify gaps and required changes; plan minimal necessary modifications.
- If no branch is specified, use or create a branch for commits.

### 5.3 Implementation Steps

- Implement only what the specs require; preserve existing behavior elsewhere.
- For Go: follow [`docs/tech_specs/go_rest_api_standards.md`](../docs/tech_specs/go_rest_api_standards.md).
- Use the project **[justfile](../justfile)** for setup, checking, and validation; run the relevant recipes after changes (see the [justfile](../justfile) for available commands).

### 5.4 Quality Assurance

- **CI:** All changes must pass **`just ci`** before considering work complete.
- **Formatting:** Apply gofmt to Go files; respect EditorConfig and markdownlint for other files.
- **Linting:** Fix all reported issues; do not bypass linters.
- **Coverage:** Expect **>= 90% code coverage via unit tests** for new and changed code (unless a tech spec explicitly states otherwise).
- **Links:** If adding or changing Markdown links, validate them with project tooling when available.

### 5.5 Version Control

- Use conventional commit format with clear descriptions.
- Do not commit secrets or generated artifacts in [`tmp/`](../tmp/) unless intentional.

## 6. File and Directory Layout

- **Specs:** [`docs/tech_specs/`](../docs/tech_specs/) - design and normative requirements.
- **BDD:** [`features/`](../features/) - Gherkin `.feature` files containing user stories and scenarios.
- **Placeholders:** [`node/`](../node/) - worker-node services; [`orchestrator/`](../orchestrator/) - orchestrator services.
- **Scratch:** [`tmp/`](../tmp/) - scratch space (excluded from Python lint configs where applicable).
- **Secure browser:** [`secure_browser/`](../secure_browser/) - rules and assets for the secure browser service.

## 7. Error Handling

### 7.1 Specification Issues

- **Halt** on specification problems or ambiguity.
- **Report to the user** - do not change specs without explicit direction.
- **No assumptions** or workarounds that contradict the specs.

### 7.2 Code Quality Issues

- Fix before committing; address all linter and test failures.
- Do not leave commented-out code or temporary hacks without user approval.

## 8. Success Criteria

- Implementation matches the relevant tech spec(s).
- **`just ci`** passes (use the justfile for setup, checks, and validation as needed).
- Existing tests pass; new code is covered where appropriate.
- **>= 90% code coverage** is achieved via **unit tests** (unless a tech spec explicitly states otherwise).
- Code is formatted and passes lint checks.
- No secrets committed; no spec changes unless explicitly requested.
- Changes are committed with clear, conventional commit messages.

This document provides an AI-optimized guide for CyNodeAI development.
Specs are the source of truth; changes are minimal and spec-driven.
