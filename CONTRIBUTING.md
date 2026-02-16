# Contributing to CyNodeAI

- [Thank You](#thank-you)
- [Key Documents](#key-documents)
- [Ground Rules](#ground-rules)
- [Development Workflow (BDD/TDD)](#development-workflow-bddtdd)
- [Quality and Verification](#quality-and-verification)
- [PR Expectations](#pr-expectations)

## Thank You

Thank you for contributing!
This repo is in the early prototype / design phase.
Process and spec alignment matter more than volume of code.

## Key Documents

- Technical specifications (source of truth): [`docs/tech_specs/_main.md`](docs/tech_specs/_main.md)
- AI-assisted workflow instructions: [`ai_files/ai_coding_instructions.md`](ai_files/ai_coding_instructions.md)
- Developer commands and local CI: [`justfile`](justfile)

## Ground Rules

- Treat [`docs/tech_specs/`](docs/tech_specs/) as authoritative.
  If a change to a tech spec seems needed, please open a discussion or PR explaining why.
- Please try to preserve existing working code and behavior.
  Prefer minimal, scoped changes that are easy to review.
- Do not commit secrets (API keys, tokens, credentials).
- Do not bypass linters or CI checks; fix issues instead.

## Development Workflow (BDD/TDD)

- Write behavior first in Gherkin:
  - Add or update `.feature` files under [`features/`](features/).
  - Capture user stories and scenarios from the user's perspective.
- Use TDD with red -> green -> refactor:
  - Red: write or adjust unit tests so scenarios fail for the right reason.
  - Green: implement the smallest change that makes tests pass.
  - Refactor: improve structure while keeping tests green.
- Coverage expectation:
  - Target **>= 90% code coverage via unit tests** for new and changed code.
    A tech spec may explicitly state otherwise.

## Quality and Verification

- Run local CI before opening a PR: `just ci`
- Run tests as needed: `just test`, `just test-go`, `just test-go-race`
  - See [`justfile`](./justfile) for full capabilities
- Format Go code with `gofmt` (or `just fmt-go` if available)

## PR Expectations

- Keep PRs small and focused.
- Link the relevant tech spec(s) and any [`features/`](features/) updates.
- Include a brief test plan (commands run and key scenarios exercised).
