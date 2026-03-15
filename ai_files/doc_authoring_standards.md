# Doc Authoring Standards (Specs, Requirements, Feature Files)

- [1 Canonical Sources](#1-canonical-sources)
- [2 When to Use](#2-when-to-use)
- [3 Agent-Specific Hooks](#3-agent-specific-hooks)

## 1 Canonical Sources

When creating or editing **technical specs**, **requirements**, or **Gherkin feature files**, follow the project documentation standards so all AI coding agents (Cursor, GitHub Copilot, Claude Code, etc.) apply the same rules.

- **Spec authoring, traceability, validation:** [docs/docs_standards/spec_authoring_writing_and_validation.md](../docs/docs_standards/spec_authoring_writing_and_validation.md)
- **Markdown conventions:** [docs/docs_standards/markdown_conventions.md](../docs/docs_standards/markdown_conventions.md)
- **Requirements domains (Spec ID and REQ-ID domains):** [docs/docs_standards/requirements_domains.md](../docs/docs_standards/requirements_domains.md)
- **Feature files (suites, tags):** [features/README.md](../features/README.md)

## 2 When to Use

- **Tech specs** (`docs/tech_specs/`, `docs/draft_specs/`): Prescriptive, explicit Spec Items; Spec ID anchors; link to requirements (Traces To); run `just lint-md` after edits.
- **Requirements** (`docs/requirements/`): Atomic `REQ-<DOMAIN>-<NNNN>` entries; spec reference links and req-* anchors; run `just lint-md` after edits.
- **Feature files** (`features/`): One suite tag above Feature; user story block; each Scenario has @req_*and @spec_* tags; run `just lint-md` if applicable.

## 3 Agent-Specific Hooks

- **Cursor:** Project skills in [.cursor/skills/](../.cursor/skills/) (spec-authoring, requirements-authoring, feature-files-authoring) summarize these standards and when to apply them.
- **GitHub Copilot:** Path-scoped instructions in [.github/instructions/](../.github/instructions/) apply when editing specs, requirements, or feature files.
