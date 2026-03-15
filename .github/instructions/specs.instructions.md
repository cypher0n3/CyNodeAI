---
applyTo:
  - "docs/tech_specs/**/*.md"
  - "docs/draft_specs/**/*.md"
---

# Tech Spec Authoring

When editing files in `docs/tech_specs/` or `docs/draft_specs/`, follow the project spec authoring standards.

## Canonical Docs

- [docs/docs_standards/spec_authoring_writing_and_validation.md](../docs/docs_standards/spec_authoring_writing_and_validation.md)
- [docs/docs_standards/markdown_conventions.md](../docs/docs_standards/markdown_conventions.md)

## Rules (Summary)

- Be prescriptive, specific, and explicit; no room for interpretation.
- Define contracts (interfaces, types, operations), algorithm logic, return values, status/error codes.
- Keep code minimal: signatures, constants, short snippets only.
- Single source of truth; link to it; do not duplicate.
- Do not add new RFC-2119 obligations in specs; reference requirements in `docs/requirements/` instead.
- Each Spec Item: numbered heading, then first line `- Spec ID: \`CYNAI.DOMAIN.PATH\` <a id="spec-cynai-domain-path"></a>` (anchor: lowercase, dots to dashes, prefix `spec-`).
- Link each Spec Item to requirements in a "Traces To" subsection at the end of the item.
- Allowed inline HTML: only `<a id="spec-..."></a>`, `<a id="ref-<lang>-..."></a>`, `<a id="algo-..."></a>`, `<a id="algo-...-step-..."></a>` in the allowed positions.
- After editing: run `just lint-md` and fix issues.
