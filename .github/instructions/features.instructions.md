---
applyTo: "features/**/*.feature"
---

# Feature Files Authoring (Gherkin)

When editing `.feature` files in `features/`, follow the project Gherkin and traceability standards.

## Canonical Docs

- [docs/docs_standards/spec_authoring_writing_and_validation.md](../docs/docs_standards/spec_authoring_writing_and_validation.md)
- [features/README.md](../features/README.md)

## Rules (Summary)

- One suite tag on the line immediately above `Feature:` (e.g. `@suite_cynork`). File must live under `features/<suite>/` (e.g. `features/cynork/`). Allowed suites: see features/README.md Section 3.1.
- User story block directly under `Feature:`: `As a ...`, `I want ...`, `So that ...`.
- Each Scenario must include both a requirement tag and a spec tag.
- Requirement tag: `@req_<domain>_<nnnn>` (from REQ-<DOMAIN>-<NNNN>: remove REQ-, replace - with _, lowercase).
- Spec tag: `@spec_<spec_id>` (from Spec ID: dots to underscores, lowercase).
- After editing: run `just lint-md` if applicable and fix issues.
