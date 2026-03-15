---
applyTo: "docs/requirements/**/*.md"
---

# Requirements Authoring

When editing files in `docs/requirements/`, follow the project requirements and markdown standards.

## Canonical Docs

- [docs/docs_standards/spec_authoring_writing_and_validation.md](../docs/docs_standards/spec_authoring_writing_and_validation.md)
- [docs/docs_standards/requirements_domains.md](../docs/docs_standards/requirements_domains.md)

## Rules (Summary)

- Requirements define **what** is required; implementation-agnostic. Use RFC-2119 (MUST, SHOULD) here.
- One obligation per REQ entry; atomic and testable.
- ID format: `REQ-<DOMAIN>-<NNNN>` (4 digits). Domains from requirements_domains.md; file name = domain lowercased.
- Entry format: list item `- REQ-<DOMAIN>-<NNNN>: <short label>.` then continuation lines with spec reference link(s) (Spec ID as link text, href to `spec-*` anchor), then `<a id="req-<domain>-<nnnn>"></a>` on its own line.
- Allowed inline HTML: only `<a id="req-..."></a>` on a continuation line under the requirement item.
- After editing: run `just lint-md` and fix issues.
